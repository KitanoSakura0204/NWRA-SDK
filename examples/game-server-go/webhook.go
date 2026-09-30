package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	lounge "github.com/wangework/lounge-sdk-go"
)

// hooks receives the user center's webhooks and applies the ones that matter to a running game
// server: a frozen, locked or deleted account is kicked at once instead of at its next join, and
// a game role review re-runs the whitelist sync. Signature first, then the JWE (DELIVERY.md
// section 6), then de-duplication by event id. See ../webhook-receiver-go for every event.
type hooks struct {
	secret    string
	tolerance time.Duration
	keys      []*lounge.DecryptionKey
	world     *world
	resync    func()
	logf      func(string, ...any)

	mu   sync.Mutex
	seen map[string]time.Time // event id -> first seen; pruned after a day
}

// accountEvent is the part of user.frozen / user.locked / user.deleted this server needs.
// A pairwise endpoint carries no uid (data.user.sub only): bind this endpoint to a public profile.
type accountEvent struct {
	UID    int64 `json:"uid"`
	Frozen *bool `json:"frozen"` // user.frozen: false means unfrozen
}

func (h *hooks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	ev, err := lounge.VerifyWebhook(h.secret, r.Header, body, h.tolerance)
	if errors.Is(err, lounge.ErrWebhookSignature) || errors.Is(err, lounge.ErrWebhookStale) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	encrypted := strings.EqualFold(r.Header.Get(lounge.HeaderEncryption), "JWE") || hasJWE(body)
	switch {
	case encrypted && len(h.keys) == 0, !encrypted && len(h.keys) > 0:
		// signed but not readable here (key rotation or a configuration mismatch): 500 keeps it queued
		h.logf("webhook %s: encrypted=%v with %d key(s): refused", ev.ID, encrypted, len(h.keys))
		http.Error(w, "cannot decrypt", http.StatusInternalServerError)
		return
	case encrypted:
		if ev, err = lounge.OpenWebhook(body, h.keys...); err != nil {
			h.logf("webhook: %v", err)
			http.Error(w, "cannot decrypt", http.StatusInternalServerError)
			return
		}
	}
	if ev.ID == "" || h.duplicate(ev.ID) {
		w.WriteHeader(http.StatusOK)
		return
	}
	h.apply(ev)
	w.WriteHeader(http.StatusOK)
}

func (h *hooks) apply(ev *lounge.WebhookEvent) {
	var d accountEvent
	_ = json.Unmarshal(ev.Data, &d) // unknown shapes leave d empty: nothing to do
	switch ev.Event {
	case "user.frozen":
		if d.Frozen == nil || *d.Frozen {
			h.world.kickUID(d.UID, "account frozen")
		}
	case "user.locked":
		h.world.kickUID(d.UID, "account locked by its owner")
	case "user.deleted":
		h.world.kickUID(d.UID, "account deleted")
	case "game_role.reviewed", "user.level_changed":
		h.resync() // the whitelist may have changed: refresh the local copy in the background
	}
}

func (h *hooks) duplicate(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	if _, ok := h.seen[id]; ok {
		return true
	}
	for k, t := range h.seen {
		if now.Sub(t) > 24*time.Hour {
			delete(h.seen, k)
		}
	}
	h.seen[id] = now
	return false
}

// hasJWE detects the encrypted shape even when a proxy dropped the header.
func hasJWE(body []byte) bool {
	var probe struct {
		JWE string `json:"jwe"`
	}
	return json.Unmarshal(body, &probe) == nil && probe.JWE != ""
}
