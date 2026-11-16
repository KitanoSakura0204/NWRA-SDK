using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace NoWork.Sdk;

/// <summary>Business error (code != 0) from the user center.</summary>
public sealed class ApiException : Exception
{
    public int Code { get; }
    public ApiException(int code, string message) : base($"nowork: api error {code}: {message}") => Code = code;
}

/// <summary>Network / decoding failure. Decide fail-open vs fail-closed in the caller.</summary>
public sealed class TransportException : Exception
{
    public TransportException(string message, Exception? inner = null) : base($"nowork: transport error: {message}", inner) { }
}

/// <summary>
/// A Server API reply was encrypted and could not be used (no decryption key configured, the JWE did not
/// open, or its ctx names another route), or a whitelist reply came plain while DecryptionKeys are set.
/// Not a TransportException on purpose: the data was never read, so never fail open on it (an
/// unreadable whitelist is not a disabled whitelist).
/// </summary>
public sealed class EncryptionException : Exception
{
    public string Route { get; }
    public string Reason { get; }
    public EncryptionException(string route, string reason, Exception? inner = null) : base($"nowork: encrypted response on {route}: {reason}", inner)
    {
        Route = route;
        Reason = reason;
    }
}

/// <summary>A Server API reply as received, data left undecoded (see <see cref="NoworkClient.RawAsync"/>).</summary>
/// <param name="Status">HTTP status code.</param>
/// <param name="Headers">Response and content headers, case-insensitive.</param>
/// <param name="Message">The envelope's message.</param>
/// <param name="Data">The envelope's data ({"jwe": "..."} when encrypted); Undefined when absent.</param>
/// <param name="Body">The whole response body (hand it to NoWorkDelivery.OpenServerData).</param>
/// <param name="Encrypted">X-NoWork-Encryption is set or data is exactly {"jwe": "&lt;string&gt;"}.</param>
public sealed record RawResponse(int Status, IReadOnlyDictionary<string, string> Headers, string Message, JsonElement Data, string Body, bool Encrypted);

/// <summary>One entry of the heartbeat player list (name required, uuid optional).</summary>
public sealed record OnlinePlayer(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("uuid"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] string? Uuid = null);

/// <summary>Live status for HeartbeatAsync; Tps and Players are optional (PROTOCOL 2.3).</summary>
/// <param name="Online">Players online now.</param>
/// <param name="Max">Capacity (0 = unknown).</param>
/// <param name="Version">Server version.</param>
/// <param name="Tps">Ticks per second; left out unless a finite number above 0.</param>
/// <param name="Players">Players online now; should match Online.</param>
public sealed record Heartbeat(int Online, int Max = 0, string Version = "", double? Tps = null, IReadOnlyList<OnlinePlayer>? Players = null);

public sealed record WhitelistEntry(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("uuid")] string? Uuid,
    [property: JsonPropertyName("uid")] long Uid,
    [property: JsonPropertyName("nickname")] string Nickname,
    [property: JsonPropertyName("securityLevel")] int SecurityLevel,
    [property: JsonPropertyName("since")] string Since);

public sealed record Whitelist(
    [property: JsonPropertyName("server")] string Server,
    [property: JsonPropertyName("mode")] int Mode,
    [property: JsonPropertyName("count")] int Count,
    [property: JsonPropertyName("entries")] List<WhitelistEntry> Entries);

public sealed record CheckResult(
    [property: JsonPropertyName("allowed")] bool Allowed,
    [property: JsonPropertyName("bound")] bool Bound,
    [property: JsonPropertyName("reason")] string Reason,
    [property: JsonPropertyName("player")] WhitelistEntry? Player)
{
    /// <summary>Extra text for some refusals, e.g. the reason of a per-server ban (Reason "banned", SERVER_USERS.md).</summary>
    [JsonPropertyName("message")] public string? Message { get; init; }
}

/// <summary>Server API client authenticated with X-Server-Key (player data and bans: ServerUsers.cs).</summary>
public sealed partial class NoworkClient
{
    /// <summary>SDK release this client implements (see CHANGELOG.md).</summary>
    public const string Version = "1.3.0";

