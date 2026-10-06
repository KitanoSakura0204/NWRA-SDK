using System;
using System.Collections.Generic;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace NoWork.Sdk;

/// <summary>Verifies webhook deliveries from the user center (X-NoWork-Signature / X-NoWork-Timestamp).</summary>
public static class NoWorkWebhook
{
    /// <summary>A verified webhook envelope.</summary>
    public sealed record Event(string Id, string EventName, long Timestamp, JsonElement Data);

    /// <summary>Thrown when the signature or timestamp is invalid.</summary>
    public sealed class InvalidSignatureException : Exception
    {
        public InvalidSignatureException(string message) : base(message) { }
    }

    /// <summary>sha256=HMAC_SHA256(secret, "&lt;ts&gt;.&lt;body&gt;")</summary>
    public static string Signature(string secret, long ts, ReadOnlySpan<byte> body)
    {
        using var mac = new HMACSHA256(Encoding.UTF8.GetBytes(secret));
        var prefix = Encoding.ASCII.GetBytes(ts + ".");
        var msg = new byte[prefix.Length + body.Length];
        prefix.CopyTo(msg, 0);
        body.CopyTo(msg.AsSpan(prefix.Length));
        return "sha256=" + Convert.ToHexString(mac.ComputeHash(msg)).ToLowerInvariant();
    }

    /// <summary>
    /// Verify and parse an incoming request.
    /// </summary>
    /// <param name="secret">Endpoint secret from the admin console.</param>
    /// <param name="header">Case-insensitive header lookup, e.g. <c>name => Request.Headers[name].ToString()</c>.</param>
    /// <param name="body">Raw request body.</param>
    /// <param name="tolerance">Replay window; <see cref="TimeSpan.Zero"/> disables.</param>
    public static Event Verify(string secret, Func<string, string?> header, byte[] body, TimeSpan tolerance)
    {
        if (!long.TryParse((header("X-NoWork-Timestamp") ?? string.Empty).Trim(), out var ts))
            throw new InvalidSignatureException("missing or invalid X-NoWork-Timestamp");
        if (tolerance > TimeSpan.Zero && Math.Abs(DateTimeOffset.UtcNow.ToUnixTimeSeconds() - ts) > tolerance.TotalSeconds)
            throw new InvalidSignatureException("webhook timestamp outside tolerance");
        var expected = Encoding.ASCII.GetBytes(Signature(secret, ts, body));
        var given = Encoding.ASCII.GetBytes((header("X-NoWork-Signature") ?? string.Empty).Trim());
        if (!CryptographicOperations.FixedTimeEquals(expected, given))
            throw new InvalidSignatureException("invalid webhook signature");
        using var doc = JsonDocument.Parse(body);
        var root = doc.RootElement;
        if (root.ValueKind != JsonValueKind.Object)
            throw new InvalidSignatureException("invalid webhook body");
        var id = root.TryGetProperty("id", out var idEl) ? idEl.GetString() ?? string.Empty : string.Empty;
        var name = root.TryGetProperty("event", out var evEl) ? evEl.GetString() ?? string.Empty : string.Empty;
        var timestamp = root.TryGetProperty("timestamp", out var tsEl) && tsEl.TryGetInt64(out var t) ? t : 0;
        var data = root.TryGetProperty("data", out var dataEl) ? dataEl.Clone() : JsonDocument.Parse("{}").RootElement.Clone();
        return new Event(id, name, timestamp, data);
    }
}
