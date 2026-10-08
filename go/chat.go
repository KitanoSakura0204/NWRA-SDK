package nowork

import (
	"context"
	"encoding/json"
	"net/http"
)

// ChatTicketInput identifies the player by exactly one of AccessToken or UID.
// UID issuance must be enabled in the server's chat configuration.
type ChatTicketInput struct {
	AccessToken string  `json:"accessToken,omitempty"`
	UID         int64   `json:"uid,omitempty"`
	PlayerName  string  `json:"playerName,omitempty"`
	PlayerUUID  string  `json:"playerUuid,omitempty"`
	Channels    []int64 `json:"channels,omitempty"`
}

type ChatTicket struct {
	Ticket    string  `json:"ticket"`
	ExpiresAt string  `json:"expiresAt"`
	Channels  []int64 `json:"channels"`
	EmbedPath string  `json:"embedPath"`
	WSPath    string  `json:"wsPath"`
}

type ChatRelayItem struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	ChannelID int64           `json:"channelId"`
	Payload   json.RawMessage `json:"payload"`
	Attempts  int             `json:"attempts"`
}

type ChatRelayFailure struct {
	ID    int64  `json:"id"`
	Error string `json:"error"`
}

type ChatPost struct {
	Player     string `json:"player"`
	PlayerUUID string `json:"playerUuid,omitempty"`
	UID        int64  `json:"uid,omitempty"`
	Text       string `json:"text"`
	Kind       string `json:"kind,omitempty"`
}

// IssueChatTicket creates a one-use chat ticket. Do not log tickets or embed URLs.
func (c *Client) IssueChatTicket(ctx context.Context, input ChatTicketInput) (*ChatTicket, error) {
	var out ChatTicket
	if err := c.do(ctx, http.MethodPost, "/server/v1/chat/tickets", nil, input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) PollChatRelay(ctx context.Context) ([]ChatRelayItem, error) {
	var out struct {
		Items []ChatRelayItem `json:"items"`
	}
	err := c.do(ctx, http.MethodGet, "/server/v1/chat/relay", nil, nil, &out)
	return out.Items, err
}

// AckChatRelay acknowledges successful delivery only. Failed entries are permanent
// failures; leave transient failures unacknowledged for redelivery.
func (c *Client) AckChatRelay(ctx context.Context, done []int64, failed []ChatRelayFailure) (int64, error) {
	var out struct {
		Updated int64 `json:"updated"`
	}
	err := c.do(ctx, http.MethodPost, "/server/v1/chat/relay/ack", nil, map[string]any{"done": done, "failed": failed}, &out)
	return out.Updated, err
}

func (c *Client) PostChatMessage(ctx context.Context, input ChatPost) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodPost, "/server/v1/chat/messages", nil, input, &out)
	return out, err
}
