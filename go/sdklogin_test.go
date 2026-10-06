package nowork

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestPKCE(t *testing.T) {
	// RFC 7636 Appendix B test vector.
	if got := S256Challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"); got != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("S256Challenge = %q", got)
	}
	p, err := NewPKCE()
	if err != nil || len(p.Verifier) != 43 || p.Challenge != S256Challenge(p.Verifier) {
		t.Fatalf("NewPKCE = %+v, %v", p, err)
	}
	if q, _ := NewPKCE(); q.Verifier == p.Verifier {
		t.Fatal("NewPKCE repeated a verifier")
	}
}

func TestBuildLoginURL(t *testing.T) {
	raw, err := BuildLoginURL("https://account.example.com/", LoginURLParams{ServerCode: "s 1", RedirectURI: "mygame://auth", State: "a&b", CodeChallenge: "ch", UILocales: "en"})
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	q := u.Query()
	if u.Host != "account.example.com" || u.EscapedPath() != "/sdk/login/s%201" || q.Get("redirect_uri") != "mygame://auth" || q.Get("state") != "a&b" ||
		q.Get("code_challenge") != "ch" || q.Get("code_challenge_method") != "S256" || q.Get("ui_locales") != "en" || q.Has("scope") {
		t.Fatalf("login URL = %s", raw)
	}
	if _, err := BuildLoginURL("https://a", LoginURLParams{ServerCode: "s1"}); err == nil {
		t.Fatal("missing code_challenge accepted")
	}
	if _, err := BuildLoginURL("https://a", LoginURLParams{CodeChallenge: "ch"}); err == nil {
		t.Fatal("missing server code accepted")
	}
}

func TestParseLoginCallback(t *testing.T) {
	code, err := ParseLoginCallback("mygame://auth?code=abc&state=st&iss=https%3A%2F%2Fid.example", "st", "https://id.example")
	if err != nil || code != "abc" {
		t.Fatalf("success: %q %v", code, err)
	}
	if _, err := ParseLoginCallback("mygame://auth?code=abc&state=other", "st", ""); err == nil {
		t.Fatal("state mismatch accepted")
	}
	if _, err := ParseLoginCallback("mygame://auth?code=abc&state=st&iss=https%3A%2F%2Fevil", "st", "https://id.example"); err == nil {
		t.Fatal("issuer mismatch accepted")
	}
	_, err = ParseLoginCallback("mygame://auth?error=access_denied&error_description=no&state=st", "st", "")
	var oe *OAuthError
	if !errors.As(err, &oe) || oe.Code != "access_denied" {
		t.Fatalf("error response: %v", err)
	}
	if got := NormalizeLoginCode(" abcd-2345 "); got != "ABCD2345" || strings.Contains(got, "-") {
		t.Fatalf("NormalizeLoginCode = %q", got)
	}
}
