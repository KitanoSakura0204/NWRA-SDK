using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;

namespace NoWork.Sdk;

/// <summary>RFC 6749 error.</summary>
public sealed class OAuthException : Exception
{
    public string Error { get; }
    public string Description { get; }
    public int Status { get; }
    public OAuthException(string error, string description, int status) : base($"nowork: oauth error {error}: {description}")
    { Error = error; Description = description; Status = status; }
}

/// <summary>Minimal OIDC relying-party helper (authorization code + PKCE, refresh, userinfo, introspection).</summary>
public sealed class NoworkOidc
{
    private readonly HttpClient _http;
    private readonly string _issuer, _clientId, _clientSecret, _redirectUri;
    private const int MaxMetadataBytes = 1024 * 1024;
    private static readonly HashSet<string> PublicDefaultHeaders = new(StringComparer.OrdinalIgnoreCase)
    { "Accept", "Accept-Encoding", "Accept-Language", "User-Agent" };

    public NoworkOidc(string issuer, string clientId, string clientSecret = "", string redirectUri = "", HttpClient? http = null)
    {
        _issuer = issuer.TrimEnd('/');
        _clientId = clientId; _clientSecret = clientSecret; _redirectUri = redirectUri;
        _http = http ?? new HttpClient(new HttpClientHandler { AllowAutoRedirect = false, UseCookies = false })
        { Timeout = TimeSpan.FromSeconds(10) };
    }

    /// <summary>Reads and validates discovery at the configured issuer's fixed path, without credentials.</summary>
    public async Task<JsonElement> DiscoverAsync(CancellationToken ct = default) =>
        (await MetadataAsync("/.well-known/openid-configuration", discovery: true, ct).ConfigureAwait(false)).Root;

    /// <summary>Reads and validates the configured issuer's /jwks.json, preserving the original JSON.</summary>
    public async Task<string> JwksAsync(CancellationToken ct = default) =>
        (await MetadataAsync("/jwks.json", discovery: false, ct).ConfigureAwait(false)).Json;

