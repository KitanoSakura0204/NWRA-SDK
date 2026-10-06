// Command game-server-go is a minimal game server wired to NoWork Account with the Go SDK:
// whitelist sync and join checks (encrypted replies included), heartbeat with TPS and the player
// list, the GM channel (typed catalogue, option lists, command polling and results), webhooks
// that kick frozen or deleted accounts at once, and the SDK login page hand-off for a game client.
//
//	game-server-go            run the server; type "join <name> [uuid]", "leave <name>", "list", "quit"
//	game-server-go login      sign a player in through the SDK login page (client side)
//
// Configuration comes from the environment; see README.md.
package main

import (
	"bufio"
	"context"
	"errors"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	nowork "github.com/wangework/nowork-sdk-go"
)

func main() {
	base := strings.TrimRight(envOr("NOWORK_BASE", "http://localhost:8081"), "/")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 && os.Args[1] == "login" {
		c := loginFromEnv(base)
		if c.server == "" || c.clientID == "" {
			log.Fatal("set NOWORK_SERVER_CODE and NOWORK_CLIENT_ID (admin console: 游戏服务 -> SDK 登录页)")
		}
		if err := login(ctx, c); err != nil {
			log.Fatal(err)
		}
		return
	}

	key := os.Getenv("NOWORK_SERVER_KEY")
	if key == "" {
		log.Fatal("set NOWORK_SERVER_KEY (admin console: 游戏服务 -> 服务器管理 -> 服务器密钥)")
	}
	keys, err := loadKeys(os.Getenv("NOWORK_DECRYPTION_KEYS"))
	if err != nil {
		log.Fatalf("NOWORK_DECRYPTION_KEYS: %v", err)
	}
	api := nowork.New(base, nowork.WithServerKey(key))
	w := newWorld(envInt("MAX_PLAYERS", 20), log.Printf)
	g := &gate{api: api, code: os.Getenv("NOWORK_SERVER_CODE"), keys: keys, outage: envOr("OUTAGE_POLICY", outageCache)}
	switch g.outage {
	case outageCache, outageDeny, outageAllow:
	default:
		log.Fatalf("OUTAGE_POLICY %q: use cache, deny or allow", g.outage)
	}

	s := &server{api: api, world: w, gate: g, version: envOr("SERVER_VERSION", "example-1.0"), quit: stop}
	s.syncWhitelist(ctx)
	s.setupGM(ctx)

	if secret := os.Getenv("NOWORK_WEBHOOK_SECRET"); secret != "" {
		h := &hooks{secret: secret, tolerance: 5 * time.Minute, keys: keys, world: w, logf: log.Printf,
			seen: map[string]time.Time{}, resync: func() { go s.syncWhitelist(ctx) }}
		mux := http.NewServeMux()
		mux.Handle(envOr("WEBHOOK_PATH", "/nowork/webhook"), h)
		srv := &http.Server{Addr: envOr("WEBHOOK_ADDR", "127.0.0.1:8095"), Handler: mux,
			ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
		go func() {
			log.Printf("webhooks on http://%s%s", srv.Addr, envOr("WEBHOOK_PATH", "/nowork/webhook"))
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("webhook listener: %v", err)
			}
		}()
		defer srv.Close()
	}

	go s.loop(ctx)
	go s.console(ctx)
	log.Printf("server up: %s, %d decryption key(s), outage policy %s", base, len(keys), g.outage)
	<-ctx.Done()
	log.Print("shutting down")
}

// server ties the pieces together.
type server struct {
	api     *nowork.Client
	world   *world
	gate    *gate
	version string
	quit    func()

	gmOn   bool
	syncMu sync.Mutex
}

// syncWhitelist refreshes the local copy (start-up, every WHITELIST_SYNC_MINUTES, after webhooks).
func (s *server) syncWhitelist(ctx context.Context) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	n, err := s.gate.refresh(ctx)
	if err != nil {
		log.Printf("whitelist sync: %v", err)
		return
	}
	log.Printf("whitelist synced: %d entries", n)
}

// setupGM publishes the catalogue when the server-gm module is enabled for this server.
func (s *server) setupGM(ctx context.Context) {
	mods, err := s.api.Modules(ctx)
	if err != nil {
		log.Printf("modules: %v", err)
		return
	}
	if _, ok := mods["server-gm"]; !ok {
		log.Print("server-gm module not enabled for this server: GM channel off")
		return
	}
	if err := publishGM(ctx, s.api, s.world, log.Printf); err != nil {
		log.Printf("GM publish: %v", err)
		return
	}
	s.gmOn = true
}

