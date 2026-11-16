package nowork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"
)

func oidcDiscoveryJSON(t *testing.T, issuer string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"issuer":                 issuer,
		"authorization_endpoint": "https://advertised.example/authorize",
		"token_endpoint":         "https://advertised.example/token",
		"jwks_uri":               "https://advertised.example/keys",
		"extension":              map[string]any{"preserved": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestOIDCPublicMetadataRequests(t *testing.T) {
	const issuer = "https://account.example/tenant"
	discovery := oidcDiscoveryJSON(t, issuer)
	const jwks = `{"keys":[{"kty":"RSA","kid":"one","extension":{"preserved":true}}],"extra":true}`
	o := NewOIDC(issuer+"///", "client", "client-secret", "https://app.example/callback")
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(issuer)
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "secret", Path: "/"}})
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request-context")
	var paths []string
	h := &http.Client{Timeout: 7 * time.Second, Jar: jar, Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		paths = append(paths, r.URL.Path)
		if r.Method != http.MethodGet || r.URL.RawQuery != "" || r.Body != nil || r.URL.User != nil {
			t.Errorf("not a credential-free GET: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Accept") != "application/json" || len(r.Header) != 1 {
			t.Errorf("unexpected headers: %v", r.Header)
		}
		if r.Context().Value(contextKey{}) != "request-context" {
			t.Error("request context lost")
		}
		body := discovery
		if r.URL.Path == "/tenant/jwks.json" {
			body = jwks
		} else if r.URL.Path != "/tenant/.well-known/openid-configuration" {
			t.Errorf("followed advertised endpoint: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	o.http = h
	metadata, err := o.Discover(ctx)
	if err != nil || metadata["issuer"] != issuer || metadata["extension"].(map[string]any)["preserved"] != true {
		t.Fatalf("discovery: %v %v", metadata, err)
	}
	keys, err := o.JWKS(ctx)
	if err != nil || string(keys) != jwks {
		t.Fatalf("JWKS fields or bytes changed: %s %v", keys, err)
	}
	if len(paths) != 2 || o.http != h || h.Timeout != 7*time.Second || h.Jar != jar || h.CheckRedirect != nil {
		t.Fatal("HTTP client configuration changed or extra endpoints requested")
	}
}

func TestOIDCDiscoveryRejectsInvalidMetadata(t *testing.T) {
	const issuer = "https://account.example"
	valid := oidcDiscoveryJSON(t, issuer)
	cases := map[string]string{
		"malformed": `{`, "array": `[]`, "null": `null`, "scalar": `"metadata"`,
		"trailing JSON": valid + ` {}`, "wrong issuer": oidcDiscoveryJSON(t, issuer+"/"),
	}
	for _, field := range []string{"issuer", "authorization_endpoint", "token_endpoint", "jwks_uri"} {
		for i, value := range []any{nil, "", "   ", 42, []any{}} {
			var metadata map[string]any
			_ = json.Unmarshal([]byte(valid), &metadata)
			metadata[field] = value
			raw, _ := json.Marshal(metadata)
			cases[fmt.Sprintf("%s invalid %d", field, i)] = string(raw)
		}
		var metadata map[string]any
		_ = json.Unmarshal([]byte(valid), &metadata)
		delete(metadata, field)
		raw, _ := json.Marshal(metadata)
		cases[field+" missing"] = string(raw)
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			o := oidcMetadataClient(body, 200)
			metadata, err := o.Discover(context.Background())
			var transport *TransportError
			if metadata != nil || !errors.As(err, &transport) {
				t.Fatalf("accepted invalid discovery: %v %v", metadata, err)
			}
		})
	}
}

func TestOIDCJWKSRejectsInvalidMetadata(t *testing.T) {
	for _, body := range []string{`{`, `[]`, `null`, `42`, `{}`, `{"keys":null}`, `{"keys":{}}`, `{"keys":[null]}`, `{"keys":[42]}`, `{"keys":[[]]}`, `{"keys":[{}]} {}`} {
		t.Run(body, func(t *testing.T) {
			raw, err := oidcMetadataClient(body, 200).JWKS(context.Background())
			var transport *TransportError
			if raw != nil || !errors.As(err, &transport) {
				t.Fatalf("accepted invalid JWKS: %s %v", raw, err)
			}
		})
	}
	if raw, err := oidcMetadataClient(`{"keys":[]}`, 200).JWKS(context.Background()); err != nil || raw == nil {
		t.Fatalf("empty JWKS rejected: %s %v", raw, err)
	}
}

func TestOIDCMetadataRejectsInvalidUTF8(t *testing.T) {
	for name, invalid := range map[string]string{
		"invalid byte":       "\xff",
		"truncated sequence": "\xe4\xb8",
		"overlong sequence":  "\xc0\xaf",
		"encoded surrogate":  "\xed\xa0\x80",
	} {
		t.Run("Discover/"+name, func(t *testing.T) {
			body := strings.TrimSuffix(oidcDiscoveryJSON(t, "https://account.example"), "}") + `,"unknown":"` + invalid + `"}`
			metadata, err := oidcMetadataClient(body, 200).Discover(context.Background())
			var transport *TransportError
			if metadata != nil || !errors.As(err, &transport) {
				t.Fatalf("accepted invalid UTF-8 discovery extension: %v %v", metadata, err)
			}
		})
		t.Run("JWKS/"+name, func(t *testing.T) {
			body := `{"keys":[{"kty":"RSA","unknown":"` + invalid + `"}]}`
			raw, err := oidcMetadataClient(body, 200).JWKS(context.Background())
			var transport *TransportError
			if raw != nil || !errors.As(err, &transport) {
				t.Fatalf("accepted invalid UTF-8 JWK extension: %x %v", raw, err)
			}
		})
	}
}

func TestOIDCMetadataPreservesUnicode(t *testing.T) {
	const value = "旅行者🙂�"
	discovery := strings.TrimSuffix(oidcDiscoveryJSON(t, "https://account.example"), "}") + `,"扩展":"` + value + `"}`
	metadata, err := oidcMetadataClient(discovery, 200).Discover(context.Background())
	if err != nil || metadata["扩展"] != value {
		t.Fatalf("valid Unicode discovery extension changed: %v %v", metadata, err)
	}
	jwks := `{"keys":[{"kty":"RSA","扩展":"` + value + `"}],"扩展":"` + value + `"}`
	raw, err := oidcMetadataClient(jwks, 200).JWKS(context.Background())
	if err != nil || string(raw) != jwks {
		t.Fatalf("valid Unicode JWKS bytes changed: %s %v", raw, err)
	}
}

func oidcMetadataClient(body string, status int) *OIDC {
	o := NewOIDC("https://account.example", "client", "secret", "")
	o.http.Transport = transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	return o
}

type oidcMetadataBody struct {
	io.Reader
	read   int
	closed bool
}

func (b *oidcMetadataBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}

func (b *oidcMetadataBody) Close() error { b.closed = true; return nil }

func TestOIDCMetadataSizeAndStatus(t *testing.T) {
	for _, discovery := range []bool{true, false} {
		name, valid := "JWKS", `{"keys":[]}`
		if discovery {
			name, valid = "Discover", oidcDiscoveryJSON(t, "https://account.example")
		}
		for _, size := range []int{(1 << 20) - 1, 1 << 20, (1 << 20) + 1, (1 << 20) + 10000} {
			t.Run(fmt.Sprintf("%s size %d", name, size), func(t *testing.T) {
				body := &oidcMetadataBody{Reader: strings.NewReader(valid + strings.Repeat(" ", size-len(valid)))}
				o := NewOIDC("https://account.example", "", "", "")
				o.http.Transport = transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Body: body}, nil
				})
				var err error
				if discovery {
					_, err = o.Discover(context.Background())
				} else {
					_, err = o.JWKS(context.Background())
				}
				if (err != nil) != (size > 1<<20) || body.read > (1<<20)+1 || !body.closed {
					t.Fatalf("size=%d read=%d closed=%v err=%v", size, body.read, body.closed, err)
				}
				if size > 1<<20 && !errors.Is(err, ErrResponseTooLarge) {
					t.Fatalf("size error did not preserve ErrResponseTooLarge: %v", err)
				}
			})
		}
		for _, status := range []int{199, 299, 302, 400, 500} {
			o := oidcMetadataClient(valid, status)
			var err error
			if discovery {
				_, err = o.Discover(context.Background())
			} else {
				_, err = o.JWKS(context.Background())
			}
			if (err == nil) != (status == 299) {
				t.Errorf("%s status %d: %v", name, status, err)
			}
		}
	}
}

