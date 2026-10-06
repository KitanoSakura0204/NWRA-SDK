package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	nowork "github.com/wangework/nowork-sdk-go"
)

func ptr(f float64) *float64 { return &f }

// catalog is what this server offers the GM panel (PROTOCOL 2.4). The "worlds" option list is
// published next to it, so the select for the time action always matches the loaded worlds.
// Labels are keyed by the admin console locale (zh_CN, zh_TW, en, ja); Title is the fallback.
func catalog() []nowork.GMAction {
	return []nowork.GMAction{
		{
			Key: "kick", Title: "踢出玩家", Description: "立即断开连接", Category: "players", Danger: "confirm",
			Labels: map[string]string{"en": "Kick player", "zh_TW": "踢出玩家", "ja": "プレイヤーをキック"},
			Params: []nowork.GMActionParam{
				{Name: "player", Type: "player", Required: true, Label: "玩家"},
				{Name: "reason", Type: "string", Label: "原因", Placeholder: "违反服务器规则", MaxLength: 200},
			},
		},
		{
			Key: "broadcast", Title: "全服广播", Category: "chat",
			Labels: map[string]string{"en": "Broadcast", "zh_TW": "全服廣播", "ja": "全体アナウンス"},
			Params: []nowork.GMActionParam{
				{Name: "message", Type: "text", Required: true, Label: "内容", MaxLength: 500},
				{Name: "repeat", Type: "number", Label: "重复次数", Min: ptr(1), Max: ptr(5), Integer: true, Default: json.RawMessage("1")},
			},
		},
		{
			Key: "time", Title: "设置时间", Category: "world",
			Labels: map[string]string{"en": "Set time", "zh_TW": "設定時間", "ja": "時間を設定"},
			Params: []nowork.GMActionParam{
				{Name: "world", Type: "select", Required: true, Label: "世界", Source: "worlds"},
				{Name: "value", Type: "select", Required: true, Label: "时间", Options: []nowork.GMOption{
					{Value: "day", Label: "白天"}, {Value: "night", Label: "夜晚"},
				}},
			},
		},
		{
			Key: "stop", Title: "关闭服务器", Description: "保存并停止", Category: "server", Danger: "critical",
			Labels: map[string]string{"en": "Stop server", "zh_TW": "關閉伺服器", "ja": "サーバーを停止"},
		},
	}
}

func worldOptions(w *world) map[string][]nowork.GMOption {
	names := w.worlds()
	opts := make([]nowork.GMOption, 0, len(names))
	for _, n := range names {
		opts = append(opts, nowork.GMOption{Value: n})
	}
	return map[string][]nowork.GMOption{"worlds": opts}
}

// publishGM sends the catalogue and the option lists; skipped entries are logged, not fatal.
func publishGM(ctx context.Context, api *nowork.Client, w *world, logf func(string, ...any)) error {
	res, err := api.PublishGMCatalog(ctx, catalog(), worldOptions(w))
	if err != nil {
		return err
	}
	logf("GM catalogue published: %d stored", res.Count)
	for _, s := range res.Skipped {
		logf("GM entry %q skipped: %s", s.Key, s.Reason)
	}
	return nil
}

// runGM executes one queued command. The error text becomes the failed result shown in the panel;
// the caller stops the server after reporting a successful "stop".
func runGM(w *world, cmd nowork.GMCommand, broadcast func(string)) (string, error) {
	var p struct {
		Player  string          `json:"player"`
		Reason  string          `json:"reason"`
		Message string          `json:"message"`
		Repeat  json.RawMessage `json:"repeat"`
		World   string          `json:"world"`
		Value   string          `json:"value"`
	}
	if len(cmd.Params) > 0 && string(cmd.Params) != "null" {
		if err := json.Unmarshal(cmd.Params, &p); err != nil {
			return "", fmt.Errorf("bad params: %v", err)
		}
	}
	switch cmd.Action {
	case "kick":
		if _, ok := w.remove(p.Player); !ok {
			return "", fmt.Errorf("%s is not online", p.Player)
		}
		return fmt.Sprintf("%s kicked (%s)", p.Player, orDefault(p.Reason, "no reason")), nil
	case "broadcast":
		msg := strings.TrimSpace(p.Message)
		if msg == "" {
			return "", fmt.Errorf("empty message")
		}
		n := 1
		// the panel sends numbers as JSON numbers; anything else keeps the default
		var f float64
		if json.Unmarshal(p.Repeat, &f) == nil && f >= 1 && f <= 5 {
			n = int(f)
		}
		for i := 0; i < n; i++ {
			broadcast(msg)
		}
		return fmt.Sprintf("broadcast sent %d time(s)", n), nil
	case "time":
		if p.Value != "day" && p.Value != "night" {
			return "", fmt.Errorf("time must be day or night")
		}
		if err := w.setTime(p.World, p.Value); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s set to %s", p.World, p.Value), nil
	case "stop":
		return "stopping", nil
	}
	return "", fmt.Errorf("unknown action %q", cmd.Action)
}

func orDefault(s, d string) string {
	if strings.TrimSpace(s) == "" {
		return d
	}
	return s
}
