package main

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type loginOutput struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (o *loginOutput) Write(b []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.b.Write(b)
}
func (o *loginOutput) String() string { o.mu.Lock(); defer o.mu.Unlock(); return o.b.String() }

func TestGameLoginVerifiesIdentityBeforeReportingSuccess(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "wrong nonce", "missing nonce", "invalid signature", "wrong issuer", "wrong audience", "expired", "just expired", "wrong authorized party", "multiple audiences", "missing ID Token", "missing subject", "UserInfo mismatch"} {
		t.Run(mode, func(t *testing.T) {
			var output loginOutput
			var userCalls atomic.Int32
			var srv *httptest.Server
			srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				enc := base64.RawURLEncoding.EncodeToString
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					_ = json.NewEncoder(w).Encode(map[string]any{"issuer": srv.URL, "authorization_endpoint": srv.URL + "/oauth2/authorize", "token_endpoint": srv.URL + "/oauth2/token", "jwks_uri": srv.URL + "/jwks.json"})
				case "/jwks.json":
					if r.Header.Get("Authorization") != "" {
						t.Error("JWKS must be unauthenticated")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "login-test", "n": enc(key.N.Bytes()), "e": enc(big.NewInt(int64(key.E)).Bytes())}}})
				case "/oauth2/token":
					var loginURL *url.URL
					for _, field := range strings.Fields(output.String()) {
						if strings.HasPrefix(field, srv.URL+"/sdk/login/") {
							loginURL, _ = url.Parse(field)
							break
						}
					}
					if loginURL == nil {
						t.Error("login URL was not printed")
						http.Error(w, "no login", 400)
						return
					}
					q := loginURL.Query()
					if q.Get("nonce") == "" || q.Get("nonce") == q.Get("state") {
						t.Error("login requires an independent nonce")
					}
					_ = r.ParseForm()
					challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
					if r.Form.Get("code") != "TESTCODE" || r.Form.Get("client_id") != "game-client" || r.Form.Get("client_secret") != "" || enc(challenge[:]) != q.Get("code_challenge") {
						t.Error("invalid public PKCE exchange")
					}
					claims := map[string]any{"iss": srv.URL, "aud": "game-client", "sub": "alice", "nonce": q.Get("nonce"), "exp": time.Now().Add(time.Hour).Unix()}
					switch mode {
					case "wrong nonce":
						claims["nonce"] = "wrong"
					case "missing nonce":
						delete(claims, "nonce")
					case "wrong issuer":
						claims["iss"] = "https://wrong.example"
					case "wrong audience":
						claims["aud"] = "other"
					case "expired":
						claims["exp"] = time.Now().Add(-time.Hour).Unix()
					case "just expired":
						claims["exp"] = time.Now().Add(-time.Second).Unix()
					case "wrong authorized party":
						claims["azp"] = "other-client"
					case "multiple audiences":
						claims["aud"] = []string{"game-client", "other-client"}
					case "missing subject":
						delete(claims, "sub")
					}
					raw, _ := json.Marshal(claims)
					input := enc([]byte(`{"alg":"RS256","kid":"login-test"}`)) + "." + enc(raw)
					hash := sha256.Sum256([]byte(input))
					sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
					if err != nil {
						t.Error(err)
						http.Error(w, "signing failed", 500)
						return
					}
					if mode == "invalid signature" {
						sig[0] ^= 1
					}
					token := input + "." + enc(sig)
					if mode == "missing ID Token" {
						token = ""
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "token_type": "Bearer", "id_token": token})
				case "/oauth2/userinfo":
					userCalls.Add(1)
					if r.Header.Get("Authorization") != "Bearer test-access" {
						t.Error("missing bearer token")
					}
					sub := "alice"
					if mode == "UserInfo mismatch" {
						sub = "mallory"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"sub": sub, "nickname": "Alice"})
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			err := runLogin(context.Background(), loginConfig{frontend: srv.URL, issuer: srv.URL, server: "survival", clientID: "game-client"}, strings.NewReader("test-code\n"), &output)
			if mode == "success" {
				if err != nil || !strings.Contains(output.String(), "Signed in: sub=alice") {
					t.Fatalf("valid login failed: %v / %s", err, output.String())
				}
			} else {
				if err == nil || strings.Contains(output.String(), "Signed in:") {
					t.Fatalf("unverified success: %v / %s", err, output.String())
				}
				if mode != "UserInfo mismatch" && userCalls.Load() != 0 {
					t.Error("invalid ID Token reached UserInfo")
				}
			}
			if strings.Contains(output.String(), "test-access") {
				t.Error("token leaked to output")
			}
		})
	}
}
