package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	nowork "github.com/wangework/nowork-sdk-go"
)

// world stands in for the game: who is online and the little state the GM actions change.
// Replace it with calls into your engine (Bukkit, a Unity server, a custom loop, ...).
type world struct {
	mu     sync.Mutex
	max    int
	online map[string]player // key: lower-case name
	clock  map[string]string // world name -> "day" | "night"
	logf   func(format string, args ...any)
}

type player struct {
	Name string
	UUID string
	UID  int64 // NoWork uid from the join check, 0 when not known
}

func newWorld(max int, logf func(string, ...any)) *world {
	return &world{max: max, online: map[string]player{}, clock: map[string]string{"overworld": "day", "nether": "night"}, logf: logf}
}

// add lets a player in; the caller already ran the whitelist check.
func (w *world) add(p player) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.online[strings.ToLower(p.Name)]; ok {
		return fmt.Errorf("%s is already online", p.Name)
	}
	if w.max > 0 && len(w.online) >= w.max {
		return fmt.Errorf("server full (%d)", w.max)
	}
	w.online[strings.ToLower(p.Name)] = p
	return nil
}

func (w *world) remove(name string) (player, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.online[strings.ToLower(name)]
	delete(w.online, strings.ToLower(name))
	return p, ok
}

func (w *world) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.online)
}

// players is the heartbeat player list (PROTOCOL 2.3), sorted so every beat reads the same.
func (w *world) players() []nowork.OnlinePlayer {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]nowork.OnlinePlayer, 0, len(w.online))
	for _, p := range w.online {
		out = append(out, nowork.OnlinePlayer{Name: p.Name, UUID: p.UUID})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// kickUID removes every online player of one NoWork account (frozen, locked or deleted).
func (w *world) kickUID(uid int64, reason string) []string {
	if uid == 0 {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var names []string
	for k, p := range w.online {
		if p.UID == uid {
			delete(w.online, k)
			names = append(names, p.Name)
			w.logf("kicked %s: %s", p.Name, reason)
		}
	}
	sort.Strings(names)
	return names
}

// worlds is the named option list "worlds" the GM panel offers for the time action.
func (w *world) worlds() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]string, 0, len(w.clock))
	for name := range w.clock {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (w *world) setTime(name, value string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.clock[name]; !ok {
		return fmt.Errorf("no world %q", name)
	}
	w.clock[name] = value
	return nil
}
