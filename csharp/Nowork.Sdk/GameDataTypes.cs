using System.Text.Json;
using System.Text.Json.Serialization;

namespace NoWork.Sdk;

/// <summary>Provider names accepted by the existing archive routes.</summary>
public enum GameProvider { Mihoyo, Hypergryph }

// Status, scope, kind and game identifiers remain strings so newly installed
// adapters are not silently dropped by a client compiled against an older SDK.
public sealed record GameFieldPolicy(
    [property: JsonPropertyName("visibility")] string Visibility,
    [property: JsonPropertyName("exportable")] bool Exportable);

public sealed record GameProvenance(
    [property: JsonPropertyName("source_id")] string SourceId,
    [property: JsonPropertyName("kind")] string Kind,
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("auth_scope")] string AuthScope,
    [property: JsonPropertyName("fetched_at")] DateTimeOffset FetchedAt,
    [property: JsonPropertyName("url")] string? Url = null,
    [property: JsonPropertyName("updated_at")] DateTimeOffset? UpdatedAt = null,
    [property: JsonPropertyName("license")] string? License = null,
    [property: JsonPropertyName("attribution")] string? Attribution = null);

public sealed record GameIcon(
    [property: JsonPropertyName("url")] string Url,
    [property: JsonPropertyName("alt")] IReadOnlyDictionary<string, string>? Alt = null,
    [property: JsonPropertyName("source_id")] string? SourceId = null,
    [property: JsonPropertyName("width")] int? Width = null,
    [property: JsonPropertyName("height")] int? Height = null);

public sealed record GameCapability(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("name")] IReadOnlyDictionary<string, string> Name,
    [property: JsonPropertyName("supported")] bool Supported,
    [property: JsonPropertyName("auth_scope")] string AuthScope,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("reason")] string? Reason = null,
    [property: JsonPropertyName("source")] GameProvenance? Source = null);

public record GameDescriptor
{
    [JsonPropertyName("id")] public string Id { get; init; } = "";
    [JsonPropertyName("provider")] public string Provider { get; init; } = "";
    [JsonPropertyName("name")] public IReadOnlyDictionary<string, string> Name { get; init; } = new Dictionary<string, string>();
    [JsonPropertyName("icon")] public GameIcon? Icon { get; init; }
    [JsonPropertyName("regions")] public IReadOnlyList<string>? Regions { get; init; }
    [JsonPropertyName("capabilities")] public IReadOnlyList<GameCapability> Capabilities { get; init; } = Array.Empty<GameCapability>();
}

public sealed record GameBoundRole(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("uid")] string Uid,
    [property: JsonPropertyName("region")] string Region,
    [property: JsonPropertyName("policy")] GameFieldPolicy Policy,
    [property: JsonPropertyName("server")] string? Server = null,
    [property: JsonPropertyName("nickname")] string? Nickname = null,
    [property: JsonPropertyName("level")] int? Level = null,
    [property: JsonPropertyName("icon")] GameIcon? Icon = null);

/// <summary>Value preserves absent, explicit null, zero, false and exact large integers. HasValue does not imply freshness.</summary>
public sealed record GameField
{
    [JsonPropertyName("key")] public string Key { get; init; } = "";
    [JsonPropertyName("name")] public IReadOnlyDictionary<string, string> Name { get; init; } = new Dictionary<string, string>();
    [JsonPropertyName("kind")] public string Kind { get; init; } = "";
    [JsonPropertyName("value"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingDefault)] public JsonElement Value { get; init; }
    [JsonPropertyName("unit")] public string? Unit { get; init; }
    [JsonPropertyName("status")] public string Status { get; init; } = "";
    [JsonPropertyName("policy")] public GameFieldPolicy Policy { get; init; } = new("owner", false);
    [JsonPropertyName("source")] public GameProvenance? Source { get; init; }
    [JsonPropertyName("updated_at")] public DateTimeOffset? UpdatedAt { get; init; }
    [JsonIgnore] public bool HasValue => Value.ValueKind is not (JsonValueKind.Undefined or JsonValueKind.Null);
}

/// <summary>Characters/items/equipment and public catalog entries share this shape. Ownership only exists when an observed field says so.</summary>
public sealed record GameEntity(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("kind")] string Kind,
    [property: JsonPropertyName("name")] IReadOnlyDictionary<string, string> Name,
    [property: JsonPropertyName("fields")] IReadOnlyList<GameField> Fields,
    [property: JsonPropertyName("policy")] GameFieldPolicy Policy,
    [property: JsonPropertyName("icon")] GameIcon? Icon = null,
    [property: JsonPropertyName("source")] GameProvenance? Source = null);

