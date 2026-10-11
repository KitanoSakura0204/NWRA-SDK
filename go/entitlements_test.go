package nowork

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const entitlementRoute = "/server/v1/delivery/entitlements"

func entServer(t *testing.T, answers map[string]string, seen *[]string) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*seen = append(*seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(body)+" key="+r.Header.Get("X-Server-Key"))
		w.Header().Set("Content-Type", "application/json")
		a, ok := answers[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, a)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, WithServerKey("test-key")), srv
}

func TestEntitlementRoutes(t *testing.T) {
	var seen []string
	c, _ := entServer(t, map[string]string{
		"GET " + entitlementRoute:    `{"code":0,"data":{"sub":"pair-1","entitlements":{"vip":{"tier":3},"beta":true}}}`,
		"PUT " + entitlementRoute:    `{"code":0,"data":{"sub":"pair-1","key":"vip","value":{"tier":4}}}`,
		"DELETE " + entitlementRoute: `{"code":0,"data":{"sub":"pair-1","key":"vip"}}`,
	}, &seen)
	ctx := context.Background()

	all, err := c.Entitlements(ctx, 100001, "")
	if err != nil || all.Sub != "pair-1" || string(all.Entitlements["vip"]) != `{"tier":3}` || string(all.Entitlements["beta"]) != "true" {
		t.Fatalf("read: %#v %v", all, err)
	}
	if seen[0] != "GET "+entitlementRoute+"?uid=100001  key=test-key" {
		t.Fatalf("uid query: %q", seen[0])
	}

	// A sub wins over a uid: it is the only form a pairwise profile accepts.
	all, err = c.Entitlements(ctx, 100001, " pair-1 ")
	if err != nil || seen[1] != "GET "+entitlementRoute+"?sub=pair-1  key=test-key" {
		t.Fatalf("sub query: %q %v", seen[1], err)
	}
	_ = all

	put, err := c.PutEntitlement(ctx, 0, "pair-1", "vip", map[string]int{"tier": 4})
	if err != nil || put.Key != "vip" || string(put.Value) != `{"tier":4}` {
		t.Fatalf("put: %#v %v", put, err)
	}
	if seen[2] != `PUT `+entitlementRoute+`? {"key":"vip","sub":"pair-1","value":{"tier":4}} key=test-key` {
		t.Fatalf("put body: %q", seen[2])
	}

	del, err := c.DeleteEntitlement(ctx, 0, "pair-1", "vip")
	if err != nil || del.Key != "vip" {
		t.Fatalf("delete: %#v %v", del, err)
	}
	if seen[3] != "DELETE "+entitlementRoute+"?key=vip&sub=pair-1  key=test-key" {
		t.Fatalf("delete query: %q", seen[3])
	}
}

func TestEntitlementRefusesIncompleteTargets(t *testing.T) {
	var seen []string
	c, _ := entServer(t, nil, &seen)
	ctx := context.Background()

	if _, err := c.Entitlements(ctx, 0, "  "); err == nil {
		t.Fatal("expected a missing uid/sub to be refused")
	}
	if _, err := c.PutEntitlement(ctx, 0, "", "", "v"); err == nil {
		t.Fatal("expected a missing key to be refused")
	}
	if _, err := c.DeleteEntitlement(ctx, 7, "", "  "); err == nil {
		t.Fatal("expected a blank key to be refused")
	}
	if len(seen) != 0 {
		t.Fatalf("no request may be sent: %q", seen)
	}
}

func TestEntitlementBusinessErrors(t *testing.T) {
	var seen []string
	c, _ := entServer(t, map[string]string{
		"GET " + entitlementRoute: `{"code":1,"message":"成对标识的投递配置只能用 sub 指定用户"}`,
	}, &seen)
	_, err := c.Entitlements(context.Background(), 100001, "")
	var api *ApiError
	if !errors.As(err, &api) || api.Code != 1 {
		t.Fatalf("expected ApiError code 1, got %v", err)
	}
}

func TestOAuthEntitlementsUseBasicAuth(t *testing.T) {
	var seen []string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		auth = r.Header.Get("Authorization")
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(body))
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"code":0,"data":{"sub":"pair-1","entitlements":{"vip":1}}}`)
		case http.MethodPut:
			_, _ = io.WriteString(w, `{"code":0,"data":{"sub":"pair-1","key":"vip","value":1}}`)
		case http.MethodDelete:
			_, _ = io.WriteString(w, `{"code":401,"message":"invalid client credentials"}`)
		}
	}))
	t.Cleanup(srv.Close)

	oidc := NewOIDC(srv.URL, "lc_app", "secret", "https://app.example/cb")
	ctx := context.Background()

	all, err := oidc.Entitlements(ctx, 0, "pair-1")
	if err != nil || all.Sub != "pair-1" || string(all.Entitlements["vip"]) != "1" {
		t.Fatalf("read: %#v %v", all, err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("lc_app:secret"))
	if auth != want {
		t.Fatalf("basic auth: %q want %q", auth, want)
	}
	if _, err := oidc.PutEntitlement(ctx, 0, "pair-1", "vip", 1); err != nil {
		t.Fatal(err)
	}
	_, err = oidc.DeleteEntitlement(ctx, 0, "pair-1", "vip")
	var api *ApiError
	if !errors.As(err, &api) || api.Code != 401 {
		t.Fatalf("expected ApiError 401, got %v", err)
	}
	if seen[1] != `PUT /oauth2/delivery/entitlements? {"key":"vip","sub":"pair-1","value":1}` {
		t.Fatalf("put body: %q", seen[1])
	}
	if seen[2] != "DELETE /oauth2/delivery/entitlements?key=vip&sub=pair-1 " {
		t.Fatalf("delete query: %q", seen[2])
	}
}

func TestOAuthEntitlementsRequireConfiguration(t *testing.T) {
	oidc := NewOIDC("https://account.example", "lc_app", "", "https://app.example/cb")
	if _, err := oidc.Entitlements(context.Background(), 0, ""); err == nil {
		t.Fatal("expected a missing uid/sub to be refused before sending")
	}
}
