package nowork

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Game data uses the normalized gamedata contract, not raw publisher responses.
// Fields retain their individual source, visibility, export permission and status.
// Public encyclopedia entities do not imply ownership or inventory quantities.

type GameLocalizedText map[string]string
type GameSourceKind string
type GameAuthScope string
type GameDataStatus string
type GameVisibility string
type GameValueKind string
type GameEntityKind string

const (
	GameSourceOfficial          GameSourceKind = "official"
	GameSourceCommunity         GameSourceKind = "community"
	GameSourceUser              GameSourceKind = "user"
	GameAuthPublic              GameAuthScope  = "public"
	GameAuthAccount             GameAuthScope  = "account"
	GameAuthRole                GameAuthScope  = "role"
	GameStatusOK                GameDataStatus = "ok"
	GameStatusPartial           GameDataStatus = "partial"
	GameStatusUnavailable       GameDataStatus = "unavailable"
	GameStatusUnsupported       GameDataStatus = "unsupported"
	GameStatusAuthRequired      GameDataStatus = "auth_required"
	GameStatusPrivate           GameDataStatus = "private"
	GameStatusRateLimited       GameDataStatus = "rate_limited"
	GameStatusChallengeRequired GameDataStatus = "challenge_required"
	GameStatusStale             GameDataStatus = "stale"
	GameVisibilityOwner         GameVisibility = "owner"
	GameVisibilityShared        GameVisibility = "shared"
	GameVisibilityPublic        GameVisibility = "public"
	GameVisibilitySecret        GameVisibility = "secret"
	GameValueText               GameValueKind  = "text"
	GameValueNumber             GameValueKind  = "number"
	GameValueBoolean            GameValueKind  = "boolean"
	GameValueTimestamp          GameValueKind  = "timestamp"
	GameValueTextList           GameValueKind  = "text_list"
	GameEntityCharacter         GameEntityKind = "character"
	GameEntityItem              GameEntityKind = "item"
	GameEntityWeapon            GameEntityKind = "weapon"
	GameEntityEquipment         GameEntityKind = "equipment"
	GameEntityQuest             GameEntityKind = "quest"
	GameEntityAchievement       GameEntityKind = "achievement"
	GameEntityOther             GameEntityKind = "other"
)

// GameFieldPolicy is descriptive authorization metadata, never an SDK-side grant.
// Missing visibility means owner-only; Exportable does not grant sharing access.
type GameFieldPolicy struct {
	Visibility GameVisibility `json:"visibility"`
	Exportable bool           `json:"exportable"`
}

type GameProvenance struct {
	SourceID    string         `json:"source_id"`
	Kind        GameSourceKind `json:"kind"`
	Name        string         `json:"name"`
	URL         string         `json:"url,omitempty"`
	AuthScope   GameAuthScope  `json:"auth_scope"`
	FetchedAt   time.Time      `json:"fetched_at"`
	UpdatedAt   *time.Time     `json:"updated_at,omitempty"`
	License     string         `json:"license,omitempty"`
	Attribution string         `json:"attribution,omitempty"`
}

type GameIcon struct {
	URL      string            `json:"url"`
	Alt      GameLocalizedText `json:"alt,omitempty"`
	SourceID string            `json:"source_id,omitempty"`
	Width    int               `json:"width,omitempty"`
	Height   int               `json:"height,omitempty"`
}

type GameCapability struct {
	ID        string            `json:"id"`
	Name      GameLocalizedText `json:"name"`
	Supported bool              `json:"supported"`
	AuthScope GameAuthScope     `json:"auth_scope"`
	Status    GameDataStatus    `json:"status"`
	Reason    string            `json:"reason,omitempty"`
	Source    *GameProvenance   `json:"source,omitempty"`
}

type GameDescriptor struct {
	ID           string            `json:"id"`
	Provider     string            `json:"provider"`
	Name         GameLocalizedText `json:"name"`
	Icon         *GameIcon         `json:"icon,omitempty"`
	Regions      []string          `json:"regions,omitempty"`
	Capabilities []GameCapability  `json:"capabilities"`
}