    private readonly HttpClient _http;
    private readonly string _base;
    private static readonly JsonSerializerOptions Json = new() { PropertyNameCaseInsensitive = true };

    public NoworkClient(string baseUrl, string serverKey, HttpClient? http = null)
    {
        _base = baseUrl.TrimEnd('/');
        _http = http ?? new HttpClient { Timeout = TimeSpan.FromSeconds(10) };
        _http.DefaultRequestHeaders.Remove("X-Server-Key");
        _http.DefaultRequestHeaders.Add("X-Server-Key", serverKey);
        _http.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
    }

    /// <summary>Header marking a Server API reply whose data is the {"jwe": "..."} envelope (DELIVERY.md section 5).</summary>
    public const string HeaderEncryption = "X-NoWork-Encryption";

    /// <summary>
    /// Private JWKs (JSON) that open encrypted replies (DELIVERY.md section 5); several during a key rotation.
    /// When set, the whitelist routes must come encrypted: a plain reply there is refused (no downgrade).
    /// </summary>
    public IReadOnlyList<string>? DecryptionKeys { get; init; }

    // The routes the backend seals when server encryption is on (DELIVERY.md section 5).
    private static readonly HashSet<string> SealedRoutes = new(StringComparer.Ordinal) { "whitelist", "whitelist/check" };

    /// <summary>Decoded data; an encrypted reply it cannot open throws <see cref="EncryptionException"/> (fail closed).</summary>
    private async Task<T> CallAsync<T>(HttpMethod method, string path, object? body, CancellationToken ct)
    {
        var res = await RawAsync(method, path, body, ct).ConfigureAwait(false);
        var route = Route(path);
        var keys = DecryptionKeys is { Count: > 0 } k ? k.ToArray() : null;
        var data = res.Data;
        if (res.Encrypted)
        {
            if (keys == null) throw new EncryptionException(route, "no decryption key configured");
            data = Open(res.Data, route, keys);
        }
        else if (keys != null && SealedRoutes.Contains(route))
        {
            throw new EncryptionException(route, "plain reply while decryption keys are configured");
        }
        if (data.ValueKind is JsonValueKind.Undefined or JsonValueKind.Null)
            throw new TransportException($"invalid response ({res.Status}): missing data");
        try { return data.Deserialize<T>(Json)!; }
        catch (Exception e) when (e is JsonException or NotSupportedException) { throw new TransportException($"invalid data ({res.Status})", e); }
    }

    /// <summary>Opens the {"jwe": "..."} data and checks its ctx names this route ("server:&lt;code&gt;:&lt;route&gt;").</summary>
    private static JsonElement Open(JsonElement data, string route, string[] keys)
    {
        var jwe = data.ValueKind == JsonValueKind.Object && data.TryGetProperty("jwe", out var j) && j.ValueKind == JsonValueKind.String ? j.GetString() : null;
        if (string.IsNullOrEmpty(jwe)) throw new EncryptionException(route, "encrypted reply without a JWE");
        NoWorkDelivery.Decrypted d;
        try { d = NoWorkDelivery.Decrypt(jwe, keys); }
        catch (Exception e) when (e is NoWorkDelivery.DecryptException or FormatException or JsonException or System.Security.Cryptography.CryptographicException or ArgumentException)
        { throw new EncryptionException(route, "the JWE did not open", e); }
        var ctx = d.Header.ValueKind == JsonValueKind.Object && d.Header.TryGetProperty("ctx", out var c) && c.ValueKind == JsonValueKind.String ? c.GetString() ?? "" : "";
        if (!CtxMatches(ctx, route)) throw new EncryptionException(route, "envelope ctx names another route: " + ctx);
        try
        {
            using var doc = JsonDocument.Parse(d.Text);
            return doc.RootElement.Clone();
        }
        catch (JsonException e) { throw new EncryptionException(route, "decrypted data is not JSON", e); }
    }

