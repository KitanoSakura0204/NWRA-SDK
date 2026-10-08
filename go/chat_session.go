package nowork

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
)

// ChatClient uses Authorization: Chat <token>. NewSiteChat instead accepts a site
// JWT for site-session creation or direct account chat access.
type ChatClient struct{ transport *Client }

func NewChat(base, token string, httpClient *http.Client) *ChatClient {
	c := NewAccount(base, token, httpClient).transport
	c.headers = http.Header{}
	if token != "" {
		c.headers.Set("Authorization", "Chat "+token)
	}
	return &ChatClient{transport: c}
}

func NewSiteChat(base, siteJWT string, httpClient *http.Client) *ChatClient {
	return &ChatClient{transport: NewAccount(base, siteJWT, httpClient).transport}
}

type ChatSession struct {
	Token     string  `json:"token"`
	ExpiresAt string  `json:"expiresAt"`
	Via       string  `json:"via"`
	Channels  []int64 `json:"channels"`
	ServerID  int64   `json:"serverId"`
	Player    string  `json:"player"`
}

// RedeemTicket returns a session token to use with NewChat. Never retry a consumed ticket.
func (c *ChatClient) RedeemTicket(ctx context.Context, ticket, server string) (*ChatSession, error) {
	return c.session(ctx, "/session", map[string]string{"ticket": ticket, "server": server})
}

func (c *ChatClient) SiteSession(ctx context.Context) (*ChatSession, error) {
	return c.session(ctx, "/session/site", map[string]string{})
}

func (c *ChatClient) session(ctx context.Context, path string, body any) (*ChatSession, error) {
	var out ChatSession
	if err := c.transport.do(ctx, http.MethodPost, "/api/chat"+path, nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *ChatClient) EndSession(ctx context.Context) error {
	return c.transport.do(ctx, http.MethodDelete, "/api/chat/session", nil, nil, nil)
}

func (c *ChatClient) Me(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := c.transport.do(ctx, http.MethodGet, "/api/chat/me", nil, nil, &out)
	return out, err
}

func (c *ChatClient) Channels(ctx context.Context) ([]map[string]any, error) {
	var out []map[string]any
	err := c.transport.do(ctx, http.MethodGet, "/api/chat/channels", nil, nil, &out)
	return out, err
}

// History preserves all message fields including the caller's ownReactions.
func (c *ChatClient) History(ctx context.Context, channelID, before, after int64, limit int) ([]map[string]any, error) {
	path, err := chatChannelPath(channelID, "/messages")
	if err != nil {
		return nil, err
	}
	query := url.Values{"before": {strconv.FormatInt(before, 10)}, "after": {strconv.FormatInt(after, 10)}, "limit": {strconv.Itoa(limit)}}
	var out []map[string]any
	err = c.transport.do(ctx, http.MethodGet, path, query, nil, &out)
	return out, err
}

type ChatSend struct {
	Content string `json:"content"`
	Kind    string `json:"kind,omitempty"`
	Extra   string `json:"extra,omitempty"`
	ReplyTo int64  `json:"replyTo,omitempty"`
	Nonce   string `json:"nonce,omitempty"` // reuse for retrying the same message
}

func (c *ChatClient) SendMessage(ctx context.Context, channelID int64, input ChatSend) (map[string]any, error) {
	path, err := chatChannelPath(channelID, "/messages")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	err = c.transport.do(ctx, http.MethodPost, path, nil, input, &out)
	return out, err
}

func (c *ChatClient) MarkRead(ctx context.Context, channelID, messageID int64) error {
	path, err := chatChannelPath(channelID, "/read")
	if err != nil {
		return err
	}
	return c.transport.do(ctx, http.MethodPost, path, nil, map[string]int64{"id": messageID}, nil)
}

func chatChannelPath(channelID int64, suffix string) (string, error) {
	if channelID <= 0 {
		return "", errors.New("nowork: channel ID must be positive")
	}
	return "/api/chat/channels/" + strconv.FormatInt(channelID, 10) + suffix, nil
}
