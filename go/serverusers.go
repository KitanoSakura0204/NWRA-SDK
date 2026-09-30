package lounge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ---- Player data and per-server bans (module "server-users", SDK/docs/SERVER_USERS.md) ----

// Business codes of the server-users routes (ApiError.Code); other refusals use code 1.
const (
	CodeNotOnRoster     = 404 // the uid has no binding and was never seen on this server
	CodeVersionConflict = 409 // the data changed since the version you read: read again, then write
	CodeAdminBan        = 403 // a ban an administrator placed: only an administrator can change it
)

// IsVersionConflict reports whether a data write was refused because of a stale version.
func IsVersionConflict(err error) bool {
	var api *ApiError
	return errors.As(err, &api) && api.Code == CodeVersionConflict
}

// UserFieldOption is one choice of a select field.
type UserFieldOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// UserField is one entry of the data schema the server owner defined.
type UserField struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Type        string            `json:"type"` // text | number | bool | select | json
	Description string            `json:"description,omitempty"`
	Visible     bool              `json:"visible"` // shown read-only to the player in the user centre
	Default     json.RawMessage   `json:"default,omitempty"`
	MaxLength   int               `json:"maxLength,omitempty"` // characters (text) or bytes (json)
	Min         *float64          `json:"min,omitempty"`
	Max         *float64          `json:"max,omitempty"`
	Integer     bool              `json:"integer,omitempty"`
	Options     []UserFieldOption `json:"options,omitempty"`
}

// UserSchema is GET /server/v1/users/schema.
type UserSchema struct {
	Fields       []UserField `json:"fields"`
	MaxUserBytes int         `json:"maxUserBytes"` // cap on one account's stored values (JSON bytes)
}

// UserData is one account's data on this server. Data holds every schema field: the stored
// value, else the field default, else null. Version is the optimistic lock (0 = nothing stored).
type UserData struct {
	UID       int64                      `json:"uid"`
	Data      map[string]json.RawMessage `json:"data"`
	Version   int64                      `json:"version"`
	UpdatedAt *string                    `json:"updatedAt"` // "2006-01-02 15:04:05" server time, nil before the first write
	UpdatedBy string                     `json:"updatedBy,omitempty"`
	Size      int                        `json:"size"`
}

// BanTarget names an account: its uid, or the name / uuid of one of its bindings on this server.
type BanTarget struct {
	UID  int64  `json:"uid,omitempty"`
	Name string `json:"name,omitempty"`
	UUID string `json:"uuid,omitempty"`
}

// BanOptions is the body of a ban: a reason plus ExpireTime ("2006-01-02 15:04:05" or RFC 3339)
// or DurationMinutes; neither means permanent.
type BanOptions struct {
	Reason          string `json:"reason,omitempty"`
	ExpireTime      string `json:"expireTime,omitempty"`
	DurationMinutes int64  `json:"durationMinutes,omitempty"`
}

// ServerBan is an active ban on this server with the account's bindings there.
type ServerBan struct {
	UID        int64    `json:"uid"`
	Names      []string `json:"names"`
	UUIDs      []string `json:"uuids"`
	Reason     string   `json:"reason"`
	ExpireTime *string  `json:"expireTime"` // nil = permanent
	CreateTime string   `json:"createTime"`
	Source     string   `json:"source"` // admin | owner | plugin
}

// KickEntry is an online player (from the last heartbeat) whose account is banned here.
type KickEntry struct {
	Name       string  `json:"name"`
	UUID       string  `json:"uuid,omitempty"`
	UID        int64   `json:"uid"`
	Reason     string  `json:"reason"`
	ExpireTime *string `json:"expireTime"`
}

