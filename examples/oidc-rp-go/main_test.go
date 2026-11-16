package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// This provider stays on loopback and validates the actual authorization request
// and PKCE exchange. Signing keys are generated for each test, never credentials.
type mockProvider struct {
	t                     *testing.T
	server                *httptest.Server
	key                   *rsa.PrivateKey
	mu                    sync.Mutex
	request               url.Values
	changeClaims          func(map[string]any)
	userSub               string
	badSignature          bool
	badDiscovery          bool
	tokenCalls, userCalls int
}

func provider(t *testing.T) *mockProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &mockProvider{t: t, key: key, userSub: "alice"}
	p.server = httptest.NewServer(http.HandlerFunc(p.serve))
	t.Cleanup(p.server.Close)
	return p
}

func (p *mockProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		if r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
			p.t.Error("credentials on discovery")
		}
		issuer := p.server.URL
		if p.badDiscovery {
			issuer += "/wrong"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer, "authorization_endpoint": p.server.URL + "/oauth2/authorize", "token_endpoint": p.server.URL + "/oauth2/token", "jwks_uri": p.server.URL + "/jwks.json"})
	case "/jwks.json":
		if r.Header.Get("Authorization") != "" || r.URL.RawQuery != "" {
			p.t.Error("credentials on JWKS")
		}
		enc := base64.RawURLEncoding.EncodeToString
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "test", "n": enc(p.key.N.Bytes()), "e": enc(big.NewInt(int64(p.key.E)).Bytes())}}})
	case "/oauth2/token":
		p.tokenCalls++
		_ = r.ParseForm()
		challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Method != http.MethodPost || r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "test-code" || r.Form.Get("client_id") != "test-client" || r.Form.Get("redirect_uri") != p.request.Get("redirect_uri") || base64.RawURLEncoding.EncodeToString(challenge[:]) != p.request.Get("code_challenge") {
			p.t.Error("invalid authorization-code / PKCE exchange")
			http.Error(w, "invalid exchange", 400)
			return
		}
		claims := map[string]any{"iss": p.server.URL, "aud": "test-client", "sub": "alice", "nonce": p.request.Get("nonce"), "exp": time.Now().Add(time.Hour).Unix()}
		if p.changeClaims != nil {
			p.changeClaims(claims)
		}
		enc := base64.RawURLEncoding.EncodeToString
		raw, _ := json.Marshal(claims)
		input := enc([]byte(`{"alg":"RS256","kid":"test"}`)) + "." + enc(raw)
		hash := sha256.Sum256([]byte(input))
		sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, hash[:])
		if err != nil {
			p.t.Error(err)
			http.Error(w, "signing failed", 500)
			return
		}
		if p.badSignature {
			sig[0] ^= 1
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "test-access", "token_type": "Bearer", "id_token": input + "." + enc(sig)})
	case "/oauth2/userinfo":
		p.userCalls++
		if r.Header.Get("Authorization") != "Bearer test-access" {
			p.t.Error("missing userinfo bearer token")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"sub": p.userSub, "name": "Alice <script>"})
	default:
		http.NotFound(w, r)
	}
}

