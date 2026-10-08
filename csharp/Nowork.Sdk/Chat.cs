using System.Text.Json;
using System.Text.Json.Serialization;

namespace NoWork.Sdk;

public sealed record ChatTicketInput(
    [property: JsonPropertyName("accessToken")] string? AccessToken = null,
    [property: JsonPropertyName("uid")] long Uid = 0,
    [property: JsonPropertyName("playerName")] string? PlayerName = null,
    [property: JsonPropertyName("playerUuid")] string? PlayerUuid = null,
    [property: JsonPropertyName("channels")] IReadOnlyList<long>? Channels = null);

public sealed record ChatTicket(
    [property: JsonPropertyName("ticket")] string Ticket,
    [property: JsonPropertyName("expiresAt")] string ExpiresAt,
    [property: JsonPropertyName("channels")] IReadOnlyList<long> Channels,
    [property: JsonPropertyName("embedPath")] string EmbedPath,
    [property: JsonPropertyName("wsPath")] string WsPath);

public sealed record ChatRelayFailure([property: JsonPropertyName("id")] long Id, [property: JsonPropertyName("error")] string Error);

public sealed partial class NoworkClient
{
    /// <summary>One-use chat login: exactly one of AccessToken or Uid; UID issuance requires server permission.</summary>
    public Task<ChatTicket> IssueChatTicketAsync(ChatTicketInput input, CancellationToken ct = default) =>
        CallAsync<ChatTicket>(HttpMethod.Post, "/server/v1/chat/tickets", input, ct);

    public async Task<JsonElement> PollChatRelayAsync(CancellationToken ct = default) =>
        (await CallAsync<JsonElement>(HttpMethod.Get, "/server/v1/chat/relay", null, ct).ConfigureAwait(false)).GetProperty("items");

    /// <summary>Failed entries are permanent failures. Leave transient failures unacknowledged.</summary>
    public async Task<long> AckChatRelayAsync(IReadOnlyList<long> done, IReadOnlyList<ChatRelayFailure>? failed = null, CancellationToken ct = default) =>
        (await CallAsync<JsonElement>(HttpMethod.Post, "/server/v1/chat/relay/ack", new { done, failed = failed ?? Array.Empty<ChatRelayFailure>() }, ct).ConfigureAwait(false)).GetProperty("updated").GetInt64();

    public Task<JsonElement> PostChatMessageAsync(string player, string text, string playerUuid = "", long uid = 0, string kind = "chat", CancellationToken ct = default) =>
        CallAsync<JsonElement>(HttpMethod.Post, "/server/v1/chat/messages", new { player, text, playerUuid, uid, kind }, ct);
}
