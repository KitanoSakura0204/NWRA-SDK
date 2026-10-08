// A console adapter for the server chat relay. Run from SDK/go:
// go run ./examples/chat-relay [-player Alex -post "hello from the game"]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	nowork "github.com/wangework/nowork-sdk-go"
)

func main() {
	player := flag.String("player", "", "bound in-game player name for -post")
	post := flag.String("post", "", "send one in-game message and exit")
	flag.Parse()
	base, key := os.Getenv("NOWORK_BASE_URL"), os.Getenv("NOWORK_SERVER_KEY")
	if base == "" || key == "" {
		log.Fatal("set NOWORK_BASE_URL and NOWORK_SERVER_KEY")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := nowork.New(base, nowork.WithServerKey(key))
	if *post != "" {
		if *player == "" {
			log.Fatal("-player is required for -post")
		}
		_, err := client.PostChatMessage(ctx, nowork.ChatPost{Player: *player, Text: *post, Kind: "chat"})
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	// Production adapters persist delivered IDs before ACKing to avoid duplicates
	// after disconnects. This console example demonstrates ACK-after-output ordering.
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		items, err := client.PollChatRelay(ctx)
		if err == nil {
			var done []int64
			for _, item := range items {
				if _, err := fmt.Fprintf(os.Stdout, "%d %s %s\n", item.ChannelID, item.Kind, item.Payload); err != nil {
					log.Printf("output failed; item %d left pending", item.ID)
					break
				}
				done = append(done, item.ID)
			}
			if len(done) > 0 {
				_, err = client.AckChatRelay(ctx, done, nil)
			}
		}
		if err != nil && ctx.Err() == nil {
			log.Print(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
