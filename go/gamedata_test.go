package nowork

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGameSnapshotPreservesSourcePrivacyAndAbsentValues(t *testing.T) {
	const payload = `{"game_id":"genshin","fetched_at":"2026-10-08T00:00:00Z","expires_at":"2026-10-08T01:00:00Z","role":{"id":"12","uid":"100000001","game_id":"genshin","region":"cn","policy":{"visibility":"owner","exportable":false}},"sections":[{"id":"characters","status":"partial","reason":"Private fields unavailable","policy":{"visibility":"owner","exportable":false},"source":{"source_id":"official-record","kind":"official","name":"Game record","url":"https://example.invalid/record","auth_scope":"role","fetched_at":"2026-10-08T00:00:00Z","updated_at":"2026-10-07T00:00:00Z","attribution":"Publisher"},"fields":[{"key":"count","kind":"number","value":0,"status":"ok","policy":{"visibility":"owner","exportable":false}},{"key":"reward","kind":"boolean","value":false,"status":"ok","policy":{"visibility":"shared","exportable":true}},{"key":"missing","kind":"number","status":"private","policy":{"visibility":"owner","exportable":false}},{"key":"null","kind":"number","value":null,"status":"unavailable","policy":{"visibility":"owner","exportable":false}},{"key":"large","kind":"number","value":9007199254740993,"status":"ok","policy":{"visibility":"owner","exportable":false}}],"entities":[{"id":"10000002","kind":"character","name":{"en":"Reference character","zh_CN":"角色"},"icon":{"url":"https://example.invalid/icon.png","source_id":"catalog","alt":{"en":"Portrait"}},"fields":[],"policy":{"visibility":"public","exportable":true}}]}]}`
	var snapshot GameSnapshot
	if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
		t.Fatal(err)
	}
	section := snapshot.Sections[0]
	if snapshot.FetchedAt.IsZero() || snapshot.ExpiresAt == nil || section.Source.AuthScope != GameAuthRole || section.Source.UpdatedAt == nil || section.Status != GameStatusPartial {
		t.Fatalf("metadata missing: %+v", snapshot)
	}
	for i, want := range []bool{true, true, false, false, true} {
		if section.Fields[i].HasValue() != want {
			t.Errorf("field %d presence", i)
		}
	}
	var precise int64
	if err := json.Unmarshal(section.Fields[4].Value, &precise); err != nil || precise != 9007199254740993 {
		t.Fatalf("integer precision lost: %d %v", precise, err)
	}
	if section.Fields[0].Policy.Exportable || section.Fields[1].Policy.Visibility != GameVisibilityShared || section.Entities[0].Icon.SourceID != "catalog" {
		t.Fatal("policy/icon metadata lost")
	}
	if len(section.Entities[0].Fields) != 0 {
		t.Fatal("catalog character invented ownership")
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "site_id") || strings.Contains(string(encoded), "account_id") {
		t.Fatal("internal scope exposed")
	}
}