// loop is the game tick (20 per second) plus the timers: heartbeat, GM polling, whitelist sync.
func (s *server) loop(ctx context.Context) {
	tick := time.NewTicker(50 * time.Millisecond)
	beat := time.NewTicker(time.Duration(envInt("HEARTBEAT_SECONDS", 60)) * time.Second)
	poll := time.NewTicker(time.Duration(envInt("GM_POLL_SECONDS", 15)) * time.Second)
	resync := time.NewTicker(time.Duration(envInt("WHITELIST_SYNC_MINUTES", 5)) * time.Minute)
	defer tick.Stop()
	defer beat.Stop()
	defer poll.Stop()
	defer resync.Stop()
	ticks, since := 0, time.Now()
	s.heartbeat(ctx, 0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			ticks++ // one game tick: the real work of your server goes here
		case <-beat.C:
			tps := math.Min(20, float64(ticks)/time.Since(since).Seconds())
			ticks, since = 0, time.Now()
			s.heartbeat(ctx, tps)
		case <-poll.C:
			if s.gmOn {
				s.pollGM(ctx)
			}
		case <-resync.C:
			s.syncWhitelist(ctx)
		}
	}
}

func (s *server) heartbeat(ctx context.Context, tps float64) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// TPS 0 (no reading yet) is left out by the SDK, and so are NaN / Inf
	hb := nowork.Heartbeat{Online: s.world.count(), Max: s.world.max, Version: s.version, TPS: tps, Players: s.world.players()}
	if err := s.api.Heartbeat(ctx, hb); err != nil {
		log.Printf("heartbeat: %v", err)
		return
	}
	// Per-server bans placed while a player is online (SERVER_USERS.md): the kick list matches the
	// players of this heartbeat against the active bans. It works with the server-users module off too.
	kick, err := s.api.KickList(ctx)
	if err != nil {
		log.Printf("kick list: %v", err)
		return
	}
	for _, k := range kick {
		if _, ok := s.world.remove(k.Name); ok {
			log.Printf("kicked %s: banned on this server (%s)", k.Name, orDefault(k.Reason, "no reason"))
		}
	}
}

func (s *server) pollGM(ctx context.Context) {
	cmds, err := s.api.PollGMCommands(ctx)
	if err != nil {
		log.Printf("GM poll: %v", err)
		return
	}
	for _, cmd := range cmds {
		result, err := runGM(s.world, cmd, func(msg string) { log.Printf("[broadcast] %s", msg) })
		ok := err == nil
		if !ok {
			result = err.Error()
		}
		log.Printf("GM #%d %s by %s (%s): ok=%v %s", cmd.ID, cmd.Action, orDefault(cmd.Operator, "-"), orDefault(cmd.Source, "admin"), ok, result)
		if err := s.api.ReportGMResult(ctx, cmd.ID, ok, result); err != nil {
			log.Printf("GM #%d result: %v", cmd.ID, err)
		}
		if ok && cmd.Action == "stop" {
			s.quit()
			return
		}
	}
}

// join is what the game calls when a player connects.
func (s *server) join(ctx context.Context, name, uuid string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	v, err := s.gate.check(ctx, name, uuid)
	if err != nil {
		log.Printf("join check for %s: %v (decided by %s)", name, err, v.Source)
	}
	if !v.Allowed {
		if v.Message != "" { // e.g. the reason of a per-server ban
			log.Printf("%s refused: %s (%s)", name, orDefault(v.Reason, "not allowed"), v.Message)
		} else {
			log.Printf("%s refused: %s", name, orDefault(v.Reason, "not allowed"))
		}
		return
	}
	p := player{Name: name, UUID: uuid}
	if v.Player != nil {
		p.UID = v.Player.UID
		if p.UUID == "" {
			p.UUID = v.Player.UUID
		}
	}
	if err := s.world.add(p); err != nil {
		log.Printf("%s refused: %v", name, err)
		return
	}
	log.Printf("%s joined (uid %d, %s)", name, p.UID, v.Source)
}

// console reads test commands from stdin; it ends quietly when stdin is closed (service mode).
func (s *server) console(ctx context.Context) {
	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		f := strings.Fields(in.Text())
		if len(f) == 0 {
			continue
		}
		switch {
		case f[0] == "join" && len(f) >= 2:
			uuid := ""
			if len(f) >= 3 {
				uuid = f[2]
			}
			s.join(ctx, f[1], uuid)
		case f[0] == "leave" && len(f) == 2:
			if _, ok := s.world.remove(f[1]); !ok {
				log.Printf("%s is not online", f[1])
			}
		case f[0] == "list":
			log.Printf("%d online: %v", s.world.count(), s.world.players())
		case f[0] == "quit":
			s.quit()
			return
		default:
			log.Print(`commands: join <name> [uuid] | leave <name> | list | quit`)
		}
	}
}

// loadKeys reads private JWKs from a comma-separated list of files; during a key rotation list
// the new and the old key (the JWE header's kid picks one, DELIVERY.md section 7).
func loadKeys(paths string) ([]*nowork.DecryptionKey, error) {
	var keys []*nowork.DecryptionKey
	for _, p := range strings.Split(paths, ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		k, err := nowork.ParseDecryptionKey(raw)
		if err != nil {
			return nil, errors.New(p + ": " + err.Error())
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func envOr(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(k))); err == nil && n > 0 {
		return n
	}
	return d
}
