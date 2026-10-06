package nowork

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// PublishGMCatalog sends {actions, options} (options left out when nil) and decodes the
// skipped entries; PublishGMOptions replaces only the lists.
func TestPublishGMCatalog(t *testing.T) {
	var bodies []map[string]json.RawMessage
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("X-Server-Key") != "test-key" {
			t.Errorf("unexpected %s request, key %q", r.Method, r.Header.Get("X-Server-Key"))
		}
		raw, _ := io.ReadAll(r.Body)
		var b map[string]json.RawMessage
		_ = json.Unmarshal(raw, &b)
		bodies, paths = append(bodies, b), append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"data":{"count":1,"skipped":[{"key":"bad key","reason":"invalid key"}]}}`)
	}))
	defer server.Close()
	c := New(server.URL, WithServerKey("test-key"))
	min, max := 1.0, 64.0
	actions := []GMAction{{Key: "give", Title: "Give", Category: "Items", Danger: "confirm", Params: []GMActionParam{
		{Name: "player", Type: "player", Required: true},
		{Name: "count", Type: "number", Integer: true, Min: &min, Max: &max},
		{Name: "world", Type: "select", Source: "worlds"},
	}}}
	res, err := c.PublishGMCatalog(context.Background(), actions, map[string][]GMOption{"worlds": {{Value: "overworld"}}})
	if err != nil || res.Count != 1 || len(res.Skipped) != 1 || res.Skipped[0].Key != "bad key" {
		t.Fatalf("PublishGMCatalog = %+v, %v", res, err)
	}
	if paths[0] != "/server/v1/gm/actions" || bodies[0]["actions"] == nil || bodies[0]["options"] == nil {
		t.Fatalf("catalog request %s %v", paths[0], bodies[0])
	}
	if _, err := c.PublishGMCatalog(context.Background(), actions, nil); err != nil {
		t.Fatal(err)
	}
	if _, sent := bodies[1]["options"]; sent {
		t.Fatal("nil options were sent (they would clear the stored lists)")
	}
	if _, err := c.PublishGMOptions(context.Background(), map[string][]GMOption{"kits": {{Value: "starter"}}}); err != nil || paths[2] != "/server/v1/gm/options" {
		t.Fatalf("PublishGMOptions: %v %v", err, paths)
	}
}