public sealed record GameSection(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("name")] IReadOnlyDictionary<string, string> Name,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("fields")] IReadOnlyList<GameField> Fields,
    [property: JsonPropertyName("entities")] IReadOnlyList<GameEntity> Entities,
    [property: JsonPropertyName("policy")] GameFieldPolicy Policy,
    [property: JsonPropertyName("reason")] string? Reason = null,
    [property: JsonPropertyName("source")] GameProvenance? Source = null,
    [property: JsonPropertyName("updated_at")] DateTimeOffset? UpdatedAt = null,
    [property: JsonPropertyName("next_cursor")] string? NextCursor = null);

public sealed record GameSnapshot(
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("sections")] IReadOnlyList<GameSection> Sections,
    [property: JsonPropertyName("fetched_at")] DateTimeOffset FetchedAt,
    [property: JsonPropertyName("role")] GameBoundRole? Role = null,
    [property: JsonPropertyName("expires_at")] DateTimeOffset? ExpiresAt = null,
    [property: JsonPropertyName("next_cursor")] string? NextCursor = null);

public sealed record GameList([property: JsonPropertyName("games")] IReadOnlyList<GameDescriptor> Games);
public sealed record GameRoleDiscovery(string Status, string? UpdatedAt, string? Reason);
public sealed record GameAccountData(IReadOnlyList<GameDescriptor> Games, IReadOnlyList<GameBoundRole> Roles, string SelectedRoleId, GameRoleDiscovery Discovery);

/// <summary>Core request selectors. RoleId is a positive binding row ID for miHoYo, an opaque discovered key for Hypergryph.</summary>
public sealed record GameSnapshotQuery
{
    public string? RoleId { get; init; }
    public IReadOnlyList<string>? Sections { get; init; }
    public string? Cursor { get; init; }
    public int? Limit { get; init; }
    public string? Locale { get; init; }
}

public sealed record GameArchivePageQuery(ulong? Before = null, int? Limit = null);

public sealed record GameMihoyoProfile(string Uid, string Nickname, string Avatar, string Introduce);
public sealed record GameMihoyoRegion(string Id, string Name, bool Qr, bool Cookie);
public sealed record GameMihoyoRecovery(string Action, string? Url);
public sealed record GameMihoyoOption(string GameBiz, string Name);
public sealed record GameMihoyoBoundRole(long Id, string GameBiz, string GameId, string GameName, string Region, string RegionName,
    string GameUid, string Nickname, int Level, bool IsChosen, string? SyncTime);

// Existing provider account timestamps use the site's timex format; keep strings
// here instead of inventing a timezone when it omits an offset.
public sealed record GameMihoyoStatus
{
    public long? AccountId { get; init; }
    public string Region { get; init; } = "";
    public IReadOnlyList<GameMihoyoRegion> Regions { get; init; } = Array.Empty<GameMihoyoRegion>();
    public GameMihoyoRecovery? Recovery { get; init; }
    public IReadOnlyList<GameDescriptor> GameCatalog { get; init; } = Array.Empty<GameDescriptor>();
    public bool Enabled { get; init; }
    public bool AllowQr { get; init; }
    public bool AllowCookie { get; init; }
    public bool RecordEnabled { get; init; }
    public bool Bound { get; init; }
    public GameMihoyoProfile? Profile { get; init; }
    public IReadOnlyList<GameMihoyoBoundRole> Roles { get; init; } = Array.Empty<GameMihoyoBoundRole>();
    public string? LastSync { get; init; }
    public string? NextSyncAt { get; init; }
    public IReadOnlyList<GameMihoyoOption> Games { get; init; } = Array.Empty<GameMihoyoOption>();
    public string? Method { get; init; }
    public string? State { get; init; }
    public string? StateMessage { get; init; }
}

public sealed record GameHypergryphStatus(bool Enabled, bool AllowSms, bool AllowToken, bool ServerApi, string TokenHelpUrl, GameHypergryphAccount? Account);
public sealed record GameHypergryphAccount
{
    public long Id { get; init; }
    public long UserId { get; init; }
    public string SklandUid { get; init; } = "";
    public string HgUid { get; init; } = "";
    public string Nickname { get; init; } = "";
    public string Avatar { get; init; } = "";
    public string Method { get; init; } = "";
    public bool Share { get; init; }
    public string Status { get; init; } = "";
    public string LastError { get; init; } = "";
    public string? LastSync { get; init; }
    public string SelectedRoleId { get; init; } = "";
    public string DiscoveryStatus { get; init; } = "";
    public string? DiscoveryAt { get; init; }
    public string? CreateTime { get; init; }
    public string? UpdateTime { get; init; }
    public IReadOnlyList<GameHypergryphLegacyRole> Roles { get; init; } = Array.Empty<GameHypergryphLegacyRole>();
}

