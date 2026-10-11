package nowork

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
)

// ---- Custom per-app values (module "delivery", SDK/docs/DELIVERY.md section 8) ----
//
// A game server reads and writes them through Client (X-Server-Key) on
// /server/v1/delivery/entitlements; a confidential OAuth client through OIDC (HTTP Basic)
// on /oauth2/delivery/entitlements. The caller must be bound to a delivery profile, and the
// user is named by uid or by the sub that caller sees. A pairwise profile accepts sub only:
// a uid would link its pseudonym back to the account.

// EntitlementValues is the GET reply: the sub this caller sees for the user plus every key
// stored for it (values are the raw JSON that was written).
type EntitlementValues struct {
	Sub          string                     `json:"sub"`
	Entitlements map[string]json.RawMessage `json:"entitlements"`
}

// Entitlement is the PUT / DELETE reply for a single key.
type Entitlement struct {
	Sub   string          `json:"sub"`
	Key   string          `json:"key,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}

// entitlementQuery names the user by sub first (the only form a pairwise profile accepts),
// then by uid. An empty sub and a non-positive uid are refused before any request is sent.
func entitlementQuery(uid int64, sub string) (url.Values, error) {
	if s := strings.TrimSpace(sub); s != "" {
		return url.Values{"sub": {s}}, nil
	}
	if uid > 0 {
		return url.Values{"uid": {strconv.FormatInt(uid, 10)}}, nil
	}
	return nil, errors.New("nowork: provide a uid or a sub")
}

// entitlementBody is the PUT body. value may be any JSON-marshalable value, including nil.
func entitlementBody(uid int64, sub, key string, value any) (map[string]any, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("nowork: entitlement key is required")
	}
	body := map[string]any{"key": key, "value": value}
	if s := strings.TrimSpace(sub); s != "" {
		body["sub"] = s
		return body, nil
	}
	if uid > 0 {
		body["uid"] = uid
		return body, nil
	}
	return nil, errors.New("nowork: provide a uid or a sub")
}

// Entitlements reads every custom value this server stored for a user.
func (c *Client) Entitlements(ctx context.Context, uid int64, sub string) (*EntitlementValues, error) {
	q, err := entitlementQuery(uid, sub)
	if err != nil {
		return nil, err
	}
	var out EntitlementValues
	if err := c.do(ctx, http.MethodGet, "/server/v1/delivery/entitlements", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutEntitlement stores one key for a user and returns what the backend stored. The key
// must start with a letter and hold only letters, digits and ". _ -" (at most 64 bytes);
// the value may be any JSON up to 4096 bytes, and a profile holds at most 50 keys per user.
func (c *Client) PutEntitlement(ctx context.Context, uid int64, sub, key string, value any) (*Entitlement, error) {
	body, err := entitlementBody(uid, sub, key, value)
	if err != nil {
		return nil, err
	}
	var out Entitlement
	if err := c.do(ctx, http.MethodPut, "/server/v1/delivery/entitlements", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEntitlement removes one key. Removing a key that was never written is not an error.
func (c *Client) DeleteEntitlement(ctx context.Context, uid int64, sub, key string) (*Entitlement, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("nowork: entitlement key is required")
	}
	q, err := entitlementQuery(uid, sub)
	if err != nil {
		return nil, err
	}
	q.Set("key", key)
	var out Entitlement
	if err := c.do(ctx, http.MethodDelete, "/server/v1/delivery/entitlements", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Entitlements reads every custom value this OAuth client stored for a user.
func (o *OIDC) Entitlements(ctx context.Context, uid int64, sub string) (*EntitlementValues, error) {
	q, err := entitlementQuery(uid, sub)
	if err != nil {
		return nil, err
	}
	var out EntitlementValues
	if err := o.deliveryCall(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PutEntitlement stores one key through the OAuth client (confidential clients only).
func (o *OIDC) PutEntitlement(ctx context.Context, uid int64, sub, key string, value any) (*Entitlement, error) {
	body, err := entitlementBody(uid, sub, key, value)
	if err != nil {
		return nil, err
	}
	var out Entitlement
	if err := o.deliveryCall(ctx, http.MethodPut, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteEntitlement removes one key through the OAuth client.
func (o *OIDC) DeleteEntitlement(ctx context.Context, uid int64, sub, key string) (*Entitlement, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("nowork: entitlement key is required")
	}
	q, err := entitlementQuery(uid, sub)
	if err != nil {
		return nil, err
	}
	q.Set("key", key)
	var out Entitlement
	if err := o.deliveryCall(ctx, http.MethodDelete, q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// deliveryCall talks to /oauth2/delivery/entitlements with HTTP Basic client credentials.
// It follows the same envelope and error split as the Server API: code != 0 is an *ApiError
// (401 invalid client credentials stays one), transport trouble a *TransportError.
func (o *OIDC) deliveryCall(ctx context.Context, method string, query url.Values, body any, out any) error {
	target := o.Issuer + "/oauth2/delivery/entitlements"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return &TransportError{err}
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return &TransportError{err}
	}
	req.SetBasicAuth(o.ClientID, o.ClientSecret)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := o.http.Do(req)
	if err != nil {
		return &TransportError{err}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return &TransportError{fmt.Errorf("http %d: invalid envelope", res.StatusCode)}
	}
	if envelope.Code != 0 {
		return &ApiError{Code: envelope.Code, Message: envelope.Message}
	}
	if res.StatusCode >= 400 {
		return &TransportError{fmt.Errorf("http %d", res.StatusCode)}
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return &TransportError{fmt.Errorf("invalid response (%d): missing data", res.StatusCode)}
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return &TransportError{err}
	}
	return nil
}
