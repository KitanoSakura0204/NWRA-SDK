package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	lounge "github.com/wangework/lounge-sdk-go"
)

// Join policy when the user center cannot be reached (network, timeout, 5xx). A reply that
// came encrypted and could not be opened is never an outage: it always rejects (fail closed).
const (
	outageCache = "cache" // let in players the last full sync listed; everyone when the whitelist was off
	outageDeny  = "deny"
	outageAllow = "allow"
)

// gate answers "may this player join?" with the Server API, a local copy of the whitelist and,
// when the server encrypts its replies (DELIVERY.md section 5), the decryption keys.
type gate struct {
	api    *lounge.Client
	code   string // server code, checked against the envelope ctx "server:<code>:<route>"
	keys   []*lounge.DecryptionKey
	outage string

	mu     sync.RWMutex
	synced bool
	mode   int
	byName map[string]lounge.WhitelistEntry // lower-case name
	byUUID map[string]lounge.WhitelistEntry // lower-case uuid
}

// verdict is the outcome of one join check.
type verdict struct {
	Allowed bool
	Reason  string
	Message string // extra text from the user center, e.g. the reason of a per-server ban
	Player  *lounge.WhitelistEntry
	Source  string // "live", "cache" or "policy"
}

// refresh pulls the full whitelist (start-up and every few minutes) into the local copy.
func (g *gate) refresh(ctx context.Context) (int, error) {
	var wl lounge.Whitelist
	if err := g.get(ctx, "whitelist", nil, &wl); err != nil {
		return 0, err
	}
	byName := make(map[string]lounge.WhitelistEntry, len(wl.Entries))
	byUUID := make(map[string]lounge.WhitelistEntry, len(wl.Entries))
	for _, e := range wl.Entries {
		if e.Name != "" {
			byName[strings.ToLower(e.Name)] = e
		}
		if e.UUID != "" {
			byUUID[strings.ToLower(e.UUID)] = e
		}
	}
	g.mu.Lock()
	g.synced, g.mode, g.byName, g.byUUID = true, wl.Mode, byName, byUUID
	g.mu.Unlock()
	return len(wl.Entries), nil
}

// check runs the live join check and falls back to the outage policy on a transport failure.
func (g *gate) check(ctx context.Context, name, uuid string) (verdict, error) {
	q := url.Values{}
	if name != "" {
		q.Set("name", name)
	}
	if uuid != "" {
		q.Set("uuid", uuid)
	}
	var res lounge.CheckResult
	err := g.get(ctx, "whitelist/check", q, &res)
	if err == nil {
		return verdict{Allowed: res.Allowed, Reason: res.Reason, Message: res.Message, Player: res.Player, Source: "live"}, nil
	}
	var enc *lounge.EncryptionError
	var api *lounge.ApiError
	switch {
	case errors.As(err, &enc):
		return verdict{Reason: "cannot read the encrypted reply", Source: "policy"}, err
	case errors.As(err, &api):
		// 401 = wrong server key, 403 = server disabled or not approved: a configuration problem
		return verdict{Reason: "user center refused the check", Source: "policy"}, err
	case errors.Is(err, lounge.ErrNotConfigured):
		return verdict{Reason: "server key missing", Source: "policy"}, err
	}
	return g.fallback(name, uuid), err
}

func (g *gate) fallback(name, uuid string) verdict {
	switch g.outage {
	case outageAllow:
		return verdict{Allowed: true, Reason: "user center unreachable", Source: "policy"}
	case outageDeny:
		return verdict{Reason: "user center unreachable", Source: "policy"}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if !g.synced {
		return verdict{Reason: "user center unreachable and no whitelist cached yet", Source: "policy"}
	}
	if g.mode == 0 {
		return verdict{Allowed: true, Reason: "whitelist disabled", Source: "cache"}
	}
	e, ok := g.byUUID[strings.ToLower(uuid)]
	if !ok || uuid == "" {
		e, ok = g.byName[strings.ToLower(name)]
	}
	if !ok {
		return verdict{Reason: "not bound (cached whitelist)", Source: "cache"}
	}
	return verdict{Allowed: true, Player: &e, Source: "cache"}
}

// get calls a whitelist route and decodes its data. With decryption keys the reply must come
// encrypted (a plain one is refused: no downgrade) and its ctx must name this server and route;
// without keys an encrypted reply is refused too. Both refusals are *lounge.EncryptionError.
func (g *gate) get(ctx context.Context, route string, q url.Values, out any) error {
	res, err := g.api.Raw(ctx, http.MethodGet, "/server/v1/"+route, q, nil)
	if err != nil {
		return err
	}
	data := []byte(res.Data)
	switch {
	case res.Encrypted() && len(g.keys) == 0:
		return &lounge.EncryptionError{Route: route, Reason: "no decryption key configured (LOUNGE_DECRYPTION_KEYS)"}
	case res.Encrypted():
		var env struct {
			JWE string `json:"jwe"`
		}
		if json.Unmarshal(res.Data, &env) != nil || env.JWE == "" {
			return &lounge.EncryptionError{Route: route, Reason: "encrypted reply without a JWE"}
		}
		plain, hdr, err := lounge.DecryptJWE(env.JWE, g.keys...)
		if err != nil {
			return &lounge.EncryptionError{Route: route, Reason: "the reply did not open", Err: err}
		}
		if got, _ := hdr["ctx"].(string); !ctxMatches(got, g.code, route) {
			return &lounge.EncryptionError{Route: route, Reason: "envelope ctx names another server or route: " + got}
		}
		data = plain
	case len(g.keys) > 0:
		return &lounge.EncryptionError{Route: route, Reason: "plain reply while decryption keys are configured"}
	}
	if len(data) == 0 || string(data) == "null" {
		return &lounge.TransportError{Err: fmt.Errorf("invalid response (%d): missing data", res.Status)}
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &lounge.TransportError{Err: err}
	}
	return nil
}

// ctxMatches checks the envelope ctx "server:<code>:<route>". Without a configured server code
// (LOUNGE_SERVER_CODE) only the kind and the route are checked.
func ctxMatches(ctx, code, route string) bool {
	if code != "" {
		return ctx == "server:"+code+":"+route
	}
	return strings.HasPrefix(ctx, "server:") && strings.HasSuffix(ctx, ":"+route) && len(ctx) > len("server:")+len(route)+1
}