func testApp(t *testing.T, p *mockProvider) (*app, *httptest.Server) {
	t.Helper()
	s := httptest.NewUnstartedServer(nil)
	a, err := newApp(config{issuer: p.server.URL, clientID: "test-client", baseURL: "http://" + s.Listener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	s.Config.Handler = a
	s.Start()
	t.Cleanup(s.Close)
	return a, s
}

func request(t *testing.T, method, target, body string, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	r, err := http.NewRequest(method, target, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range cookies {
		if c != nil {
			r.AddCookie(c)
		}
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(b)
}

func cookieNamed(res *http.Response, name string) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func beginLogin(t *testing.T, p *mockProvider, base string) (string, *http.Cookie) {
	t.Helper()
	res, body := request(t, "GET", base+"/login", "")
	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: %d %s", res.StatusCode, body)
	}
	u, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme+"://"+u.Host != p.server.URL || u.Path != "/oauth2/authorize" || q.Get("response_type") != "code" || q.Get("scope") != "openid profile" || q.Get("nonce") == "" || q.Get("state") == "" || q.Get("state") == q.Get("nonce") || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" || q.Get("client_secret") != "" {
		t.Fatalf("invalid authorization request: %s", u)
	}
	p.mu.Lock()
	p.request = q
	p.mu.Unlock()
	c := cookieNamed(res, "nowork_attempt")
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 300 || c.Path != "/" || c.Value == q.Get("state") {
		t.Fatalf("unsafe attempt cookie: %+v", c)
	}
	return base + "/callback?" + url.Values{"code": {"test-code"}, "state": {q.Get("state")}, "iss": {p.server.URL}}.Encode(), c
}

func TestLoginCreatesVerifiedSession(t *testing.T) {
	p := provider(t)
	_, s := testApp(t, p)
	callback, attempt := beginLogin(t, p, s.URL)
	res, body := request(t, "GET", callback, "", attempt)
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
		t.Fatalf("callback: %d %s", res.StatusCode, body)
	}
	session := cookieNamed(res, "nowork_session")
	if session == nil || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode || session.MaxAge <= 0 || session.MaxAge > 3600 {
		t.Fatalf("invalid session cookie: %+v", session)
	}
	res, body = request(t, "GET", s.URL+"/", "", session)
	if res.StatusCode != 200 || !strings.Contains(body, "alice") || !strings.Contains(body, "Alice &lt;script&gt;") || strings.Contains(body, "test-access") {
		t.Fatalf("home: %d %s", res.StatusCode, body)
	}
}

func TestLogoutRequiresSessionBoundCSRFAndClearsSession(t *testing.T) {
	p := provider(t)
	_, s := testApp(t, p)
	callback, attempt := beginLogin(t, p, s.URL)
	res, _ := request(t, "GET", callback, "", attempt)
	session := cookieNamed(res, "nowork_session")
	_, home := request(t, "GET", s.URL+"/", "", session)
	match := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(home)
	if len(match) != 2 {
		t.Fatal("missing logout CSRF field")
	}
	for _, tc := range []struct {
		method, body string
		status       int
	}{
		{"GET", "", http.StatusMethodNotAllowed},
		{"POST", "", http.StatusForbidden},
		{"POST", "csrf=wrong", http.StatusForbidden},
		{"POST", "csrf=" + match[1] + "&csrf=" + match[1], http.StatusForbidden},
	} {
		res, _ = request(t, tc.method, s.URL+"/logout", tc.body, session)
		if res.StatusCode != tc.status {
			t.Errorf("logout %s %q: %d, want %d", tc.method, tc.body, res.StatusCode, tc.status)
		}
	}
	res, _ = request(t, "POST", s.URL+"/logout", "csrf="+match[1], session)
	if res.StatusCode != http.StatusSeeOther || cookieNamed(res, "nowork_session") == nil || cookieNamed(res, "nowork_session").MaxAge != -1 {
		t.Fatalf("logout did not clear cookie: %d", res.StatusCode)
	}
	_, home = request(t, "GET", s.URL+"/", "", session) // replay the old session handle
	if !strings.Contains(home, "You are signed out") {
		t.Fatal("logout left a reusable session")
	}
}

func TestCallbackRejectsInvalidIDTokenBeforeUserInfo(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(map[string]any)
		signature bool
	}{
		{"nonce mismatch", func(c map[string]any) { c["nonce"] = "wrong" }, false},
		{"missing nonce", func(c map[string]any) { delete(c, "nonce") }, false},
		{"non-string nonce", func(c map[string]any) { c["nonce"] = 123 }, false},
		{"issuer mismatch", func(c map[string]any) { c["iss"] = "https://wrong.example" }, false},
		{"audience mismatch", func(c map[string]any) { c["aud"] = "wrong-client" }, false},
		{"expired", func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, false},
		{"just expired", func(c map[string]any) { c["exp"] = time.Now().Add(-time.Second).Unix() }, false},
		{"not yet valid", func(c map[string]any) { c["nbf"] = time.Now().Add(time.Hour).Unix() }, false},
		{"no subject", func(c map[string]any) { delete(c, "sub") }, false},
		{"wrong authorized party", func(c map[string]any) { c["azp"] = "wrong-client" }, false},
		{"multiple audiences without authorized party", func(c map[string]any) { c["aud"] = []string{"test-client", "other"} }, false},
		{"signature", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := provider(t)
			p.changeClaims, p.badSignature = tc.change, tc.signature
			_, s := testApp(t, p)
			callback, attempt := beginLogin(t, p, s.URL)
			res, body := request(t, "GET", callback, "", attempt)
			if res.StatusCode != http.StatusUnauthorized || cookieNamed(res, "nowork_session") != nil {
				t.Fatalf("accepted ID Token: %d %s", res.StatusCode, body)
			}
			p.mu.Lock()
			calls := p.userCalls
			p.mu.Unlock()
			if calls != 0 {
				t.Fatal("UserInfo was fetched before all ID Token checks passed")
			}
		})
	}
}

func TestCallbackRequiresMatchingUserInfoSubject(t *testing.T) {
	for _, sub := range []string{"different-user", ""} {
		t.Run(sub, func(t *testing.T) {
			p := provider(t)
			p.userSub = sub
			_, s := testApp(t, p)
			callback, attempt := beginLogin(t, p, s.URL)
			res, _ := request(t, "GET", callback, "", attempt)
			if res.StatusCode != http.StatusUnauthorized || cookieNamed(res, "nowork_session") != nil {
				t.Fatal("mismatched UserInfo created a session")
			}
		})
	}
}

