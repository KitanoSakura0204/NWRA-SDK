package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	nowork "github.com/wangework/nowork-sdk-go"
)

// loginConfig is the game-client side of the SDK login page (PROTOCOL.md section 6). The OAuth
// client is a public one (no secret): PKCE S256 protects the code.
type loginConfig struct {
	frontend string // user-center web address (BASE when the backend serves the SPA)
	issuer   string // BASE
	server   string // server code
	clientID string
	redirect string // registered loopback redirect, e.g. http://127.0.0.1:47823/cb; "" = code-display
	locale   string
}

// login opens (prints) the login page URL and waits for the code: on the loopback redirect, or
// typed by the player in code-display mode. Then it exchanges the code and reads userinfo.
func login(ctx context.Context, c loginConfig) error {
	pkce, err := nowork.NewPKCE()
	if err != nil {
		return err
	}
	state, err := nowork.NewState()
	if err != nil {
		return err
	}
	page, err := nowork.BuildLoginURL(c.frontend, nowork.LoginURLParams{
		ServerCode: c.server, RedirectURI: c.redirect, State: state, CodeChallenge: pkce.Challenge,
		Scope: "openid profile", UILocales: c.locale,
	})
	if err != nil {
		return err
	}
	fmt.Println("Open this page in a browser and sign in:")
	fmt.Println("  " + page)

	var code string
	if c.redirect == "" {
		code, err = typedCode()
	} else {
		code, err = awaitRedirect(ctx, c.redirect, state, c.issuer)
	}
	if err != nil {
		return err
	}
	oidc := nowork.NewOIDC(c.issuer, c.clientID, "", c.redirect)
	tok, err := oidc.Exchange(ctx, code, pkce.Verifier)
	if err != nil {
		return fmt.Errorf("token exchange: %w", err)
	}
	info, err := oidc.UserInfo(ctx, tok.AccessToken)
	if err != nil {
		return fmt.Errorf("userinfo: %w", err)
	}
	fmt.Printf("Signed in: sub=%s uid=%d nickname=%q trust level %d\n", info.Sub, info.UID, info.Nickname, info.SecurityLevel)
	return nil
}

// typedCode reads a code-display code ("ABCD-2345") from the terminal.
func typedCode() (string, error) {
	fmt.Print("Login code shown on the page: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	code := nowork.NormalizeLoginCode(line)
	if code == "" {
		return "", errors.New("no code entered")
	}
	return code, nil
}

// awaitRedirect serves the loopback redirect URI once and returns the checked code.
func awaitRedirect(ctx context.Context, redirect, state, issuer string) (string, error) {
	u, err := url.Parse(redirect)
	if err != nil || u.Scheme != "http" {
		return "", fmt.Errorf("NOWORK_REDIRECT_URI %q: only a loopback http address can be served here", redirect)
	}
	if h := u.Hostname(); h != "127.0.0.1" && h != "localhost" && h != "::1" {
		return "", fmt.Errorf("NOWORK_REDIRECT_URI %q: host must be loopback", redirect)
	}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return "", err
	}
	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	path := u.Path
	if path == "" {
		path = "/"
	}
	srv := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		code, err := nowork.ParseLoginCallback(r.URL.String(), state, issuer)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if err != nil {
			fmt.Fprintln(w, "Login failed, return to the game.")
		} else {
			fmt.Fprintln(w, "Signed in, return to the game.")
		}
		select {
		case done <- result{code, err}:
		default:
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute) // the code itself expires after 300 s
	defer cancel()
	select {
	case r := <-done:
		return r.code, r.err
	case <-ctx.Done():
		return "", errors.New("no login within 5 minutes")
	}
}

// loginFromEnv reads the login settings; "" redirect selects code-display mode.
func loginFromEnv(base string) loginConfig {
	return loginConfig{
		frontend: strings.TrimRight(envOr("NOWORK_FRONTEND", base), "/"),
		issuer:   base,
		server:   os.Getenv("NOWORK_SERVER_CODE"),
		clientID: os.Getenv("NOWORK_CLIENT_ID"),
		redirect: os.Getenv("NOWORK_REDIRECT_URI"),
		locale:   os.Getenv("NOWORK_UI_LOCALES"),
	}
}
