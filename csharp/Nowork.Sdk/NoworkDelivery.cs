using System;
using System.Linq;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace NoWork.Sdk;

/// <summary>
/// Delivery helpers (SDK/docs/DELIVERY.md): decrypt what the user center sends when an app registered an
/// encryption key. RSA-OAEP-256 or ECDH-ES+A256KW over P-256, enc A256GCM. X25519 keys are not supported
/// (the .NET class library has no X25519); register an RSA or EC P-256 key for .NET apps.
/// </summary>
public static class NoWorkDelivery
{
    /// <summary>A JWE could not be opened, or a check on its content failed.</summary>
    public sealed class DecryptException : Exception
    {
        public DecryptException(string message = "cannot decrypt the JWE") : base(message) { }
    }

    /// <summary>A decrypted JWE: plaintext and protected header.</summary>
    public sealed record Decrypted(byte[] Bytes, JsonElement Header)
    {
        public string Text => Encoding.UTF8.GetString(Bytes);
    }

    private static readonly byte[] DefaultIv = { 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6 };

    private static byte[] B64(string s)
    {
        var t = s.Replace('-', '+').Replace('_', '/');
        return Convert.FromBase64String(t + new string('=', (4 - t.Length % 4) % 4));
    }

    private static byte[] Pad(byte[] b, int n) => b.Length >= n ? b : new byte[n - b.Length].Concat(b).ToArray();

    private static string? Str(JsonElement o, string name) =>
        o.ValueKind == JsonValueKind.Object && o.TryGetProperty(name, out var v) && v.ValueKind == JsonValueKind.String ? v.GetString() : null;

    /// <summary>Opens a compact JWE; the header kid picks among several private JWKs, without a kid every key is tried.</summary>
    public static Decrypted Decrypt(string compact, params string[] privateJwks)
    {
        var parts = compact.Split('.');
        if (parts.Length != 5) throw new DecryptException();
        JsonElement hdr;
        byte[][] seg;
        try
        {
            seg = parts.Select(B64).ToArray();
            hdr = JsonDocument.Parse(seg[0]).RootElement.Clone();
        }
        catch (Exception) { throw new DecryptException(); }
        if (Str(hdr, "enc") != "A256GCM" || hdr.TryGetProperty("zip", out _) || hdr.TryGetProperty("crit", out _)) throw new DecryptException();
        var reason = "cannot decrypt the JWE";
        foreach (var text in privateJwks)
        {
            try
            {
                var jwk = JsonDocument.Parse(text).RootElement;
                var kid = Str(hdr, "kid");
                if (kid != null && Str(jwk, "kid") is string mine && mine != kid) continue;
                var cek = ContentKey(hdr, seg[1], jwk);
                var plain = new byte[seg[3].Length];
#if NET8_0_OR_GREATER
                using var gcm = new AesGcm(cek, 16);
#else
                using var gcm = new AesGcm(cek);
#endif
                gcm.Decrypt(seg[2], seg[3], seg[4], plain, Encoding.ASCII.GetBytes(parts[0]));
                return new Decrypted(plain, hdr);
            }
            catch (NotSupportedException e) { reason = e.Message; }
            catch (Exception) { /* wrong key or tampered: try the next key */ }
        }
        throw new DecryptException(reason);
    }

    private static byte[] Lp(byte[] b) =>
        new[] { (byte)(b.Length >> 24), (byte)(b.Length >> 16), (byte)(b.Length >> 8), (byte)b.Length }.Concat(b).ToArray();

