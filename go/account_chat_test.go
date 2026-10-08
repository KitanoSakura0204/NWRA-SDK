package nowork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPortalCredentialsAndOpenServerRoleRoute(t *testing.T) {
	var paths []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer site-jwt" || r.Header.Get("X-Server-Key") != "" {
			t.Error("account API received wrong credential type")
		}
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":3,"requiresWhitelist":false}]}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["roleName"] != "Alex" {
			t.Errorf("bad role body: %v", body)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":3,"roles":[{"roleName":"Alex"}]}}`))
	}))
	defer s.Close()
	c := NewAccount(s.URL, "site-jwt", nil)
	servers, err := c.Servers(context.Background())
	if err != nil || len(servers) != 1 || servers[0]["requiresWhitelist"] != false {
		t.Fatalf("servers: %v %v", servers, err)
	}
	if _, err := c.BindRole(context.Background(), 3, PortalRoleInput{RoleName: "Alex"}); err != nil {
		t.Fatal(err)
	}
	if paths[1] != "/api/account/portal/servers/3/roles" {
		t.Fatalf("open-server role was sent to whitelist-only route: %v", paths)
	}
}

func TestChatRelayUsesDoneAndPreservesPayload(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Server-Key") != "key" {
			t.Error("missing server key")
		}
		switch r.URL.Path {
		case "/server/v1/chat/relay":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":9,"kind":"message","channelId":2,"payload":{"text":"hello"},"attempts":1}]}}`))
		case "/server/v1/chat/relay/ack":
			var body map[string]json.RawMessage
			_ = json.NewDecoder(r.Body).Decode(&body)
			if string(body["done"]) != "[9]" || body["ids"] != nil {
				t.Errorf("chat ACK confused with bot ACK: %s", body)
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"updated":1}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer s.Close()
	c := New(s.URL, WithServerKey("key"))
	items, err := c.PollChatRelay(context.Background())
	if err != nil || len(items) != 1 || string(items[0].Payload) != `{"text":"hello"}` {
		t.Fatalf("relay: %v %v", items, err)
	}
	if updated, err := c.AckChatRelay(context.Background(), []int64{9}, nil); err != nil || updated != 1 {
		t.Fatalf("ack: %d %v", updated, err)
	}
}

func TestChatSessionAuthNonceAndEmptyRead(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Chat cs_session" || r.Header.Get("X-Server-Key") != "" {
			t.Error("chat session used incorrect authentication")
		}
		if r.URL.Path == "/api/chat/channels/3/read" {
			_, _ = w.Write([]byte(`{"code":0,"data":null}`))
			return
		}
		var body ChatSend
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Nonce != "retry-1" || body.Content != "hello" {
			t.Errorf("retry nonce lost: %+v", body)
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":9,"ownReactions":["heart"]}}`))
	}))
	defer s.Close()
	c := NewChat(s.URL, "cs_session", nil)
	message, err := c.SendMessage(context.Background(), 3, ChatSend{Content: "hello", Nonce: "retry-1"})
	if err != nil || message["ownReactions"] == nil {
		t.Fatalf("send: %v %v", message, err)
	}
	if err := c.MarkRead(context.Background(), 3, 9); err != nil {
		t.Fatal(err)
	}
}
