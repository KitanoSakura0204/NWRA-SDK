using System.Buffers;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;

namespace NoWork.Sdk;

/// <summary>Per-client limits. No cookies, default headers, or credentials are inherited from another client.</summary>
public sealed record GameDataClientOptions
{
    public TimeSpan Timeout { get; init; } = TimeSpan.FromSeconds(30);
    public int MaxResponseBytes { get; init; } = 8 * 1024 * 1024;
    public int MaxExportBytes { get; init; } = 24 * 1024 * 1024;
    /// <summary>Optional caller-owned handler for tests/proxies. It must not follow redirects or inject credentials/cookies.</summary>
    public HttpMessageHandler? Handler { get; init; }
}

/// <summary>Additional information attached to the SDK's existing ApiException/TransportException.</summary>
public sealed record GameDataErrorDetails(int? HttpStatus, string Reason, JsonElement Data, string? RetryAfter);

public static class GameDataErrors
{
    private const string Key = "NoWork.Sdk.GameDataErrorDetails";
    public static GameDataErrorDetails? Details(Exception error) => error.Data[Key] as GameDataErrorDetails;
    internal static T Attach<T>(T error, int? status, string reason, JsonElement data = default, string? retryAfter = null) where T : Exception
    {
        error.Data[Key] = new GameDataErrorDetails(status, reason, data, retryAfter);
        return error;
    }
}

/// <summary>Iteration stopped without silently presenting a partial result as complete.</summary>
public sealed class GameDataPaginationException : Exception
{
    public string Reason { get; }
    public int PagesRead { get; }
    internal GameDataPaginationException(string reason, int pagesRead)
        : base("nowork: game pagination stopped: " + reason) => (Reason, PagesRead) = (reason, pagesRead);
}

// Intentionally separate from the server-key client: a caller-supplied HttpClient there
// carries mutable default credentials. Each of these transports owns its request headers.
internal sealed class GameDataTransport : IDisposable
{
    internal static readonly JsonSerializerOptions Json = new() { PropertyNameCaseInsensitive = true };
    private readonly HttpClient _http;
    private readonly Uri _base;
    private readonly string _prefix;
    private readonly string? _token;
    private readonly GameDataClientOptions _options;

    internal GameDataTransport(string baseUrl, string prefix, string? token, GameDataClientOptions? options)
    {
        if (!Uri.TryCreate(baseUrl, UriKind.Absolute, out var uri) ||
            (uri.Scheme != "https" && uri.Scheme != "http") || uri.UserInfo.Length != 0 ||
            uri.Query.Length != 0 || uri.Fragment.Length != 0)
            throw new ArgumentException("Use an absolute HTTP(S) site URL without credentials, query, or fragment.", nameof(baseUrl));
        if (prefix != "/api/hypergryph/" && (string.IsNullOrWhiteSpace(token) || token.Any(char.IsWhiteSpace) || token.Any(char.IsControl)))
            throw new ArgumentException("A site login JWT is required for this client.", nameof(token));
        _options = options ?? new GameDataClientOptions();
        if (_options.Timeout <= TimeSpan.Zero || _options.Timeout > TimeSpan.FromMinutes(10))
            throw new ArgumentOutOfRangeException(nameof(options), "Timeout must be positive and at most ten minutes.");
        if (_options.MaxResponseBytes < 1 || _options.MaxResponseBytes > 32 * 1024 * 1024 ||
            _options.MaxExportBytes < 1 || _options.MaxExportBytes > 32 * 1024 * 1024)
            throw new ArgumentOutOfRangeException(nameof(options), "Response limits must be between one byte and 32 MiB.");
        var handler = _options.Handler;
        // A supplied ordinary handler cannot accidentally keep its permissive defaults.
        if (handler is HttpClientHandler h && (h.AllowAutoRedirect || h.UseCookies || h.Credentials != null || h.UseDefaultCredentials) ||
            handler is SocketsHttpHandler s && (s.AllowAutoRedirect || s.UseCookies || s.Credentials != null))
            throw new ArgumentException("The handler must disable redirects, cookies, and automatic credentials.", nameof(options));
        _http = handler == null
            ? new HttpClient(new HttpClientHandler { AllowAutoRedirect = false, UseCookies = false })
            : new HttpClient(handler, disposeHandler: false);
        _http.Timeout = System.Threading.Timeout.InfiniteTimeSpan;
        _base = new Uri(uri.AbsoluteUri.TrimEnd('/') + "/");
        _prefix = prefix;
        _token = token;
    }