    // "server:<code>:<route>": the server code is not known here, so the kind and the route are checked.
    private static bool CtxMatches(string ctx, string route) =>
        ctx.StartsWith("server:", StringComparison.Ordinal) && ctx.EndsWith(":" + route, StringComparison.Ordinal)
        && ctx.Length > "server:".Length + route.Length + 1;

    /// <summary>
    /// Call a Server API route (e.g. "/server/v1/whitelist", query included) and return the reply without
    /// decoding data. Transport failures, unreadable replies and business errors (code != 0) still throw.
    /// </summary>
    public async Task<RawResponse> RawAsync(HttpMethod method, string path, object? body = null, CancellationToken ct = default)
    {
        using var req = new HttpRequestMessage(method, _base + path);
        if (body != null)
        {
            string json;
            try { json = JsonSerializer.Serialize(body); }
            catch (Exception e) when (e is ArgumentException or NotSupportedException or JsonException) { throw new TransportException("cannot encode the request body", e); }
            req.Content = new StringContent(json, Encoding.UTF8, "application/json");
        }
        HttpResponseMessage res;
        try { res = await _http.SendAsync(req, ct).ConfigureAwait(false); }
        catch (Exception e) when (e is HttpRequestException or TaskCanceledException) { throw new TransportException(e.Message, e); }
        using (res)
        {
            var text = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            var status = (int)res.StatusCode;
            int code;
            string message;
            JsonElement data;
            try
            {
                using var doc = JsonDocument.Parse(text);
                var root = doc.RootElement;
                if (root.ValueKind != JsonValueKind.Object || !root.TryGetProperty("code", out var c) || c.ValueKind != JsonValueKind.Number || !c.TryGetInt32(out code))
                    throw new TransportException($"invalid response ({status}): missing code");
                message = root.TryGetProperty("message", out var m) && m.ValueKind == JsonValueKind.String ? m.GetString() ?? "" : "";
                data = root.TryGetProperty("data", out var d) ? d.Clone() : default;
            }
            catch (JsonException e) { throw new TransportException($"invalid json ({status})", e); }
            if (code != 0) throw new ApiException(code, message);
            if (status >= 400) throw new TransportException($"http {status}");
            var headers = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            foreach (var h in res.Headers.Concat(res.Content.Headers)) headers[h.Key] = string.Join(", ", h.Value);
            var encrypted = (headers.TryGetValue(HeaderEncryption, out var enc) && enc.Length > 0) || IsJweData(data);
            return new RawResponse(status, headers, message, data, text, encrypted);
        }
    }

    /// <summary>"whitelist/check" for "/server/v1/whitelist/check?name=...".</summary>
    private static string Route(string path)
    {
        var p = path.Split('?')[0];
        return p.StartsWith("/server/v1/", StringComparison.Ordinal) ? p.Substring("/server/v1/".Length) : p;
    }

    private static bool IsJweData(JsonElement data) =>
        data.ValueKind == JsonValueKind.Object && data.EnumerateObject().Count() == 1
        && data.TryGetProperty("jwe", out var jwe) && jwe.ValueKind == JsonValueKind.String;

    /// <summary>Full whitelist for this server.</summary>
    public Task<Whitelist> WhitelistAsync(CancellationToken ct = default) =>
        CallAsync<Whitelist>(HttpMethod.Get, "/server/v1/whitelist", null, ct);

    /// <summary>Join check by name and/or uuid.</summary>
    public Task<CheckResult> CheckWhitelistAsync(string? name, string? uuid, CancellationToken ct = default)
    {
        var q = new List<string>();
        if (!string.IsNullOrEmpty(name)) q.Add("name=" + Uri.EscapeDataString(name));
        if (!string.IsNullOrEmpty(uuid)) q.Add("uuid=" + Uri.EscapeDataString(uuid));
        return CallAsync<CheckResult>(HttpMethod.Get, "/server/v1/whitelist/check?" + string.Join("&", q), null, ct);
    }