func TestGameAccountRoutesUseJWTAndOpaqueCursor(t *testing.T) {
	var paths []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer fixture-jwt" || r.Header.Get("X-Server-Key") != "" {
			t.Error("mixed account and server credentials")
		}
		switch r.URL.Path {
		case "/api/account/mihoyo/status":
			_, _ = w.Write([]byte(`{"code":0,"data":{"accountId":7,"bound":true,"region":"cn","recovery":{"action":"verify","url":"https://example.invalid/recover"},"roles":[{"id":12,"gameBiz":"hk4e_cn","gameId":"genshin","gameUid":"100000001","syncTime":"2026-10-08 08:00:00"}]}}`))
		case "/api/account/mihoyo/games":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":"genshin","provider":"mihoyo","name":{"en":"Genshin Impact"},"capabilities":[{"id":"bag","supported":false,"auth_scope":"role","status":"unsupported","reason":"No supported bag API"}]}]}`))
		case "/api/account/hypergryph/games":
			_, _ = w.Write([]byte(`{"code":0,"data":{"games":[],"roles":[{"id":"arknights:official:1","game_id":"arknights","uid":"1","region":"cn","policy":{"visibility":"owner","exportable":false}}],"selectedRoleId":"arknights:official:1","discovery":{"status":"ok","updatedAt":"2026-10-08T00:00:00Z"}}}`))
		case "/api/account/hypergryph":
			_, _ = w.Write([]byte(`{"code":0,"data":{"enabled":true,"account":{"id":9,"status":"ok","roles":[{"id":1,"accountId":9,"uid":"1","summary":null}]}}}`))
		case "/api/account/hypergryph/role":
			var body map[string]string
			if r.Method != http.MethodPut {
				t.Error("role selection must use PUT")
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["roleId"] != "arknights:official:1" {
				t.Errorf("role selection: %v %v", body, err)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":null}`))
		default:
			if r.URL.Path != "/api/account/mihoyo/games/genshin/data" && r.URL.Path != "/api/account/hypergryph/games/arknights/snapshot" {
				t.Errorf("unexpected route %s", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("cursor") != "opaque+/=&?" || q.Get("sections") != "characters" || q.Get("locale") != "ja" || q.Get("limit") != "20" || q.Get("roleId") == "" {
				t.Errorf("query lost: %v", q)
			}
			if q.Has("uid") || q.Has("account_id") || q.Has("site_id") {
				t.Error("scope override sent")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"game_id":"fixture","fetched_at":"2026-10-08T00:00:00Z","sections":[]}}`))
		}
	}))
	defer s.Close()
	c := NewAccount(s.URL, "fixture-jwt", nil)
	status, err := c.MihoyoStatus(context.Background())
	if err != nil || len(status.Roles) != 1 || status.Roles[0].ID != 12 || status.AccountID != 7 || status.Recovery.Action != "verify" {
		t.Fatalf("status: %v %v", status, err)
	}
	games, err := c.MihoyoGames(context.Background())
	if err != nil || len(games) != 1 || games[0].Capabilities[0].Status != GameStatusUnsupported {
		t.Fatalf("games: %v %v", games, err)
	}
	hg, err := c.HypergryphGames(context.Background())
	if err != nil || hg.SelectedRoleID != "arknights:official:1" {
		t.Fatalf("discovery: %v %v", hg, err)
	}
	hgStatus, err := c.HypergryphStatus(context.Background())
	if err != nil || hgStatus.Account == nil || hgStatus.Account.ID != 9 || hgStatus.Account.Roles[0].UID != "1" || hgStatus.Account.Roles[0].Summary != nil {
		t.Fatalf("HG account binding: %v %v", hgStatus, err)
	}
	if err = c.SelectHypergryphRole(context.Background(), hg.SelectedRoleID); err != nil {
		t.Fatal(err)
	}
	page := GameDataPage{Sections: []string{"characters"}, Cursor: "opaque+/=&?", Limit: 20, Locale: "ja"}
	if _, err = c.MihoyoGameData(context.Background(), "genshin", MihoyoGameDataOptions{GameDataPage: page, RoleID: status.Roles[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.HypergryphGameData(context.Background(), "arknights", HypergryphGameDataOptions{GameDataPage: page, RoleID: hg.SelectedRoleID}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 7 {
		t.Fatalf("calls: %v", paths)
	}
}

func TestGamePaginationSingleSectionDoesNotMergeAndStops(t *testing.T) {
	for _, tc := range []struct {
		name   string
		max    int
		repeat bool
		stop   bool
		want   error
		calls  int
	}{
		{"complete", 3, false, false, nil, 2},
		{"budget", 1, false, false, ErrGamePageLimit, 1},
		{"loop", 3, true, false, ErrGameCursorLoop, 2},
		{"callback", 3, false, true, context.Canceled, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cursors []string
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				cursor := r.URL.Query().Get("cursor")
				cursors = append(cursors, cursor)
				next := ""
				if cursor == "" || tc.repeat {
					next = "opaque-next"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": GameSnapshot{GameID: "genshin", FetchedAt: time.Now(), Sections: []GameSection{{ID: "characters", Status: GameStatusPartial, NextCursor: next}}}})
			}))
			defer s.Close()
			visits := 0
			err := NewAccount(s.URL, "fixture-jwt", nil).WalkMihoyoGameData(context.Background(), "genshin", MihoyoGameDataOptions{GameDataPage: GameDataPage{Sections: []string{"characters"}, Limit: 20}, RoleID: 12}, tc.max, func(snapshot *GameSnapshot) error {
				visits++
				if len(snapshot.Sections[0].Entities) != 0 || snapshot.Sections[0].Status != GameStatusPartial {
					t.Fatal("partial data changed")
				}
				if tc.stop {
					return context.Canceled
				}
				return nil
			})
			if !errors.Is(err, tc.want) || len(cursors) != tc.calls || visits != tc.calls {
				t.Fatalf("calls=%v visits=%d err=%v", cursors, visits, err)
			}
			if len(cursors) == 2 && !reflect.DeepEqual(cursors, []string{"", "opaque-next"}) {
				t.Fatalf("cursor not forwarded: %v", cursors)
			}
		})
	}
}

func TestGameDataErrorsCancellationAndInputValidation(t *testing.T) {
	var calls atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"code":429,"message":"retry later","data":null}`))
	}))
	defer s.Close()
	c := NewAccount(s.URL, "fixture-jwt", nil)
	if _, err := c.MihoyoGameData(context.Background(), "../admin", MihoyoGameDataOptions{}); err == nil {
		t.Fatal("path traversal accepted")
	}
	if _, err := c.MihoyoGameData(context.Background(), "genshin", MihoyoGameDataOptions{RoleID: -1}); err == nil {
		t.Fatal("negative role accepted")
	}
	if _, err := c.HypergryphGameData(context.Background(), "arknights", HypergryphGameDataOptions{GameDataPage: GameDataPage{Limit: 101}}); err == nil {
		t.Fatal("overlarge page accepted")
	}
	if _, err := c.HypergryphGameData(context.Background(), "arknights", HypergryphGameDataOptions{GameDataPage: GameDataPage{Cursor: strings.Repeat("a", 2049)}}); err == nil {
		t.Fatal("overlarge cursor accepted")
	}
	if err := c.WalkMihoyoGameData(context.Background(), "genshin", MihoyoGameDataOptions{}, 2, func(*GameSnapshot) error { return nil }); err == nil {
		t.Fatal("ambiguous section cursor accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input sent a request")
	}
	_, err := c.MihoyoGames(context.Background())
	var api *ApiError
	if !errors.As(err, &api) || api.Code != 429 {
		t.Fatalf("API error replaced: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.HypergryphGames(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("canceled request reached server")
	}
}

