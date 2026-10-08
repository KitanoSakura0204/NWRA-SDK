package nowork

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExchangeGMCodeContractAndDenial(t *testing.T) {
	for _, denied := range []bool{false, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/server/v1/gm/sso/exchange" || r.Header.Get("X-Server-Key") != "test-key" {
				t.Errorf("unexpected exchange request: %s %s", r.Method, r.URL.Path)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["code"] != "single-use" || body["codeVerifier"] != "private-verifier" || body["redirectUri"] != "https://gm.example/callback" {
				t.Errorf("bad exchange body: %#v, %v", body, err)
			}
			if denied {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"code":403,"message":"invalid authorization"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"serverId":2,"serverCode":"s1","subject":"gm-role:7","character":{"id":7,"roleName":"Alex","roleUuid":"abc"},"authenticatedAt":123,"scope":"character:login"}}`))
		}))
		c := New(s.URL, WithServerKey("test-key"))
		result, err := c.ExchangeGMCode(context.Background(), "single-use", "private-verifier", "https://gm.example/callback")
		s.Close()
		if denied {
			var api *ApiError
			if !errors.As(err, &api) || api.Code != 403 || result != nil {
				t.Fatalf("denial became a usable session: %+v, %v", result, err)
			}
		} else if err != nil || result.Character.ID != 7 || result.Scope != "character:login" {
			t.Fatalf("exchange: %+v, %v", result, err)
		}
	}
}

func TestLoginCallbackRejectsAmbiguousState(t *testing.T) {
	for _, callback := range []string{"https://gm.example/cb?code=x", "https://gm.example/cb?code=x&state=s&state=evil", "https://gm.example/cb?code=x&code=y&state=s"} {
		if _, err := ParseLoginCallback(callback, "s", ""); err == nil {
			t.Errorf("accepted ambiguous callback %s", callback)
		}
	}
	if _, err := ParseLoginCallback("https://gm.example/cb?code=x", "", ""); err == nil {
		t.Fatal("accepted a callback without a bound state")
	}
}

func TestCatalogNilPreservesActionsAndEmptyClears(t *testing.T) {
	var bodies []map[string]json.RawMessage
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		_, _ = w.Write([]byte(`{"code":0,"data":{"count":0}}`))
	}))
	defer s.Close()
	c := New(s.URL, WithServerKey("key"))
	for _, actions := range [][]GMAction{nil, {}} {
		if _, err := c.PublishGMCatalog(context.Background(), actions, map[string][]GMOption{}); err != nil {
			t.Fatal(err)
		}
	}
	if bodies[0]["actions"] != nil || string(bodies[1]["actions"]) != "[]" {
		t.Fatalf("catalog lost preserve/clear semantics: %v", bodies)
	}
}
