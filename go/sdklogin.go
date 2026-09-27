package lounge

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

// PKCE is a code verifier and its S256 challenge (RFC 7636). Keep Verifier on the client;
// only Challenge goes on the login URL.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE returns a fresh 43-character verifier and its S256 challenge.
func NewPKCE() (PKCE, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return PKCE{}, err
	}
	v := base64.RawURLEncoding.EncodeToString(b)
	return PKCE{Verifier: v, Challenge: S256Challenge(v)}, nil
}

// S256Challenge is BASE64URL(SHA256(verifier)) without padding.
func S256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// NewState returns a random state value for one login attempt.
func NewState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// LoginURLParams describes one SDK login page request (PROTOCOL.md §6).
type LoginURLParams struct {
	ServerCode    string // game server code, e.g. "survival"
	RedirectURI   string // exact redirect URI registered for the server (optional in code-display mode)
	State         string // echoed back unchanged (optional in code-display mode)
	CodeChallenge string // S256 challenge, required
	Scope         string // optional; defaults to the server's configured scopes
	Nonce         string // optional; copied into the ID token
	UILocales     string // optional: zh-CN / zh-TW / en / ja
}

// BuildLoginURL returns the URL a game client opens to sign a player in. frontend is the
// user-centre web address (the SPA origin, which is BASE when the backend serves the SPA).
func BuildLoginURL(frontend string, p LoginURLParams) (string, error) {
	if p.ServerCode == "" {
		return "", errors.New("lounge: ServerCode is required")
	}
	if p.CodeChallenge == "" {
		return "", errors.New("lounge: CodeChallenge is required (PKCE S256)")
	}
	q := url.Values{}
	for k, v := range map[string]string{"redirect_uri": p.RedirectURI, "state": p.State, "scope": p.Scope, "nonce": p.Nonce, "ui_locales": p.UILocales} {
		if v != "" {
			q.Set(k, v)
		}
	}
	q.Set("code_challenge", p.CodeChallenge)
	q.Set("code_challenge_method", "S256")
	return strings.TrimRight(frontend, "/") + "/sdk/login/" + url.PathEscape(p.ServerCode) + "?" + q.Encode(), nil
}

// ParseLoginCallback reads the redirect a login page sent to the client's redirect URI. It
// checks state (and iss when wantIssuer is set, RFC 9207) and returns the authorization code;
// an error response comes back as *OAuthError.
func ParseLoginCallback(callbackURL, wantState, wantIssuer string) (string, error) {
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if q.Get("state") != wantState {
		return "", errors.New("lounge: state mismatch")
	}
	if wantIssuer != "" && q.Get("iss") != "" && q.Get("iss") != wantIssuer {
		return "", errors.New("lounge: issuer mismatch")
	}
	if e := q.Get("error"); e != "" {
		return "", &OAuthError{Code: e, Description: q.Get("error_description")}
	}
	code := q.Get("code")
	if code == "" {
		return "", errors.New("lounge: callback has no code")
	}
	return code, nil
}

// NormalizeLoginCode cleans a code-display code the player typed ("abcd-2345" → "ABCD2345").
// Use it for code-display mode only; codes from redirects and postMessage are case-sensitive.
func NormalizeLoginCode(s string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "", "\t", "").Replace(strings.TrimSpace(s)))
}
