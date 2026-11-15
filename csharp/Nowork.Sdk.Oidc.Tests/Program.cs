using System.Net;
using System.Net.Http.Headers;
using System.Runtime.InteropServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using NoWork.Sdk;

internal static class Program
{
    private const string Issuer = "https://account.example.test";
    private const int Limit = 1024 * 1024;

    public static async Task Main()
    {
        var tests = new (string Name, Func<Task> Run)[]
        {
            ("fixed unauthenticated metadata routes and lossless fields", MetadataRoutes),
            ("invalid status and malformed/incomplete metadata fail closed", InvalidMetadata),
            ("invalid UTF-8 in unknown discovery and JWK strings fails closed", InvalidUtf8Metadata),
            ("default credentials are rejected without modifying the client", DefaultCredentials),
            ("declared/chunked bodies respect both response limits", ResponseLimits),
            ("body timeout and caller cancellation remain effective", Cancellation),
            ("signed nonce binding preserves existing verification and validation order", SignedNonce),
            ("encrypted nonce binding verifies the inner signature and preserves params calls", EncryptedNonce),
        };
        foreach (var (name, run) in tests) { await run(); Console.WriteLine("PASS " + name); }
        Console.WriteLine($"{tests.Length} tests passed on {RuntimeInformation.FrameworkDescription}.");
    }

