// Package lounge is the Go SDK for the Lounge Account user center:
// game-server whitelist API and OpenID Connect helpers. Standard library only.
package lounge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ApiError is a business error returned by the user center (code != 0).
type ApiError struct {
	Code    int
	Message string
}

func (e *ApiError) Error() string { return fmt.Sprintf("lounge: api error %d: %s", e.Code, e.Message) }

// TransportError wraps network / decoding failures.
type TransportError struct{ Err error }

func (e *TransportError) Error() string { return "lounge: transport error: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

// Client talks to the Server API with a server key.
type Client struct {
	base      string
	serverKey string
	http      *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithServerKey sets the X-Server-Key issued by the admin console.
func WithServerKey(key string) Option { return func(c *Client) { c.serverKey = key } }

// WithHTTPClient replaces the default http.Client (10s timeout).
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// New creates a Server API client for the given base URL (the user-center backend / issuer).
func New(base string, opts ...Option) *Client {
	c := &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 10 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}

// WhitelistEntry is one approved binding.
type WhitelistEntry struct {
	Name          string `json:"name"`
	UUID          string `json:"uuid"`
	UID           int64  `json:"uid"`
	Nickname      string `json:"nickname"`
	SecurityLevel int    `json:"securityLevel"`
	Since         string `json:"since"`
}

// Whitelist is the full list for a server.
type Whitelist struct {
	Server  string           `json:"server"`
	Mode    int              `json:"mode"`
	Count   int              `json:"count"`
	Entries []WhitelistEntry `json:"entries"`
}

// CheckResult answers a join check.
type CheckResult struct {
	Allowed bool            `json:"allowed"`
	Bound   bool            `json:"bound"`
	Reason  string          `json:"reason"`
	Player  *WhitelistEntry `json:"player"`
}

// Heartbeat is the live status payload.
type Heartbeat struct {
	Online  int    `json:"online"`
	Max     int    `json:"max,omitempty"`
	Version string `json:"version,omitempty"`
}

type envelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	if c.serverKey == "" {
		return ErrNotConfigured
	}
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &TransportError{err}
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return &TransportError{err}
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.serverKey != "" {
		req.Header.Set("X-Server-Key", c.serverKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return &TransportError{err}
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return &TransportError{err}
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &TransportError{fmt.Errorf("decode (%d): %w", res.StatusCode, err)}
	}
	if env.Code == nil {
		return &TransportError{fmt.Errorf("invalid response (%d): missing code", res.StatusCode)}
	}
	if *env.Code != 0 {
		return &ApiError{Code: *env.Code, Message: env.Message}
	}
	if res.StatusCode >= 400 {
		return &TransportError{fmt.Errorf("http %d", res.StatusCode)}
	}
	if out != nil {
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return &TransportError{fmt.Errorf("invalid response (%d): missing data", res.StatusCode)}
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return &TransportError{err}
		}
	}
	return nil
}

// Whitelist fetches every approved binding of this server.
func (c *Client) Whitelist(ctx context.Context) (*Whitelist, error) {
	var out Whitelist
	if err := c.do(ctx, http.MethodGet, "/server/v1/whitelist", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CheckWhitelist validates a joining player by name and/or uuid.
func (c *Client) CheckWhitelist(ctx context.Context, name, uuid string) (*CheckResult, error) {
	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	if uuid != "" {
		q.Set("uuid", uuid)
	}
	var out CheckResult
	if err := c.do(ctx, http.MethodGet, "/server/v1/whitelist/check", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Heartbeat reports live status (call roughly every 60s).
func (c *Client) Heartbeat(ctx context.Context, hb Heartbeat) error {
	return c.do(ctx, http.MethodPost, "/server/v1/heartbeat", nil, hb, nil)
}

// IsNotAllowed reports whether err/result means the player must be rejected.
func IsNotAllowed(res *CheckResult, err error) bool {
	if err != nil {
		return false // transport failure: let the caller decide (fail-open vs fail-closed)
	}
	return res != nil && !res.Allowed
}

// ErrNotConfigured is returned when the server key is missing.
var ErrNotConfigured = errors.New("lounge: server key not configured")

// ---- GM module (server-scoped feature module "server-gm") ----

// GMOption is one choice of a select parameter or of a named option list.
type GMOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// GMActionParam describes one input of a GM action. Only Name / Type / Required / Label
// existed before 2026-09-28; every other field is optional (see PROTOCOL.md section 2.4).
type GMActionParam struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"` // string | text | number | boolean | select | player
	Required    bool            `json:"required"`
	Label       string          `json:"label"`
	Description string          `json:"description,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
	Step        *float64        `json:"step,omitempty"`
	Integer     bool            `json:"integer,omitempty"`
	MaxLength   int             `json:"maxLength,omitempty"`
	Options     []GMOption      `json:"options,omitempty"`
	// Source names an option list published with PublishGMCatalog / PublishGMOptions.
	Source string `json:"source,omitempty"`
}

// GMAction is one operator action the plugin offers to the GM panel.
type GMAction struct {
	Key         string            `json:"key"` // no colon, no whitespace, at most 64 bytes
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Params      []GMActionParam   `json:"params,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Category    string            `json:"category,omitempty"` // catalogue group
	Danger      string            `json:"danger,omitempty"`   // normal | confirm | critical
}

// GMCommand is a queued action the GM panel wants the plugin to run.
type GMCommand struct {
	ID       int64           `json:"id"`
	Action   string          `json:"action"`
	Params   json.RawMessage `json:"params"`
	Status   string          `json:"status"`
	Source   string          `json:"source,omitempty"`   // admin | owner | grant
	Operator string          `json:"operator,omitempty"` // "staff" for admin commands on owner-run servers
}

// GMPublishResult reports what a publish stored; entries it could not accept are skipped
// (never a failure for the whole call).
type GMPublishResult struct {
	Count   int `json:"count"`
	Skipped []struct {
		Key    string `json:"key"`
		Reason string `json:"reason"`
	} `json:"skipped"`
}

// Modules returns the feature modules enabled for this server with their configs.
func (c *Client) Modules(ctx context.Context) (map[string]json.RawMessage, error) {
	var out struct {
		Modules map[string]json.RawMessage `json:"modules"`
	}
	if err := c.do(ctx, http.MethodGet, "/server/v1/modules", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.Modules, nil
}

// PublishGMActions replaces the action catalogue the admin panel shows for this server.
// Call it on start-up and whenever the set of actions changes.
func (c *Client) PublishGMActions(ctx context.Context, actions []GMAction) error {
	return c.do(ctx, http.MethodPut, "/server/v1/gm/actions", nil, actions, nil)
}

// PublishGMCatalog replaces the actions and the named option lists (select / suggestion
// sources, e.g. worlds or kits) in one call and reports the skipped entries. A nil options
// map keeps the stored lists.
func (c *Client) PublishGMCatalog(ctx context.Context, actions []GMAction, options map[string][]GMOption) (*GMPublishResult, error) {
	body := map[string]any{"actions": actions}
	if options != nil {
		body["options"] = options
	}
	var out GMPublishResult
	if err := c.do(ctx, http.MethodPut, "/server/v1/gm/actions", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PublishGMOptions replaces only the named option lists (the published actions stay); call
// it when a list changes, e.g. a world was loaded.
func (c *Client) PublishGMOptions(ctx context.Context, options map[string][]GMOption) (*GMPublishResult, error) {
	var out GMPublishResult
	if err := c.do(ctx, http.MethodPut, "/server/v1/gm/options", nil, options, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PollGMCommands fetches queued commands (each is marked sent). Poll alongside the
// heartbeat; commands not fetched within 5 minutes expire.
func (c *Client) PollGMCommands(ctx context.Context) ([]GMCommand, error) {
	var out []GMCommand
	if err := c.do(ctx, http.MethodGet, "/server/v1/gm/commands", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ReportGMResult tells the panel how a command went; result is free text (<= 4000 chars).
func (c *Client) ReportGMResult(ctx context.Context, id int64, ok bool, result string) error {
	body := map[string]any{"ok": ok, "result": result}
	return c.do(ctx, http.MethodPost, "/server/v1/gm/commands/"+strconv.FormatInt(id, 10)+"/result", nil, body, nil)
}
