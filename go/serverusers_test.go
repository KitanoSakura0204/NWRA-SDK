package nowork

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// suServer records each request (method, path?query, body) and answers from a route table.
func suServer(t *testing.T, answers map[string]string, seen *[]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		key := r.Method + " " + r.URL.Path
		*seen = append(*seen, key+"?"+r.URL.RawQuery+" "+string(body))
		w.Header().Set("Content-Type", "application/json")
		a, ok := answers[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, a)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, WithServerKey("test-key"))
}

func TestUserDataRoutes(t *testing.T) {
	var seen []string
	stored := `{"code":0,"data":{"uid":100001,"data":{"coins":12,"rank":null},"version":2,"updatedAt":"2026-09-29 10:00:00","updatedBy":"plugin","size":11}}`
	c := suServer(t, map[string]string{
		"GET /server/v1/users/schema":        `{"code":0,"data":{"fields":[{"key":"coins","label":"Coins","type":"number","visible":true,"integer":true,"min":0}],"maxUserBytes":8192}}`,
		"GET /server/v1/users/100001/data":   stored,
		"PUT /server/v1/users/100001/data":   stored,
		"PATCH /server/v1/users/100001/data": stored,
		"PUT /server/v1/users/100002/data":   `{"code":409,"message":"stale"}`,
		"GET /server/v1/users/100003/data":   `{"code":404,"message":"not on roster"}`,
	}, &seen)
	ctx := context.Background()

	s, err := c.UserSchema(ctx)
	if err != nil || s.MaxUserBytes != 8192 || len(s.Fields) != 1 || !s.Fields[0].Integer || *s.Fields[0].Min != 0 {
		t.Fatalf("schema: %#v %v", s, err)
	}
	d, err := c.UserData(ctx, 100001)
	if err != nil || d.Version != 2 || string(d.Data["coins"]) != "12" || string(d.Data["rank"]) != "null" || *d.UpdatedAt != "2026-09-29 10:00:00" {
		t.Fatalf("read: %#v %v", d, err)
	}
	if _, err := c.PutUserData(ctx, 100001, map[string]any{"coins": 1}, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PatchUserData(ctx, 100001, map[string]any{"rank": nil}, -1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PutUserData(ctx, 100001, nil, 0); err != nil {
		t.Fatal(err)
	}
	want := []string{
		`PUT /server/v1/users/100001/data? {"data":{"coins":1},"version":2}`,
		`PATCH /server/v1/users/100001/data? {"data":{"rank":null}}`,
		`PUT /server/v1/users/100001/data? {"data":{},"version":0}`,
	}
	for i, w := range want {
		if seen[2+i] != w {
			t.Errorf("request %d:\n got %s\nwant %s", i, seen[2+i], w)
		}
	}
	if _, err := c.PutUserData(ctx, 100002, map[string]any{"coins": 1}, 1); !IsVersionConflict(err) {
		t.Fatalf("stale version: %v", err)
	}
	if _, err := c.UserData(ctx, 100003); err == nil || IsVersionConflict(err) {
		t.Fatalf("not on roster: %v", err)
	}
	if _, err := c.UserData(ctx, 0); err == nil {
		t.Fatal("uid 0 accepted")
	}
}

func TestBanRoutes(t *testing.T) {
	var seen []string
	c := suServer(t, map[string]string{
		"GET /server/v1/bans":       `{"code":0,"data":{"count":1,"bans":[{"uid":100001,"names":["Steve"],"uuids":["069a79f4-44e9-4726-a5be-fca90e38aaf5"],"reason":"x-ray","expireTime":null,"createTime":"2026-09-29 10:00:00","source":"plugin"}]}}`,
		"GET /server/v1/bans/kick":  `{"code":0,"data":{"count":0,"players":[]}}`,
		"POST /server/v1/bans":      `{"code":0,"data":{"uid":100001,"ban":{"uid":100001,"names":["Steve"],"uuids":[],"reason":"x-ray","expireTime":"2026-09-30 10:00:00","createTime":"2026-09-29 10:00:00","source":"plugin"}}}`,
		"POST /server/v1/bans/lift": `{"code":0,"data":{"uid":100001,"lifted":true}}`,
	}, &seen)
	ctx := context.Background()

	bans, err := c.Bans(ctx, BanTarget{Name: "Steve"})
	if err != nil || len(bans) != 1 || bans[0].ExpireTime != nil || bans[0].Names[0] != "Steve" || bans[0].Source != "plugin" {
		t.Fatalf("bans: %#v %v", bans, err)
	}
	kick, err := c.KickList(ctx)
	if err != nil || kick == nil || len(kick) != 0 {
		t.Fatalf("kick: %#v %v", kick, err)
	}
	uid, ban, err := c.Ban(ctx, BanTarget{Name: "Steve"}, BanOptions{Reason: "x-ray", DurationMinutes: 1440})
	if err != nil || uid != 100001 || ban == nil || *ban.ExpireTime != "2026-09-30 10:00:00" {
		t.Fatalf("ban: %d %#v %v", uid, ban, err)
	}
	if uid, err := c.Unban(ctx, BanTarget{UID: 100001}, "appeal"); err != nil || uid != 100001 {
		t.Fatalf("unban: %d %v", uid, err)
	}
	var placed, lifted map[string]any
	_ = json.Unmarshal([]byte(seen[2][len("POST /server/v1/bans? "):]), &placed)
	_ = json.Unmarshal([]byte(seen[3][len("POST /server/v1/bans/lift? "):]), &lifted)
	if placed["name"] != "Steve" || placed["durationMinutes"] != float64(1440) || placed["uid"] != nil || placed["expireTime"] != nil {
		t.Errorf("ban body: %v", placed)
	}
	if lifted["uid"] != float64(100001) || lifted["reason"] != "appeal" || lifted["name"] != nil {
		t.Errorf("unban body: %v", lifted)
	}
	if seen[0] != "GET /server/v1/bans?name=Steve " {
		t.Errorf("bans query: %s", seen[0])
	}
}

func TestJoinCheckCarriesTheBanMessage(t *testing.T) {
	var seen []string
	c := suServer(t, map[string]string{
		"GET /server/v1/whitelist/check": `{"code":0,"data":{"allowed":false,"bound":true,"reason":"banned","message":"x-ray","player":null}}`,
	}, &seen)
	res, err := c.CheckWhitelist(context.Background(), "Steve", "")
	if err != nil || res.Allowed || res.Reason != "banned" || res.Message != "x-ray" || !IsNotAllowed(res, err) {
		t.Fatalf("check: %#v %v", res, err)
	}
}
