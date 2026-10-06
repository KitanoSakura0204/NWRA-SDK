using System.Text.Json;
using System.Text.Json.Serialization;

namespace Lounge.Sdk;

// Player data and per-server bans (module "server-users", SDK/docs/SERVER_USERS.md).

/// <summary>Business codes of the server-users routes (ApiException.Code); other refusals use code 1.</summary>
public static class ServerUsersCodes
{
    /// <summary>The uid has no binding and was never seen on this server.</summary>
    public const int NotOnRoster = 404;
    /// <summary>The data changed since the version you read: read again, then write.</summary>
    public const int VersionConflict = 409;
    /// <summary>A ban an administrator placed: only an administrator can change it.</summary>
    public const int AdminBan = 403;

    /// <summary>Whether a data write was refused because of a stale version.</summary>
    public static bool IsVersionConflict(Exception e) => e is ApiException { Code: VersionConflict };
}

/// <summary>One choice of a select field.</summary>
public sealed record UserFieldOption(
    [property: JsonPropertyName("value")] string Value,
    [property: JsonPropertyName("label")] string? Label = null);

/// <summary>One entry of the data schema the server owner defined. Type: text | number | bool | select | json.</summary>
public sealed record UserField(
    [property: JsonPropertyName("key")] string Key,
    [property: JsonPropertyName("label")] string Label,
    [property: JsonPropertyName("type")] string Type)
{
    /// <summary>Help text.</summary>
    [JsonPropertyName("description")] public string? Description { get; init; }
    /// <summary>Shown read-only to the player in the user centre.</summary>
    [JsonPropertyName("visible")] public bool Visible { get; init; }
    /// <summary>Default value; Undefined when the field has none.</summary>
    [JsonPropertyName("default")] public JsonElement Default { get; init; }
    /// <summary>Characters (text) or bytes (json); 0 = not set.</summary>
    [JsonPropertyName("maxLength")] public int MaxLength { get; init; }
    /// <summary>number only.</summary>
    [JsonPropertyName("min")] public double? Min { get; init; }
    /// <summary>number only.</summary>
    [JsonPropertyName("max")] public double? Max { get; init; }
    /// <summary>number only: whole numbers.</summary>
    [JsonPropertyName("integer")] public bool Integer { get; init; }
    /// <summary>select only.</summary>
    [JsonPropertyName("options")] public List<UserFieldOption>? Options { get; init; }
}

/// <summary>GET /server/v1/users/schema.</summary>
public sealed record UserSchema(
    [property: JsonPropertyName("fields")] List<UserField> Fields,
    [property: JsonPropertyName("maxUserBytes")] int MaxUserBytes);

/// <summary>
/// One account's data: every schema field with its stored value, else the default, else null. Version is
/// the optimistic lock (0 = nothing stored yet); UpdatedAt is "2006-01-02 15:04:05" server time or null.
/// </summary>
public sealed record UserData(
    [property: JsonPropertyName("uid")] long Uid,
    [property: JsonPropertyName("data")] Dictionary<string, JsonElement> Data,
    [property: JsonPropertyName("version")] long Version,
    [property: JsonPropertyName("updatedAt")] string? UpdatedAt,
    [property: JsonPropertyName("updatedBy")] string? UpdatedBy,
    [property: JsonPropertyName("size")] int Size);

/// <summary>An active ban on this server with the account's bindings there; ExpireTime null = permanent.</summary>
public sealed record ServerBan(
    [property: JsonPropertyName("uid")] long Uid,
    [property: JsonPropertyName("names")] List<string> Names,
    [property: JsonPropertyName("uuids")] List<string> Uuids,
    [property: JsonPropertyName("reason")] string Reason,
    [property: JsonPropertyName("expireTime")] string? ExpireTime,
    [property: JsonPropertyName("createTime")] string CreateTime,
    [property: JsonPropertyName("source")] string Source);

/// <summary>An online player (from the last heartbeat) whose account is banned here.</summary>
public sealed record KickEntry(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("uuid")] string? Uuid,
    [property: JsonPropertyName("uid")] long Uid,
    [property: JsonPropertyName("reason")] string Reason,
    [property: JsonPropertyName("expireTime")] string? ExpireTime);

/// <summary>Names an account: its uid, or the name / uuid of one of its bindings on this server.</summary>
public sealed record BanTarget(long Uid = 0, string? Name = null, string? Uuid = null)
{
    internal Dictionary<string, object> ToBody()
    {
        var b = new Dictionary<string, object>();
        if (Uid > 0) b["uid"] = Uid;
        if (!string.IsNullOrEmpty(Name)) b["name"] = Name;
        if (!string.IsNullOrEmpty(Uuid)) b["uuid"] = Uuid;
        return b;
    }
}