    private static async Task InvalidUtf8Metadata()
    {
        var accepted = new List<string>();
        foreach (var discovery in new[] { true, false })
        {
            foreach (var invalid in new[] { new byte[] { 0xff }, new byte[] { 0xc2 }, new byte[] { 0xc0, 0xaf }, new byte[] { 0xed, 0xa0, 0x80 } })
            {
                var prefix = discovery ? Discovery()[..^1] + ",\"unknown\":\"" : "{\"keys\":[{\"future\":\"";
                var suffix = discovery ? "\"}" : "\"}]}";
                var bytes = Encoding.UTF8.GetBytes(prefix).Concat(invalid).Concat(Encoding.UTF8.GetBytes(suffix)).ToArray();
                foreach (var streamed in new[] { false, true })
                {
                    using var handler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
                    { Content = streamed ? new StreamContent(new NonSeekableStream(bytes)) : new ByteArrayContent(bytes) }));
                    using var http = new HttpClient(handler);
                    var oidc = new NoworkOidc(Issuer, "client", http: http);
                    try
                    {
                        if (discovery) await oidc.DiscoverAsync(); else await oidc.JwksAsync();
                        accepted.Add($"{(discovery ? "DiscoverAsync" : "JwksAsync")}({Convert.ToHexString(invalid)}, streamed={streamed})");
                    }
                    catch (TransportException error) { True(error.InnerException is DecoderFallbackException); }
                }
            }
        }
        if (accepted.Count != 0) throw new Exception("Invalid UTF-8 was accepted by " + string.Join(", ", accepted));
        const string unicode = "未来🙂";
        var validDiscovery = Discovery()[..^1] + ",\"unknown\":\"" + unicode + "\"}";
        var validJwks = "{\"keys\":[{\"future\":\"" + unicode + "\"}]}";
        using var validHandler = new FakeHandler((request, _) => Task.FromResult(Raw(
            request.RequestUri!.AbsolutePath == "/jwks.json" ? validJwks : validDiscovery)));
        using var validHttp = new HttpClient(validHandler);
        var validOidc = new NoworkOidc(Issuer, "client", http: validHttp);
        Eq(unicode, (await validOidc.DiscoverAsync()).GetProperty("unknown").GetString());
        Eq(validJwks, await validOidc.JwksAsync());
    }

    private static Task SignedNonce()
    {
        using var signing = RSA.Create(2048);
        var jwks = PublicJwks(signing);
        var claims = Claims();
        var token = Sign(signing, claims);
        var verified = NoWorkDelivery.VerifyIdToken(token, jwks, Issuer, "client", "nonce-1");
        Eq("account-42", verified.GetProperty("sub").GetString());
        Eq(ulong.MaxValue, verified.GetProperty("future").GetUInt64());
        Eq(claims["exp"], verified.GetProperty("exp").GetInt64());
        NoWorkDelivery.VerifyIdToken(token, jwks, Issuer, "client");
        foreach (var expectation in new[] { "nonce-2", "", null! })
            ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.VerifyIdToken(token, jwks, Issuer, "client", expectation));
        foreach (var nonce in new object?[] { null, 42, new[] { "nonce-1" }, true, "" })
        {
            var invalid = Claims(); invalid["nonce"] = nonce;
            ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.VerifyIdToken(Sign(signing, invalid), jwks, Issuer, "client", "nonce-1"));
        }
        var missing = Claims(); missing.Remove("nonce"); var noNonce = Sign(signing, missing);
        NoWorkDelivery.VerifyIdToken(noNonce, jwks, Issuer, "client");
        ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.VerifyIdToken(noNonce, jwks, Issuer, "client", "nonce-1"));
        claims["aud"] = new[] { "another-client", "client" };
        NoWorkDelivery.VerifyIdToken(Sign(signing, claims), jwks, Issuer, "client", "nonce-1");
        foreach (var (claim, value, reason) in new (string, object, string)[]
        {
            ("iss", "https://wrong.example.test", "issuer mismatch"), ("aud", "wrong-client", "audience mismatch"),
            ("exp", DateTimeOffset.UtcNow.ToUnixTimeSeconds() - 600, "expired"),
            ("nbf", DateTimeOffset.UtcNow.ToUnixTimeSeconds() + 600, "not valid yet")
        })
        {
            var invalid = Claims(); invalid[claim] = value;
            var error = ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.VerifyIdToken(Sign(signing, invalid), jwks, Issuer, "client", "wrong-nonce"));
            True(error.Message.Contains(reason));
        }
        var tampered = TamperSignature(token);
        True(ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.VerifyIdToken(tampered, jwks, Issuer, "client", "wrong-nonce")).Message.Contains("signature"));
        return Task.CompletedTask;
    }

    private static Task EncryptedNonce()
    {
        using var signing = RSA.Create(2048);
        using var encryption = RSA.Create(2048);
        using var wrongEncryption = RSA.Create(2048);
        var jwks = PublicJwks(signing); var key = PrivateJwk(encryption); var wrongKey = PrivateJwk(wrongEncryption);
        var token = Sign(signing, Claims()); var encrypted = Encrypt(encryption, token);
        Eq("nonce-1", NoWorkDelivery.DecryptIdTokenWithNonce(encrypted, jwks, Issuer, "client", "nonce-1", key).GetProperty("nonce").GetString());
        NoWorkDelivery.DecryptIdTokenWithNonce(encrypted, jwks, Issuer, "client", "nonce-1", wrongKey, key);
        NoWorkDelivery.DecryptIdTokenWithNonce(encrypted, jwks, Issuer, "client", "nonce-1", new[] { wrongKey, key });
        // These positional calls must keep treating every trailing string as a private JWK.
        NoWorkDelivery.DecryptIdToken(encrypted, jwks, Issuer, "client", key);
        NoWorkDelivery.DecryptIdToken(encrypted, jwks, Issuer, "client", wrongKey, key);
        NoWorkDelivery.DecryptIdToken(encrypted, jwks, Issuer, "client", new[] { wrongKey, key });
        ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.DecryptIdTokenWithNonce(encrypted, jwks, Issuer, "client", "wrong-nonce", key));
        ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.DecryptIdTokenWithNonce(encrypted, jwks, Issuer, "client", "", key));
        var missing = Claims(); missing.Remove("nonce");
        ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.DecryptIdTokenWithNonce(Encrypt(encryption, Sign(signing, missing)), jwks, Issuer, "client", "nonce-1", key));
        var unsignedInner = Encrypt(encryption, "{\"nonce\":\"nonce-1\"}");
        ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.DecryptIdTokenWithNonce(unsignedInner, jwks, Issuer, "client", "nonce-1", key));
        var badSignature = Encrypt(encryption, TamperSignature(token));
        True(ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.DecryptIdTokenWithNonce(badSignature, jwks, Issuer, "client", "nonce-1", key)).Message.Contains("signature"));
        var expired = Claims(); expired["exp"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds() - 600;
        True(ThrowsSync<NoWorkDelivery.DecryptException>(() => NoWorkDelivery.DecryptIdTokenWithNonce(
            Encrypt(encryption, Sign(signing, expired)), jwks, Issuer, "client", "nonce-1", key)).Message.Contains("expired"));
        return Task.CompletedTask;
    }

    private static Dictionary<string, object?> Claims() => new()
    {
        ["iss"] = Issuer, ["aud"] = "client", ["sub"] = "account-42", ["nonce"] = "nonce-1",
        ["exp"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds() + 600, ["nbf"] = DateTimeOffset.UtcNow.ToUnixTimeSeconds() - 10,
        ["future"] = ulong.MaxValue
    };
    private static string B64(byte[] bytes) => Convert.ToBase64String(bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_');
    private static string Sign(RSA key, Dictionary<string, object?> claims)
    {
        var header = B64(Encoding.UTF8.GetBytes("{\"alg\":\"RS256\",\"kid\":\"signing\"}"));
        var body = B64(JsonSerializer.SerializeToUtf8Bytes(claims)); var input = header + "." + body;
        return input + "." + B64(key.SignData(Encoding.ASCII.GetBytes(input), HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1));
    }
    private static string PublicJwks(RSA key)
    {
        var p = key.ExportParameters(false);
        return JsonSerializer.Serialize(new { keys = new[] { new { kty = "RSA", kid = "signing", n = B64(p.Modulus!), e = B64(p.Exponent!) } } });
    }
    private static string PrivateJwk(RSA key)
    {
        var p = key.ExportParameters(true);
        return JsonSerializer.Serialize(new { kty = "RSA", n = B64(p.Modulus!), e = B64(p.Exponent!), d = B64(p.D!),
            p = B64(p.P!), q = B64(p.Q!), dp = B64(p.DP!), dq = B64(p.DQ!), qi = B64(p.InverseQ!) });
    }
    private static string Encrypt(RSA key, string plaintext)
    {
        var header = B64(Encoding.UTF8.GetBytes("{\"alg\":\"RSA-OAEP-256\",\"enc\":\"A256GCM\",\"cty\":\"JWT\"}"));
        var cek = RandomNumberGenerator.GetBytes(32); var iv = RandomNumberGenerator.GetBytes(12);
        var input = Encoding.UTF8.GetBytes(plaintext); var encrypted = new byte[input.Length]; var tag = new byte[16];
        using var aes = new AesGcm(cek);
        aes.Encrypt(iv, input, encrypted, tag, Encoding.ASCII.GetBytes(header));
        return string.Join(".", header, B64(key.Encrypt(cek, RSAEncryptionPadding.OaepSHA256)), B64(iv), B64(encrypted), B64(tag));
    }
    private static string TamperSignature(string token)
    {
        var segments = token.Split('.');
        segments[2] = (segments[2][0] == 'A' ? "B" : "A") + segments[2][1..];
        return string.Join(".", segments);
    }

    private static async Task InvalidMetadata()
    {
        var invalidDiscovery = new[] { "", "{", "[]", "null", "42", "{}",
            Discovery().Replace(Issuer, Issuer + "/", StringComparison.Ordinal),
            Discovery().Replace("\"issuer\":", "\"other\":", StringComparison.Ordinal),
            Discovery().Replace("\"jwks_uri\":", "\"missing\":", StringComparison.Ordinal),
            Discovery().Replace("https://advertised.example.test/token", "", StringComparison.Ordinal),
            Discovery().Replace("\"https://advertised.example.test/authorize\"", "17", StringComparison.Ordinal) };
        var invalidJwks = new[] { "", "{", "[]", "null", "{}", "{\"keys\":{}}", "{\"keys\":[null]}",
            "{\"keys\":[\"key\"]}", "{\"keys\":[{},1]}", "{\"keys\":false}" };
        using var handler = new FakeHandler((_, _) => Task.FromResult(Raw("{}")));
        using var http = new HttpClient(handler);
        var oidc = new NoworkOidc(Issuer, "client", http: http);
        foreach (var body in invalidDiscovery)
        {
            using var h = new FakeHandler((_, _) => Task.FromResult(Raw(body)));
            using var c = new HttpClient(h);
            await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: c).DiscoverAsync());
            Eq(1, h.Requests.Count);
        }
        foreach (var body in invalidJwks)
        {
            using var h = new FakeHandler((_, _) => Task.FromResult(Raw(body)));
            using var c = new HttpClient(h);
            await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: c).JwksAsync());
        }
        foreach (var status in new[] { 204, 301, 302, 304, 400, 500 })
        {
            using var h = new FakeHandler((request, _) => Task.FromResult(Raw(status == 204 ? "" :
                request.RequestUri!.AbsolutePath == "/jwks.json" ? "{\"keys\":[]}" : Discovery(), status)));
            using var c = new HttpClient(h);
            var o = new NoworkOidc(Issuer, "client", http: c);
            await Throws<TransportException>(() => o.DiscoverAsync()); await Throws<TransportException>(() => o.JwksAsync());
        }
        using var emptyKeysHandler = new FakeHandler((_, _) => Task.FromResult(Raw("{\"keys\":[]}")));
        using var emptyKeysClient = new HttpClient(emptyKeysHandler);
        Eq("{\"keys\":[]}", await new NoworkOidc(Issuer, "client", http: emptyKeysClient).JwksAsync());
        using var redirectHandler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
        { Content = new StringContent(Discovery()), RequestMessage = new HttpRequestMessage(HttpMethod.Get, "https://other.example.test/metadata") }));
        using var redirectClient = new HttpClient(redirectHandler);
        await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: redirectClient).DiscoverAsync());
        using var changingRedirectHandler = new FakeHandler((request, _) =>
        {
            request.RequestUri = new Uri("https://other.example.test/metadata");
            return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            { Content = new StringContent(Discovery()), RequestMessage = request });
        });
        using var changingRedirectClient = new HttpClient(changingRedirectHandler);
        await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: changingRedirectClient).DiscoverAsync());
        foreach (var issuer in new[] { "https://user:secret@example.test", Issuer + "?secret=value", Issuer + "#fragment", "relative" })
            await Throws<ArgumentException>(() => new NoworkOidc(issuer, "client", http: http).DiscoverAsync());
        Eq(0, handler.Requests.Count);
    }

    private static async Task DefaultCredentials()
    {
        foreach (var header in new[] { "Authorization", "Proxy-Authorization", "Cookie", "X-Server-Key", "X-Client-Secret", "X-Api-Key" })
        {
            using var handler = new FakeHandler((_, _) => Task.FromResult(Raw(Discovery())));
            using var http = new HttpClient(handler);
            http.DefaultRequestHeaders.TryAddWithoutValidation(header, "secret");
            var oidc = new NoworkOidc(Issuer, "client", http: http);
            await Throws<InvalidOperationException>(() => oidc.DiscoverAsync());
            await Throws<InvalidOperationException>(() => oidc.JwksAsync());
            Eq(0, handler.Requests.Count); Eq("secret", http.DefaultRequestHeaders.GetValues(header).Single());
        }
        using var mutableHandler = new FakeHandler((_, _) => Task.FromResult(Raw(Discovery())));
        using var mutableHttp = new HttpClient(mutableHandler);
        var mutableOidc = new NoworkOidc(Issuer, "client", http: mutableHttp);
        await mutableOidc.DiscoverAsync();
        mutableHttp.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", "late-secret");
        await Throws<InvalidOperationException>(() => mutableOidc.DiscoverAsync()); Eq(1, mutableHandler.Requests.Count);
        Eq("Bearer late-secret", mutableHttp.DefaultRequestHeaders.Authorization.ToString());
    }

    private static async Task ResponseLimits()
    {
        foreach (var chunked in new[] { false, true })
        {
            using var handler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            { Content = chunked ? new StreamContent(new NonSeekableStream(Encoding.UTF8.GetBytes(new string(' ', Limit + 1)))) : new StringContent(new string(' ', Limit + 1)) }));
            using var http = new HttpClient(handler);
            var oidc = new NoworkOidc(Issuer, "client", http: http);
            await Throws<TransportException>(() => oidc.DiscoverAsync()); await Throws<TransportException>(() => oidc.JwksAsync());
        }
        var exact = "{\"keys\":[]}" + new string(' ', Limit - "{\"keys\":[]}".Length);
        using var exactHandler = new FakeHandler((_, _) => Task.FromResult(Raw(exact)));
        using var exactHttp = new HttpClient(exactHandler);
        Eq(exact, await new NoworkOidc(Issuer, "client", http: exactHttp).JwksAsync());
        foreach (var chunked in new[] { false, true })
        {
            using var h = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            { Content = chunked ? new StreamContent(new NonSeekableStream(Encoding.UTF8.GetBytes(Discovery()))) : new StringContent(Discovery()) }));
            using var c = new HttpClient(h) { MaxResponseContentBufferSize = 64 };
            await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: c).DiscoverAsync());
            Eq(64L, c.MaxResponseContentBufferSize);
        }
    }

    private static async Task Cancellation()
    {
        using var preHandler = new FakeHandler((_, _) => Task.FromResult(Raw(Discovery())));
        using var preHttp = new HttpClient(preHandler);
        using var pre = new CancellationTokenSource(); pre.Cancel();
        var canceled = await Throws<OperationCanceledException>(() => new NoworkOidc(Issuer, "client", http: preHttp).DiscoverAsync(pre.Token));
        Eq(pre.Token, canceled.CancellationToken); Eq(0, preHandler.Requests.Count);
        var body = new BlockingStream();
        using var handler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new StreamContent(body) }));
        using var http = new HttpClient(handler);
        using var ct = new CancellationTokenSource();
        var request = new NoworkOidc(Issuer, "client", http: http).JwksAsync(ct.Token);
        await body.Started.Task.WaitAsync(TimeSpan.FromSeconds(2)); ct.Cancel();
        Eq(ct.Token, (await Throws<OperationCanceledException>(() => request)).CancellationToken); True(body.Disposed);
        var slow = new BlockingStream();
        using var timeoutHandler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new StreamContent(slow) }));
        using var timeoutHttp = new HttpClient(timeoutHandler) { Timeout = TimeSpan.FromMilliseconds(100) };
        var error = await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: timeoutHttp).DiscoverAsync());
        True(error.InnerException is OperationCanceledException); True(slow.Disposed); Eq(TimeSpan.FromMilliseconds(100), timeoutHttp.Timeout);
        using var headerHandler = new FakeHandler(async (_, ct) => { await Task.Delay(Timeout.Infinite, ct); return Raw(Discovery()); });
        using var headerHttp = new HttpClient(headerHandler) { Timeout = TimeSpan.FromMilliseconds(100) };
        True((await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: headerHttp).JwksAsync())).InnerException is OperationCanceledException);
        using var networkHandler = new FakeHandler((_, _) => throw new HttpRequestException("fixture network failure"));
        using var networkHttp = new HttpClient(networkHandler);
        True((await Throws<TransportException>(() => new NoworkOidc(Issuer, "client", http: networkHttp).DiscoverAsync())).InnerException is HttpRequestException);
    }

    private static async Task MetadataRoutes()
    {
        var discovery = Discovery();
        const string jwks = "{\"keys\":[{\"kty\":\"RSA\",\"kid\":\"key-1\",\"future\":{\"large\":18446744073709551615}}],\"rotation\":false}";
        using var handler = new FakeHandler((request, _) => Task.FromResult(Raw(
            request.RequestUri!.AbsolutePath == "/jwks.json" ? jwks : discovery)));
        using var http = new HttpClient(handler);
        http.DefaultRequestHeaders.UserAgent.ParseAdd("oidc-test/1.0");
        var oidc = new NoworkOidc(Issuer + "///", "client", "client-secret", "https://app.test/callback", http);
        var metadata = await oidc.DiscoverAsync();
        Eq(Issuer, metadata.GetProperty("issuer").GetString());
        Eq(ulong.MaxValue, metadata.GetProperty("future").GetProperty("large").GetUInt64());
        Eq(jwks, await oidc.JwksAsync());
        Eq(2, handler.Requests.Count);
        Eq(Issuer + "/.well-known/openid-configuration", handler.Requests[0].Uri);
        Eq(Issuer + "/jwks.json", handler.Requests[1].Uri);
        foreach (var request in handler.Requests)
        {
            Eq("GET", request.Method); Eq("application/json", request.Accept);
            True(!request.HasCredentials); True(!request.HasBody);
            True(!request.Uri.Contains("client-secret"));
        }
        Eq("oidc-test/1.0", http.DefaultRequestHeaders.UserAgent.ToString());
    }

    private static string Discovery() => JsonSerializer.Serialize(new
    {
        issuer = Issuer, authorization_endpoint = "https://advertised.example.test/authorize",
        token_endpoint = "https://advertised.example.test/token", jwks_uri = "https://advertised.example.test/keys",
        future = new { large = ulong.MaxValue }
    });
    private static HttpResponseMessage Raw(string body, int status = 200) => new((HttpStatusCode)status)
    { Content = new StringContent(body, Encoding.UTF8, "application/json") };
    private static void True(bool condition) { if (!condition) throw new Exception("Assertion failed."); }
    private static void Eq<T>(T expected, T actual)
    { if (!EqualityComparer<T>.Default.Equals(expected, actual)) throw new Exception($"Expected {expected}, got {actual}."); }
    private static async Task<T> Throws<T>(Func<Task> action) where T : Exception
    {
        try { await action(); } catch (T error) { return error; }
        throw new Exception("Expected " + typeof(T).Name);
    }
    private static T ThrowsSync<T>(Action action) where T : Exception
    {
        try { action(); } catch (T error) { return error; }
        throw new Exception("Expected " + typeof(T).Name);
    }

    private sealed record Recorded(string Uri, string Method, string Accept, bool HasCredentials, bool HasBody);
    private sealed class FakeHandler : HttpMessageHandler
    {
        private readonly Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> _respond;
        internal List<Recorded> Requests { get; } = new();
        internal FakeHandler(Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> respond) => _respond = respond;
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken ct)
        {
            Requests.Add(new Recorded(request.RequestUri!.AbsoluteUri, request.Method.Method, request.Headers.Accept.ToString(),
                request.Headers.Any(h => h.Key.Equals("Authorization", StringComparison.OrdinalIgnoreCase) ||
                    h.Key.Equals("Cookie", StringComparison.OrdinalIgnoreCase) || h.Key.Contains("Key", StringComparison.OrdinalIgnoreCase) ||
                    h.Key.Contains("Secret", StringComparison.OrdinalIgnoreCase)), request.Content != null));
            return _respond(request, ct);
        }
    }

    private class NonSeekableStream : Stream
    {
        private readonly MemoryStream _inner;
        internal NonSeekableStream(byte[] bytes) => _inner = new MemoryStream(bytes);
        public override bool CanRead => true;
        public override bool CanSeek => false;
        public override bool CanWrite => false;
        public override long Length => throw new NotSupportedException();
        public override long Position { get => throw new NotSupportedException(); set => throw new NotSupportedException(); }
        public override void Flush() => throw new NotSupportedException();
        public override int Read(byte[] buffer, int offset, int count) => _inner.Read(buffer, offset, count);
        public override ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken ct = default) => _inner.ReadAsync(buffer, ct);
        public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
        public override void SetLength(long value) => throw new NotSupportedException();
        public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();
        protected override void Dispose(bool disposing) { _inner.Dispose(); base.Dispose(disposing); }
    }
    private sealed class BlockingStream : NonSeekableStream
    {
        internal TaskCompletionSource<bool> Started { get; } = new(TaskCreationOptions.RunContinuationsAsynchronously);
        internal bool Disposed { get; private set; }
        internal BlockingStream() : base(Array.Empty<byte>()) { }
        public override async ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken ct = default)
        { Started.TrySetResult(true); await Task.Delay(Timeout.Infinite, ct); return 0; }
        protected override void Dispose(bool disposing) { Disposed = true; base.Dispose(disposing); }
    }
}
