// Command external-app is an example NoWork external-app plugin (standard library only). The
// user centre POSTs every event, filter, job, GM action and page call to one endpoint, signed
// with the plugin's secret; the app answers and may call back with its scoped token.
//
//	NOWORK_PLUGIN_SECRET=lps_...   signing secret shown once at install (verifies our requests)
//	NOWORK_PLUGIN_TOKEN=lpt_...    callback token shown once at install (calls the plugin API)
//	NOWORK_API=https://account.example.com/api/plugin/v1
//	go run . -addr :8090
//
// Put it behind HTTPS and set external.endpoint in plugin.json to its public URL.
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// envelope is what the user centre sends (SDK/plugins/ABI.md section 1).
type envelope struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"` // event | filter | job | action | ui
	Point    string         `json:"point"`
	Settings map[string]any `json:"settings"`
	Server   map[string]any `json:"server"`
	User     map[string]any `json:"user"`
	Data     map[string]any `json:"data"`
}

var (
	secret = os.Getenv("NOWORK_PLUGIN_SECRET")
	token  = os.Getenv("NOWORK_PLUGIN_TOKEN")
	api    = os.Getenv("NOWORK_API")
)

// verify checks X-NoWork-Signature = sha256=<hex HMAC-SHA256(secret, "<timestamp>.<body>")>
// and refuses timestamps older than five minutes (replays).
func verify(r *http.Request, body []byte) bool {
	ts, err := strconv.ParseInt(r.Header.Get("X-NoWork-Timestamp"), 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.", ts)
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(r.Header.Get("X-NoWork-Signature")))
}

// call runs one host function through the plugin API (same functions as the wasm ABI).
func call(fn string, args, out any) error {
	b, _ := json.Marshal(args)
	req, _ := http.NewRequest(http.MethodPost, api+"/call/"+fn, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Plugin-Token", token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return err
	}
	if env.Code != 0 {
		return fmt.Errorf("%s: %s (HTTP %d)", fn, env.Message, res.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || !verify(r, body) {
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	switch {
	case env.Type == "event" && env.Point == "user.login":
		// Answer fast (the user centre waits at most 10 s); do slow work after replying.
		go remember(env)
		w.WriteHeader(http.StatusNoContent)
	case env.Type == "action" && env.Data["action"] == "greet":
		params, _ := env.Data["params"].(map[string]any)
		writeJSON(w, map[string]any{"message": fmt.Sprintf("Hello, %v!", params["player"])})
	case env.Type == "event":
		w.WriteHeader(http.StatusNoContent)
	default:
		// A business error answers 200 with {"error": ...}: non-2xx counts as a failure and
		// repeated failures switch the plugin off.
		writeJSON(w, map[string]any{"error": "not supported: " + env.Type + " " + env.Point})
	}
}

// remember counts logins in the plugin's own storage on the user centre (capability kv).
func remember(env envelope) {
	key := fmt.Sprintf("logins:%v", env.Data["userId"])
	var cur struct {
		Value float64 `json:"value"`
	}
	if err := call("kv.get", map[string]any{"key": key}, &cur); err != nil {
		log.Print(err)
		return
	}
	if err := call("kv.set", map[string]any{"key": key, "value": cur.Value + 1}, nil); err != nil {
		log.Print(err)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()
	if secret == "" || token == "" || api == "" {
		log.Fatal("set NOWORK_PLUGIN_SECRET, NOWORK_PLUGIN_TOKEN and NOWORK_API")
	}
	http.HandleFunc("/nowork", handle)
	log.Printf("listening on %s/nowork", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
