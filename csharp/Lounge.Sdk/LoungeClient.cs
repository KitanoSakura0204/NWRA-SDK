using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace Lounge.Sdk;

/// <summary>Business error (code != 0) from the user center.</summary>
public sealed class ApiException : Exception
{
    public int Code { get; }
    public ApiException(int code, string message) : base($"lounge: api error {code}: {message}") => Code = code;
}

/// <summary>Network / decoding failure. Decide fail-open vs fail-closed in the caller.</summary>
public sealed class TransportException : Exception
{
    public TransportException(string message, Exception? inner = null) : base($"lounge: transport error: {message}", inner) { }
}

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
    [property: JsonPropertyName("player")] WhitelistEntry? Player);

internal sealed record Envelope<T>(
    [property: JsonPropertyName("code")] int Code,
    [property: JsonPropertyName("message")] string? Message,
    [property: JsonPropertyName("data")] T? Data);

/// <summary>Server API client authenticated with X-Server-Key.</summary>
public sealed class LoungeClient
{
    private readonly HttpClient _http;
    private readonly string _base;
    private static readonly JsonSerializerOptions Json = new() { PropertyNameCaseInsensitive = true };

    public LoungeClient(string baseUrl, string serverKey, HttpClient? http = null)
    {
        _base = baseUrl.TrimEnd('/');
        _http = http ?? new HttpClient { Timeout = TimeSpan.FromSeconds(10) };
        _http.DefaultRequestHeaders.Remove("X-Server-Key");
        _http.DefaultRequestHeaders.Add("X-Server-Key", serverKey);
        _http.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
    }

    private async Task<T> CallAsync<T>(HttpMethod method, string path, object? body, CancellationToken ct)
    {
        using var req = new HttpRequestMessage(method, _base + path);
        if (body != null)
            req.Content = new StringContent(JsonSerializer.Serialize(body), Encoding.UTF8, "application/json");
        HttpResponseMessage res;
        try { res = await _http.SendAsync(req, ct).ConfigureAwait(false); }
        catch (Exception e) when (e is HttpRequestException or TaskCanceledException) { throw new TransportException(e.Message, e); }
        var text = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        Envelope<T>? env;
        try { env = JsonSerializer.Deserialize<Envelope<T>>(text, Json); }
        catch (JsonException e) { throw new TransportException($"invalid json ({(int)res.StatusCode})", e); }
        if (env == null) throw new TransportException("empty response");
        if (env.Code != 0) throw new ApiException(env.Code, env.Message ?? "");
        return env.Data!;
    }

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
        CallAsync<object>(HttpMethod.Post, "/server/v1/heartbeat", new { online, max, version }, ct);

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

    /// <summary>Queued GM commands (each becomes "sent"); poll alongside the heartbeat.</summary>
    public async Task<List<GMCommand>> PollGMCommandsAsync(CancellationToken ct = default) =>
        await CallAsync<List<GMCommand>>(HttpMethod.Get, "/server/v1/gm/commands", null, ct).ConfigureAwait(false) ?? new List<GMCommand>();

    /// <summary>Report how a command went; result is free text (at most 4000 characters).</summary>
    public Task ReportGMResultAsync(long commandId, bool ok, string result = "", CancellationToken ct = default) =>
        CallAsync<object>(HttpMethod.Post, $"/server/v1/gm/commands/{commandId}/result", new { ok, result }, ct);
}

internal sealed record ModulesEnvelope([property: JsonPropertyName("modules")] Dictionary<string, JsonElement>? Modules);
internal sealed record CountEnvelope([property: JsonPropertyName("count")] int Count);

/// <summary>One input of a GM action (type: string | number | boolean | player).</summary>
public sealed record GMActionParam(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("type")] string Type,
    [property: JsonPropertyName("required")] bool Required = false,
    [property: JsonPropertyName("label")] string? Label = null);

/// <summary>A GM action the plugin offers to the admin panel.</summary>
public sealed record GMAction(
    [property: JsonPropertyName("key")] string Key,
    [property: JsonPropertyName("title")] string Title,
    [property: JsonPropertyName("description")] string? Description = null,
    [property: JsonPropertyName("params")] List<GMActionParam>? Params = null,
    [property: JsonPropertyName("labels")] Dictionary<string, string>? Labels = null);

/// <summary>A queued GM command the admin panel wants the plugin to run.</summary>
public sealed record GMCommand(
    [property: JsonPropertyName("id")] long Id,
    [property: JsonPropertyName("action")] string Action,
    [property: JsonPropertyName("params")] JsonElement Params,
    [property: JsonPropertyName("status")] string Status);
