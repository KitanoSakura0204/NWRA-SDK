package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	nowork "github.com/wangework/nowork-sdk-go"
)

// Encrypted deliveries (SDK/docs/DELIVERY.md section 6). When the endpoint's
// delivery profile has webhook.encrypt on, the body is
// {"id","event","timestamp","jwe"} plus the header X-NoWork-Encryption: JWE.
// The signature covers that outer body, so it is verified first and opened
// afterwards; the JWE holds the complete original document.

var (
	// errNoKeys: an encrypted delivery arrived but NOWORK_DECRYPTION_KEYS is empty.
	errNoKeys = errors.New("encrypted delivery but no decryption key configured")
	// errNotEncrypted: keys are configured but the delivery is plain. Refuse it
	// rather than silently accept plaintext from an endpoint that should encrypt.
	errNotEncrypted = errors.New("decryption keys configured but the delivery is not encrypted")
)

// loadKeys reads private JWKs from a comma-separated list of files. During a
// key rotation list both the new and the old key: the JWE header's kid picks
// the right one (DELIVERY.md section 7).
func loadKeys(paths string) ([]*nowork.DecryptionKey, error) {
	var keys []*nowork.DecryptionKey
	for _, p := range strings.Split(paths, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		k, err := nowork.ParseDecryptionKey(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// open returns the event to dispatch. A plain delivery is returned as verified;
// an encrypted one is decrypted (OpenWebhook also checks that the inner id,
// event and timestamp repeat the signed outer ones).
func (rc *receiver) open(h http.Header, body []byte, ev *nowork.WebhookEvent) (*nowork.WebhookEvent, error) {
	encrypted := strings.EqualFold(h.Get("X-NoWork-Encryption"), "JWE") || hasJWE(body)
	switch {
	case !encrypted && len(rc.keys) > 0:
		return nil, errNotEncrypted
	case !encrypted:
		return ev, nil
	case len(rc.keys) == 0:
		return nil, errNoKeys
	}
	return nowork.OpenWebhook(body, rc.keys...)
}

// hasJWE detects the encrypted shape even when a proxy dropped the header.
func hasJWE(body []byte) bool {
	var probe struct {
		JWE string `json:"jwe"`
	}
	return json.Unmarshal(body, &probe) == nil && probe.JWE != ""
}
