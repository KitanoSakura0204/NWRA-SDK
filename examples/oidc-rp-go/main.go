// Command oidc-rp-go demonstrates a local OIDC authorization-code relying party.
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	nowork "github.com/wangework/nowork-sdk-go"
)

const (
	attemptTTL    = 5 * time.Minute
	sessionTTL    = time.Hour
	attemptCookie = "nowork_attempt"
	sessionCookie = "nowork_session"
)

type config struct{ issuer, clientID, clientSecret, baseURL string }

type loginAttempt struct {
	state, nonce, verifier string
	expires                time.Time
}

type session struct {
	sub, name, csrf string
	expires         time.Time
}

type app struct {
	oidc     *nowork.OIDC
	baseURL  string
	secure   bool
	now      func() time.Time
	capacity int
	mu       sync.Mutex
	attempts map[string]loginAttempt
	sessions map[string]session
}

func newApp(c config) (*app, error) {
	_, err := checkedURL(c.issuer, false)
	if err != nil {
		return nil, errors.New("NOWORK_ISSUER must be an HTTPS URL (HTTP is allowed for loopback), without credentials, query or fragment")
	}
	base, err := checkedURL(c.baseURL, true)
	if err != nil {
		return nil, errors.New("APP_BASE_URL must be an HTTPS origin (HTTP is allowed for loopback)")
	}
	if strings.TrimSpace(c.clientID) == "" {
		return nil, errors.New("NOWORK_CLIENT_ID is required")
	}
	origin := base.String()
	// Only the application origin is canonicalized. The SDK preserves the
	// configured issuer for exact metadata/token matching (trimming trailing /).
	return &app{oidc: nowork.NewOIDC(c.issuer, c.clientID, c.clientSecret, origin+"/callback"), baseURL: origin, secure: base.Scheme == "https", now: time.Now, capacity: 1024, attempts: make(map[string]loginAttempt), sessions: make(map[string]session)}, nil
}

func checkedURL(raw string, originOnly bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("invalid URL")
	}
	if originOnly && u.Path != "" && u.Path != "/" {
		return nil, errors.New("origin required")
	}
	host := strings.ToLower(u.Hostname())
	loopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, errors.New("HTTPS required")
	}
	if originOnly {
		// Browsers lowercase scheme/host, normalize IP literals and ports, and
		// omit the scheme's default port when serializing an Origin header.
		if address, err := netip.ParseAddr(host); err == nil {
			host = address.String()
		}
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		port := u.Port()
		if port != "" {
			number, err := strconv.ParseUint(port, 10, 16)
			if err != nil {
				return nil, errors.New("invalid application port")
			}
			port = strconv.FormatUint(number, 10)
		}
		if port != "" && !(u.Scheme == "https" && port == "443") && !(u.Scheme == "http" && port == "80") {
			host += ":" + port
		}
		u.Host, u.Path, u.RawPath = host, "", ""
	}
	return u, nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'none'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	a.cleanup()
	switch r.URL.Path {
	case "/":
		if !method(w, r, http.MethodGet) {
			return
		}
		a.home(w, r)
	case "/login":
		if !method(w, r, http.MethodGet) {
			return
		}
		a.login(w, r)
	case "/callback":
		if !method(w, r, http.MethodGet) {
			return
		}
		a.callback(w, r)
	case "/logout":
		if !method(w, r, http.MethodPost) {
			return
		}
		a.logout(w, r)
	default:
		http.NotFound(w, r)
	}
}

func method(w http.ResponseWriter, r *http.Request, want string) bool {
	if r.Method == want {
		return true
	}
	w.Header().Set("Allow", want)
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (a *app) setCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	age := int((expires.Sub(a.now()) + time.Second - 1) / time.Second)
	if age < 1 {
		age = 1
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: age})
}

func (a *app) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(1, 0), MaxAge: -1})
}

