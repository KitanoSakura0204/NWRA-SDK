package nowork

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type clientEncryptionFixture struct {
	keys           map[string]*DecryptionKey
	compact        map[string]string
	serverResponse string
	serverPlain    string
}

// Reuse the backend-generated fixtures; transport tests add no cryptographic implementation.
func loadClientEncryptionFixture(t *testing.T) clientEncryptionFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var set deliveryVectors
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	f := clientEncryptionFixture{keys: map[string]*DecryptionKey{}, compact: map[string]string{}}
	for name, jwk := range set.Keys {
		f.keys[name], err = ParseDecryptionKey(jwk)
		if err != nil {
			t.Fatalf("parse fixture %s: %v", name, err)
		}
	}
	for _, v := range set.Vectors {
		f.compact[v.Name] = v.Compact
		if v.Kind == "server" {
			f.serverResponse, f.serverPlain = v.Response, v.Plaintext
		}
	}
	if f.serverResponse == "" || f.keys["ec"] == nil {
		t.Fatal("missing backend server fixture")
	}
	return f
}

func clientEncryptionHTTP(body string, status int, marker string) *http.Client {
	return &http.Client{Transport: transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
		h := make(http.Header)
		if marker != "" {
			h.Set(HeaderEncryption, marker)
		}
		return &http.Response{StatusCode: status, Header: h, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func clientEncryptionResponse(t *testing.T, compact string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"code": 0, "data": map[string]string{"jwe": compact}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestClientDeliveryDecryptsAndPreservesRaw(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	for _, server := range []string{"", "survival"} {
		for _, marker := range []string{"", "JWE"} {
			t.Run(server+"/"+marker, func(t *testing.T) {
				c := New("https://example.test", WithServerKey("fixture-server-key"),
					WithHTTPClient(clientEncryptionHTTP(f.serverResponse, http.StatusOK, marker)),
					WithDecryptionKeys(f.keys["rsa"], nil, f.keys["ec"], f.keys["x25519"]), WithServerCode(server))
				wl, err := c.Whitelist(context.Background())
				if err != nil || wl == nil || wl.Server != "survival" || wl.Mode != 1 || wl.Count != 1 || len(wl.Entries) != 1 {
					t.Fatalf("encrypted whitelist: %#v, %v", wl, err)
				}
				if wl.Entries[0].Name != "Steve" || wl.Entries[0].Nickname != "旅行者" || wl.Entries[0].SecurityLevel != 2 {
					t.Fatalf("decryption changed player data: %#v", wl.Entries[0])
				}
				res, err := c.Raw(context.Background(), http.MethodGet, "/server/v1/whitelist", nil, nil)
				if err != nil || res == nil || !res.Encrypted() || string(res.Body) != f.serverResponse || !isJWEData(res.Data) {
					t.Fatalf("Raw did not preserve the encrypted reply: %v", err)
				}
				plain, err := OpenServerData(res.Body, "server:survival:whitelist", f.keys["ec"])
				if err != nil || string(plain) != f.serverPlain {
					t.Fatalf("Raw no longer works with OpenServerData: %v", err)
				}
			})
		}
	}
}

func TestClientDeliverySnapshotsRotationKeys(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	keys := []*DecryptionKey{f.keys["rsa"], f.keys["ec"]}
	option := WithDecryptionKeys(keys...)
	keys[1] = nil
	f.keys["ec"].Kid = "caller-mutated-kid"
	for i := 0; i < 2; i++ {
		c := New("https://example.test", WithServerKey("fixture-server-key"), option,
			WithHTTPClient(clientEncryptionHTTP(f.serverResponse, http.StatusOK, "JWE")))
		if wl, err := c.Whitelist(context.Background()); err != nil || wl == nil || wl.Mode != 1 {
			t.Fatalf("option reuse %d did not retain the configured key: %v", i, err)
		}
	}
	// A JWK without a kid can still open the backend-selected key during rotation.
	keyWithoutID := *f.keys["ec"]
	keyWithoutID.Kid = ""
	c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(f.keys["rsa"], &keyWithoutID),
		WithHTTPClient(clientEncryptionHTTP(f.serverResponse, http.StatusOK, "")))
	if wl, err := c.Whitelist(context.Background()); err != nil || wl == nil || wl.Mode != 1 {
		t.Fatalf("rotation without a local kid failed: %v", err)
	}
}

func TestClientDeliveryRejectsWrongKeyAndContext(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	parts := strings.Split(f.compact["server-envelope"], ".")
	if parts[3][0] == 'A' {
		parts[3] = "B" + parts[3][1:]
	} else {
		parts[3] = "A" + parts[3][1:]
	}
	wrongKey := *f.keys["rsa"]
	wrongKey.Kid = f.keys["ec"].Kid
	for _, tc := range []struct {
		name, compact, server string
		key                   *DecryptionKey
		decryptFailure        bool
	}{
		{"wrong key", f.compact["server-envelope"], "survival", f.keys["rsa"], true},
		{"wrong key matching kid", f.compact["server-envelope"], "survival", &wrongKey, true},
		{"nil key", f.compact["server-envelope"], "survival", nil, true},
		{"wrong server", f.compact["server-envelope"], "other", f.keys["ec"], false},
		{"server case mismatch", f.compact["server-envelope"], "Survival", f.keys["ec"], false},
		{"missing ctx", f.compact["jwe-ec"], "", f.keys["ec"], false},
		{"webhook ctx", f.compact["webhook-envelope"], "", f.keys["x25519"], false},
		{"ID token", f.compact["nested-id-token"], "", f.keys["rsa"], false},
		{"corrupted ciphertext", strings.Join(parts, "."), "survival", f.keys["ec"], true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(tc.key), WithServerCode(tc.server),
				WithHTTPClient(clientEncryptionHTTP(clientEncryptionResponse(t, tc.compact), http.StatusOK, "JWE")))
			wl, err := c.Whitelist(context.Background())
			var enc *EncryptionError
			if wl != nil || !errors.As(err, &enc) || enc.Route != "whitelist" || !IsNotAllowed(nil, err) {
				t.Fatalf("unusable encrypted data was not refused: %#v, %v", wl, err)
			}
			if errors.Is(err, ErrDecrypt) != tc.decryptFailure {
				t.Fatalf("decryption/context error classification changed: %v", err)
			}
			if strings.Contains(err.Error(), tc.compact) || strings.Contains(err.Error(), "fixture-server-key") {
				t.Fatal("encryption error exposed ciphertext or a credential")
			}
		})
	}
}

func TestClientDeliveryRejectsMalformedEnvelopes(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	for _, data := range []string{`{"jwe":null}`, `{"jwe":17}`, `{"jwe":false}`, `{"jwe":{}}`, `{"jwe":[]}`, `{"jwe":""}`, `{"jwe":"not-a-jwe"}`} {
		for _, marker := range []string{"", "JWE"} {
			for _, keys := range [][]*DecryptionKey{nil, {f.keys["ec"]}} {
				c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(keys...),
					WithHTTPClient(clientEncryptionHTTP(`{"code":0,"data":`+data+`}`, http.StatusOK, marker)))
				wl, err := c.Whitelist(context.Background())
				var enc *EncryptionError
				if wl != nil || !errors.As(err, &enc) || !IsNotAllowed(nil, err) {
					t.Fatalf("malformed %s, marker %q, keys %d did not fail closed: %v", data, marker, len(keys), err)
				}
			}
		}
	}
	for _, data := range []string{`null`, `{"mode":0}`, `{"jwe":"ignored","mode":0}`} {
		c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(f.keys["ec"]),
			WithHTTPClient(clientEncryptionHTTP(`{"code":0,"data":`+data+`}`, http.StatusOK, "JWE")))
		var enc *EncryptionError
		if err := c.Heartbeat(context.Background(), Heartbeat{}); !errors.As(err, &enc) {
			t.Fatalf("marked invalid envelope was accepted by a write-only method: %v", err)
		}
	}
}

