package nowork

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerProtocol(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Server-Key") != "test-key" {
			t.Error("missing server authentication header")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/server/v1/whitelist":
			io.WriteString(w, `{"code":0,"data":{"server":"survival","mode":1,"count":1,"entries":[{"name":"Steve","uid":100001,"nickname":"Player","securityLevel":2,"since":"2026-09-13 10:00:00"}]}}`)
		case "/server/v1/whitelist/check":
			if r.URL.Query().Get("name") != "Player & One" {
				t.Error("player query was not escaped correctly")
			}
			io.WriteString(w, `{"code":0,"data":{"allowed":true,"bound":false,"reason":"whitelist disabled"}}`)
		case "/server/v1/heartbeat":
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
				t.Error("invalid heartbeat request")
			}
			io.WriteString(w, `{"code":0,"data":{"server":"survival","status":"ok"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := New(server.URL+"/", WithServerKey("test-key"))
	wl, err := c.Whitelist(context.Background())
	if err != nil || wl.Count != 1 || wl.Entries[0].SecurityLevel != 2 || wl.Entries[0].UUID != "" {
		t.Fatalf("whitelist: %#v, %v", wl, err)
	}
	check, err := c.CheckWhitelist(context.Background(), "Player & One", "")
	if err != nil || !check.Allowed || check.Bound || check.Player != nil {
		t.Fatalf("check: %#v, %v", check, err)
	}
	if err := c.Heartbeat(context.Background(), Heartbeat{Online: 2, Max: 20}); err != nil {
		t.Fatal(err)
	}
}

func TestServerRejectsMalformedSuccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"missing code", 200, `{"data":{"allowed":true}}`},
		{"missing data", 200, `{"code":0}`},
		{"null data", 200, `{"code":0,"data":null}`},
		{"HTTP failure", 503, `{"code":0,"data":{"allowed":true}}`},
		{"HTML response", 502, `<html>upstream unavailable</html>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				io.WriteString(w, tc.body)
			}))
			defer server.Close()
			_, err := New(server.URL, WithServerKey("test-key")).CheckWhitelist(context.Background(), "Steve", "")
			var transport *TransportError
			if !errors.As(err, &transport) {
				t.Fatalf("expected TransportError, got %v", err)
			}
		})
	}
}

func TestServerBusinessErrorAndMissingKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"code":401,"message":"invalid server key"}`)
	}))
	defer server.Close()
	_, err := New(server.URL, WithServerKey("invalid-key")).Whitelist(context.Background())
	var apiErr *ApiError
	if !errors.As(err, &apiErr) || apiErr.Code != 401 {
		t.Fatalf("expected 401 ApiError, got %v", err)
	}
	_, err = New(server.URL).Whitelist(context.Background())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestEncryptedReplyFailsClosed(t *testing.T) {
	const sealed = `{"code":0,"message":"ok","data":{"jwe":"aGVhZA.a2V5.aXY.Y2lwaGVy.dGFn"}}`
	for _, header := range []string{"JWE", ""} { // a proxy may drop the header: the body shape still counts
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if header != "" {
				w.Header().Set(HeaderEncryption, header)
			}
			io.WriteString(w, sealed)
		}))
		c := New(server.URL, WithServerKey("test-key"))
		wl, err := c.Whitelist(context.Background())
		var enc *EncryptionError
		if !errors.As(err, &enc) || enc.Route != "whitelist" || wl != nil {
			t.Fatalf("header %q: whitelist must fail closed, got %#v, %v", header, wl, err)
		}
		check, err := c.CheckWhitelist(context.Background(), "Steve", "")
		if !errors.As(err, &enc) || enc.Route != "whitelist/check" || !IsNotAllowed(check, err) {
			t.Fatalf("header %q: check must fail closed, got %#v, %v", header, check, err)
		}
		if _, err := c.PollGMCommands(context.Background()); !errors.As(err, &enc) {
			t.Fatalf("header %q: gm/commands must fail closed, got %v", header, err)
		}
		res, err := c.Raw(context.Background(), http.MethodGet, "/server/v1/whitelist", nil, nil)
		if err != nil || !res.Encrypted() || string(res.Data) != `{"jwe":"aGVhZA.a2V5.aXY.Y2lwaGVy.dGFn"}` || string(res.Body) != sealed {
			t.Fatalf("header %q: raw: %#v, %v", header, res, err)
		}
		server.Close()
	}
}

func TestPlainDataWithJWEFieldIsNotEncrypted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":0,"data":{"jwe":"x","mode":1,"count":0,"entries":[]}}`)
	}))
	defer server.Close()
	wl, err := New(server.URL, WithServerKey("k")).Whitelist(context.Background())
	if err != nil || wl.Mode != 1 {
		t.Fatalf("got %#v, %v", wl, err)
	}
}

func TestHeartbeatOptionalFields(t *testing.T) {
	var got []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, string(b))
		io.WriteString(w, `{"code":0,"data":{"status":"ok"}}`)
	}))
	defer server.Close()
	c := New(server.URL, WithServerKey("k"))
	players := []OnlinePlayer{{Name: "Steve", UUID: "069a79f4-44e9-4726-a5be-fca90e38aaf5"}, {Name: "Alex"}}
	for _, hb := range []Heartbeat{
		{Online: 2, Max: 20, Version: "1.21.1", TPS: 19.8, Players: players},
		{Online: 0, TPS: math.NaN()},
		{Online: 0, TPS: math.Inf(1)},
	} {
		if err := c.Heartbeat(context.Background(), hb); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		`{"online":2,"max":20,"version":"1.21.1","tps":19.8,"players":[{"name":"Steve","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5"},{"name":"Alex"}]}`,
		`{"online":0}`,
		`{"online":0}`,
	}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Fatalf("heartbeat %d: got %v, want %s", i, got, want[i])
		}
	}
}