// Ambiguous cookies are rejected instead of silently choosing one supplied by a
// sibling path/domain. The cookie contains only an unpredictable lookup handle.
func cookieValue(r *http.Request, name string) string {
	value := ""
	count := 0
	for _, c := range r.Cookies() {
		if c.Name == name {
			value = c.Value
			count++
		}
	}
	if count != 1 {
		return ""
	}
	return value
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	// Metadata validates the configured issuer; SDK operations deliberately use
	// that issuer's fixed paths, never endpoints advertised by a remote document.
	if _, err := a.oidc.Discover(ctx); err != nil {
		http.Error(w, "Provider discovery failed", http.StatusBadGateway)
		return
	}
	state, err := nowork.NewState()
	if err != nil {
		http.Error(w, "Random source unavailable", 500)
		return
	}
	nonce, err := nowork.NewState()
	if err != nil {
		http.Error(w, "Random source unavailable", 500)
		return
	}
	pkce, err := nowork.NewPKCE()
	if err != nil {
		http.Error(w, "Random source unavailable", 500)
		return
	}
	id, err := nowork.NewState()
	if err != nil {
		http.Error(w, "Random source unavailable", 500)
		return
	}
	expires := a.now().Add(attemptTTL)
	a.mu.Lock()
	delete(a.attempts, cookieValue(r, attemptCookie))
	if len(a.attempts) >= a.capacity {
		a.mu.Unlock()
		http.Error(w, "Too many pending logins; try again later", http.StatusServiceUnavailable)
		return
	}
	a.attempts[id] = loginAttempt{state: state, nonce: nonce, verifier: pkce.Verifier, expires: expires}
	a.mu.Unlock()
	a.setCookie(w, attemptCookie, id, expires)
	http.Redirect(w, r, a.oidc.AuthorizeURL(state, nonce, "openid profile", pkce.Challenge), http.StatusSeeOther)
}