func TestClientDeliveryEveryServerMethodChecksEncryption(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	ctx := context.Background()
	for _, keyed := range []bool{false, true} {
		for _, tc := range []struct {
			name, route string
			call        func(*Client) error
		}{
			{"whitelist", "whitelist", func(c *Client) error { _, err := c.Whitelist(ctx); return err }},
			{"check", "whitelist/check", func(c *Client) error { _, err := c.CheckWhitelist(ctx, "Player & One", ""); return err }},
			{"heartbeat", "heartbeat", func(c *Client) error { return c.Heartbeat(ctx, Heartbeat{Online: 1}) }},
			{"modules", "modules", func(c *Client) error { _, err := c.Modules(ctx); return err }},
			{"GM publish", "gm/actions", func(c *Client) error { return c.PublishGMActions(ctx, []GMAction{}) }},
			{"GM catalog", "gm/actions", func(c *Client) error { _, err := c.PublishGMCatalog(ctx, []GMAction{}, nil); return err }},
			{"GM options", "gm/options", func(c *Client) error { _, err := c.PublishGMOptions(ctx, map[string][]GMOption{}); return err }},
			{"GM poll", "gm/commands", func(c *Client) error { _, err := c.PollGMCommands(ctx); return err }},
			{"GM result", "gm/commands/7/result", func(c *Client) error { return c.ReportGMResult(ctx, 7, true, "done") }},
			{"GM SSO", "gm/sso/exchange", func(c *Client) error {
				_, err := c.ExchangeGMCode(ctx, "code", "verifier", "https://plugin.example/cb")
				return err
			}},
			{"user schema", "users/schema", func(c *Client) error { _, err := c.UserSchema(ctx); return err }},
			{"user get", "users/7/data", func(c *Client) error { _, err := c.UserData(ctx, 7); return err }},
			{"user put", "users/7/data", func(c *Client) error { _, err := c.PutUserData(ctx, 7, nil, -1); return err }},
			{"user patch", "users/7/data", func(c *Client) error { _, err := c.PatchUserData(ctx, 7, nil, -1); return err }},
			{"bans", "bans", func(c *Client) error { _, err := c.Bans(ctx, BanTarget{}); return err }},
			{"kick list", "bans/kick", func(c *Client) error { _, err := c.KickList(ctx); return err }},
			{"ban", "bans", func(c *Client) error { _, _, err := c.Ban(ctx, BanTarget{UID: 7}, BanOptions{}); return err }},
			{"unban", "bans/lift", func(c *Client) error { _, err := c.Unban(ctx, BanTarget{UID: 7}, "appeal"); return err }},
			{"chat ticket", "chat/tickets", func(c *Client) error { _, err := c.IssueChatTicket(ctx, ChatTicketInput{UID: 7}); return err }},
			{"chat poll", "chat/relay", func(c *Client) error { _, err := c.PollChatRelay(ctx); return err }},
			{"chat ack", "chat/relay/ack", func(c *Client) error { _, err := c.AckChatRelay(ctx, []int64{7}, nil); return err }},
			{"chat post", "chat/messages", func(c *Client) error {
				_, err := c.PostChatMessage(ctx, ChatPost{Player: "Steve", Text: "hello"})
				return err
			}},
			{"shared roles", "hypergryph/roles", func(c *Client) error { _, err := c.SharedHypergryphRoles(ctx, 7); return err }},
		} {
			name := "no key/" + tc.name
			if keyed {
				name = "keyed replay/" + tc.name
			}
			t.Run(name, func(t *testing.T) {
				c := New("https://example.test", WithServerKey("fixture-server-key"),
					WithHTTPClient(clientEncryptionHTTP(f.serverResponse, http.StatusOK, "JWE")))
				if keyed {
					WithDecryptionKeys(f.keys["ec"])(c)
				}
				err := tc.call(c)
				if keyed && tc.route == "whitelist" {
					if err != nil {
						t.Fatalf("correct route refused: %v", err)
					}
					return
				}
				var enc *EncryptionError
				if !errors.As(err, &enc) || enc.Route != tc.route || !IsNotAllowed(nil, err) {
					t.Fatalf("method accepted unreadable/replayed envelope: %v", err)
				}
			})
		}
	}
}

