package lounge

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestWebhookVerify(t *testing.T) {
	secret := "whsec_test"
	body := []byte(`{"id":"e1","event":"user.login","timestamp":1,"data":{"userId":7}}`)
	ts := time.Now().Unix()
	h := http.Header{}
	h.Set("X-Lounge-Timestamp", strconv.FormatInt(ts, 10))
	h.Set("X-Lounge-Signature", WebhookSignature(secret, ts, body))
	ev, err := VerifyWebhook(secret, h, body, 5*time.Minute)
	if err != nil || ev.Event != "user.login" || string(ev.Data) != `{"userId":7}` {
		t.Fatalf("verify: %v %+v", err, ev)
	}
	h.Set("X-Lounge-Signature", "sha256=deadbeef")
	if _, err := VerifyWebhook(secret, h, body, 0); err != ErrWebhookSignature {
		t.Fatalf("expected signature error, got %v", err)
	}
	old := ts - 3600
	h.Set("X-Lounge-Timestamp", strconv.FormatInt(old, 10))
	h.Set("X-Lounge-Signature", WebhookSignature(secret, old, body))
	if _, err := VerifyWebhook(secret, h, body, 5*time.Minute); err != ErrWebhookStale {
		t.Fatalf("expected stale error, got %v", err)
	}
}