func (a *app) callback(w http.ResponseWriter, r *http.Request) {
	id := cookieValue(r, attemptCookie)
	a.mu.Lock()
	pending, ok := a.attempts[id]
	delete(a.attempts, id) // consume once, including failed/denied callbacks
	a.mu.Unlock()
	a.clearCookie(w, attemptCookie)
	if !ok || !a.now().Before(pending.expires) {
		http.Error(w, "Login expired or already used; start again", http.StatusBadRequest)
		return
	}
	if _, err := url.ParseQuery(r.URL.RawQuery); err != nil {
		http.Error(w, "Invalid callback", http.StatusBadRequest)
		return
	}
	code, err := nowork.ParseLoginCallback(r.URL.String(), pending.state, a.oidc.Issuer)
	if err != nil {
		http.Error(w, "Invalid or denied callback; start again", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	token, err := a.oidc.Exchange(ctx, code, pending.verifier)
	if err != nil || token.IDToken == "" || token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") {
		http.Error(w, "Token exchange failed", http.StatusBadGateway)
		return
	}
	jwks, err := a.oidc.JWKS(ctx)
	if err != nil {
		http.Error(w, "Provider signing keys unavailable", http.StatusBadGateway)
		return
	}
	claims, err := nowork.VerifyIDTokenWithNonce(token.IDToken, jwks, a.oidc.Issuer, a.oidc.ClientID, pending.nonce, a.now())
	if err != nil {
		http.Error(w, "ID Token verification failed", http.StatusUnauthorized)
		return
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		http.Error(w, "ID Token has no subject", http.StatusUnauthorized)
		return
	}
	azp, hasAZP := claims["azp"]
	audiences, multiple := claims["aud"].([]any)
	if (hasAZP && azp != a.oidc.ClientID) || (multiple && len(audiences) > 1 && azp != a.oidc.ClientID) {
		http.Error(w, "ID Token authorized party mismatch", http.StatusUnauthorized)
		return
	}
	// The SDK permits 60 seconds of clock leeway. A new local session uses a
	// stricter expiry and must still be valid before UserInfo is requested.
	expires := a.now().Add(sessionTTL)
	tokenExpiry := time.Unix(int64(claims["exp"].(float64)), 0)
	if tokenExpiry.Before(expires) {
		expires = tokenExpiry
	}
	if !a.now().Before(expires) {
		http.Error(w, "ID Token expired", http.StatusUnauthorized)
		return
	}
	info, err := a.oidc.UserInfo(ctx, token.AccessToken)
	if err != nil {
		http.Error(w, "UserInfo failed", http.StatusBadGateway)
		return
	}
	if info.Sub != sub {
		http.Error(w, "UserInfo subject mismatch", http.StatusUnauthorized)
		return
	}
	// Keep no access, refresh or ID tokens. The local session cannot outlive
	// either its fixed one-hour TTL or the signed ID Token's expiry.
	if !a.now().Before(expires) {
		http.Error(w, "ID Token expired", http.StatusUnauthorized)
		return
	}
	sessionID, err := nowork.NewState()
	if err != nil {
		http.Error(w, "Random source unavailable", 500)
		return
	}
	csrf, err := nowork.NewState()
	if err != nil {
		http.Error(w, "Random source unavailable", 500)
		return
	}
	a.mu.Lock()
	delete(a.sessions, cookieValue(r, sessionCookie))
	if len(a.sessions) >= a.capacity {
		a.mu.Unlock()
		http.Error(w, "Too many sessions; try again later", http.StatusServiceUnavailable)
		return
	}
	a.sessions[sessionID] = session{sub: sub, name: info.Name, csrf: csrf, expires: expires}
	a.mu.Unlock()
	a.setCookie(w, sessionCookie, sessionID, expires)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

var homePage = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>NoWork OIDC example</title></head>
<body><h1>NoWork OIDC example</h1>{{if .Sub}}<p>Signed in as {{.Name}} ({{.Sub}}).</p>
<form method="post" action="/logout"><input type="hidden" name="csrf" value="{{.CSRF}}"><button type="submit">Sign out locally</button></form>
{{else}}<p>You are signed out.</p><a href="/login">Sign in</a>{{end}}</body></html>`))

func (a *app) home(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	s, ok := a.sessions[cookieValue(r, sessionCookie)]
	a.mu.Unlock()
	if !ok || !a.now().Before(s.expires) {
		s = session{}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = homePage.Execute(w, struct{ Sub, Name, CSRF string }{s.sub, s.name, s.csrf})
}

// This ends only the local application session. It does not log the browser out
// of the identity provider and does not send tokens to a logout URL.
func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && origin != a.baseURL {
		http.Error(w, "Invalid logout origin", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil || len(r.PostForm["csrf"]) != 1 {
		http.Error(w, "Invalid logout request", http.StatusForbidden)
		return
	}
	id := cookieValue(r, sessionCookie)
	a.mu.Lock()
	s, ok := a.sessions[id]
	if !ok || !a.now().Before(s.expires) || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(s.csrf)) != 1 {
		a.mu.Unlock()
		http.Error(w, "Invalid logout request", http.StatusForbidden)
		return
	}
	delete(a.sessions, id)
	delete(a.attempts, cookieValue(r, attemptCookie))
	a.mu.Unlock()
	a.clearCookie(w, sessionCookie)
	a.clearCookie(w, attemptCookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) cleanup() {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	for id, p := range a.attempts {
		if !now.Before(p.expires) {
			delete(a.attempts, id)
		}
	}
	for id, s := range a.sessions {
		if !now.Before(s.expires) {
			delete(a.sessions, id)
		}
	}
}

func (a *app) cleanUntil(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			a.cleanup()
		}
	}
}

func main() {
	base := os.Getenv("APP_BASE_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	a, err := newApp(config{issuer: os.Getenv("NOWORK_ISSUER"), clientID: os.Getenv("NOWORK_CLIENT_ID"), clientSecret: os.Getenv("NOWORK_CLIENT_SECRET"), baseURL: base})
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go a.cleanUntil(ctx)
	srv := &http.Server{Addr: "127.0.0.1:8080", Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	log.Print("OIDC example listening on 127.0.0.1:8080; open APP_BASE_URL in your browser")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