internal sealed record BanList([property: JsonPropertyName("bans")] List<ServerBan>? Bans);
internal sealed record KickListEnvelope([property: JsonPropertyName("players")] List<KickEntry>? Players);
internal sealed record BanPlaced([property: JsonPropertyName("uid")] long Uid, [property: JsonPropertyName("ban")] ServerBan? Ban);
internal sealed record BanLifted([property: JsonPropertyName("uid")] long Uid);

public sealed partial class LoungeClient
{
    /// <summary>The data schema the server owner defined (needs the module on for this server).</summary>
    public Task<UserSchema> UserSchemaAsync(CancellationToken ct = default) =>
        CallAsync<UserSchema>(HttpMethod.Get, "/server/v1/users/schema", null, ct);

    /// <summary>One account's data (uid from the join check or the whitelist).</summary>
    public Task<UserData> UserDataAsync(long uid, CancellationToken ct = default) =>
        CallAsync<UserData>(HttpMethod.Get, UserPath(uid), null, ct);

    /// <summary>
    /// Replace the account's stored values (fields left out are unset). Pass the Version you last read to
    /// refuse a concurrent change (ApiException 409, see ServerUsersCodes.IsVersionConflict); null writes
    /// unconditionally. Returns the data as stored.
    /// </summary>
    public Task<UserData> PutUserDataAsync(long uid, IReadOnlyDictionary<string, object?> data, long? version = null, CancellationToken ct = default) =>
        CallAsync<UserData>(HttpMethod.Put, UserPath(uid), WriteBody(data, version), ct);

    /// <summary>Merge values into the stored ones; a null value deletes that field.</summary>
    public Task<UserData> PatchUserDataAsync(long uid, IReadOnlyDictionary<string, object?> data, long? version = null, CancellationToken ct = default) =>
        CallAsync<UserData>(HttpMethod.Patch, UserPath(uid), WriteBody(data, version), ct);

    /// <summary>
    /// Active bans on this server: all (only null; at most 5000, newest first) or those of the matching
    /// accounts. Works with the module off too.
    /// </summary>
    public async Task<List<ServerBan>> BansAsync(BanTarget? only = null, CancellationToken ct = default)
    {
        var q = new List<string>();
        if (only != null)
            foreach (var kv in only.ToBody()) q.Add(kv.Key + "=" + Uri.EscapeDataString(Convert.ToString(kv.Value, System.Globalization.CultureInfo.InvariantCulture) ?? ""));
        var path = "/server/v1/bans" + (q.Count > 0 ? "?" + string.Join("&", q) : "");
        var d = await CallAsync<BanList>(HttpMethod.Get, path, null, ct).ConfigureAwait(false);
        return d.Bans ?? new List<ServerBan>();
    }

    /// <summary>Online players (as of the last heartbeat) whose account is banned here: disconnect them.</summary>
    public async Task<List<KickEntry>> KickListAsync(CancellationToken ct = default)
    {
        var d = await CallAsync<KickListEnvelope>(HttpMethod.Get, "/server/v1/bans/kick", null, ct).ConfigureAwait(false);
        return d.Players ?? new List<KickEntry>();
    }

    /// <summary>
    /// Ban an account on this server (module on, pluginBans allowed). Give expireTime ("2006-01-02 15:04:05"
    /// or RFC 3339) or durationMinutes; neither = permanent. A ban an administrator placed cannot be replaced
    /// (ApiException 403). Returns the ban now in force.
    /// </summary>
    public async Task<ServerBan?> BanAsync(BanTarget target, string? reason = null, string? expireTime = null, long durationMinutes = 0, CancellationToken ct = default)
    {
        var body = target.ToBody();
        if (!string.IsNullOrEmpty(reason)) body["reason"] = reason;
        if (!string.IsNullOrEmpty(expireTime)) body["expireTime"] = expireTime;
        if (durationMinutes > 0) body["durationMinutes"] = durationMinutes;
        var d = await CallAsync<BanPlaced>(HttpMethod.Post, "/server/v1/bans", body, ct).ConfigureAwait(false);
        return d.Ban;
    }

    /// <summary>Lift the account's active ban on this server (not one an administrator placed); returns the uid.</summary>
    public async Task<long> UnbanAsync(BanTarget target, string? reason = null, CancellationToken ct = default)
    {
        var body = target.ToBody();
        if (!string.IsNullOrEmpty(reason)) body["reason"] = reason;
        var d = await CallAsync<BanLifted>(HttpMethod.Post, "/server/v1/bans/lift", body, ct).ConfigureAwait(false);
        return d.Uid;
    }

    private static string UserPath(long uid) =>
        uid > 0 ? $"/server/v1/users/{uid}/data" : throw new ArgumentOutOfRangeException(nameof(uid), uid, "lounge: uid must be positive");

    private static Dictionary<string, object?> WriteBody(IReadOnlyDictionary<string, object?> data, long? version)
    {
        // the server wants a data object, even an empty one
        var body = new Dictionary<string, object?> { ["data"] = data ?? new Dictionary<string, object?>() };
        if (version is long v) body["version"] = v;
        return body;
    }
}
