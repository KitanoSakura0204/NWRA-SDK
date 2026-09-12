// Command paper-check demonstrates the Server API: full sync, join check and heartbeat.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	lounge "github.com/wangework/lounge-sdk-go"
)

func main() {
	base := envOr("LOUNGE_BASE", "http://localhost:8081")
	key := os.Getenv("LOUNGE_SERVER_KEY")
	if key == "" {
		log.Fatal("set LOUNGE_SERVER_KEY (from admin console → 服务器管理)")
	}
	c := lounge.New(base, lounge.WithServerKey(key))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wl, err := c.Whitelist(ctx)
	if err != nil {
		log.Fatalf("whitelist: %v", err)
	}
	fmt.Printf("server=%s mode=%d entries=%d\n", wl.Server, wl.Mode, wl.Count)

	res, err := c.CheckWhitelist(ctx, "Steve", "")
	if err != nil {
		log.Fatalf("check: %v", err)
	}
	fmt.Printf("Steve allowed=%v reason=%q\n", res.Allowed, res.Reason)

	if err := c.Heartbeat(ctx, lounge.Heartbeat{Online: 1, Max: 20, Version: "example"}); err != nil {
		log.Fatalf("heartbeat: %v", err)
	}
	fmt.Println("heartbeat ok")
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
