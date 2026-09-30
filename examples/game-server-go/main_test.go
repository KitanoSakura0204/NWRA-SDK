package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	lounge "github.com/wangework/lounge-sdk-go"
)

func quiet(string, ...any) {}

// fakeCenter answers the whitelist routes with fixed bodies (no network beyond loopback).
func fakeCenter(t *testing.T, header string, bodies map[string]string) *lounge.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := bodies[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if header != "" {
			w.Header().Set(lounge.HeaderEncryption, header)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return lounge.New(srv.URL, lounge.WithServerKey("lsk_test"))
}

const plainList = `{"code":0,"data":{"server":"survival","mode":1,"count":1,"entries":[{"name":"Steve","uuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","uid":100001}]}}`

func TestPlainWhitelistAndOutageCache(t *testing.T) {
	api := fakeCenter(t, "", map[string]string{"/server/v1/whitelist": plainList})
	g := &gate{api: api, outage: outageCache}
	if n, err := g.refresh(context.Background()); err != nil || n != 1 {
		t.Fatalf("refresh: %d %v", n, err)
	}
	// the join-check route answers 502: the cached copy decides
	v, err := g.check(context.Background(), "steve", "")
	if err == nil || !v.Allowed || v.Source != "cache" || v.Player.UID != 100001 {
		t.Fatalf("cached Steve: %+v %v", v, err)
	}
	if v, _ := g.check(context.Background(), "Alex", ""); v.Allowed {
		t.Fatalf("Alex is not cached: %+v", v)
	}
	g.outage = outageDeny
	if v, _ := g.check(context.Background(), "Steve", ""); v.Allowed {
		t.Fatalf("deny policy let Steve in: %+v", v)
	}
}

func TestEncryptedRepliesFailClosed(t *testing.T) {
	sealed := `{"code":0,"data":{"jwe":"a.b.c.d.e"}}`
	api := fakeCenter(t, "JWE", map[string]string{"/server/v1/whitelist/check": sealed})
	g := &gate{api: api, outage: outageAllow} // even the allow policy must not let this through
	v, err := g.check(context.Background(), "Steve", "")
	var enc *lounge.EncryptionError
	if !errors.As(err, &enc) || v.Allowed {
		t.Fatalf("no keys: %+v %v", v, err)
	}
}

type vectorFile struct {
	Keys    map[string]json.RawMessage `json:"keys"`
	Vectors []struct {
		Name     string            `json:"name"`
		Response string            `json:"response"`
		Body     string            `json:"body"`
		Secret   string            `json:"secret"`
		Headers  map[string]string `json:"headers"`
	} `json:"vectors"`
}

func vectors(t *testing.T) vectorFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "go", "testdata", "vectors.json"))
	if err != nil {
		t.Skip("SDK test vectors not found (example copied out of the repo)")
	}
	var vf vectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatal(err)
	}
	return vf
}

func vector(t *testing.T, vf vectorFile, name string) (response, body, secret string, headers map[string]string) {
	t.Helper()
	for _, v := range vf.Vectors {
		if v.Name == name {
			return v.Response, v.Body, v.Secret, v.Headers
		}
	}
	t.Fatalf("vector %s missing", name)
	return "", "", "", nil
}