type oidcMetadataErrorReader struct{ err error }

func (r oidcMetadataErrorReader) Read([]byte) (int, error) { return 0, r.err }

func TestOIDCMetadataRejectsReadErrorsAndInvalidIssuer(t *testing.T) {
	for _, discovery := range []bool{true, false} {
		valid := `{"keys":[]}`
		if discovery {
			valid = oidcDiscoveryJSON(t, "https://account.example")
		}
		readErr := errors.New("metadata body read failed")
		body := &oidcMetadataBody{Reader: io.MultiReader(strings.NewReader(valid), oidcMetadataErrorReader{readErr})}
		o := NewOIDC("https://account.example", "", "secret", "")
		calls := 0
		o.http.Transport = transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 200, Body: body}, nil
		})
		var err error
		if discovery {
			_, err = o.Discover(context.Background())
		} else {
			_, err = o.JWKS(context.Background())
		}
		if !errors.Is(err, readErr) || !body.closed {
			t.Fatalf("body error ignored or body not closed: %v", err)
		}
		for _, issuer := range []string{"https://client:secret@account.example", "https://account.example?client_secret=secret", "https://account.example#fragment"} {
			o.Issuer = issuer
			if discovery {
				_, err = o.Discover(context.Background())
			} else {
				_, err = o.JWKS(context.Background())
			}
			if err == nil || calls != 1 {
				t.Fatalf("invalid issuer caused a request: calls=%d err=%v", calls, err)
			}
		}
	}
}

func TestOIDCMetadataRedirectAndTimeout(t *testing.T) {
	for _, discovery := range []bool{true, false} {
		o := NewOIDC("https://account.example", "", "secret", "")
		calls := 0
		o.http.Transport = transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://other.example/metadata"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		var err error
		if discovery {
			_, err = o.Discover(context.Background())
		} else {
			_, err = o.JWKS(context.Background())
		}
		if err == nil || calls != 1 {
			t.Fatalf("followed redirect: calls=%d err=%v", calls, err)
		}
		o.http.Timeout = 10 * time.Millisecond
		o.http.Transport = transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		})
		if discovery {
			_, err = o.Discover(context.Background())
		} else {
			_, err = o.JWKS(context.Background())
		}
		var transport *TransportError
		if !errors.As(err, &transport) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout lost: %v", err)
		}
	}
}