func TestClientDeliveryRefusesOnlyProtectedPlaintextRoutes(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, data string
		call       func(*Client) error
	}{
		{"heartbeat", `{"server":"survival","status":"ok"}`, func(c *Client) error { return c.Heartbeat(ctx, Heartbeat{}) }},
		{"modules", `{"modules":{"server-gm":{}}}`, func(c *Client) error { _, err := c.Modules(ctx); return err }},
		{"GM publish", `{"count":0,"skipped":[]}`, func(c *Client) error { return c.PublishGMActions(ctx, nil) }},
		{"GM catalog", `{"count":0,"skipped":[]}`, func(c *Client) error { _, err := c.PublishGMCatalog(ctx, nil, nil); return err }},
		{"GM options", `{"count":0,"skipped":[]}`, func(c *Client) error { _, err := c.PublishGMOptions(ctx, nil); return err }},
		{"GM poll", `[]`, func(c *Client) error { _, err := c.PollGMCommands(ctx); return err }},
		{"GM result", `null`, func(c *Client) error { return c.ReportGMResult(ctx, 7, true, "done") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(f.keys["ec"]), WithServerCode("survival"),
				WithHTTPClient(clientEncryptionHTTP(`{"code":0,"data":`+tc.data+`}`, http.StatusOK, "")))
			if err := tc.call(c); err != nil {
				t.Fatalf("valid plaintext unprotected route refused: %v", err)
			}
		})
	}
	for _, keys := range [][]*DecryptionKey{nil, {f.keys["ec"]}, {nil}} {
		body := `{"code":0,"data":` + f.serverPlain + `}`
		c := New("https://example.test", WithServerKey("fixture-server-key"), WithServerCode("survival"), WithDecryptionKeys(keys...),
			WithHTTPClient(clientEncryptionHTTP(body, http.StatusOK, "")))
		wl, err := c.Whitelist(ctx)
		var enc *EncryptionError
		if len(keys) == 0 {
			if err != nil || wl == nil || wl.Mode != 1 {
				t.Fatalf("server code alone required encryption: %v", err)
			}
		} else if wl != nil || !errors.As(err, &enc) || enc.Route != "whitelist" {
			t.Fatalf("plaintext whitelist downgrade accepted: %#v, %v", wl, err)
		}
		res, err := c.Raw(ctx, http.MethodGet, "/server/v1/whitelist", nil, nil)
		if err != nil || res == nil || res.Encrypted() || string(res.Body) != body {
			t.Fatalf("Raw stopped returning the original plaintext reply: %v", err)
		}
	}
	c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(f.keys["ec"]),
		WithHTTPClient(clientEncryptionHTTP(`{"code":0,"data":{"allowed":true,"bound":false}}`, http.StatusOK, "")))
	check, err := c.CheckWhitelist(ctx, "Steve", "")
	var enc *EncryptionError
	if check != nil || !errors.As(err, &enc) || enc.Route != "whitelist/check" || !IsNotAllowed(check, err) {
		t.Fatalf("plaintext join-check downgrade accepted: %#v, %v", check, err)
	}
}

