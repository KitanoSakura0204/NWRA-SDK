package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	lounge "github.com/wangework/lounge-sdk-go"
)

const testSecret = "whsec_test"

func newReceiver(secret string, tolerance time.Duration, keys []*lounge.DecryptionKey) *receiver {
	return &receiver{secret: secret, tolerance: tolerance, seen: newSeenSet(100), keys: keys}
}

// post calls the handler directly (no listener, no network).
func post(rc *receiver, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/lounge/webhook", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	rc.handle(rec, req)
	return rec
}

func signed(secret string, ts int64, body string) map[string]string {
	return map[string]string{
		"Content-Type":       "application/json",
		"X-Lounge-Timestamp": strconv.FormatInt(ts, 10),
		"X-Lounge-Signature": lounge.WebhookSignature(secret, ts, []byte(body)),
	}
}

func status(t *testing.T, rec *httptest.ResponseRecorder, want int, wantStatus string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("HTTP %d, want %d (%s)", rec.Code, want, rec.Body.String())
	}
	if wantStatus == "" {
		return
	}
	var out struct{ Status string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != wantStatus {
		t.Fatalf("status %q, want %q", out.Status, wantStatus)
	}
}

func TestPlainDeliveryAndDuplicate(t *testing.T) {
	rc := newReceiver(testSecret, 5*time.Minute, nil)
	now := time.Now().Unix()
	body := `{"id":"evt-1","event":"user.level_changed","timestamp":` + strconv.FormatInt(now, 10) +
		`,"data":{"userId":1,"uid":100001,"username":"alice","from":1,"to":2}}`
	status(t, post(rc, body, signed(testSecret, now, body)), http.StatusOK, "ok")
	status(t, post(rc, body, signed(testSecret, now, body)), http.StatusOK, "duplicate")
}

func TestRejections(t *testing.T) {
	rc := newReceiver(testSecret, 5*time.Minute, nil)
	now := time.Now().Unix()
	body := `{"id":"evt-2","event":"ping","timestamp":1,"data":{}}`

	status(t, post(rc, body, signed("wrong", now, body)), http.StatusUnauthorized, "")
	status(t, post(rc, body+" ", signed(testSecret, now, body)), http.StatusUnauthorized, "")
	status(t, post(rc, body, signed(testSecret, now-3600, body)), http.StatusUnauthorized, "")
	status(t, post(rc, "not json", signed(testSecret, now, "not json")), http.StatusBadRequest, "")
	noID := `{"event":"ping","timestamp":1,"data":{}}`
	status(t, post(rc, noID, signed(testSecret, now, noID)), http.StatusBadRequest, "")

	rec := httptest.NewRecorder()
	rc.handle(rec, httptest.NewRequest(http.MethodGet, "/lounge/webhook", nil))
	status(t, rec, http.StatusMethodNotAllowed, "")
}

// The shared SDK test vectors hold a signed, encrypted webhook (X25519 key).
type vectorFile struct {
	Keys    map[string]json.RawMessage `json:"keys"`
	Vectors []struct {
		Name    string            `json:"name"`
		Body    string            `json:"body"`
		Secret  string            `json:"secret"`
		Headers map[string]string `json:"headers"`
	} `json:"vectors"`
}

func TestEncryptedDelivery(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "go", "testdata", "vectors.json"))
	if err != nil {
		t.Skip("SDK test vectors not found (example copied out of the repo)")
	}
	var vf vectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(t.TempDir(), "x25519.jwk")
	if err := os.WriteFile(keyFile, vf.Keys["x25519"], 0o600); err != nil {
		t.Fatal(err)
	}
	keys, err := loadKeys(" " + keyFile + " ,")
	if err != nil || len(keys) != 1 {
		t.Fatalf("loadKeys: %v (%d keys)", err, len(keys))
	}
	for _, v := range vf.Vectors {
		if v.Name != "webhook-envelope" {
			continue
		}
		// The vector's timestamp is fixed, so the replay window is off (tolerance 0).
		status(t, post(newReceiver(v.Secret, 0, keys), v.Body, v.Headers), http.StatusOK, "ok")
		status(t, post(newReceiver(v.Secret, 0, nil), v.Body, v.Headers), http.StatusInternalServerError, "")

		plain := `{"id":"evt-3","event":"ping","timestamp":1,"data":{}}`
		now := time.Now().Unix()
		status(t, post(newReceiver(v.Secret, 0, keys), plain, signed(v.Secret, now, plain)), http.StatusInternalServerError, "")
		return
	}
	t.Fatal("webhook-envelope vector missing")
}

func TestWhoPrefersPairwiseSub(t *testing.T) {
	if got := who(100001, pairwiseUser{}); got != "uid=100001" {
		t.Fatalf("who = %q", got)
	}
	if got := who(0, pairwiseUser{Sub: "2f1c8e0a"}); got != "sub=2f1c8e0a" {
		t.Fatalf("who = %q", got)
	}
}
