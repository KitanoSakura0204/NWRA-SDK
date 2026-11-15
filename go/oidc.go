package nowork

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthError is an RFC 6749 error returned by the token/introspection endpoints.
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
	Status      int    `json:"-"`
}

func (e *OAuthError) Error() string {
	return fmt.Sprintf("nowork: oauth error %s: %s", e.Code, e.Description)
}

// OIDC is a minimal OpenID Connect relying-party helper for the user center.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	http         *http.Client
}

// NewOIDC creates an OIDC helper. ClientSecret may be empty for public (PKCE) clients.
func NewOIDC(issuer, clientID, clientSecret, redirectURI string) *OIDC {
	return &OIDC{Issuer: strings.TrimRight(issuer, "/"), ClientID: clientID, ClientSecret: clientSecret, RedirectURI: redirectURI,
		http: &http.Client{Timeout: 10 * time.Second}}
}

// Discover reads the configured issuer's /.well-known/openid-configuration without
// credentials. The issuer must match exactly and the four required endpoint fields must
// be nonempty strings. Unknown fields are preserved; advertised endpoints are not followed.
func (o *OIDC) Discover(ctx context.Context) (map[string]any, error) {
	_, metadata, err := o.publicJSON(ctx, "/.well-known/openid-configuration")
	if err != nil {
		return nil, err
	}
	for _, field := range []string{"issuer", "authorization_endpoint", "token_endpoint", "jwks_uri"} {
		value, ok := metadata[field].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return nil, &TransportError{fmt.Errorf("incomplete OIDC discovery: invalid %s", field)}
		}
	}
	if metadata["issuer"] != o.Issuer {
		return nil, &TransportError{fmt.Errorf("OIDC discovery issuer mismatch")}
	}
	return metadata, nil
}

// JWKS reads the configured issuer's /jwks.json without credentials. It requires keys
// to be an array of objects and preserves the complete response bytes and unknown fields.
func (o *OIDC) JWKS(ctx context.Context) ([]byte, error) {
	raw, metadata, err := o.publicJSON(ctx, "/jwks.json")
	if err != nil {
		return nil, err
	}
	keys, ok := metadata["keys"].([]any)
	if !ok {
		return nil, &TransportError{fmt.Errorf("invalid JWKS: keys must be an array of objects")}
	}
	for _, key := range keys {
		if _, ok := key.(map[string]any); !ok {
			return nil, &TransportError{fmt.Errorf("invalid JWKS: keys must be an array of objects")}
		}
	}
	return raw, nil
}

// publicJSON bounds unauthenticated metadata reads to 1 MiB, retaining the configured
// HTTP client's transport and timeout. Redirects and cookies cannot change the issuer or
// turn a public metadata request into an authenticated one.
func (o *OIDC) publicJSON(ctx context.Context, path string) ([]byte, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Issuer+path, nil)
	if err != nil {
		return nil, nil, &TransportError{err}
	}
	if req.URL.User != nil || req.URL.RawQuery != "" || req.URL.Fragment != "" {
		return nil, nil, &TransportError{fmt.Errorf("OIDC issuer must not contain credentials, a query or a fragment")}
	}
	req.Header.Set("Accept", "application/json")
	h := *o.http
	h.Jar = nil
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := h.Do(req)
	if err != nil {
		return nil, nil, &TransportError{err}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, nil, &TransportError{fmt.Errorf("http %d", res.StatusCode)}
	}
	const limit = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, nil, &TransportError{err}
	}
	if len(raw) > limit {
		return nil, nil, &TransportError{fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, limit)}
	}
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, nil, &TransportError{err}
	}
	if metadata == nil {
		return nil, nil, &TransportError{fmt.Errorf("OIDC metadata must be a JSON object")}
	}
	return raw, metadata, nil
}

// Token is the token endpoint response.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
}

// UserInfo is the userinfo response.
type UserInfo struct {
	Sub                 string     `json:"sub"`
	UID                 int64      `json:"uid"`
	Name                string     `json:"name"`
	Nickname            string     `json:"nickname"`
	PreferredUsername   string     `json:"preferred_username"`
	Picture             string     `json:"picture"`
	Alias               string     `json:"alias"`
	Email               string     `json:"email"`
	EmailVerified       bool       `json:"email_verified"`
	PhoneNumber         string     `json:"phone_number"`
	PhoneNumberVerified bool       `json:"phone_number_verified"`
	SecurityLevel       int        `json:"security_level"`
	RealnameVerified    bool       `json:"realname_verified"`
	GameRoles           []GameRole `json:"game_roles"`
}

