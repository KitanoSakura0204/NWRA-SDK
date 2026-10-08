package nowork

import (
	"context"
	"net/http"
)

// GMCharacter is the server-local identity granted by character SSO. It does not
// grant administrator permissions or expose the account's private profile.
type GMCharacter struct {
	ID       int64  `json:"id"`
	RoleName string `json:"roleName"`
	RoleUUID string `json:"roleUuid"`
}

// GMSession is returned only after a one-use, PKCE-bound code is exchanged.
type GMSession struct {
	ServerID        int64       `json:"serverId"`
	ServerCode      string      `json:"serverCode"`
	Subject         string      `json:"subject"`
	Character       GMCharacter `json:"character"`
	AuthenticatedAt int64       `json:"authenticatedAt"`
	Scope           string      `json:"scope"`
}

// ExchangeGMCode is a backend-only exchange. First validate the callback state
// against the GM browser session; never place the server key or verifier in a URL.
// Authorization failures (including consumed/expired codes) return *ApiError 403.
func (c *Client) ExchangeGMCode(ctx context.Context, code, verifier, redirectURI string) (*GMSession, error) {
	var out GMSession
	body := map[string]string{"code": code, "codeVerifier": verifier, "redirectUri": redirectURI}
	if err := c.do(ctx, http.MethodPost, "/server/v1/gm/sso/exchange", nil, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