/// <summary>Legacy Arknights summaries have their original schema, not the normalized per-field policy contract.</summary>
public sealed record GameHypergryphLegacyRole
{
    public long Id { get; init; }
    public long AccountId { get; init; }
    public long UserId { get; init; }
    public string Uid { get; init; } = "";
    public string Nickname { get; init; } = "";
    public string ChannelName { get; init; } = "";
    public string ChannelId { get; init; } = "";
    public bool IsOfficial { get; init; }
    public bool IsDefault { get; init; }
    public int Level { get; init; }
    public string SyncError { get; init; } = "";
    public string? LastSync { get; init; }
    public string? CreateTime { get; init; }
    public string? UpdateTime { get; init; }
    public int ApNow { get; init; }
    public JsonElement Summary { get; init; }
}

public sealed record GameCatalogEvidence(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("url")] string Url,
    [property: JsonPropertyName("kind")] string Kind,
    [property: JsonPropertyName("verified_at")] DateTimeOffset VerifiedAt,
    [property: JsonPropertyName("claim")] string Claim);
public sealed record GameCatalogBinding(
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("game_biz")] string GameBiz,
    [property: JsonPropertyName("provider")] string Provider,
    [property: JsonPropertyName("region")] string Region,
    [property: JsonPropertyName("role_discovery")] bool RoleDiscovery,
    [property: JsonPropertyName("evidence_ids")] IReadOnlyList<string> EvidenceIds);
public sealed record GameMihoyoCatalogEntry : GameDescriptor
{
    [JsonPropertyName("lifecycle")] public string Lifecycle { get; init; } = "";
    [JsonPropertyName("summary")] public IReadOnlyDictionary<string, string> Summary { get; init; } = new Dictionary<string, string>();
    [JsonPropertyName("aliases")] public IReadOnlyList<string>? Aliases { get; init; }
    [JsonPropertyName("official_urls")] public IReadOnlyDictionary<string, string> OfficialUrls { get; init; } = new Dictionary<string, string>();
    [JsonPropertyName("bindings")] public IReadOnlyList<GameCatalogBinding> Bindings { get; init; } = Array.Empty<GameCatalogBinding>();
    [JsonPropertyName("evidence")] public IReadOnlyList<GameCatalogEvidence> Evidence { get; init; } = Array.Empty<GameCatalogEvidence>();
}

public sealed record GameSnapshotMetadata(
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("role_id")] string RoleId,
    [property: JsonPropertyName("version")] int Version,
    [property: JsonPropertyName("revision")] ulong Revision,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("stale")] bool Stale,
    [property: JsonPropertyName("bytes")] int Bytes,
    [property: JsonPropertyName("fetched_at")] DateTimeOffset FetchedAt,
    [property: JsonPropertyName("expires_at")] DateTimeOffset ExpiresAt);
public sealed record GameArchivedSnapshot(
    [property: JsonPropertyName("provider")] string Provider,
    [property: JsonPropertyName("role_id")] string RoleId,
    [property: JsonPropertyName("version")] int Version,
    [property: JsonPropertyName("revision")] ulong Revision,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("stale")] bool Stale,
    [property: JsonPropertyName("expires_at")] DateTimeOffset ExpiresAt,
    [property: JsonPropertyName("snapshot")] GameSnapshot Snapshot);
public sealed record GameArchiveHistory(
    [property: JsonPropertyName("id")] ulong Id,
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("role_id")] string RoleId,
    [property: JsonPropertyName("revision")] ulong Revision,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("bytes")] int Bytes,
    [property: JsonPropertyName("created_at")] DateTimeOffset CreatedAt);
public sealed record GameStoredFieldPolicy(
    [property: JsonPropertyName("id")] ulong Id,
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("path")] string Path,
    [property: JsonPropertyName("visibility")] string Visibility,
    [property: JsonPropertyName("exportable")] bool Exportable,
    [property: JsonPropertyName("revision")] ulong Revision,
    [property: JsonPropertyName("updated_at")] DateTimeOffset UpdatedAt);