// GameRole is a bound in-game identity released with the game_roles scope.
type GameRole struct {
	Server     string `json:"server"`
	ServerName string `json:"server_name"`
	Name       string `json:"name"`
	UUID       string `json:"uuid"`
}

// Introspection is the RFC 7662 response.
type Introspection struct {
	Active    bool   `json:"active"`
	TokenType string `json:"token_type"`
	ClientID  string `json:"client_id"`
	Scope     string `json:"scope"`
	Sub       string `json:"sub"`
	UID       int64  `json:"uid"`
	Username  string `json:"username"`
	Exp       int64  `json:"exp"`
	Iat       int64  `json:"iat"`
}

// AuthorizeURL builds the authorization request. codeChallenge may be empty (confidential clients).
func (o *OIDC) AuthorizeURL(state, nonce, scope string, codeChallenge ...string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", o.ClientID)
	q.Set("redirect_uri", o.RedirectURI)
	q.Set("scope", scope)
	q.Set("state", state)
	if nonce != "" {
		q.Set("nonce", nonce)
	}
	if len(codeChallenge) > 0 && codeChallenge[0] != "" {
		q.Set("code_challenge", codeChallenge[0])
		q.Set("code_challenge_method", "S256")
	}
	return o.Issuer + "/oauth2/authorize?" + q.Encode()
}

func (o *OIDC) postForm(ctx context.Context, path string, form url.Values, out any) error {
	form.Set("client_id", o.ClientID)
	if o.ClientSecret != "" {
		form.Set("client_secret", o.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Issuer+path, strings.NewReader(form.Encode()))
	if err != nil {
		return &TransportError{err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := o.http.Do(req)
	if err != nil {
		return &TransportError{err}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 400 {
		var oe OAuthError
		if json.Unmarshal(raw, &oe) == nil && oe.Code != "" {
			oe.Status = res.StatusCode
			return &oe
		}
		return &TransportError{fmt.Errorf("http %d: %s", res.StatusCode, string(raw))}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return &TransportError{err}
		}
	}
	return nil
}

// Exchange trades an authorization code for tokens. codeVerifier is required when PKCE was used.
func (o *OIDC) Exchange(ctx context.Context, code string, codeVerifier ...string) (*Token, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {o.RedirectURI}}
	if len(codeVerifier) > 0 && codeVerifier[0] != "" {
		form.Set("code_verifier", codeVerifier[0])
	}
	var t Token
	if err := o.postForm(ctx, "/oauth2/token", form, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Refresh obtains a new token pair from a refresh token (the old refresh token is rotated).
func (o *OIDC) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	var t Token
	if err := o.postForm(ctx, "/oauth2/token", url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ClientCredentials obtains a machine token (confidential clients only).
func (o *OIDC) ClientCredentials(ctx context.Context, scope string) (*Token, error) {
	var t Token
	if err := o.postForm(ctx, "/oauth2/token", url.Values{"grant_type": {"client_credentials"}, "scope": {scope}}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// UserInfo fetches the profile for an access token.
func (o *OIDC) UserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Issuer+"/oauth2/userinfo", nil)
	if err != nil {
		return nil, &TransportError{err}
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := o.http.Do(req)
	if err != nil {
		return nil, &TransportError{err}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 400 {
		var oe OAuthError
		if json.Unmarshal(raw, &oe) == nil && oe.Code != "" {
			oe.Status = res.StatusCode
			return nil, &oe
		}
		return nil, &TransportError{fmt.Errorf("http %d", res.StatusCode)}
	}
	var u UserInfo
	if err := json.Unmarshal(raw, &u); err != nil {
		return nil, &TransportError{err}
	}
	return &u, nil
}

// Introspect validates an access or refresh token server-side.
func (o *OIDC) Introspect(ctx context.Context, token string) (*Introspection, error) {
	var out Introspection
	if err := o.postForm(ctx, "/oauth2/introspect", url.Values{"token": {token}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke invalidates a token.
func (o *OIDC) Revoke(ctx context.Context, token string) error {
	return o.postForm(ctx, "/oauth2/revoke", url.Values{"token": {token}}, nil)
}

// LogoutURL builds the RP-initiated logout URL.
func (o *OIDC) LogoutURL(idToken, postLogoutRedirectURI, state string) string {
	q := url.Values{"client_id": {o.ClientID}}
	if idToken != "" {
		q.Set("id_token_hint", idToken)
	}
	if postLogoutRedirectURI != "" {
		q.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	}
	if state != "" {
		q.Set("state", state)
	}
	return o.Issuer + "/oauth2/logout?" + q.Encode()
}