    /// <summary>Report live status (about every 60s).</summary>
    public Task HeartbeatAsync(int online, int max = 0, string version = "", CancellationToken ct = default) =>
        HeartbeatAsync(new Heartbeat(online, max, version), ct);

    /// <summary>
    /// Report live status with the optional TPS and player list (PROTOCOL 2.3). A Tps that is not a finite
    /// number above 0 is left out, and so is an empty player list.
    /// </summary>
    public Task HeartbeatAsync(Heartbeat status, CancellationToken ct = default)
    {
        var body = new Dictionary<string, object> { ["online"] = status.Online, ["max"] = status.Max, ["version"] = status.Version ?? "" };
        if (status.Tps is double tps && double.IsFinite(tps) && tps > 0) body["tps"] = tps;
        if (status.Players is { Count: > 0 } players) body["players"] = players;
        return CallAsync<object>(HttpMethod.Post, "/server/v1/heartbeat", body, ct);
    }

    // ---- Feature modules / GM channel (PROTOCOL 2.4) ----

    /// <summary>Modules enabled for this server, keyed by module name, with their per-server config.</summary>
    public async Task<Dictionary<string, JsonElement>> ModulesAsync(CancellationToken ct = default)
    {
        var d = await CallAsync<ModulesEnvelope>(HttpMethod.Get, "/server/v1/modules", null, ct).ConfigureAwait(false);
        return d.Modules ?? new Dictionary<string, JsonElement>();
    }

    /// <summary>Replace the GM action catalogue shown in the admin panel (call on start-up and on change).</summary>
    public async Task<int> PublishGMActionsAsync(IEnumerable<GMAction> actions, CancellationToken ct = default)
    {
        var d = await CallAsync<CountEnvelope>(HttpMethod.Put, "/server/v1/gm/actions", actions.ToList(), ct).ConfigureAwait(false);
        return d.Count;
    }

    /// <summary>
    /// Replace the actions and the named option lists (select / suggestion sources such as worlds or kits)
    /// in one call; null actions or options keep what is stored (an empty list clears). Entries the server
    /// cannot accept are skipped and listed in the result, never a failure for the whole call.
    /// </summary>
    public async Task<GMPublishResult> PublishGMCatalogAsync(IEnumerable<GMAction>? actions, IReadOnlyDictionary<string, IReadOnlyList<GMOption>>? options = null, CancellationToken ct = default)
    {
        var body = new Dictionary<string, object>();
        if (actions != null) body["actions"] = actions.ToList();
        if (options != null) body["options"] = OptionLists(options);
        var r = await CallAsync<GMPublishResult>(HttpMethod.Put, "/server/v1/gm/actions", body, ct).ConfigureAwait(false);
        return r with { Skipped = r.Skipped ?? new List<GMSkipped>() };
    }

    /// <summary>Replace only the named option lists (the published actions stay), e.g. when a world loads.</summary>
    public async Task<GMPublishResult> PublishGMOptionsAsync(IReadOnlyDictionary<string, IReadOnlyList<GMOption>> options, CancellationToken ct = default)
    {
        var r = await CallAsync<GMPublishResult>(HttpMethod.Put, "/server/v1/gm/options", OptionLists(options), ct).ConfigureAwait(false);
        return r with { Skipped = r.Skipped ?? new List<GMSkipped>() };
    }

    private static Dictionary<string, List<GMOption>> OptionLists(IReadOnlyDictionary<string, IReadOnlyList<GMOption>> options) =>
        options.ToDictionary(kv => kv.Key, kv => kv.Value.ToList());

    /// <summary>Queued GM commands (each becomes "sent"); poll alongside the heartbeat.</summary>
    public async Task<List<GMCommand>> PollGMCommandsAsync(CancellationToken ct = default) =>
        await CallAsync<List<GMCommand>>(HttpMethod.Get, "/server/v1/gm/commands", null, ct).ConfigureAwait(false) ?? new List<GMCommand>();

    /// <summary>Report how a command went; result is free text (at most 4000 characters).</summary>
    public Task ReportGMResultAsync(long commandId, bool ok, string result = "", CancellationToken ct = default) =>
        CallAsync<object>(HttpMethod.Post, $"/server/v1/gm/commands/{commandId}/result", new { ok, result }, ct);
}