public sealed record GamePolicyAudit(
    [property: JsonPropertyName("id")] ulong Id,
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("path")] string Path,
    [property: JsonPropertyName("revision")] ulong Revision,
    [property: JsonPropertyName("previous")] string Previous,
    [property: JsonPropertyName("current")] string Current,
    [property: JsonPropertyName("created_at")] DateTimeOffset CreatedAt);
/// <summary>Explicit create/update revision; 0 creates, a stale revision returns the server's conflict error.</summary>
public sealed record GameFieldPolicyInput(string GameId, string Path, string Visibility, bool Exportable, ulong Revision);
public sealed record GameArchiveScope(
    [property: JsonPropertyName("site_id")] ulong SiteId,
    [property: JsonPropertyName("user_id")] ulong UserId,
    [property: JsonPropertyName("account_id")] ulong AccountId);
/// <summary>Only the server's policy-filtered export route produces this DTO. Serializing an owner snapshot is not an authorized export.</summary>
public sealed record GameArchiveExport(
    [property: JsonPropertyName("scope")] GameArchiveScope Scope,
    [property: JsonPropertyName("provider")] string Provider,
    [property: JsonPropertyName("exported_at")] DateTimeOffset ExportedAt,
    [property: JsonPropertyName("snapshots")] IReadOnlyList<GameArchivedSnapshot> Snapshots,
    [property: JsonPropertyName("history")] IReadOnlyList<GameArchiveHistory> History,
    [property: JsonPropertyName("policies")] IReadOnlyList<GameStoredFieldPolicy> Policies,
    [property: JsonPropertyName("policy_audit")] IReadOnlyList<GamePolicyAudit> PolicyAudit);

public sealed record GameAssetEndpoint(
    [property: JsonPropertyName("locale")] string Locale,
    [property: JsonPropertyName("version")] string Version,
    [property: JsonPropertyName("url")] string Url);
public sealed record GameAssetManifest(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("game_id")] string GameId,
    [property: JsonPropertyName("name")] IReadOnlyDictionary<string, string> Name,
    [property: JsonPropertyName("source")] GameProvenance Source,
    [property: JsonPropertyName("endpoints")] IReadOnlyList<GameAssetEndpoint> Endpoints,
    [property: JsonPropertyName("media_hosts")] IReadOnlyList<string> MediaHosts,
    [property: JsonPropertyName("ttl_seconds")] int TtlSeconds,
    [property: JsonPropertyName("max_bytes")] long MaxBytes,
    [property: JsonPropertyName("max_entries")] int MaxEntries);
public sealed record GameAssetCache
{
    [JsonPropertyName("provider")] public string Provider { get; init; } = "";
    [JsonPropertyName("locale")] public string Locale { get; init; } = "";
    [JsonPropertyName("version")] public string Version { get; init; } = "";
    [JsonPropertyName("disabled")] public bool Disabled { get; init; }
    [JsonPropertyName("source_url")] public string SourceUrl { get; init; } = "";
    [JsonPropertyName("content_hash")] public string? ContentHash { get; init; }
    [JsonPropertyName("entry_count")] public int EntryCount { get; init; }
    [JsonPropertyName("fetched_at")] public DateTimeOffset? FetchedAt { get; init; }
    [JsonPropertyName("checked_at")] public DateTimeOffset? CheckedAt { get; init; }
    [JsonPropertyName("expires_at")] public DateTimeOffset? ExpiresAt { get; init; }
    [JsonPropertyName("attempted_at")] public DateTimeOffset? AttemptedAt { get; init; }
    [JsonPropertyName("last_error_code")] public string? LastErrorCode { get; init; }
}
public sealed record GameAssetProvider(
    [property: JsonPropertyName("manifest")] GameAssetManifest Manifest,
    [property: JsonPropertyName("caches")] IReadOnlyList<GameAssetCache> Caches);
public sealed record GameAssetPage(
    [property: JsonPropertyName("provider")] GameAssetManifest Provider,
    [property: JsonPropertyName("cache")] GameAssetCache Cache,
    [property: JsonPropertyName("status")] string Status,
    [property: JsonPropertyName("list")] IReadOnlyList<GameEntity> List,
    [property: JsonPropertyName("count")] int Count,
    [property: JsonPropertyName("page")] int Page,
    [property: JsonPropertyName("limit")] int Limit);
public sealed record GameAssetQuery
{
    public string Provider { get; init; } = "";
    public string? Locale { get; init; }
    public string? Version { get; init; }
    public string? Search { get; init; }
    public string? Kind { get; init; }
    public int? Page { get; init; }
    public int? Limit { get; init; }
}
public sealed record GameAssetVariant(string? Locale = null, string? Version = null);