func TestClientDeliveryDecryptedTypeErrorsAreNotNetworkFailures(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(f.keys["ec"]),
		WithHTTPClient(clientEncryptionHTTP(f.serverResponse, http.StatusOK, "JWE")))
	var wrongType []int
	err := c.do(context.Background(), http.MethodGet, "/server/v1/whitelist?since=0", nil, nil, &wrongType)
	var enc *EncryptionError
	var transport *TransportError
	if !errors.As(err, &enc) || errors.As(err, &transport) || enc.Route != "whitelist" || enc.Reason != "decrypted data is invalid" {
		t.Fatalf("decrypted type failure was not classified safely: %v", err)
	}
}

func TestClientDeliveryContextBinding(t *testing.T) {
	for _, tc := range []struct {
		ctx, server, route string
		want               bool
	}{
		{"server:survival:whitelist", "", "whitelist", true},
		{"server:survival:whitelist", "survival", "whitelist", true},
		{"server:survival:whitelist/check", "survival", "whitelist/check", true},
		{"server:survival:whitelist", "other", "whitelist", false},
		{"server:survival:whitelist/check", "", "whitelist", false},
		{"server:survival:whitelist", "", "whitelist/check", false},
		{"server:survival:gm/commands/7/result", "survival", "gm/commands/7/result", true},
		{"server:survival:gm/commands/8/result", "survival", "gm/commands/7/result", false},
		{"webhook:survival:whitelist", "", "whitelist", false},
		{"server::whitelist", "", "whitelist", false},
		{"server: :whitelist", "", "whitelist", false},
		{"server:survival:other:whitelist", "", "whitelist", false},
		{"server:survival:whitelist/", "", "whitelist", false},
		{"server:survival:", "survival", "", false},
		{"", "", "whitelist", false},
	} {
		if got := serverResponseContextMatches(tc.ctx, tc.server, tc.route); got != tc.want {
			t.Fatalf("ctx=%q server=%q route=%q: got %v", tc.ctx, tc.server, tc.route, got)
		}
	}
}

