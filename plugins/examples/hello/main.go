//go:build wasip1

// Command hello is an example NoWork WebAssembly plugin. It counts sign-ins per user (event
// user.login), adds the count to tokens (filter token.claims), greets a player (GM action),
// logs every hour (job) and shows the count on a user-centre page (UI slot + bridge invoke).
//
// Build: ./build.sh (or build.ps1), which runs
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
package main

import (
	"fmt"

	"github.com/wangework/nowork-sdk-plugins/guest"
)

// main never runs: a c-shared (reactor) module only initialises and waits for the host to call
// the exported handlers.
func main() {}

func userKey(env *guest.Envelope) string {
	id, _ := env.Data["userId"].(float64)
	if id == 0 && env.User != nil {
		id, _ = env.User["userId"].(float64)
	}
	return fmt.Sprintf("logins:%d", int64(id))
}

// count reads a counter from the plugin's own key-value storage (capability kv).
func count(key string) float64 {
	var res struct {
		Found bool    `json:"found"`
		Value float64 `json:"value"`
	}
	if err := guest.Call("kv.get", map[string]any{"key": key}, &res); err != nil {
		return 0
	}
	return res.Value
}

func done(err error) int32 {
	if err != nil {
		return 1
	}
	return 0
}

//go:wasmexport on_login
func onLogin() int32 {
	env, err := guest.Input()
	if err != nil {
		return 1
	}
	key := userKey(env)
	n := count(key) + 1
	if err := guest.Call("kv.set", map[string]any{"key": key, "value": n}, nil); err != nil {
		guest.Log(guest.LevelError, "kv.set: "+err.Error())
		return 1
	}
	name, _ := env.Data["username"].(string)
	guest.Log(guest.LevelInfo, fmt.Sprintf("%s signed in (%v times)", name, n))
	return 0
}

// claims answers token.claims; only the keys declared in plugin.json are published.
//
//go:wasmexport claims
func claims() int32 {
	env, err := guest.Input()
	if err != nil {
		return 1
	}
	return done(guest.Output(map[string]any{"logins": count(userKey(env))}))
}

//go:wasmexport greet
func greet() int32 {
	env, err := guest.Input()
	if err != nil {
		return 1
	}
	params, _ := env.Data["params"].(map[string]any)
	player, _ := params["player"].(string)
	greeting, _ := env.Settings["greeting"].(string)
	if greeting == "" {
		greeting = "Hello"
	}
	return done(guest.Output(map[string]any{"message": fmt.Sprintf("%s, %s!", greeting, player)}))
}

//go:wasmexport tick
func tick() int32 {
	guest.Log(guest.LevelInfo, "hourly tick")
	return 0
}

// page handles the user-centre page's bridge calls (nowork.invoke('stats')).
//
//go:wasmexport page
func page() int32 {
	env, err := guest.Input()
	if err != nil {
		return 1
	}
	if env.Data["action"] != "stats" {
		_ = guest.Output(map[string]string{"error": "unknown action"})
		return 1
	}
	return done(guest.Output(map[string]any{"logins": count(userKey(env))}))
}
