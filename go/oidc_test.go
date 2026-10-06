package nowork

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestOIDCProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/userinfo" {
			if r.Header.Get("Authorization") != "Bearer access" {
				t.Error("missing bearer token")
			}
			io.WriteString(w, `{"sub":"subject","uid":100001,"security_level":2,"game_roles":[]}`)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.PostForm.Get("client_id") != "client" || r.PostForm.Get("client_secret") != "secret" {
			t.Error("missing client authentication")
		}
		switch r.URL.Path {
		case "/oauth2/token":
			if r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("code_verifier") != "verifier" {
				t.Error("invalid code exchange")
			}
			io.WriteString(w, `{"access_token":"access","token_type":"Bearer","expires_in":3600}`)
		case "/oauth2/introspect":
			io.WriteString(w, `{"active":true,"sub":"subject","scope":"openid security_level"}`)
		case "/oauth2/revoke":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	o := NewOIDC(server.URL, "client", "secret", "https://app.example/callback")
	redirect, err := url.Parse(o.AuthorizeURL("state + &", "nonce", "openid security_level", "challenge"))
	if err != nil || redirect.Query().Get("state") != "state + &" || redirect.Query().Get("code_challenge_method") != "S256" {
		t.Fatal("invalid authorization URL")
	}
	token, err := o.Exchange(context.Background(), "code", "verifier")
	if err != nil || token.AccessToken != "access" {
		t.Fatalf("exchange: %v, %v", token, err)
	}
	user, err := o.UserInfo(context.Background(), token.AccessToken)
	if err != nil || user.SecurityLevel != 2 {
		t.Fatalf("userinfo: %v, %v", user, err)
	}
	info, err := o.Introspect(context.Background(), token.AccessToken)
	if err != nil || !info.Active {
		t.Fatalf("introspection: %v, %v", info, err)
	}
	if err := o.Revoke(context.Background(), token.AccessToken); err != nil {
		t.Fatal(err)
	}
}

func TestOAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"expired code"}`)
	}))
	defer server.Close()
	_, err := NewOIDC(server.URL, "client", "secret", "").Exchange(context.Background(), "expired")
	var oauthErr *OAuthError
	if !errors.As(err, &oauthErr) || oauthErr.Code != "invalid_grant" || oauthErr.Status != 400 {
		t.Fatalf("unexpected error: %v", err)
	}
}
