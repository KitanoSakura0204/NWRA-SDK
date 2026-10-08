using System.Text.Json.Serialization;

namespace NoWork.Sdk;

public sealed record GMCharacter(
    [property: JsonPropertyName("id")] long Id,
    [property: JsonPropertyName("roleName")] string RoleName,
    [property: JsonPropertyName("roleUuid")] string RoleUuid);

/// <summary>Character identity only; this grants no GM administrator permissions.</summary>
public sealed record GMSession(
    [property: JsonPropertyName("serverId")] long ServerId,
    [property: JsonPropertyName("serverCode")] string ServerCode,
    [property: JsonPropertyName("subject")] string Subject,
    [property: JsonPropertyName("character")] GMCharacter Character,
    [property: JsonPropertyName("authenticatedAt")] long AuthenticatedAt,
    [property: JsonPropertyName("scope")] string Scope);

public sealed partial class NoworkClient
{
    /// <summary>Backend-only exchange. Validate callback state against the GM browser session first.</summary>
    public Task<GMSession> ExchangeGMCodeAsync(string code, string codeVerifier, string redirectUri, CancellationToken ct = default) =>
        CallAsync<GMSession>(HttpMethod.Post, "/server/v1/gm/sso/exchange", new { code, codeVerifier, redirectUri }, ct);
}