func key(t *testing.T, vf vectorFile, name string) *lounge.DecryptionKey {
	t.Helper()
	k, err := lounge.ParseDecryptionKey(vf.Keys[name])
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestEncryptedWhitelistOpensWithTheVectorKey(t *testing.T) {
	vf := vectors(t)
	response, _, _, _ := vector(t, vf, "server-envelope") // ctx server:survival:whitelist
	ec := key(t, vf, "ec")
	api := fakeCenter(t, "JWE", map[string]string{"/server/v1/whitelist": response, "/server/v1/whitelist/check": response})

	g := &gate{api: api, code: "survival", keys: []*lounge.DecryptionKey{ec}, outage: outageCache}
	if n, err := g.refresh(context.Background()); err != nil || n != 1 {
		t.Fatalf("refresh: %d %v", n, err)
	}
	var enc *lounge.EncryptionError
	// the same ciphertext served on the join check names another route
	if v, err := g.check(context.Background(), "Steve", ""); !errors.As(err, &enc) || v.Allowed {
		t.Fatalf("route swap: %+v %v", v, err)
	}
	// another server code
	g2 := &gate{api: api, code: "creative", keys: []*lounge.DecryptionKey{ec}}
	if _, err := g2.refresh(context.Background()); !errors.As(err, &enc) {
		t.Fatalf("server swap: %v", err)
	}
	// wrong key
	g3 := &gate{api: api, keys: []*lounge.DecryptionKey{key(t, vf, "x25519")}}
	if _, err := g3.refresh(context.Background()); !errors.As(err, &enc) {
		t.Fatalf("wrong key: %v", err)
	}
	// keys configured, plain reply: refused (no downgrade)
	plain := &gate{api: fakeCenter(t, "", map[string]string{"/server/v1/whitelist": plainList}), keys: []*lounge.DecryptionKey{ec}}
	if _, err := plain.refresh(context.Background()); !errors.As(err, &enc) {
		t.Fatalf("downgrade: %v", err)
	}
}

func TestGMCommands(t *testing.T) {
	w := newWorld(2, quiet)
	if err := w.add(player{Name: "Steve"}); err != nil {
		t.Fatal(err)
	}
	var said []string
	say := func(m string) { said = append(said, m) }
	run := func(action, params string) (string, error) {
		return runGM(w, lounge.GMCommand{ID: 1, Action: action, Params: json.RawMessage(params)}, say)
	}
	if _, err := run("broadcast", `{"message":"hi","repeat":2}`); err != nil || len(said) != 2 {
		t.Fatalf("broadcast: %v %v", err, said)
	}
	if _, err := run("time", `{"world":"nether","value":"day"}`); err != nil || w.clock["nether"] != "day" {
		t.Fatalf("time: %v %v", err, w.clock)
	}
	if _, err := run("time", `{"world":"moon","value":"day"}`); err == nil {
		t.Fatal("unknown world accepted")
	}
	if _, err := run("kick", `{"player":"steve"}`); err != nil || w.count() != 0 {
		t.Fatalf("kick: %v, %d online", err, w.count())
	}
	if _, err := run("kick", `{"player":"Steve"}`); err == nil {
		t.Fatal("kicking an offline player succeeded")
	}
	if _, err := run("fly", `{}`); err == nil {
		t.Fatal("unknown action succeeded")
	}
	// the catalogue references only option lists it publishes
	lists := worldOptions(w)
	for _, a := range catalog() {
		for _, p := range a.Params {
			if p.Source != "" && lists[p.Source] == nil {
				t.Fatalf("%s.%s uses unpublished list %q", a.Key, p.Name, p.Source)
			}
		}
	}
}

func post(h *hooks, body string, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodPost, "/lounge/webhook", strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func signed(secret, body string) map[string]string {
	ts := time.Now().Unix()
	return map[string]string{"X-Lounge-Timestamp": strconv.FormatInt(ts, 10), "X-Lounge-Signature": lounge.WebhookSignature(secret, ts, []byte(body))}
}

func TestWebhookKicksFrozenAccounts(t *testing.T) {
	w := newWorld(0, quiet)
	_ = w.add(player{Name: "Steve", UID: 100001})
	_ = w.add(player{Name: "Alex", UID: 100002})
	resynced := 0
	h := &hooks{secret: "whsec_test", tolerance: time.Minute, world: w, logf: quiet, seen: map[string]time.Time{}, resync: func() { resynced++ }}

	unfreeze := `{"id":"e1","event":"user.frozen","timestamp":1,"data":{"uid":100001,"frozen":false}}`
	if code := post(h, unfreeze, signed(h.secret, unfreeze)); code != http.StatusOK || w.count() != 2 {
		t.Fatalf("unfreeze: %d, %d online", code, w.count())
	}
	freeze := `{"id":"e2","event":"user.frozen","timestamp":2,"data":{"uid":100001,"frozen":true}}`
	if code := post(h, freeze, signed(h.secret, freeze)); code != http.StatusOK || w.count() != 1 {
		t.Fatalf("freeze: %d, %d online", code, w.count())
	}
	if code := post(h, freeze, map[string]string{"X-Lounge-Timestamp": "1", "X-Lounge-Signature": "sha256=00"}); code != http.StatusUnauthorized {
		t.Fatalf("bad signature: %d", code)
	}
	review := `{"id":"e3","event":"game_role.reviewed","timestamp":3,"data":{"roleId":1,"status":1}}`
	post(h, review, signed(h.secret, review))
	post(h, review, signed(h.secret, review)) // duplicate id: acknowledged, not applied again
	if resynced != 1 {
		t.Fatalf("resync ran %d times", resynced)
	}
}

func TestEncryptedWebhook(t *testing.T) {
	vf := vectors(t)
	_, body, secret, headers := vector(t, vf, "webhook-envelope")
	x := key(t, vf, "x25519")
	mk := func(keys ...*lounge.DecryptionKey) *hooks {
		// the vector's timestamp is fixed, so the replay window is off (tolerance 0)
		return &hooks{secret: secret, keys: keys, world: newWorld(0, quiet), logf: quiet, seen: map[string]time.Time{}, resync: func() {}}
	}
	if code := post(mk(x), body, headers); code != http.StatusOK {
		t.Fatalf("with key: %d", code)
	}
	if code := post(mk(), body, headers); code != http.StatusInternalServerError {
		t.Fatalf("without key: %d", code)
	}
	plain := `{"id":"e9","event":"ping","timestamp":1,"data":{}}`
	if code := post(mk(x), plain, signed(secret, plain)); code != http.StatusInternalServerError {
		t.Fatalf("plain while keys are set: %d", code)
	}
}

func TestBanMessageReachesTheVerdict(t *testing.T) {
	api := fakeCenter(t, "", map[string]string{"/server/v1/whitelist/check": `{"code":0,"data":{"allowed":false,"bound":true,"reason":"banned","message":"x-ray","player":null}}`})
	v, err := (&gate{api: api, outage: outageCache}).check(context.Background(), "Steve", "")
	if err != nil || v.Allowed || v.Reason != "banned" || v.Message != "x-ray" {
		t.Fatalf("banned: %+v %v", v, err)
	}
}
