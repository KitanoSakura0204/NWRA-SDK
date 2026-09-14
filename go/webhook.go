package lounge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// WebhookEvent is the envelope the user center POSTs to a webhook endpoint.
type WebhookEvent struct {
	ID        string          `json:"id"`
	Event     string          `json:"event"`
	Timestamp int64           `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// ErrWebhookSignature is returned when the signature does not match.
var ErrWebhookSignature = errors.New("lounge: invalid webhook signature")

// ErrWebhookStale is returned when the timestamp is outside the tolerance window.
var ErrWebhookStale = errors.New("lounge: webhook timestamp outside tolerance")

// WebhookSignature computes sha256=HMAC_SHA256(secret, "<ts>.<body>").
func WebhookSignature(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhook checks the X-Lounge-Signature / X-Lounge-Timestamp headers against body.
// tolerance limits replay (0 disables the check).
func VerifyWebhook(secret string, headers http.Header, body []byte, tolerance time.Duration) (*WebhookEvent, error) {
	tsRaw := strings.TrimSpace(headers.Get("X-Lounge-Timestamp"))
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return nil, ErrWebhookSignature
	}
	if tolerance > 0 {
		if d := time.Since(time.Unix(ts, 0)); d > tolerance || d < -tolerance {
			return nil, ErrWebhookStale
		}
	}
	expect := WebhookSignature(secret, ts, body)
	if !hmac.Equal([]byte(expect), []byte(strings.TrimSpace(headers.Get("X-Lounge-Signature")))) {
		return nil, ErrWebhookSignature
	}
	var ev WebhookEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		return nil, &TransportError{err}
	}
	return &ev, nil
}

// ReadWebhook reads and verifies an incoming webhook request (max 1 MiB body).
func ReadWebhook(secret string, r *http.Request, tolerance time.Duration) (*WebhookEvent, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, &TransportError{err}
	}
	return VerifyWebhook(secret, r.Header, body, tolerance)
}