    private static byte[] ContentKey(JsonElement hdr, byte[] ek, JsonElement jwk)
    {
        var alg = Str(hdr, "alg");
        if (alg == "RSA-OAEP-256" && Str(jwk, "kty") == "RSA")
        {
            var n = B64(Str(jwk, "n")!);
            var half = (n.Length + 1) / 2;
            using var rsa = RSA.Create();
            rsa.ImportParameters(new RSAParameters
            {
                Modulus = n, Exponent = B64(Str(jwk, "e")!), D = Pad(B64(Str(jwk, "d")!), n.Length),
                P = Pad(B64(Str(jwk, "p")!), half), Q = Pad(B64(Str(jwk, "q")!), half),
                DP = Pad(B64(Str(jwk, "dp")!), half), DQ = Pad(B64(Str(jwk, "dq")!), half), InverseQ = Pad(B64(Str(jwk, "qi")!), half),
            });
            return rsa.Decrypt(ek, RSAEncryptionPadding.OaepSHA256);
        }
        if (alg != "ECDH-ES+A256KW" || !hdr.TryGetProperty("epk", out var epk)) throw new DecryptException();
        if (Str(jwk, "kty") == "OKP" || Str(epk, "kty") == "OKP")
            throw new NotSupportedException("X25519 is not supported by .NET; register an RSA or EC P-256 key");
        if (Str(jwk, "kty") != "EC" || Str(jwk, "crv") != "P-256" || Str(epk, "crv") != "P-256") throw new DecryptException();
        using var priv = ECDiffieHellman.Create(new ECParameters
        {
            Curve = ECCurve.NamedCurves.nistP256, D = Pad(B64(Str(jwk, "d")!), 32),
            Q = new ECPoint { X = Pad(B64(Str(jwk, "x")!), 32), Y = Pad(B64(Str(jwk, "y")!), 32) },
        });
        using var peer = ECDiffieHellman.Create(new ECParameters
        {
            Curve = ECCurve.NamedCurves.nistP256,
            Q = new ECPoint { X = Pad(B64(Str(epk, "x")!), 32), Y = Pad(B64(Str(epk, "y")!), 32) },
        });
        // Concat KDF (JWA 4.6.2) for a 256-bit key is one round: SHA-256(counter || Z || OtherInfo).
        var apu = Str(hdr, "apu") is string u ? B64(u) : Array.Empty<byte>();
        var apv = Str(hdr, "apv") is string v ? B64(v) : Array.Empty<byte>();
        var info = Lp(Encoding.ASCII.GetBytes(alg)).Concat(Lp(apu)).Concat(Lp(apv)).Concat(new byte[] { 0, 0, 1, 0 }).ToArray();
        var kek = priv.DeriveKeyFromHash(peer.PublicKey, HashAlgorithmName.SHA256, new byte[] { 0, 0, 0, 1 }, info);
        return AesKeyUnwrap(kek, ek);
    }

    /// <summary>RFC 3394 AES key unwrap.</summary>
    private static byte[] AesKeyUnwrap(byte[] kek, byte[] wrapped)
    {
        if (wrapped.Length % 8 != 0 || wrapped.Length < 24) throw new DecryptException();
        using var aes = Aes.Create();
        aes.Key = kek;
        int n = wrapped.Length / 8 - 1;
        var a = wrapped[..8];
        var r = wrapped[8..];
        var buf = new byte[16];
        for (int j = 5; j >= 0; j--)
        {
            for (int i = n; i >= 1; i--)
            {
                ulong t = (ulong)(n * j + i);
                for (int k = 0; k < 8; k++) buf[k] = (byte)(a[k] ^ (byte)(t >> (56 - 8 * k)));
                Array.Copy(r, (i - 1) * 8, buf, 8, 8);
                var b = aes.DecryptEcb(buf, PaddingMode.None);
                Array.Copy(b, 0, a, 0, 8);
                Array.Copy(b, 8, r, (i - 1) * 8, 8);
            }
        }
        if (!CryptographicOperations.FixedTimeEquals(a, DefaultIv)) throw new DecryptException();
        return r;
    }

