// Command webhook-receiver-go is a minimal NoWork Account webhook endpoint:
// it verifies X-NoWork-Signature with the Go SDK, opens encrypted deliveries
// (JWE, SDK/docs/DELIVERY.md section 6), de-duplicates deliveries by event id
// and reacts to a few events. Configuration comes from the environment (see
// README.md).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	nowork "github.com/wangework/nowork-sdk-go"
)

// maxBody is the largest body accepted, the same cap as the SDK's ReadWebhook.
const maxBody = 1 << 20

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

// userLevelChanged is user.level_changed: the trust level moved from -> to.
type userLevelChanged struct {
	UserID   int64        `json:"userId"`
	UID      int64        `json:"uid"`
	Username string       `json:"username"`
	From     int          `json:"from"`
	To       int          `json:"to"`
	User     pairwiseUser `json:"user"`
}

// connectionChanged is connection.bound / connection.unbound: a third-party
// account (provider "discord", "github", ...) was linked or unlinked.
type connectionChanged struct {
	UserID       int64        `json:"userId"`
	UID          int64        `json:"uid"`
	Provider     string       `json:"provider"`
	ExternalID   string       `json:"externalId"`
	ExternalName string       `json:"externalName"` // bound only
	User         pairwiseUser `json:"user"`
}

// pairwiseUser is data.user. An endpoint bound to a delivery profile with
// subject "pairwise" gets no userId / uid / username / ip / externalId /
// externalName / email / phone; data.user.sub identifies the account instead
// (DELIVERY.md section 2). Other profile rules may add fields to data.user.
type pairwiseUser struct {
	Sub string `json:"sub"`
}

// who names the account in logs, whichever identifier the endpoint receives.
func who(uid int64, u pairwiseUser) string {
	if u.Sub != "" {
		return "sub=" + u.Sub
	}
	return "uid=" + strconv.FormatInt(uid, 10)
}

type receiver struct {
	secret    string
	tolerance time.Duration
	seen      *seenSet
	keys      []*nowork.DecryptionKey // empty: the endpoint is not encrypted
}

func main() {
	secret := os.Getenv("NOWORK_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set NOWORK_WEBHOOK_SECRET (admin console: 开放平台 -> Webhook -> 签名密钥)")
	}
	addr := envOr("LISTEN_ADDR", ":8090")
	path := envOr("WEBHOOK_PATH", "/nowork/webhook")
	tolerance := time.Duration(envInt("WEBHOOK_TOLERANCE", 300)) * time.Second

	keys, err := loadKeys(os.Getenv("NOWORK_DECRYPTION_KEYS"))
	if err != nil {
		log.Fatalf("NOWORK_DECRYPTION_KEYS: %v", err)
	}
	rc := &receiver{secret: secret, tolerance: tolerance, seen: newSeenSet(envInt("SEEN_CAPACITY", 10000)), keys: keys}

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
		log.Printf("listening on %s%s (timestamp tolerance %s, %d decryption key(s))", addr, path, tolerance, len(keys))
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
	delivery := r.Header.Get("X-NoWork-Delivery")

	// Keep the raw bytes: the signature covers "<timestamp>.<body>" exactly as sent,
	// and an encrypted body is opened only after it verified.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		log.Printf("delivery=%s rejected: %v", delivery, err)
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "payload too large"})
		return
	}
	ev, err := nowork.VerifyWebhook(rc.secret, r.Header, body, rc.tolerance)
	if err == nil {
		ev, err = rc.open(r.Header, body, ev)
		if errors.Is(err, errNotEncrypted) || errors.Is(err, errNoKeys) || errors.Is(err, nowork.ErrDecrypt) {
			// Signed by the user center but not readable here: a key rotation or a
			// configuration mismatch. 500 keeps the delivery queued for a retry.
			log.Printf("delivery=%s cannot open: %v", delivery, err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "cannot decrypt"})
			return
		}
	}
	switch {
	case errors.Is(err, nowork.ErrWebhookStale):
		log.Printf("delivery=%s rejected: stale timestamp", delivery)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "stale timestamp"})
		return
	case errors.Is(err, nowork.ErrWebhookSignature):
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
func (rc *receiver) dispatch(ev *nowork.WebhookEvent) error {
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
	case "user.level_changed":
		var d userLevelChanged
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("decode %s: %w", ev.Event, err)
		}
		log.Printf("[%s] trust level %d -> %d: %s", at, d.From, d.To, who(d.UID, d.User))
		// e.g. re-check a rank that requires a minimum trust level
	case "connection.bound", "connection.unbound":
		var d connectionChanged
		if err := json.Unmarshal(ev.Data, &d); err != nil {
			return fmt.Errorf("decode %s: %w", ev.Event, err)
		}
		log.Printf("[%s] %s provider=%s external=%q: %s", at, ev.Event, d.Provider, d.ExternalID, who(d.UID, d.User))
		// e.g. grant or revoke a Discord role (see ../discord-role-sync)
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
