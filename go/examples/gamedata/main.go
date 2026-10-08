// Read actual account game data with bounded pagination, or read a server's
// consent-filtered Arknights summary. Credentials come only from environment.
//
// Account mode: NOWORK_BASE_URL and NOWORK_ACCOUNT_JWT; flags -provider, -game,
// -role (owned binding ID from MihoyoStatus or HypergryphGames), -section, -pages.
// Server mode: NOWORK_BASE_URL and NOWORK_SERVER_KEY; flag -server-uid.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	nowork "github.com/wangework/nowork-sdk-go"
)

func main() {
	provider := flag.String("provider", "mihoyo", "mihoyo or hypergryph")
	gameID := flag.String("game", "", "game ID returned by the provider's game list")
	roleID := flag.String("role", "", "owned role ID; never an arbitrary upstream UID")
	section := flag.String("section", "characters", "one capability ID returned by the game list")
	pages := flag.Int("pages", 3, "maximum pages, 1..1000")
	serverUID := flag.Int64("server-uid", 0, "site UID for server-key consent-filtered summary mode")
	flag.Parse()
	base := os.Getenv("NOWORK_BASE_URL")
	if base == "" {
		log.Fatal("set NOWORK_BASE_URL to the user-center base URL")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if *serverUID != 0 {
		key := os.Getenv("NOWORK_SERVER_KEY")
		if key == "" {
			log.Fatal("server mode requires NOWORK_SERVER_KEY")
		}
		data, err := nowork.New(base, nowork.WithServerKey(key)).SharedHypergryphRoles(ctx, *serverUID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Shared Arknights roles: %d; binding status: %s\n", len(data.Roles), data.Status)
		return
	}
	token := os.Getenv("NOWORK_ACCOUNT_JWT")
	if token == "" {
		log.Fatal("account mode requires NOWORK_ACCOUNT_JWT (the site's login JWT)")
	}
	c := nowork.NewAccount(base, token, nil)
	page := nowork.GameDataPage{Sections: []string{*section}, Limit: 20, Locale: "en"}
	visit := func(snapshot *nowork.GameSnapshot) error {
		fmt.Printf("Game: %s; fetched: %s\n", snapshot.GameID, snapshot.FetchedAt.Format(time.RFC3339))
		for _, section := range snapshot.Sections {
			fmt.Printf("  %s: %s; %d entities; %d named fields\n", section.ID, section.Status, len(section.Entities), len(section.Fields))
			if section.Reason != "" {
				fmt.Printf("  Availability: %s\n", section.Reason)
			}
			if section.Source != nil {
				fmt.Printf("  Source: %s (%s, auth %s)\n", section.Source.Name, section.Source.Kind, section.Source.AuthScope)
			}
			// Values and role identifiers are deliberately not logged. Applications
			// can decode GameField.Value, respecting each field's Policy and Status.
		}
		return nil
	}
	var err error
	switch *provider {
	case "mihoyo":
		if *gameID == "" {
			games, e := c.MihoyoGames(ctx)
			if e != nil {
				log.Fatal(e)
			}
			for _, game := range games {
				fmt.Printf("Game: %s\n", game.ID)
				for _, capability := range game.Capabilities {
					fmt.Printf("  Section: %s (%s, auth %s)\n", capability.ID, capability.Status, capability.AuthScope)
				}
			}
			fmt.Println("Choose -game and -section from the returned game capabilities; get owned role IDs with MihoyoStatus.")
			return
		}
		var id int64
		if *roleID != "" {
			id, err = strconv.ParseInt(*roleID, 10, 64)
			if err != nil || id <= 0 {
				log.Fatal("miHoYo -role must be a positive owned role ID from MihoyoStatus")
			}
		}
		err = c.WalkMihoyoGameData(ctx, *gameID, nowork.MihoyoGameDataOptions{GameDataPage: page, RoleID: id}, *pages, visit)
	case "hypergryph":
		if *gameID == "" {
			account, e := c.HypergryphGames(ctx)
			if e != nil {
				log.Fatal(e)
			}
			for _, game := range account.Games {
				fmt.Printf("Game: %s\n", game.ID)
				for _, capability := range game.Capabilities {
					fmt.Printf("  Section: %s (%s, auth %s)\n", capability.ID, capability.Status, capability.AuthScope)
				}
			}
			fmt.Println("Choose -game and -section from the returned game capabilities; get owned role IDs with HypergryphGames.")
			return
		}
		err = c.WalkHypergryphGameData(ctx, *gameID, nowork.HypergryphGameDataOptions{GameDataPage: page, RoleID: *roleID}, *pages, visit)
	default:
		log.Fatal("-provider must be mihoyo or hypergryph")
	}
	if errors.Is(err, nowork.ErrGamePageLimit) {
		fmt.Println("Page budget reached; more data exists. Resume with the last snapshot/section cursor in an SDK application.")
		return
	}
	if err != nil {
		log.Fatal(err)
	}
}