    /// <summary>Verifies an RS256 ID token against the provider JWKS JSON: signature, iss, aud, exp / nbf (60 s leeway).</summary>
    public static JsonElement VerifyIdToken(string token, string jwks, string? issuer, string? audience)
    {
        var parts = token.Split('.');
        if (parts.Length != 3) throw new DecryptException("not a signed JWT");
        var hdr = JsonDocument.Parse(B64(parts[0])).RootElement;
        if (Str(hdr, "alg") != "RS256") throw new DecryptException("ID token must be RS256");
        var kid = Str(hdr, "kid");
        JsonElement? found = null;
        foreach (var k in JsonDocument.Parse(jwks).RootElement.GetProperty("keys").EnumerateArray())
        {
            if (Str(k, "kty") == "RSA" && (kid == null || Str(k, "kid") == kid)) { found = k; break; }
        }
        if (found is not JsonElement jwk) throw new DecryptException("signing key not in the JWKS");
        using var rsa = RSA.Create();
        rsa.ImportParameters(new RSAParameters { Modulus = B64(Str(jwk, "n")!), Exponent = B64(Str(jwk, "e")!) });
        if (!rsa.VerifyData(Encoding.ASCII.GetBytes(parts[0] + "." + parts[1]), B64(parts[2]), HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1))
            throw new DecryptException("invalid ID token signature");
        var claims = JsonDocument.Parse(B64(parts[1])).RootElement.Clone();
        var now = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
        if (issuer != null && Str(claims, "iss") != issuer) throw new DecryptException("ID token issuer mismatch");
        if (audience != null && !HasAudience(claims, audience)) throw new DecryptException("ID token audience mismatch");
        if (!claims.TryGetProperty("exp", out var exp) || exp.ValueKind != JsonValueKind.Number || exp.GetInt64() + 60 < now)
            throw new DecryptException("ID token expired");
        if (claims.TryGetProperty("nbf", out var nbf) && nbf.ValueKind == JsonValueKind.Number && nbf.GetInt64() - 60 > now)
            throw new DecryptException("ID token not valid yet");
        return claims;
    }

    private static bool HasAudience(JsonElement claims, string audience)
    {
        if (!claims.TryGetProperty("aud", out var aud)) return false;
        if (aud.ValueKind == JsonValueKind.String) return aud.GetString() == audience;
        return aud.ValueKind == JsonValueKind.Array && aud.EnumerateArray().Any(a => a.ValueKind == JsonValueKind.String && a.GetString() == audience);
    }

    /// <summary>Opens a signed-then-encrypted ID token and verifies the inner token.</summary>
    public static JsonElement DecryptIdToken(string token, string jwks, string? issuer, string? audience, params string[] privateJwks) =>
        VerifyIdToken(Decrypt(token, privateJwks).Text, jwks, issuer, audience);

    /// <summary>
    /// Opens the plugin-API envelope (data {"jwe"}, or the whole response body) and returns the plaintext JSON.
    /// ctx, when not null, must equal the header ctx. A plain body is refused (fail closed).
    /// </summary>
    public static string OpenServerData(string response, string? ctx, params string[] privateJwks)
    {
        var root = JsonDocument.Parse(response).RootElement;
        var jwe = Str(root, "jwe");
        if (jwe == null && root.ValueKind == JsonValueKind.Object && root.TryGetProperty("data", out var data)) jwe = Str(data, "jwe");
        if (jwe == null) throw new DecryptException("response is not encrypted");
        var d = Decrypt(jwe, privateJwks);
        if (ctx != null && Str(d.Header, "ctx") != ctx) throw new DecryptException("envelope ctx mismatch");
        return d.Text;
    }

    /// <summary>
    /// Opens an encrypted webhook body ({"id","event","timestamp","jwe"}); verify the signature with
    /// NoWorkWebhook first. Returns the inner document JSON (it must repeat id, event and timestamp).
    /// </summary>
    public static string OpenWebhook(string body, params string[] privateJwks)
    {
        var outer = JsonDocument.Parse(body).RootElement;
        var jwe = Str(outer, "jwe") ?? throw new DecryptException("webhook is not encrypted");
        var d = Decrypt(jwe, privateJwks);
        if (!(Str(d.Header, "ctx") ?? "").StartsWith("webhook:", StringComparison.Ordinal)) throw new DecryptException("envelope ctx mismatch");
        var doc = JsonDocument.Parse(d.Bytes).RootElement;
        if (Str(doc, "id") != Str(outer, "id") || Str(doc, "event") != Str(outer, "event")
            || doc.GetProperty("timestamp").GetInt64() != outer.GetProperty("timestamp").GetInt64())
            throw new DecryptException("webhook envelope does not match its content");
        return d.Text;
    }
}
