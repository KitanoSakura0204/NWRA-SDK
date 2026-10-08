package nowork

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// AccountClient uses the site's login JWT for the player portal. OAuth access
// tokens and X-Server-Key cannot be used for these account routes.
type AccountClient struct{ transport *Client }

// NewAccount creates a portal client with a 10-second default timeout. A custom
// HTTP client retains its timeout/transport; redirects are rejected to protect credentials.
func NewAccount(base, token string, httpClient *http.Client) *AccountClient {
	c := New(base)
	c.account = true
	if httpClient != nil {
		copy := *httpClient
		c.http = &copy
	}
	c.http.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return errors.New("nowork: account redirect refused") }
	c.headers = make(http.Header)
	if token = strings.TrimSpace(token); token != "" {
		c.headers.Set("Authorization", "Bearer "+token)
	}
	return &AccountClient{transport: c}
}

// responseLimit selects a cap for this request without changing the shared client.
// Archive exports contain up to 16 MiB of snapshots plus their policy/history metadata.
func (c *Client) responseLimit(method, path string) int64 {
	if !c.account || method != http.MethodGet {
		return defaultResponseLimit
	}
	parts := strings.Split(path, "/")
	if len(parts) != 8 || parts[0] != "" || parts[1] != "api" || parts[2] != "account" ||
		(parts[3] != "mihoyo" && parts[3] != "hypergryph") || parts[4] != "accounts" ||
		parts[6] != "archive" || parts[7] != "export" {
		return defaultResponseLimit
	}
	if id, err := strconv.ParseInt(parts[5], 10, 64); err != nil || id <= 0 {
		return defaultResponseLimit
	}
	return archiveExportResponseLimit
}

// PortalRoleInput binds a player identity or applies for a whitelist role.
type PortalRoleInput struct {
	RoleName string `json:"roleName"`
	RoleUUID string `json:"roleUuid,omitempty"`
	Remark   string `json:"remark,omitempty"`
}

// PortalRecord preserves the complete server/role DTO, including newer module fields.
type PortalRecord map[string]any

func (c *AccountClient) Servers(ctx context.Context) ([]PortalRecord, error) {
	var out []PortalRecord
	err := c.transport.do(ctx, http.MethodGet, "/api/account/portal/servers", nil, nil, &out)
	return out, err
}

func (c *AccountClient) WhitelistApplications(ctx context.Context) ([]PortalRecord, error) {
	var out []PortalRecord
	err := c.transport.do(ctx, http.MethodGet, "/api/account/whitelist/applications", nil, nil, &out)
	return out, err
}

func (c *AccountClient) WhitelistStatus(ctx context.Context, serverID int64) (PortalRecord, error) {
	return c.serverCall(ctx, http.MethodGet, serverID, "/whitelist-status", nil)
}

func (c *AccountClient) ApplyWhitelist(ctx context.Context, serverID int64, input PortalRoleInput) (PortalRecord, error) {
	return c.serverCall(ctx, http.MethodPost, serverID, "/whitelist-apply", input)
}

// BindRole also works when the server's whitelist mode is off.
func (c *AccountClient) BindRole(ctx context.Context, serverID int64, input PortalRoleInput) (PortalRecord, error) {
	return c.serverCall(ctx, http.MethodPost, serverID, "/roles", input)
}

func (c *AccountClient) serverCall(ctx context.Context, method string, serverID int64, suffix string, body any) (PortalRecord, error) {
	if serverID <= 0 {
		return nil, errors.New("nowork: server ID must be positive")
	}
	var out PortalRecord
	err := c.transport.do(ctx, method, "/api/account/portal/servers/"+strconv.FormatInt(serverID, 10)+suffix, nil, body, &out)
	return out, err
}

type GMLaunch struct {
	StartURL  string `json:"startUrl"`
	ExpiresIn int    `json:"expiresIn"`
}

func (c *AccountClient) LaunchGM(ctx context.Context, serverID, roleID int64) (*GMLaunch, error) {
	if serverID <= 0 || roleID <= 0 {
		return nil, errors.New("nowork: server and role IDs must be positive")
	}
	var out GMLaunch
	err := c.transport.do(ctx, http.MethodPost, "/api/account/gm/servers/"+strconv.FormatInt(serverID, 10)+"/launch", nil, map[string]int64{"roleId": roleID}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