func TestClientDeliveryRequiresExplicitWhitelistDecisions(t *testing.T) {
	for _, tc := range []struct {
		route, data string
		want        bool
	}{
		{"whitelist", `{"mode":0}`, true},
		{"whitelist", `{"mode":1}`, true},
		{"whitelist", `{"mode":2}`, true},
		{"whitelist", `{}`, false},
		{"whitelist", `{"mode":null}`, false},
		{"whitelist", `{"mode":"0"}`, false},
		{"whitelist", `{"mode":false}`, false},
		{"whitelist", `{"mode":0.5}`, false},
		{"whitelist", `{"mode":-1}`, false},
		{"whitelist", `{"mode":3}`, false},
		{"whitelist", `{"Mode":0}`, false},
		{"whitelist/check", `{"allowed":true}`, true},
		{"whitelist/check", `{"allowed":false}`, true},
		{"whitelist/check", `{}`, false},
		{"whitelist/check", `{"allowed":null}`, false},
		{"whitelist/check", `{"allowed":1}`, false},
		{"whitelist/check", `{"allowed":"false"}`, false},
		{"whitelist/check", `{"Allowed":true}`, false},
		{"whitelist/check", `[]`, false},
		{"gm/commands", `[]`, true},
		{"heartbeat", `{}`, true},
		{"heartbeat", `null`, false},
		{"heartbeat", `not JSON`, false},
		{"heartbeat", "{\"name\":\"\xff\"}", false},
	} {
		if got := validDecryptedServerData([]byte(tc.data), tc.route); got != tc.want {
			t.Fatalf("route=%s data=%q: valid=%v", tc.route, tc.data, got)
		}
	}
}

func TestClientDeliveryPreservesHTTPAndBusinessErrors(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	for _, tc := range []struct {
		name, body string
		status     int
		apiCode    int
	}{
		{"business failure", `{"code":403,"message":"server disabled"}`, http.StatusForbidden, 403},
		{"business failure with ciphertext", strings.Replace(f.serverResponse, `"code":0`, `"code":409`, 1), http.StatusOK, 409},
		{"HTTP failure", f.serverResponse, http.StatusServiceUnavailable, 0},
		{"missing code", `{"data":{"jwe":"broken"}}`, http.StatusOK, 0},
		{"invalid envelope JSON", `<html>unavailable</html>`, http.StatusBadGateway, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("https://example.test", WithServerKey("fixture-server-key"), WithDecryptionKeys(f.keys["ec"]),
				WithHTTPClient(clientEncryptionHTTP(tc.body, tc.status, "JWE")))
			wl, err := c.Whitelist(context.Background())
			var enc *EncryptionError
			if wl != nil || err == nil || errors.As(err, &enc) {
				t.Fatalf("HTTP/business failure was swallowed or reclassified: %#v, %v", wl, err)
			}
			if tc.apiCode != 0 {
				var api *ApiError
				if !errors.As(err, &api) || api.Code != tc.apiCode {
					t.Fatalf("business code changed: %v", err)
				}
			} else {
				var transport *TransportError
				if !errors.As(err, &transport) {
					t.Fatalf("transport failure changed: %v", err)
				}
			}
		})
	}
}