func TestCallbackIsBrowserBoundAndSingleUse(t *testing.T) {
	for _, mode := range []string{"missing cookie", "different browser", "state mismatch", "duplicate state", "issuer mismatch", "provider denial", "malformed query", "expired", "replay"} {
		t.Run(mode, func(t *testing.T) {
			p := provider(t)
			a, s := testApp(t, p)
			callback, attempt := beginLogin(t, p, s.URL)
			original := callback
			cookies := []*http.Cookie{attempt}
			switch mode {
			case "missing cookie":
				cookies = nil
			case "different browser":
				cookies = []*http.Cookie{{Name: "nowork_attempt", Value: "unrelated-browser"}}
			case "state mismatch":
				u, _ := url.Parse(callback)
				q := u.Query()
				q.Set("state", "wrong")
				u.RawQuery = q.Encode()
				callback = u.String()
			case "duplicate state":
				callback += "&state=second"
			case "issuer mismatch":
				u, _ := url.Parse(callback)
				q := u.Query()
				q.Set("iss", "https://wrong.example")
				u.RawQuery = q.Encode()
				callback = u.String()
			case "provider denial":
				callback += "&error=access_denied"
			case "malformed query":
				callback += "&bad=%zz"
			case "expired":
				future := time.Now().Add(6 * time.Minute)
				a.now = func() time.Time { return future }
			case "replay":
				res, _ := request(t, "GET", callback, "", attempt)
				if res.StatusCode != http.StatusSeeOther {
					t.Fatal("first callback failed")
				}
			}
			res, _ := request(t, "GET", callback, "", cookies...)
			if res.StatusCode != http.StatusBadRequest || cookieNamed(res, "nowork_session") != nil {
				t.Fatalf("invalid callback accepted: %d", res.StatusCode)
			}
			p.mu.Lock()
			calls := p.tokenCalls
			p.mu.Unlock()
			wantCalls := 0
			if mode == "replay" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("unbound/replayed code reached token endpoint: %d calls", calls)
			}
			if mode == "state mismatch" || mode == "provider denial" {
				res, _ = request(t, "GET", original, "", attempt)
				if res.StatusCode != http.StatusBadRequest {
					t.Fatal("failed callback could be reused")
				}
			}
		})
	}
}

func TestDiscoveryFailureStopsLogin(t *testing.T) {
	p := provider(t)
	p.badDiscovery = true
	_, s := testApp(t, p)
	res, _ := request(t, "GET", s.URL+"/login", "")
	if res.StatusCode != http.StatusBadGateway || res.Header.Get("Location") != "" || cookieNamed(res, "nowork_attempt") != nil {
		t.Fatal("failed discovery fell back to authorization")
	}
}

func TestSessionExpiresAtSignedTokenExpiry(t *testing.T) {
	p := provider(t)
	p.changeClaims = func(c map[string]any) { c["exp"] = time.Now().Add(90 * time.Second).Unix() }
	a, s := testApp(t, p)
	callback, attempt := beginLogin(t, p, s.URL)
	res, _ := request(t, "GET", callback, "", attempt)
	c := cookieNamed(res, "nowork_session")
	if c == nil || c.MaxAge > 90 {
		t.Fatal("session outlives signed token")
	}
	future := time.Now().Add(91 * time.Second)
	a.now = func() time.Time { return future }
	_, home := request(t, "GET", s.URL+"/", "", c)
	if !strings.Contains(home, "You are signed out") {
		t.Fatal("expired server-side session still authenticates")
	}
}

func TestStoresAreBoundedAndExpiredEntriesAreReclaimed(t *testing.T) {
	p := provider(t)
	a, s := testApp(t, p)
	a.capacity = 1
	_, _ = beginLogin(t, p, s.URL)
	res, _ := request(t, "GET", s.URL+"/login", "")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("pending login capacity was exceeded")
	}
	future := time.Now().Add(6 * time.Minute)
	a.now = func() time.Time { return future }
	_, _ = beginLogin(t, p, s.URL) // expired attempts are reclaimed on the next request
}

func TestSessionCapacityDoesNotEvictActiveUsers(t *testing.T) {
	p := provider(t)
	a, s := testApp(t, p)
	a.capacity = 1
	callback, attempt := beginLogin(t, p, s.URL)
	res, _ := request(t, "GET", callback, "", attempt)
	first := cookieNamed(res, "nowork_session")
	callback, attempt = beginLogin(t, p, s.URL)
	res, _ = request(t, "GET", callback, "", attempt)
	if res.StatusCode != http.StatusServiceUnavailable || cookieNamed(res, "nowork_session") != nil {
		t.Fatal("session capacity was exceeded")
	}
	_, home := request(t, "GET", s.URL+"/", "", first)
	if !strings.Contains(home, "Signed in as") {
		t.Fatal("capacity pressure evicted an existing user")
	}
}