func TestSharedHypergryphRolesUseServerKeyAndKeepNullSummary(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/server/v1/hypergryph/roles" || r.Header.Get("X-Server-Key") != "fixture-key" || r.Header.Get("Authorization") != "" {
			t.Error("server auth or route mismatch")
		}
		if r.URL.Query().Get("uid") == "43" {
			_, _ = w.Write([]byte(`{"code":403,"message":"not shared","data":null}`))
			return
		}
		if r.URL.Query().Get("uid") != "42" {
			t.Error("site UID missing")
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"uid":42,"status":"ok","lastSync":null,"serverTime":1791417600,"roles":[{"uid":"100001","nickname":"Fixture","summary":null,"lastSync":null},{"uid":"100002","summary":{"rosterTrimmed":true,"daily":null,"roster":[{"id":"char_1","skills":[{"id":"skill_1","spec":2}],"equips":[{"id":"mod_1","level":3}]}]}}]}}`))
	}))
	defer s.Close()
	out, err := New(s.URL, WithServerKey("fixture-key")).SharedHypergryphRoles(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Roles) != 2 || out.Roles[0].Summary != nil || out.Roles[1].Summary.Daily != nil || !out.Roles[1].Summary.RosterTrimmed || out.Roles[1].Summary.Roster[0].Skills[0].Spec != 2 {
		t.Fatalf("summary changed: %+v", out)
	}
	if _, err = New(s.URL).SharedHypergryphRoles(context.Background(), 42); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing key: %v", err)
	}
	refused, err := New(s.URL, WithServerKey("fixture-key")).SharedHypergryphRoles(context.Background(), 43)
	var api *ApiError
	if refused != nil || !errors.As(err, &api) || api.Code != 403 {
		t.Fatalf("privacy refusal became empty success: %v %v", refused, err)
	}
}

func TestGameDataMalformedAndEncryptedResponsesFailClosed(t *testing.T) {
	for _, payload := range []string{`{"code":0,"data":null}`, `{"code":0,"data":{"sections":"wrong type"}}`, `{"code":0,"data":{"jwe":"encrypted"}}`, `{"data":{}}`} {
		t.Run(payload, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(payload)) }))
			defer s.Close()
			if snapshot, err := NewAccount(s.URL, "fixture-jwt", nil).MihoyoGameData(context.Background(), "genshin", MihoyoGameDataOptions{}); err == nil || snapshot != nil {
				t.Fatalf("malformed data accepted: %v %v", snapshot, err)
			}
		})
	}
}

func TestGameAccountRedirectDoesNotForwardJWT(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(true)
		_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()
	if _, err := NewAccount(source.URL, "fixture-jwt", nil).MihoyoGames(context.Background()); err == nil {
		t.Fatal("authenticated redirect accepted")
	}
	if leaked.Load() {
		t.Fatal("redirect target received account request")
	}
}

func TestGameInFlightCancellationStopsHTTP(t *testing.T) {
	entered := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := NewAccount(s.URL, "fixture-jwt", nil).MihoyoGames(ctx)
		done <- err
	}()
	select {
	case <-entered:
		cancel()
	case err := <-done:
		t.Fatalf("request failed before reaching server: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("request never reached server")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not cancel")
	}
}

func TestGameArchiveRoutesPaginationAndExportPolicies(t *testing.T) {
	var paths []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer fixture-jwt" || r.Header.Get("X-Server-Key") != "" {
			t.Error("archive must use account JWT")
		}
		const prefix = "/api/account/mihoyo/accounts/7/archive"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			t.Errorf("wrong binding route: %s", r.URL.Path)
		}
		switch strings.TrimPrefix(r.URL.Path, prefix) {
		case "/snapshots":
			if r.URL.Query().Get("game_id") == "" {
				_, _ = w.Write([]byte(`{"code":0,"data":[{"game_id":"genshin","role_id":"12","version":1,"revision":2,"status":"stale","stale":true,"bytes":120,"fetched_at":"2026-10-07T00:00:00Z","expires_at":"2026-10-07T06:00:00Z"}]}`))
			} else {
				if r.URL.Query().Get("role_id") != "12" || r.URL.Query().Has("roleId") {
					t.Error("archive requires snake_case role query")
				}
				_, _ = w.Write([]byte(`{"code":0,"data":{"provider":"mihoyo","role_id":"12","version":1,"revision":2,"status":"stale","stale":true,"expires_at":"2026-10-07T06:00:00Z","snapshot":{"game_id":"genshin","fetched_at":"2026-10-07T00:00:00Z","sections":[]}}}`))
			}
		case "/history", "/policy-audit":
			if r.URL.Query().Get("before") != "18446744073709551615" || r.URL.Query().Get("limit") != "20" {
				t.Error("archive before/limit lost precision")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
		case "/policies":
			if r.URL.Query().Get("game_id") != "genshin" {
				t.Error("game policy query missing")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":1,"game_id":"genshin","path":"characters/fields/level","visibility":"owner","exportable":false,"revision":2,"updated_at":"2026-10-07T00:00:00Z"}]}`))
		case "/export":
			_, _ = w.Write([]byte(`{"code":0,"data":{"scope":{"site_id":0,"user_id":9,"account_id":7},"provider":"mihoyo","exported_at":"2026-10-08T00:00:00Z","snapshots":[],"history":[],"policies":[],"policy_audit":[]}}`))
		case "":
			if r.Method != http.MethodDelete {
				t.Error("archive erase must use DELETE")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":null}`))
		default:
			t.Errorf("unknown archive route: %s", r.URL.Path)
		}
	}))
	defer s.Close()
	c := NewAccount(s.URL, "fixture-jwt", nil)
	ctx := context.Background()
	metadata, err := c.GameArchiveSnapshots(ctx, GameProviderMihoyo, 7)
	if err != nil || len(metadata) != 1 || !metadata[0].Stale {
		t.Fatalf("metadata: %v %v", metadata, err)
	}
	entry, err := c.GameArchiveSnapshot(ctx, GameProviderMihoyo, 7, "genshin", "12")
	if err != nil || entry.Status != GameStatusStale || entry.Version != 1 {
		t.Fatalf("snapshot: %v %v", entry, err)
	}
	page := GameArchivePage{Before: ^uint64(0), Limit: 20}
	if _, err = c.GameArchiveHistory(ctx, GameProviderMihoyo, 7, page); err != nil {
		t.Fatal(err)
	}
	if _, err = c.GameArchivePolicyAudit(ctx, GameProviderMihoyo, 7, page); err != nil {
		t.Fatal(err)
	}
	policies, err := c.GameArchivePolicies(ctx, GameProviderMihoyo, 7, "genshin")
	if err != nil || len(policies) != 1 || policies[0].Exportable {
		t.Fatalf("policies: %v %v", policies, err)
	}
	export, err := c.ExportGameArchive(ctx, GameProviderMihoyo, 7)
	if err != nil || export.Scope.AccountID != 7 || len(export.Snapshots) != 0 {
		t.Fatalf("redacted export changed: %v %v", export, err)
	}
	if err = c.DeleteGameArchive(ctx, GameProviderMihoyo, 7); err != nil {
		t.Fatal(err)
	}
	if _, err = c.GameArchiveSnapshots(ctx, GameProvider("../admin"), 7); err == nil {
		t.Fatal("provider path injection accepted")
	}
	if _, err = c.GameArchiveHistory(ctx, GameProviderMihoyo, 7, GameArchivePage{Limit: 101}); err == nil {
		t.Fatal("invalid archive limit accepted")
	}
	if len(paths) != 7 {
		t.Fatalf("request count: %d", len(paths))
	}
}
