// Command webhook-receiver-go is a minimal Lounge Account webhook endpoint:
// it verifies X-Lounge-Signature with the Go SDK, de-duplicates deliveries by
// event id and reacts to a few events. Configuration comes from the environment
// (see README.md).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	lounge "github.com/wangework/lounge-sdk-go"
)

// Review results carried by game_role.reviewed -> data.status.
const (
	roleStatusPending  = 0
	roleStatusApproved = 1
	roleStatusRejected = 2
	roleStatusBanned   = 3
)

// Payloads of the events this receiver understands. Field names are exactly what
// the user center emits; anything else stays available in ev.Data.
type gameRoleReviewed struct {
	RoleID     int64  `json:"roleId"`
	UserID     int64  `json:"userId"`
	ServerID   int64  `json:"serverId"`
	RoleName   string `json:"roleName"`
	RoleUUID   string `json:"roleUuid"`
	Status     int    `json:"status"`
	ReviewerID int64  `json:"reviewerId"`
}

type userDeleted struct {
	UserID   int64  `json:"userId"`
	UID      int64  `json:"uid"`
	Username string `json:"username"`
	Reason   string `json:"reason"`
}

type serverOffline struct {
	ServerID      int64  `json:"serverId"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	LastHeartbeat string `json:"lastHeartbeat"` // "2006-01-02 15:04:05" or null
}

type receiver struct {
	secret    string
	tolerance time.Duration
	seen      *seenSet
}

func main() {
	secret := os.Getenv("LOUNGE_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set LOUNGE_WEBHOOK_SECRET (admin console: 开发者 -> Webhook 事件推送 -> 签名密钥)")
	}
	addr := envOr("LISTEN_ADDR", ":8090")
	path := envOr("WEBHOOK_PATH", "/lounge/webhook")
	tolerance := time.Duration(envInt("WEBHOOK_TOLERANCE", 300)) * time.Second

	rc := &receiver{secret: secret, tolerance: tolerance, seen: newSeenSet(envInt("SEEN_CAPACITY", 10000))}

	mux := http.NewServeMux()
	mux.HandleFunc(path, rc.handle)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s%s (timestamp tolerance %s)", addr, path, tolerance)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

// handle is the webhook endpoint. Status codes matter: anything outside 2xx (or a
// timeout) makes the user center retry with back-off, so bad signatures get 401
// and handler failures get 500.
func (rc *receiver) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
		return
	}
	delivery := r.Header.Get("X-Lounge-Delivery")

	ev, err := lounge.ReadWebhook(rc.secret, r, rc.tolerance)
	switch {
	case errors.Is(err, lounge.ErrWebhookStale):
		log.Printf("delivery=%s rejected: stale timestamp", delivery)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "stale timestamp"})
		return
	case errors.Is(err, lounge.ErrWebhookSignature):
		log.Printf("delivery=%s rejected: bad signature", delivery)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid signature"})
		return
	case err != nil:
		// body unreadable or not JSON: a retry would not help, answer 400
		log.Printf("delivery=%s rejected: %v", delivery, err)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid payload"})
		return
	}
	if ev.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing event id"})
		return
	}

	// Idempotency: the same event id may arrive more than once (retry after a
	// timeout, manual redelivery from the console). Acknowledge without re-running.
	if rc.seen.Seen(ev.ID) {
		log.Printf("delivery=%s event=%s id=%s duplicate, ignored", delivery, ev.Event, ev.ID)
		writeJSON(w, http.StatusOK, map[string]any{"status": "duplicate"})
		return
	}

	if err := rc.dispatch(ev); err != nil {
		log.Printf("delivery=%s event=%s id=%s handler failed: %v", delivery, ev.Event, ev.ID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "handler failed"})
		return
	}
	rc.seen.Add(ev.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// dispatch routes a verified event. Keep the work short (the sender waits at most
// 10 s); push anything heavier onto a queue and return.
func (rc *receiver) dispatch(ev *lounge.WebhookEvent) error {
	at := time.Unix(ev.Timestamp, 0).Format(time.RFC3339)
	switch ev.Event {
	case "game_role.reviewed":
		var d gameRoleReviewed
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("decode %s: %w", ev.Event, err)
		}
		switch d.Status {
		case roleStatusApproved:
			log.Printf("[%s] role APPROVED: role=%q uuid=%s user=%d server=%d reviewer=%d", at, d.RoleName, d.RoleUUID, d.UserID, d.ServerID, d.ReviewerID)
			// e.g. add the player to the server whitelist / grant a rank
		case roleStatusRejected:
			log.Printf("[%s] role REJECTED: role=%q user=%d server=%d reviewer=%d", at, d.RoleName, d.UserID, d.ServerID, d.ReviewerID)
		case roleStatusBanned:
			log.Printf("[%s] role BANNED: role=%q user=%d server=%d reviewer=%d", at, d.RoleName, d.UserID, d.ServerID, d.ReviewerID)
			// e.g. kick the player if online
		case roleStatusPending:
			log.Printf("[%s] role back to PENDING: role=%q user=%d server=%d", at, d.RoleName, d.UserID, d.ServerID)
		default:
			log.Printf("[%s] role status %d (unknown): role=%q user=%d", at, d.Status, d.RoleName, d.UserID)
		}
	case "user.deleted":
		var d userDeleted
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("decode %s: %w", ev.Event, err)
		}
		log.Printf("[%s] user DELETED: userId=%d uid=%d username=%q reason=%q", at, d.UserID, d.UID, d.Username, d.Reason)
		// e.g. purge local data tied to this account
	case "server.offline":
		var d serverOffline
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("decode %s: %w", ev.Event, err)
		}
		log.Printf("[%s] server OFFLINE: code=%s name=%q id=%d lastHeartbeat=%q", at, d.Code, d.Name, d.ServerID, d.LastHeartbeat)
		// e.g. page the on-call channel
	case "ping":
		// sent by the "测试" button in the admin console
		log.Printf("[%s] ping from console: %s", at, string(ev.Data))
	default:
		log.Printf("[%s] event %s ignored (id=%s)", at, ev.Event, ev.ID)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return d
}