func TestHTTPSCookiesAreSecureAndLogoutChecksOrigin(t *testing.T) {
	p := provider(t)
	a, err := newApp(config{issuer: p.server.URL, clientID: "test-client", baseURL: "https://rp.example"})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest("GET", "https://rp.example/login", nil))
	res := w.Result()
	defer res.Body.Close()
	c := cookieNamed(res, "nowork_attempt")
	if c == nil || !c.Secure || !c.HttpOnly || c.Domain != "" || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unsafe HTTPS cookie: %+v", c)
	}
	u, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	p.request = u.Query()
	p.mu.Unlock()
	r := httptest.NewRequest("GET", "https://rp.example/callback?"+url.Values{"state": {u.Query().Get("state")}, "code": {"test-code"}}.Encode(), nil)
	r.AddCookie(c)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	res = w.Result()
	defer res.Body.Close()
	s := cookieNamed(res, "nowork_session")
	if s == nil || !s.Secure {
		t.Fatal("HTTPS session cookie is not Secure")
	}
	r = httptest.NewRequest("GET", "https://rp.example/", nil)
	r.AddCookie(s)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	csrf := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(csrf) != 2 {
		t.Fatal("no signed-in logout form")
	}
	r = httptest.NewRequest("POST", "https://rp.example/logout", strings.NewReader("csrf="+csrf[1]))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://other.example")
	r.AddCookie(s)
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatal("cross-origin logout was accepted")
	}
}

func TestConfigurationRejectsUnsafeURLs(t *testing.T) {
	for _, c := range []config{
		{issuer: "http://account.example", clientID: "client", baseURL: "http://127.0.0.1:8080"},
		{issuer: "https://user:password@account.example", clientID: "client", baseURL: "http://127.0.0.1:8080"},
		{issuer: "https://account.example?query=1", clientID: "client", baseURL: "http://127.0.0.1:8080"},
		{issuer: "https://account.example", clientID: "client", baseURL: "https://rp.example/app"},
		{issuer: "https://account.example", clientID: "client", baseURL: "http://rp.example"},
		{issuer: "https://account.example", baseURL: "http://127.0.0.1:8080"},
	} {
		if _, err := newApp(c); err == nil {
			t.Fatal("unsafe or incomplete configuration accepted")
		}
	}
}

func TestExpiredSessionCapacityIsReclaimed(t *testing.T) {
	p := provider(t)
	a, s := testApp(t, p)
	a.capacity = 1
	callback, attempt := beginLogin(t, p, s.URL)
	res, _ := request(t, "GET", callback, "", attempt)
	if cookieNamed(res, "nowork_session") == nil {
		t.Fatal("initial login failed")
	}
	future := time.Now().Add(2 * time.Hour)
	a.now = func() time.Time { return future }
	p.mu.Lock()
	p.changeClaims = func(c map[string]any) { c["exp"] = future.Add(time.Hour).Unix() }
	p.mu.Unlock()
	callback, attempt = beginLogin(t, p, s.URL)
	res, _ = request(t, "GET", callback, "", attempt)
	if res.StatusCode != http.StatusSeeOther || cookieNamed(res, "nowork_session") == nil {
		t.Fatal("expired session still occupies capacity")
	}
}

func TestConcurrentCallbacksConsumeAttemptOnce(t *testing.T) {
	p := provider(t)
	_, s := testApp(t, p)
	callback, attempt := beginLogin(t, p, s.URL)
	results := make(chan int, 8)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for i := 0; i < 8; i++ {
		go func() {
			r, err := http.NewRequest("GET", callback, nil)
			if err != nil {
				results <- 0
				return
			}
			r.AddCookie(attempt)
			res, err := client.Do(r)
			if err != nil {
				results <- 0
				return
			}
			defer res.Body.Close()
			_, _ = io.Copy(io.Discard, res.Body)
			results <- res.StatusCode
		}()
	}
	successes := 0
	for i := 0; i < 8; i++ {
		status := <-results
		if status == http.StatusSeeOther {
			successes++
		} else if status != http.StatusBadRequest {
			t.Errorf("unexpected concurrent callback status: %d", status)
		}
	}
	if successes != 1 {
		t.Fatalf("one attempt created %d sessions", successes)
	}
	p.mu.Lock()
	calls := p.tokenCalls
	p.mu.Unlock()
	if calls != 1 {
		t.Fatalf("one attempt exchanged %d times", calls)
	}
}