// UserSchema reads the data schema (needs the module on for this server).
func (c *Client) UserSchema(ctx context.Context) (*UserSchema, error) {
	var s UserSchema
	if err := c.do(ctx, http.MethodGet, "/server/v1/users/schema", nil, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// UserData reads one account's data (uid from the join check or the whitelist).
func (c *Client) UserData(ctx context.Context, uid int64) (*UserData, error) {
	return c.userData(ctx, http.MethodGet, uid, nil)
}

// PutUserData replaces the account's stored values (fields left out are unset). version is
// the Version you last read; pass -1 to write unconditionally. A stale version fails with
// IsVersionConflict. Returns the data as stored.
func (c *Client) PutUserData(ctx context.Context, uid int64, data map[string]any, version int64) (*UserData, error) {
	return c.userData(ctx, http.MethodPut, uid, writeBody(data, version))
}

// PatchUserData merges values into the stored ones; a nil value deletes that field.
// version works as for PutUserData.
func (c *Client) PatchUserData(ctx context.Context, uid int64, data map[string]any, version int64) (*UserData, error) {
	return c.userData(ctx, http.MethodPatch, uid, writeBody(data, version))
}

func writeBody(data map[string]any, version int64) map[string]any {
	if data == nil {
		data = map[string]any{} // the server wants a data object, even an empty one
	}
	body := map[string]any{"data": data}
	if version >= 0 {
		body["version"] = version
	}
	return body
}

func (c *Client) userData(ctx context.Context, method string, uid int64, body any) (*UserData, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("lounge: uid must be positive, got %d", uid)
	}
	var d UserData
	if err := c.do(ctx, method, "/server/v1/users/"+strconv.FormatInt(uid, 10)+"/data", nil, body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Bans lists the active bans on this server; a zero target lists all of them (at most 5000,
// newest first), otherwise those of the matching accounts. Works with the module off too.
func (c *Client) Bans(ctx context.Context, only BanTarget) ([]ServerBan, error) {
	q := url.Values{}
	if only.UID > 0 {
		q.Set("uid", strconv.FormatInt(only.UID, 10))
	}
	if only.Name != "" {
		q.Set("name", only.Name)
	}
	if only.UUID != "" {
		q.Set("uuid", only.UUID)
	}
	var out struct {
		Bans []ServerBan `json:"bans"`
	}
	if err := c.do(ctx, http.MethodGet, "/server/v1/bans", q, nil, &out); err != nil {
		return nil, err
	}
	if out.Bans == nil {
		out.Bans = []ServerBan{}
	}
	return out.Bans, nil
}

// KickList names the online players (as of the last heartbeat) whose account is banned here:
// disconnect them. Poll it after each heartbeat.
func (c *Client) KickList(ctx context.Context) ([]KickEntry, error) {
	var out struct {
		Players []KickEntry `json:"players"`
	}
	if err := c.do(ctx, http.MethodGet, "/server/v1/bans/kick", nil, nil, &out); err != nil {
		return nil, err
	}
	if out.Players == nil {
		out.Players = []KickEntry{}
	}
	return out.Players, nil
}

// Ban bans an account on this server (module on and its pluginBans setting allowed). A ban an
// administrator placed cannot be replaced. Returns the account uid and the ban now in force.
func (c *Client) Ban(ctx context.Context, target BanTarget, opts BanOptions) (int64, *ServerBan, error) {
	body := struct {
		BanTarget
		BanOptions
	}{target, opts}
	var out struct {
		UID int64      `json:"uid"`
		Ban *ServerBan `json:"ban"`
	}
	if err := c.do(ctx, http.MethodPost, "/server/v1/bans", nil, body, &out); err != nil {
		return 0, nil, err
	}
	return out.UID, out.Ban, nil
}

// Unban lifts the account's active ban on this server (not one an administrator placed).
func (c *Client) Unban(ctx context.Context, target BanTarget, reason string) (int64, error) {
	body := struct {
		BanTarget
		Reason string `json:"reason,omitempty"`
	}{target, reason}
	var out struct {
		UID int64 `json:"uid"`
	}
	if err := c.do(ctx, http.MethodPost, "/server/v1/bans/lift", nil, body, &out); err != nil {
		return 0, err
	}
	return out.UID, nil
}