// GameBoundRole.ID is an owned binding identifier, distinct from the upstream UID.
// The server derives account/site scope from authentication; clients cannot set it.
type GameBoundRole struct {
	ID       string          `json:"id"`
	GameID   string          `json:"game_id"`
	UID      string          `json:"uid"`
	Region   string          `json:"region"`
	Server   string          `json:"server,omitempty"`
	Nickname string          `json:"nickname,omitempty"`
	Level    *int            `json:"level,omitempty"`
	Icon     *GameIcon       `json:"icon,omitempty"`
	Policy   GameFieldPolicy `json:"policy"`
}

type GameField struct {
	Key  string            `json:"key"`
	Name GameLocalizedText `json:"name"`
	Kind GameValueKind     `json:"kind"`
	// RawMessage preserves absent, null, false, zero and large integers distinctly.
	// Values are scalars or string lists, never arbitrary upstream payloads.
	Value     json.RawMessage `json:"value,omitempty"`
	Unit      string          `json:"unit,omitempty"`
	Status    GameDataStatus  `json:"status"`
	Policy    GameFieldPolicy `json:"policy"`
	Source    *GameProvenance `json:"source,omitempty"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

// HasValue distinguishes omitted/null values from an observed zero or false.
// Inspect Status as well: a present value can be stale.
func (f GameField) HasValue() bool {
	v := strings.TrimSpace(string(f.Value))
	return v != "" && v != "null"
}

type GameEntity struct {
	ID     string            `json:"id"`
	Kind   GameEntityKind    `json:"kind"`
	Name   GameLocalizedText `json:"name"`
	Icon   *GameIcon         `json:"icon,omitempty"`
	Fields []GameField       `json:"fields"`
	Policy GameFieldPolicy   `json:"policy"`
	Source *GameProvenance   `json:"source,omitempty"`
}

type GameSection struct {
	ID         string            `json:"id"`
	Name       GameLocalizedText `json:"name"`
	Status     GameDataStatus    `json:"status"`
	Reason     string            `json:"reason,omitempty"`
	Fields     []GameField       `json:"fields"`
	Entities   []GameEntity      `json:"entities"`
	Policy     GameFieldPolicy   `json:"policy"`
	Source     *GameProvenance   `json:"source,omitempty"`
	UpdatedAt  *time.Time        `json:"updated_at,omitempty"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type GameSnapshot struct {
	GameID     string         `json:"game_id"`
	Role       *GameBoundRole `json:"role,omitempty"`
	Sections   []GameSection  `json:"sections"`
	FetchedAt  time.Time      `json:"fetched_at"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// GameArchiveEntry is a normalized saved snapshot, never an upstream raw payload.
type GameArchiveEntry struct {
	Provider  string         `json:"provider"`
	RoleID    string         `json:"role_id"`
	Version   int            `json:"version"`
	Revision  uint64         `json:"revision"`
	Status    GameDataStatus `json:"status"`
	Stale     bool           `json:"stale"`
	ExpiresAt time.Time      `json:"expires_at"`
	Snapshot  GameSnapshot   `json:"snapshot"`
}

// GameDataPage controls one page. Cursor is opaque and must be used with the
// same game, binding, section, locale and limit that produced it. Zero Limit
// leaves the provider default in force. No credentials or scope overrides exist.
type GameDataPage struct {
	Sections []string
	Cursor   string
	Limit    int
	Locale   string
}

type MihoyoGameDataOptions struct {
	GameDataPage
	RoleID int64 // /api/account/mihoyo/status roles[].id; zero for public sections
}

type HypergryphGameDataOptions struct {
	GameDataPage
	RoleID string // /api/account/hypergryph/games roles[].id
}

type HypergryphGameDiscovery struct {
	Status    string  `json:"status"`
	UpdatedAt *string `json:"updatedAt"`
	Reason    string  `json:"reason"`
}

type HypergryphGameAccount struct {
	Games          []GameDescriptor        `json:"games"`
	Roles          []GameBoundRole         `json:"roles"`
	Discovery      HypergryphGameDiscovery `json:"discovery"`
	SelectedRoleID string                  `json:"selectedRoleId"`
}

type HypergryphAccountStatus struct {
	Enabled      bool                      `json:"enabled"`
	AllowSMS     bool                      `json:"allowSms"`
	AllowToken   bool                      `json:"allowToken"`
	ServerAPI    bool                      `json:"serverApi"`
	TokenHelpURL string                    `json:"tokenHelpUrl"`
	Account      *HypergryphAccountBinding `json:"account"`
}

// HypergryphAccountBinding.ID is the owned account ID for archive requests.
// Credentials are intentionally absent from both the API response and this type.
type HypergryphAccountBinding struct {
	ID              int64                 `json:"id"`
	UserID          int64                 `json:"userId"`
	SklandUID       string                `json:"sklandUid"`
	HgUID           string                `json:"hgUid"`
	Nickname        string                `json:"nickname"`
	Avatar          string                `json:"avatar"`
	Method          string                `json:"method"`
	Share           bool                  `json:"share"`
	Status          string                `json:"status"`
	LastError       string                `json:"lastError"`
	LastSync        *string               `json:"lastSync"`
	SelectedRoleID  string                `json:"selectedRoleId"`
	DiscoveryStatus string                `json:"discoveryStatus"`
	DiscoveryAt     *string               `json:"discoveryAt"`
	CreateTime      string                `json:"createTime"`
	UpdateTime      string                `json:"updateTime"`
	Roles           []HypergryphOwnedRole `json:"roles"`
}

type HypergryphOwnedRole struct {
	HypergryphSharedRole
	ID         int64  `json:"id"`
	AccountID  int64  `json:"accountId"`
	UserID     int64  `json:"userId"`
	ChannelID  string `json:"channelId"`
	SyncError  string `json:"syncError"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

func (c *AccountClient) HypergryphStatus(ctx context.Context) (*HypergryphAccountStatus, error) {
	var out HypergryphAccountStatus
	if err := c.transport.do(ctx, http.MethodGet, "/api/account/hypergryph", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MihoyoAccountStatus supplies the numeric owned role IDs used by game requests.
// Legacy portal timestamps are strings (server-local format), unlike normalized
// snapshot/provenance timestamps, which are RFC 3339 and use time.Time.
type MihoyoAccountStatus struct {
	AccountID     int64                 `json:"accountId,omitempty"`
	Region        string                `json:"region"`
	Regions       []MihoyoAccountRegion `json:"regions"`
	Recovery      MihoyoRecovery        `json:"recovery"`
	GameCatalog   []GameDescriptor      `json:"gameCatalog"`
	Enabled       bool                  `json:"enabled"`
	AllowQR       bool                  `json:"allowQr"`
	AllowCookie   bool                  `json:"allowCookie"`
	RecordEnabled bool                  `json:"recordEnabled"`
	Bound         bool                  `json:"bound"`
	Profile       *MihoyoProfile        `json:"profile"`
	Roles         []MihoyoOwnedRole     `json:"roles"`
	LastSync      *string               `json:"lastSync"`
	NextSyncAt    *string               `json:"nextSyncAt"`
	Games         []MihoyoGameOption    `json:"games"`
	Method        string                `json:"method,omitempty"`
	State         string                `json:"state,omitempty"`
	StateMessage  string                `json:"stateMessage,omitempty"`
}

type MihoyoProfile struct {
	UID       string `json:"uid"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Introduce string `json:"introduce"`
}

type MihoyoOwnedRole struct {
	ID         int64   `json:"id"`
	GameBiz    string  `json:"gameBiz"`
	GameID     string  `json:"gameId"`
	GameName   string  `json:"gameName"`
	Region     string  `json:"region"`
	RegionName string  `json:"regionName"`
	GameUID    string  `json:"gameUid"`
	Nickname   string  `json:"nickname"`
	Level      int     `json:"level"`
	IsChosen   bool    `json:"isChosen"`
	SyncTime   *string `json:"syncTime"`
}

type MihoyoAccountRegion struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	QR     bool   `json:"qr"`
	Cookie bool   `json:"cookie"`
}

// Recovery identifies the provider's interactive recovery flow; SDK reads do
// not retry, bypass or solve upstream verification challenges.
type MihoyoRecovery struct {
	Action string `json:"action"`
	URL    string `json:"url,omitempty"`
}

type MihoyoGameOption struct {
	GameBiz string `json:"gameBiz"`
	Name    string `json:"name"`
}

func (c *AccountClient) MihoyoStatus(ctx context.Context) (*MihoyoAccountStatus, error) {
	var out MihoyoAccountStatus
	if err := c.transport.do(ctx, http.MethodGet, "/api/account/mihoyo/status", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

var gameDataID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

func gameDataQuery(page GameDataPage, maxLimit int) (url.Values, error) {
	if page.Limit < 0 || page.Limit > maxLimit || len(page.Cursor) > 2048 || len(page.Sections) > 32 || len(page.Locale) > 32 {
		return nil, errors.New("nowork: invalid game-data pagination options")
	}
	q := url.Values{}
	for _, section := range page.Sections {
		if !gameDataID.MatchString(section) {
			return nil, errors.New("nowork: invalid game-data section")
		}
	}
	if len(page.Sections) > 0 {
		q.Set("sections", strings.Join(page.Sections, ","))
	}
	if page.Cursor != "" {
		q.Set("cursor", page.Cursor)
	}
	if page.Limit != 0 {
		q.Set("limit", strconv.Itoa(page.Limit))
	}
	if page.Locale != "" {
		q.Set("locale", page.Locale)
	}
	return q, nil
}

// MihoyoGames returns the configured game descriptors and their actual capabilities.
// These account methods require the site's JWT, not an OAuth token or server key.
func (c *AccountClient) MihoyoGames(ctx context.Context) ([]GameDescriptor, error) {
	var out []GameDescriptor
	err := c.transport.do(ctx, http.MethodGet, "/api/account/mihoyo/games", nil, nil, &out)
	return out, err
}

func (c *AccountClient) MihoyoGameData(ctx context.Context, gameID string, options MihoyoGameDataOptions) (*GameSnapshot, error) {
	if !gameDataID.MatchString(gameID) || len(gameID) > 64 || options.RoleID < 0 {
		return nil, errors.New("nowork: invalid game or owned role ID")
	}
	q, err := gameDataQuery(options.GameDataPage, 100)
	if err != nil {
		return nil, err
	}
	if options.RoleID != 0 {
		q.Set("roleId", strconv.FormatInt(options.RoleID, 10))
	}
	var out GameSnapshot
	if err = c.transport.do(ctx, http.MethodGet, "/api/account/mihoyo/games/"+url.PathEscape(gameID)+"/data", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AccountClient) HypergryphGames(ctx context.Context) (*HypergryphGameAccount, error) {
	var out HypergryphGameAccount
	if err := c.transport.do(ctx, http.MethodGet, "/api/account/hypergryph/games", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AccountClient) HypergryphGameData(ctx context.Context, gameID string, options HypergryphGameDataOptions) (*GameSnapshot, error) {
	if !gameDataID.MatchString(gameID) || len(gameID) > 64 || (options.RoleID != "" && !gameDataID.MatchString(options.RoleID)) {
		return nil, errors.New("nowork: invalid game or owned role ID")
	}
	q, err := gameDataQuery(options.GameDataPage, 100)
	if err != nil {
		return nil, err
	}
	if options.RoleID != "" {
		q.Set("roleId", options.RoleID)
	}
	var out GameSnapshot
	if err = c.transport.do(ctx, http.MethodGet, "/api/account/hypergryph/games/"+url.PathEscape(gameID)+"/snapshot", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SelectHypergryphRole persists an owned binding selection. The server verifies
// ownership; an upstream UID cannot substitute for the discovered role ID.
func (c *AccountClient) SelectHypergryphRole(ctx context.Context, roleID string) error {
	if !gameDataID.MatchString(roleID) {
		return errors.New("nowork: invalid owned role ID")
	}
	return c.transport.do(ctx, http.MethodPut, "/api/account/hypergryph/role", nil, map[string]string{"roleId": roleID}, nil)
}

// ErrGamePageLimit means the requested page budget ended with more data available.
// Delivered pages remain valid; callers can resume from their last returned cursor.
var ErrGamePageLimit = errors.New("nowork: game-data page limit reached")
var ErrGameCursorLoop = errors.New("nowork: game-data cursor repeated")

// WalkMihoyoGameData visits pages without accumulating private data in SDK state.
// Select exactly one section: different sections can have independent cursors.
// maxPages is mandatory (1..1000). Callback errors and cancellation stop fetching.
func (c *AccountClient) WalkMihoyoGameData(ctx context.Context, gameID string, options MihoyoGameDataOptions, maxPages int, visit func(*GameSnapshot) error) error {
	return walkGameData(ctx, options.GameDataPage, maxPages, visit, func(cursor string) (*GameSnapshot, error) {
		options.Cursor = cursor
		return c.MihoyoGameData(ctx, gameID, options)
	})
}

func (c *AccountClient) WalkHypergryphGameData(ctx context.Context, gameID string, options HypergryphGameDataOptions, maxPages int, visit func(*GameSnapshot) error) error {
	return walkGameData(ctx, options.GameDataPage, maxPages, visit, func(cursor string) (*GameSnapshot, error) {
		options.Cursor = cursor
		return c.HypergryphGameData(ctx, gameID, options)
	})
}

func walkGameData(ctx context.Context, options GameDataPage, maxPages int, visit func(*GameSnapshot) error, fetch func(string) (*GameSnapshot, error)) error {
	if len(options.Sections) != 1 || maxPages < 1 || maxPages > 1000 || visit == nil {
		return errors.New("nowork: pagination needs one section, a callback and maxPages in 1..1000")
	}
	cursor := options.Cursor
	seen := map[string]bool{}
	if cursor != "" {
		seen[cursor] = true
	}
	for page := 0; page < maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		snapshot, err := fetch(cursor)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return &TransportError{errors.New("missing game snapshot")}
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		// Read the cursor before callbacks so their edits cannot change the request.
		next := snapshot.NextCursor
		if next == "" {
			for _, section := range snapshot.Sections {
				if section.ID == options.Sections[0] {
					next = section.NextCursor
					break
				}
			}
		}
		if err = visit(snapshot); err != nil {
			return err
		}
		if next == "" {
			return ctx.Err()
		}
		if len(next) > 2048 {
			return &TransportError{errors.New("game-data cursor exceeds limit")}
		}
		if seen[next] {
			return ErrGameCursorLoop
		}
		seen[next] = true
		cursor = next
	}
	return ErrGamePageLimit
}

// SharedHypergryphRoles reads the legacy consent-filtered Arknights summary for
// a site UID, using X-Server-Key. The backend also requires an approved binding
// on this server and enabled module/API/sharing settings. Refusals remain errors.
// There is no rich multi-game server-key route or miHoYo server-key route.
func (c *Client) SharedHypergryphRoles(ctx context.Context, uid int64) (*HypergryphSharedRoles, error) {
	if uid <= 0 {
		return nil, fmt.Errorf("nowork: site UID must be positive")
	}
	q := url.Values{"uid": {strconv.FormatInt(uid, 10)}}
	var out HypergryphSharedRoles
	if err := c.do(ctx, http.MethodGet, "/server/v1/hypergryph/roles", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type HypergryphSharedRoles struct {
	UID        int64                  `json:"uid"`
	Status     string                 `json:"status"`
	LastSync   *string                `json:"lastSync"`
	Roles      []HypergryphSharedRole `json:"roles"`
	ServerTime int64                  `json:"serverTime"`
}

type HypergryphSharedRole struct {
	UID         string             `json:"uid"`
	Nickname    string             `json:"nickname"`
	ChannelName string             `json:"channelName"`
	IsOfficial  bool               `json:"isOfficial"`
	IsDefault   bool               `json:"isDefault"`
	Level       int                `json:"level"`
	APNow       int                `json:"apNow"`
	Summary     *HypergryphSummary `json:"summary"`
	LastSync    *string            `json:"lastSync"`
}

// Legacy summary fields mirror the consent-filtered server endpoint. Optional
// sections remain nil; legacy scalar zero can mean absent or unknown upstream.
type HypergryphOperator struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Rarity     int               `json:"rarity"` // 1-6 stars, 0 = unknown
	Class      string            `json:"class"`  // profession code as Skland sends it (WARRIOR, SNIPER, ...)
	Elite      int               `json:"elite"`  // evolve phase 0-2
	Level      int               `json:"level"`
	Potential  int               `json:"potential"`  // potentialRank 0-5 (the page shows it plus one)
	SkillLevel int               `json:"skillLevel"` // shared skill rank 1-7
	Skills     []HypergryphSkill `json:"skills"`
	Equips     []HypergryphEquip `json:"equips"` // modules (模组)
	Favor      int               `json:"favor"`  // trust percent
	GainTs     int64             `json:"gainTs"`
}

// HypergryphSkill is one skill of an operator; Spec is the mastery (专精) 0-3.
type HypergryphSkill struct {
	ID   string `json:"id"`
	Spec int    `json:"spec"`
}

// HypergryphEquip is one module of an operator (names come from equipmentInfoMap when present).
type HypergryphEquip struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Level   int    `json:"level"`
	Default bool   `json:"default"` // the module currently equipped
}

// HypergryphSummary is the part of the player info this module keeps: every section the user page shows.
// Sections the answer lacks stay nil / empty, so the page can show "no data" per section.
type HypergryphSummary struct {
	UID           string               `json:"uid"`
	Name          string               `json:"name"`
	Level         int                  `json:"level"`
	AvatarURL     string               `json:"avatarUrl"` // https only; "" when the answer carries none
	AvatarID      string               `json:"avatarId"`
	Resume        string               `json:"resume"`
	RegisterTs    int64                `json:"registerTs"`
	LastOnlineTs  int64                `json:"lastOnlineTs"`
	MainStage     string               `json:"mainStage"` // stage code of the main-story progress ("" = cleared or unknown)
	Secretary     string               `json:"secretary"`
	APCurrent     int                  `json:"apCurrent"`
	APMax         int                  `json:"apMax"`
	APLastAddTs   int64                `json:"apLastAddTs"`
	APCompleteTs  int64                `json:"apCompleteTs"`
	CharCount     int                  `json:"charCount"`
	SkinCount     int                  `json:"skinCount"`
	FurnitureCnt  int                  `json:"furnitureCount"`
	Medals        int                  `json:"medals"`
	Daily         *HypergryphProgress  `json:"daily"`
	Weekly        *HypergryphProgress  `json:"weekly"`
	Campaign      *HypergryphCampaign  `json:"campaign"`
	Tower         *HypergryphTower     `json:"tower"`
	Rogue         []HypergryphRogue    `json:"rogue"`
	Recruit       []HypergryphRecruit  `json:"recruit"`
	Building      *HypergryphBuilding  `json:"building"`
	Assist        []HypergryphOperator `json:"assist"`
	Roster        []HypergryphOperator `json:"roster"`
	RosterTrimmed bool                 `json:"rosterTrimmed"`
}

type HypergryphProgress struct {
	Current int `json:"current"`
	Total   int `json:"total"`
}

// HypergryphRecord is one named entry of a mode (an annihilation map, a tower, a stage).
type HypergryphRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Value   int    `json:"value"` // best kills (剿灭) or best level (保全派驻)
	Picture string `json:"picture"`
}

// HypergryphCampaign is 剿灭作战: the weekly reward progress and the best result per map.
type HypergryphCampaign struct {
	Reward  HypergryphProgress `json:"reward"`
	Records []HypergryphRecord `json:"records"`
}

// HypergryphTower is 保全派驻: the season reward progress and the best level per tower.
type HypergryphTower struct {
	Higher  HypergryphProgress `json:"higher"`
	Lower   HypergryphProgress `json:"lower"`
	Ts      int64              `json:"ts"` // season end
	Records []HypergryphRecord `json:"records"`
}

// HypergryphRogue is one integrated strategies (集成战略) theme.
type HypergryphRogue struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Picture    string `json:"picture"`
	Relics     int    `json:"relics"` // collected relics (藏品)
	Bank       int    `json:"bank"`   // banked points this season
	BankRecord int    `json:"bankRecord"`
}

// HypergryphRecruit is one public recruitment slot; State is Skland's raw value (0 locked, 1 idle,
// 2 recruiting as documented by the community), the page reads it together with FinishTs.
type HypergryphRecruit struct {
	State    int   `json:"state"`
	StartTs  int64 `json:"startTs"`
	FinishTs int64 `json:"finishTs"`
}

// HypergryphRoom is one base facility; Value / Limit depend on the kind (goods made / capacity,
// orders in stock / limit, clues on the board / 7).
type HypergryphRoom struct {
	Kind  string `json:"kind"` // control, power, manufacture, trading, dormitory, meeting, hire
	Level int    `json:"level"`
	Chars int    `json:"chars"`
	Value int    `json:"value"`
	Limit int    `json:"limit"`
}

// HypergryphBuilding is the base (基建) overview.
type HypergryphBuilding struct {
	ControlLevel   int              `json:"controlLevel"`
	Labor          int              `json:"labor"` // drones at LaborTs
	LaborMax       int              `json:"laborMax"`
	LaborRemainSec int64            `json:"laborRemainSec"` // seconds to full at LaborTs
	LaborTs        int64            `json:"laborTs"`
	Tired          int              `json:"tired"` // operators out of morale
	HireRefresh    int              `json:"hireRefresh"`
	Rooms          []HypergryphRoom `json:"rooms"`
}

type GameProvider string

const (
	GameProviderMihoyo     GameProvider = "mihoyo"
	GameProviderHypergryph GameProvider = "hypergryph"
)

type GameArchiveMetadata struct {
	GameID    string         `json:"game_id"`
	RoleID    string         `json:"role_id"`
	Version   int            `json:"version"`
	Revision  uint64         `json:"revision"`
	Status    GameDataStatus `json:"status"`
	Stale     bool           `json:"stale"`
	Bytes     int            `json:"bytes"`
	FetchedAt time.Time      `json:"fetched_at"`
	ExpiresAt time.Time      `json:"expires_at"`
}

type GameArchiveHistoryEntry struct {
	ID        uint64         `json:"id"`
	GameID    string         `json:"game_id"`
	RoleID    string         `json:"role_id"`
	Revision  uint64         `json:"revision"`
	Status    GameDataStatus `json:"status"`
	Bytes     int            `json:"bytes"`
	CreatedAt time.Time      `json:"created_at"`
}

// GameArchivePolicy describes a server-enforced restriction. Account clients
// can read it; changing policies requires separate administrator permissions.
type GameArchivePolicy struct {
	ID         uint64         `json:"id"`
	GameID     string         `json:"game_id"`
	Path       string         `json:"path"`
	Visibility GameVisibility `json:"visibility"`
	Exportable bool           `json:"exportable"`
	Revision   uint64         `json:"revision"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type GameArchivePolicyAuditEntry struct {
	ID        uint64    `json:"id"`
	GameID    string    `json:"game_id"`
	Path      string    `json:"path"`
	Revision  uint64    `json:"revision"`
	Previous  string    `json:"previous"`
	Current   string    `json:"current"`
	CreatedAt time.Time `json:"created_at"`
}

// GameArchiveExport preserves the server's export redaction. Scope describes
// the exported binding only; no request accepts a user/site scope override.
type GameArchiveExport struct {
	Scope struct {
		SiteID    uint64 `json:"site_id"`
		UserID    uint64 `json:"user_id"`
		AccountID uint64 `json:"account_id"`
	} `json:"scope"`
	Provider    GameProvider                  `json:"provider"`
	ExportedAt  time.Time                     `json:"exported_at"`
	Snapshots   []*GameArchiveEntry           `json:"snapshots"`
	History     []GameArchiveHistoryEntry     `json:"history"`
	Policies    []GameArchivePolicy           `json:"policies"`
	PolicyAudit []GameArchivePolicyAuditEntry `json:"policy_audit"`
}

// GameArchivePage pages history/audit backwards by ID. For another page, set
// Before to the last returned ID; stop on an empty page. Zero Limit defaults to 50.
type GameArchivePage struct {
	Before uint64
	Limit  int
}

func gameArchivePath(provider GameProvider, accountID int64) (string, error) {
	if accountID <= 0 || (provider != GameProviderMihoyo && provider != GameProviderHypergryph) {
		return "", errors.New("nowork: archive needs a supported provider and positive owned account ID")
	}
	return "/api/account/" + string(provider) + "/accounts/" + strconv.FormatInt(accountID, 10) + "/archive", nil
}

func gameArchivePageQuery(page GameArchivePage) (url.Values, error) {
	if page.Limit < 0 || page.Limit > 100 {
		return nil, errors.New("nowork: archive limit must be 1..100, or zero for the default")
	}
	q := url.Values{}
	if page.Before != 0 {
		q.Set("before", strconv.FormatUint(page.Before, 10))
	}
	if page.Limit != 0 {
		q.Set("limit", strconv.Itoa(page.Limit))
	}
	return q, nil
}

// GameArchiveSnapshots lists bounded metadata for one owned publisher account.
// accountID comes from the provider's binding status, not an upstream game UID.
func (c *AccountClient) GameArchiveSnapshots(ctx context.Context, provider GameProvider, accountID int64) ([]GameArchiveMetadata, error) {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return nil, err
	}
	var out []GameArchiveMetadata
	err = c.transport.do(ctx, http.MethodGet, path+"/snapshots", nil, nil, &out)
	return out, err
}

func (c *AccountClient) GameArchiveSnapshot(ctx context.Context, provider GameProvider, accountID int64, gameID, roleID string) (*GameArchiveEntry, error) {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return nil, err
	}
	if !gameDataID.MatchString(gameID) || len(gameID) > 64 || (roleID != "" && !gameDataID.MatchString(roleID)) {
		return nil, errors.New("nowork: invalid archive game or role ID")
	}
	q := url.Values{"game_id": {gameID}}
	if roleID != "" {
		q.Set("role_id", roleID)
	}
	var out GameArchiveEntry
	if err = c.transport.do(ctx, http.MethodGet, path+"/snapshots", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AccountClient) GameArchiveHistory(ctx context.Context, provider GameProvider, accountID int64, page GameArchivePage) ([]GameArchiveHistoryEntry, error) {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return nil, err
	}
	q, err := gameArchivePageQuery(page)
	if err != nil {
		return nil, err
	}
	var out []GameArchiveHistoryEntry
	err = c.transport.do(ctx, http.MethodGet, path+"/history", q, nil, &out)
	return out, err
}

func (c *AccountClient) GameArchivePolicies(ctx context.Context, provider GameProvider, accountID int64, gameID string) ([]GameArchivePolicy, error) {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return nil, err
	}
	if !gameDataID.MatchString(gameID) || len(gameID) > 64 {
		return nil, errors.New("nowork: invalid archive game ID")
	}
	var out []GameArchivePolicy
	err = c.transport.do(ctx, http.MethodGet, path+"/policies", url.Values{"game_id": {gameID}}, nil, &out)
	return out, err
}

func (c *AccountClient) GameArchivePolicyAudit(ctx context.Context, provider GameProvider, accountID int64, page GameArchivePage) ([]GameArchivePolicyAuditEntry, error) {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return nil, err
	}
	q, err := gameArchivePageQuery(page)
	if err != nil {
		return nil, err
	}
	var out []GameArchivePolicyAuditEntry
	err = c.transport.do(ctx, http.MethodGet, path+"/policy-audit", q, nil, &out)
	return out, err
}

// ExportGameArchive asks the backend to apply export policies to this binding.
// Do not substitute marshaling an owner snapshot: its fields may be nonexportable.
func (c *AccountClient) ExportGameArchive(ctx context.Context, provider GameProvider, accountID int64) (*GameArchiveExport, error) {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return nil, err
	}
	var out GameArchiveExport
	if err = c.transport.do(ctx, http.MethodGet, path+"/export", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteGameArchive clears this binding's saved snapshots and fetch history.
// It preserves administrator policies, their audit history and the publisher binding.
// This mutation is never retried.
func (c *AccountClient) DeleteGameArchive(ctx context.Context, provider GameProvider, accountID int64) error {
	path, err := gameArchivePath(provider, accountID)
	if err != nil {
		return err
	}
	return c.transport.do(ctx, http.MethodDelete, path, nil, nil, nil)
}