internal sealed record ModulesEnvelope([property: JsonPropertyName("modules")] Dictionary<string, JsonElement>? Modules);
internal sealed record CountEnvelope([property: JsonPropertyName("count")] int Count);

/// <summary>One choice of a select parameter or of a named option list.</summary>
public sealed record GMOption(
    [property: JsonPropertyName("value")] string Value,
    [property: JsonPropertyName("label"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] string? Label = null);

/// <summary>
/// One input of a GM action. Only Name / Type / Required / Label existed before 2026-09-28; every other
/// field is optional (PROTOCOL 2.4). Type: string | text | number | boolean | select | player (aliases
/// int / integer / bool / textarea).
/// </summary>
public sealed record GMActionParam(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("type")] string Type,
    [property: JsonPropertyName("required")] bool Required = false,
    [property: JsonPropertyName("label")] string? Label = null)
{
    [JsonPropertyName("description"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public string? Description { get; init; }
    [JsonPropertyName("placeholder"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public string? Placeholder { get; init; }
    /// <summary>Ignored by the server when it does not fit the definition.</summary>
    [JsonPropertyName("default"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public object? Default { get; init; }
    /// <summary>number only.</summary>
    [JsonPropertyName("min"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public double? Min { get; init; }
    [JsonPropertyName("max"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public double? Max { get; init; }
    [JsonPropertyName("step"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public double? Step { get; init; }
    [JsonPropertyName("integer"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingDefault)] public bool Integer { get; init; }
    /// <summary>Text length limit; 0 = server default (1000, text 4000, player 64).</summary>
    [JsonPropertyName("maxLength"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingDefault)] public int MaxLength { get; init; }
    /// <summary>Inline select choices (1-200, unique values).</summary>
    [JsonPropertyName("options"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public List<GMOption>? Options { get; init; }
    /// <summary>Names an option list published with PublishGMCatalogAsync / PublishGMOptionsAsync.</summary>
    [JsonPropertyName("source"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public string? Source { get; init; }
}

/// <summary>A GM action the plugin offers to the admin panel (key: no colon, no whitespace, at most 64 bytes).</summary>
public sealed record GMAction(
    [property: JsonPropertyName("key")] string Key,
    [property: JsonPropertyName("title")] string Title,
    [property: JsonPropertyName("description")] string? Description = null,
    [property: JsonPropertyName("params")] List<GMActionParam>? Params = null,
    [property: JsonPropertyName("labels")] Dictionary<string, string>? Labels = null)
{
    /// <summary>Catalogue group.</summary>
    [JsonPropertyName("category"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public string? Category { get; init; }
    /// <summary>normal | confirm | critical (the last two need a confirmation before the command is sent).</summary>
    [JsonPropertyName("danger"), JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)] public string? Danger { get; init; }
}

/// <summary>What a publish stored; entries the server could not accept are skipped and listed.</summary>
public sealed record GMPublishResult(
    [property: JsonPropertyName("count")] int Count,
    [property: JsonPropertyName("skipped")] IReadOnlyList<GMSkipped>? Skipped);

/// <summary>One entry a publish skipped, with the reason.</summary>
public sealed record GMSkipped(
    [property: JsonPropertyName("key")] string Key,
    [property: JsonPropertyName("reason")] string Reason);

/// <summary>A queued GM command the admin panel wants the plugin to run.</summary>
public sealed record GMCommand(
    [property: JsonPropertyName("id")] long Id,
    [property: JsonPropertyName("action")] string Action,
    [property: JsonPropertyName("params")] JsonElement Params,
    [property: JsonPropertyName("status")] string Status)
{
    /// <summary>admin | owner | grant.</summary>
    [JsonPropertyName("source")] public string? Source { get; init; }
    /// <summary>"staff" for admin commands on owner-run servers, the login name otherwise.</summary>
    [JsonPropertyName("operator")] public string? Operator { get; init; }
}
