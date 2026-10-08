package nowork

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportTestRoundTripper func(*http.Request) (*http.Response, error)

func (f transportTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type transportTestBody struct {
	io.Reader
	read   int64
	closed bool
}

func (b *transportTestBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += int64(n)
	return n, err
}

func (b *transportTestBody) Close() error { b.closed = true; return nil }

type transportTestSpaces struct{}

func (transportTestSpaces) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = ' '
	}
	return len(p), nil
}

func TestTransportRejectsCredentialScopeBypass(t *testing.T) {
	calls := 0
	h := &http.Client{Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected request")
	})}
	plain := New("https://example.test", WithHTTPClient(h))
	plain.headers = http.Header{"Authorization": {"Bearer fixture-jwt"}}
	account := NewAccount("https://example.test", "fixture-jwt", h).transport
	chat := NewChat("https://example.test", "fixture-chat", h).transport
	for _, client := range []*Client{plain, account, chat} {
		if _, err := client.Whitelist(context.Background()); !errors.Is(err, ErrNotConfigured) {
			t.Fatalf("server API accepted a non-server credential: %v", err)
		}
	}
	if _, err := chat.Raw(context.Background(), http.MethodGet, "/api/account/portal/servers", nil, nil); err == nil {
		t.Fatal("chat token accepted for an account route")
	}
	if _, err := NewAccount("https://example.test", " \t", h).Servers(context.Background()); err == nil {
		t.Fatal("blank account JWT accepted")
	}
	anonymous := NewChat("https://example.test", "", h)
	if _, err := anonymous.Me(context.Background()); err == nil {
		t.Fatal("anonymous chat identity accepted")
	}
	if _, err := anonymous.SiteSession(context.Background()); err == nil {
		t.Fatal("anonymous site-session creation accepted")
	}
	if err := anonymous.EndSession(context.Background()); err == nil {
		t.Fatal("anonymous session deletion accepted")
	}
	if calls != 0 {
		t.Fatalf("invalid credentials reached transport %d times", calls)
	}
}

func TestTransportAllowsAnonymousChatTicketRedemption(t *testing.T) {
	calls := 0
	h := &http.Client{Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat/session" || r.URL.RawQuery != "" {
			t.Error("ticket redemption used the wrong route or exposed a query")
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Server-Key") != "" {
			t.Error("anonymous ticket redemption sent credentials")
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["ticket"] != "fixture-ticket" || body["server"] != "demo" {
			t.Error("ticket redemption body changed")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"token":"fixture-session"}}`))}, nil
	})}
	session, err := NewChat("https://example.test", "", h).RedeemTicket(context.Background(), "fixture-ticket", "demo")
	if err != nil || session == nil || session.Token != "fixture-session" || calls != 1 {
		t.Fatalf("ticket redemption failed: %v; calls=%d", err, calls)
	}
}

func TestTransportResponseCapBoundaryAndClosedBody(t *testing.T) {
	const prefix = `{"code":0,"data":{"private":"do-not-log-response"}}`
	for _, extra := range []int64{0, 1} {
		body := &transportTestBody{Reader: io.MultiReader(strings.NewReader(prefix), io.LimitReader(transportTestSpaces{}, defaultResponseLimit-int64(len(prefix))+extra))}
		h := &http.Client{Transport: transportTestRoundTripper(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body}, nil
		})}
		response, err := New("https://example.test", WithServerKey("fixture-key"), WithHTTPClient(h)).Raw(context.Background(), http.MethodGet, "/server/v1/whitelist", nil, nil)
		if extra == 0 && (err != nil || response == nil) {
			t.Fatalf("response at exact cap rejected: %v", err)
		}
		if extra != 0 {
			var transport *TransportError
			if response != nil || !errors.Is(err, ErrResponseTooLarge) || !errors.As(err, &transport) {
				t.Fatalf("oversized response was not classified: %v", err)
			}
			if strings.Contains(err.Error(), "do-not-log-response") || strings.Contains(err.Error(), "fixture-key") {
				t.Fatal("oversize error exposed response or credentials")
			}
		}
		if !body.closed || body.read > defaultResponseLimit+1 {
			t.Fatalf("body not closed/bounded: closed=%v read=%d", body.closed, body.read)
		}
	}
}

func TestTransportArchiveExportCapIsPerRequest(t *testing.T) {
	const payload = `{"code":0,"data":{"scope":{"account_id":7},"provider":"mihoyo","snapshots":[{"snapshot":{"sections":[{"id":"fixture","fields":[{"key":"zero","value":0},{"key":"withheld","policy":{"exportable":false}}]}]}}],"history":[],"policies":[],"policy_audit":[]}}`
	var bodies []*transportTestBody
	oversize := false
	h := &http.Client{Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer fixture-jwt" || r.Header.Get("X-Server-Key") != "" {
			t.Error("export lost JWT credential isolation")
		}
		length := int64(16<<20) + int64(len(payload))
		if oversize {
			length = archiveExportResponseLimit + 100
		}
		body := &transportTestBody{Reader: io.MultiReader(strings.NewReader(payload), io.LimitReader(transportTestSpaces{}, length-int64(len(payload))))}
		bodies = append(bodies, body)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body}, nil
	})}
	c := NewAccount("https://example.test", "fixture-jwt", h)
	out, err := c.ExportGameArchive(context.Background(), GameProviderMihoyo, 7)
	if err != nil || out == nil || len(out.Snapshots) != 1 {
		t.Fatalf("16 MiB export plus metadata rejected: %v", err)
	}
	fields := out.Snapshots[0].Snapshot.Sections[0].Fields
	if len(fields) != 2 || string(fields[0].Value) != "0" || fields[1].HasValue() || fields[1].Policy.Exportable {
		t.Fatal("export field redaction or observed zero changed")
	}
	if _, err := c.Servers(context.Background()); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("export raised the cap for subsequent account calls: %v", err)
	}
	oversize = true
	if out, err := c.ExportGameArchive(context.Background(), GameProviderMihoyo, 7); out != nil || !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("export cap not enforced: %v", err)
	}
	for i, body := range bodies {
		limit := archiveExportResponseLimit
		if i == 1 {
			limit = defaultResponseLimit
		}
		if !body.closed || body.read > limit+1 {
			t.Fatalf("response %d not closed/bounded: closed=%v read=%d", i, body.closed, body.read)
		}
	}
}

func TestTransportArchivePreservesCustomClientDeadline(t *testing.T) {
	redirectSentinel := errors.New("caller redirect policy")
	h := &http.Client{
		Timeout:       20 * time.Millisecond,
		CheckRedirect: func(*http.Request, []*http.Request) error { return redirectSentinel },
		Transport: transportTestRoundTripper(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}),
	}
	c := NewAccount("https://example.test", "fixture-jwt", h)
	if c.transport.http == h || c.transport.http.Timeout != h.Timeout || h.CheckRedirect(nil, nil) != redirectSentinel {
		t.Fatal("account constructor changed caller HTTP client or timeout")
	}
	if c.transport.http.CheckRedirect(nil, nil) == nil {
		t.Fatal("account redirect protection lost")
	}
	_, err := c.ExportGameArchive(context.Background(), GameProviderMihoyo, 7)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("custom timeout did not survive archive transport: %v", err)
	}
	if strings.Contains(err.Error(), "fixture-jwt") {
		t.Fatal("timeout exposed account token")
	}
	if NewAccount("https://example.test", "fixture-jwt", nil).transport.http.Timeout != 10*time.Second {
		t.Fatal("default timeout changed")
	}
}