    internal async Task<T> SendAsync<T>(HttpMethod method, string path, object? body, CancellationToken ct, bool export = false, bool allowEmpty = false)
    {
        ct.ThrowIfCancellationRequested();
        if (!path.StartsWith(_prefix, StringComparison.Ordinal)) throw new InvalidOperationException("Game API token scope mismatch.");
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(ct);
        deadline.CancelAfter(_options.Timeout);
        var token = deadline.Token;
        try
        {
            using var request = new HttpRequestMessage(method, new Uri(_base, path.TrimStart('/')));
            request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
            if (_token != null) request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _token);
            if (body != null) request.Content = new StringContent(JsonSerializer.Serialize(body, Json), Encoding.UTF8, "application/json");
            using var response = await _http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, token).ConfigureAwait(false);
            var status = (int)response.StatusCode;
            var retry = response.Headers.TryGetValues("Retry-After", out var values) ? string.Join(", ", values) : null;
            if (status >= 300 && status < 400)
                throw Failure("Redirects and bodyless cache responses are not accepted.", status, "redirect_or_not_modified", retryAfter: retry);
            var max = export ? _options.MaxExportBytes : _options.MaxResponseBytes;
            if (response.Content.Headers.ContentLength > max) throw Failure("Game response exceeded its byte limit.", status, "response_too_large");
            await using var stream = await response.Content.ReadAsStreamAsync(token).ConfigureAwait(false);
            using var buffer = new MemoryStream();
            var chunk = ArrayPool<byte>.Shared.Rent(16384);
            try
            {
                while (true)
                {
                    var count = await stream.ReadAsync(chunk.AsMemory(0, Math.Min(chunk.Length, max - (int)buffer.Length + 1)), token).ConfigureAwait(false);
                    if (count == 0) break;
                    if (buffer.Length + count > max) throw Failure("Game response exceeded its byte limit.", status, "response_too_large");
                    buffer.Write(chunk, 0, count);
                }
            }
            finally { ArrayPool<byte>.Shared.Return(chunk, clearArray: true); }
            token.ThrowIfCancellationRequested();
            JsonDocument document;
            try { document = JsonDocument.Parse(buffer.GetBuffer().AsMemory(0, checked((int)buffer.Length))); }
            catch (JsonException e) { throw Failure("Invalid game response JSON.", status, "invalid_json", e, retry); }
            using (document)
            {
                var root = document.RootElement;
                if (root.ValueKind != JsonValueKind.Object || !root.TryGetProperty("code", out var codeValue) || !codeValue.TryGetInt32Safe(out var code))
                    throw Failure("Game response is missing a numeric code.", status, "invalid_envelope", retryAfter: retry);
                var data = root.TryGetProperty("data", out var d) ? d : default;
                var message = root.TryGetProperty("message", out var m) && m.ValueKind == JsonValueKind.String ? m.GetString() ?? "" : "";
                if (code != 0) throw GameDataErrors.Attach(new ApiException(code, message), status, "api_error", data.ValueKind == JsonValueKind.Undefined ? default : data.Clone(), retry);
                if (!response.IsSuccessStatusCode) throw Failure("Game response HTTP status was not successful.", status, "http_error", retryAfter: retry);
                if (response.Headers.Contains(NoworkClient.HeaderEncryption) || data.ValueKind == JsonValueKind.Object && data.TryGetProperty("jwe", out _))
                    throw new EncryptionException(path.Split('?')[0], "encrypted data is not valid for this JWT/public route");
                if (data.ValueKind is JsonValueKind.Undefined or JsonValueKind.Null)
                {
                    if (allowEmpty) return default!;
                    throw Failure("Game response is missing data.", status, "missing_data");
                }
                try
                {
                    var result = data.Deserialize<T>(Json);
                    token.ThrowIfCancellationRequested();
                    if (result is null) throw Failure("Game response is missing data.", status, "missing_data");
                    if (result is GameSnapshot snapshot && (string.IsNullOrEmpty(snapshot.GameId) || snapshot.Sections == null ||
                        !data.TryGetProperty("fetched_at", out _)))
                        throw Failure("Game snapshot is missing required metadata.", status, "invalid_data");
                    return result;
                }
                catch (JsonException e) { throw Failure("Invalid game response data.", status, "invalid_data", e); }
            }
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested) { throw new OperationCanceledException(ct); }
        catch (OperationCanceledException e) { throw Failure("Game request timed out.", null, "timeout", e); }
        catch (Exception e) when (e is HttpRequestException or IOException) { throw Failure("Game request transport failed.", null, "network", e); }
    }

    private static TransportException Failure(string message, int? status, string reason, Exception? inner = null, string? retryAfter = null) =>
        GameDataErrors.Attach(new TransportException(message, inner), status, reason, retryAfter: retryAfter);

    public void Dispose() => _http.Dispose();
}

internal static class GameJsonValue
{
    internal static bool TryGetInt32Safe(this JsonElement value, out int result)
    {
        result = default;
        return value.ValueKind == JsonValueKind.Number && value.TryGetInt32(out result);
    }
}