func TestClientDeliveryKeepsResponseCapAndClosesBody(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	for _, extra := range []int64{0, 1} {
		body := &transportTestBody{Reader: io.MultiReader(strings.NewReader(f.serverResponse),
			io.LimitReader(transportTestSpaces{}, defaultResponseLimit-int64(len(f.serverResponse))+extra))}
		h := &http.Client{Transport: transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
			header := make(http.Header)
			header.Set(HeaderEncryption, "JWE")
			return &http.Response{StatusCode: http.StatusOK, Header: header, Body: body}, nil
		})}
		c := New("https://example.test", WithServerKey("fixture-server-key"), WithHTTPClient(h), WithDecryptionKeys(f.keys["ec"]))
		wl, err := c.Whitelist(context.Background())
		if extra == 0 && (err != nil || wl == nil || wl.Mode != 1) {
			t.Fatalf("encrypted reply at exact wire cap refused: %v", err)
		}
		if extra != 0 {
			var transport *TransportError
			if wl != nil || !errors.Is(err, ErrResponseTooLarge) || !errors.As(err, &transport) {
				t.Fatalf("encrypted reply bypassed wire cap: %v", err)
			}
			if strings.Contains(err.Error(), f.compact["server-envelope"]) || strings.Contains(err.Error(), "fixture-server-key") {
				t.Fatal("wire-cap error exposed encrypted data or credentials")
			}
		}
		if !body.closed || body.read != defaultResponseLimit+extra {
			t.Fatalf("encrypted response was not bounded/closed: read=%d closed=%v", body.read, body.closed)
		}
	}
}

func TestClientDeliveryKeysDoNotReplaceCredentials(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	calls := 0
	h := &http.Client{Transport: transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected request")
	})}
	for _, authorization := range []string{"", "Bearer fixture-jwt", "ApiKey fixture-api-key", "Chat fixture-chat-token"} {
		c := New("https://example.test", WithHTTPClient(h), WithDecryptionKeys(f.keys["ec"]), WithServerCode("survival"))
		c.headers = make(http.Header)
		c.headers.Set("Authorization", authorization)
		if wl, err := c.Whitelist(context.Background()); wl != nil || !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("decryption keys bypassed server-key authentication: %v", err)
		}
	}
	for _, c := range []*Client{NewAccount("https://example.test", "fixture-jwt", h).transport, NewChat("https://example.test", "fixture-chat", h).transport} {
		WithDecryptionKeys(f.keys["ec"])(c)
		if _, err := c.Whitelist(context.Background()); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("decryption keys widened account/chat authority: %v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid credentials reached transport %d times", calls)
	}
}

func TestClientDeliveryPreservesServerHeadersAndTimeout(t *testing.T) {
	f := loadClientEncryptionFixture(t)
	h := &http.Client{Timeout: 7 * time.Second, Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Server-Key") != "fixture-server-key" || r.Header.Get("Authorization") != "" {
			t.Error("configured delivery keys changed transport authentication")
		}
		if r.Method != http.MethodGet || r.URL.Path != "/server/v1/whitelist" || r.URL.RawQuery != "" || r.Body != nil {
			t.Error("configured keys/server code were added to the request")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(f.serverResponse))}, nil
	})}
	c := New("https://example.test", WithServerKey("fixture-server-key"), WithHTTPClient(h), WithDecryptionKeys(f.keys["ec"]), WithServerCode("survival"))
	if c.http != h || c.http.Timeout != 7*time.Second {
		t.Fatal("configured encryption changed the caller HTTP client")
	}
	if wl, err := c.Whitelist(context.Background()); err != nil || wl == nil || wl.Mode != 1 {
		t.Fatalf("server-authenticated decryption failed: %v", err)
	}
	if New("https://example.test", WithDecryptionKeys(f.keys["ec"])).http.Timeout != 10*time.Second {
		t.Fatal("default timeout changed")
	}
	timedHTTP := &http.Client{Timeout: 10 * time.Millisecond, Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	c = New("https://example.test", WithServerKey("fixture-server-key"), WithHTTPClient(timedHTTP), WithDecryptionKeys(f.keys["ec"]))
	wl, err := c.Whitelist(context.Background())
	var transport *TransportError
	if wl != nil || !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &transport) {
		t.Fatalf("configured delivery keys lost the custom timeout: %v", err)
	}
}
