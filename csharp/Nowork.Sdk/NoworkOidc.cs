using System.Net.Http.Headers;
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

    public NoworkOidc(string issuer, string clientId, string clientSecret = "", string redirectUri = "", HttpClient? http = null)
    {
        _issuer = issuer.TrimEnd('/');
        _clientId = clientId; _clientSecret = clientSecret; _redirectUri = redirectUri;
        _http = http ?? new HttpClient { Timeout = TimeSpan.FromSeconds(10) };
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