    private async Task<(string Json, JsonElement Root)> MetadataAsync(string path, bool discovery, CancellationToken ct)
    {
        ct.ThrowIfCancellationRequested();
        if (!Uri.TryCreate(_issuer, UriKind.Absolute, out var issuerUri) ||
            (issuerUri.Scheme != "http" && issuerUri.Scheme != "https") || issuerUri.UserInfo.Length != 0 ||
            issuerUri.Query.Length != 0 || issuerUri.Fragment.Length != 0)
            throw new ArgumentException("Metadata requires an absolute HTTP(S) issuer without credentials, query, or fragment.");
        // HttpClient merges defaults during SendAsync. Reject unknown defaults instead of modifying a shared client
        // or guessing which custom header contains credentials. Caller-owned handlers must also be unauthenticated.
        if (_http.DefaultRequestHeaders.Any(header => !PublicDefaultHeaders.Contains(header.Key)))
            throw new InvalidOperationException("Metadata requests require a client with only Accept, Accept-Encoding, Accept-Language, or User-Agent default headers.");
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(ct);
        if (_http.Timeout != Timeout.InfiniteTimeSpan) deadline.CancelAfter(_http.Timeout);
        var token = deadline.Token;
        var max = (int)Math.Min(MaxMetadataBytes, _http.MaxResponseContentBufferSize);
        try
        {
            var requestedUri = new Uri(_issuer + path);
            using var request = new HttpRequestMessage(HttpMethod.Get, requestedUri);
            request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
            using var response = await _http.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, token).ConfigureAwait(false);
            if (!response.IsSuccessStatusCode)
                throw new TransportException($"Metadata HTTP status {(int)response.StatusCode} was not successful.");
            if (request.RequestUri != requestedUri || response.RequestMessage?.RequestUri is Uri finalUri && finalUri != requestedUri)
                throw new TransportException("Metadata redirects are not accepted.");
            if (response.Content.Headers.ContentLength > max)
                throw new TransportException("Metadata response exceeded its byte limit.");
            await using var stream = await response.Content.ReadAsStreamAsync(token).ConfigureAwait(false);
            using var buffer = new MemoryStream();
            var chunk = new byte[Math.Min(16384, max + 1)];
            while (true)
            {
                var count = await stream.ReadAsync(chunk.AsMemory(0, Math.Min(chunk.Length, max - (int)buffer.Length + 1)), token).ConfigureAwait(false);
                if (count == 0) break;
                if (buffer.Length + count > max) throw new TransportException("Metadata response exceeded its byte limit.");
                buffer.Write(chunk, 0, count);
            }
            token.ThrowIfCancellationRequested();
            var bytes = buffer.GetBuffer().AsMemory(0, (int)buffer.Length);
            using var document = JsonDocument.Parse(bytes);
            var root = document.RootElement;
            if (root.ValueKind != JsonValueKind.Object) throw new TransportException("Metadata JSON must be an object.");
            if (discovery)
            {
                foreach (var name in new[] { "issuer", "authorization_endpoint", "token_endpoint", "jwks_uri" })
                    if (!root.TryGetProperty(name, out var value) || value.ValueKind != JsonValueKind.String || string.IsNullOrWhiteSpace(value.GetString()))
                        throw new TransportException("Discovery is missing a required string: " + name);
                if (!string.Equals(root.GetProperty("issuer").GetString(), _issuer, StringComparison.Ordinal))
                    throw new TransportException("Discovery issuer mismatch.");
            }
            else if (!root.TryGetProperty("keys", out var keys) || keys.ValueKind != JsonValueKind.Array ||
                keys.EnumerateArray().Any(key => key.ValueKind != JsonValueKind.Object))
                throw new TransportException("JWKS keys must be an array of objects.");
            token.ThrowIfCancellationRequested();
            return (Encoding.UTF8.GetString(bytes.Span), root.Clone());
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested) { throw new OperationCanceledException(ct); }
        catch (OperationCanceledException e) { throw new TransportException("Metadata request timed out.", e); }
        catch (Exception e) when (e is HttpRequestException or IOException or JsonException)
        { throw new TransportException("Metadata transport or JSON decoding failed.", e); }
    }

    public string AuthorizeUrl(string state, string? nonce = null, string scope = "openid profile email", string? codeChallenge = null)
    {
        var q = new List<string>
        {
            "response_type=code", "client_id=" + Uri.EscapeDataString(_clientId),
            "redirect_uri=" + Uri.EscapeDataString(_redirectUri), "scope=" + Uri.EscapeDataString(scope),
            "state=" + Uri.EscapeDataString(state)
        };
        if (nonce != null) q.Add("nonce=" + Uri.EscapeDataString(nonce));
        if (codeChallenge != null) { q.Add("code_challenge=" + Uri.EscapeDataString(codeChallenge)); q.Add("code_challenge_method=S256"); }
        return $"{_issuer}/oauth2/authorize?{string.Join("&", q)}";
    }

    private async Task<JsonElement> PostAsync(string path, Dictionary<string, string?> form, CancellationToken ct)
    {
        var f = form.Where(kv => kv.Value != null).ToDictionary(kv => kv.Key, kv => kv.Value!);
        f["client_id"] = _clientId;
        if (!string.IsNullOrEmpty(_clientSecret)) f["client_secret"] = _clientSecret;
        using var req = new HttpRequestMessage(HttpMethod.Post, _issuer + path) { Content = new FormUrlEncodedContent(f) };
        req.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        return await SendAsync(req, ct).ConfigureAwait(false);
    }

    private async Task<JsonElement> SendAsync(HttpRequestMessage req, CancellationToken ct)
    {
        HttpResponseMessage res;
        try { res = await _http.SendAsync(req, ct).ConfigureAwait(false); }
        catch (Exception e) when (e is HttpRequestException or TaskCanceledException) { throw new TransportException(e.Message, e); }
        var text = await res.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        var json = string.IsNullOrWhiteSpace(text) ? JsonDocument.Parse("{}").RootElement : JsonDocument.Parse(text).RootElement;
        if ((int)res.StatusCode >= 400)
        {
            var err = json.TryGetProperty("error", out var e1) ? e1.GetString() ?? "server_error" : "server_error";
            var desc = json.TryGetProperty("error_description", out var e2) ? e2.GetString() ?? "" : "";
            throw new OAuthException(err, desc, (int)res.StatusCode);
        }
        return json;
    }

    public Task<JsonElement> ExchangeAsync(string code, string? codeVerifier = null, CancellationToken ct = default) =>
        PostAsync("/oauth2/token", new() { ["grant_type"] = "authorization_code", ["code"] = code, ["redirect_uri"] = _redirectUri, ["code_verifier"] = codeVerifier }, ct);

    public Task<JsonElement> RefreshAsync(string refreshToken, CancellationToken ct = default) =>
        PostAsync("/oauth2/token", new() { ["grant_type"] = "refresh_token", ["refresh_token"] = refreshToken }, ct);

    public Task<JsonElement> ClientCredentialsAsync(string scope = "", CancellationToken ct = default) =>
        PostAsync("/oauth2/token", new() { ["grant_type"] = "client_credentials", ["scope"] = scope }, ct);

    public Task<JsonElement> IntrospectAsync(string token, CancellationToken ct = default) =>
        PostAsync("/oauth2/introspect", new() { ["token"] = token }, ct);

    public Task RevokeAsync(string token, CancellationToken ct = default) =>
        PostAsync("/oauth2/revoke", new() { ["token"] = token }, ct);

    public async Task<JsonElement> UserInfoAsync(string accessToken, CancellationToken ct = default)
    {
        using var req = new HttpRequestMessage(HttpMethod.Get, _issuer + "/oauth2/userinfo");
        req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", accessToken);
        req.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        return await SendAsync(req, ct).ConfigureAwait(false);
    }

    public string LogoutUrl(string? idToken = null, string? postLogoutRedirectUri = null, string? state = null)
    {
        var q = new List<string> { "client_id=" + Uri.EscapeDataString(_clientId) };
        if (idToken != null) q.Add("id_token_hint=" + Uri.EscapeDataString(idToken));
        if (postLogoutRedirectUri != null) q.Add("post_logout_redirect_uri=" + Uri.EscapeDataString(postLogoutRedirectUri));
        if (state != null) q.Add("state=" + Uri.EscapeDataString(state));
        return $"{_issuer}/oauth2/logout?{string.Join("&", q)}";
    }
}
