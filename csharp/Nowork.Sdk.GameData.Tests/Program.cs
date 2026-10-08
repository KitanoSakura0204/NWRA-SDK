using System.Net;
using System.Text;
using System.Text.Json;
using NoWork.Sdk;

internal static class Program
{
    private const string Site = "https://account.example.test";
    private const string Stamp = "2026-10-08T00:00:00Z";
    private const string AccountGames = "{\"games\":[],\"roles\":[{\"id\":\"arknights:cn:42\",\"game_id\":\"arknights\",\"uid\":\"42\",\"region\":\"cn\",\"policy\":{\"visibility\":\"owner\",\"exportable\":true}}],\"selectedRoleId\":\"arknights:cn:42\",\"discovery\":{\"status\":\"stale\",\"updatedAt\":null,\"reason\":\"resync_needed\"}}";

    public static async Task Main()
    {
        var tests = new (string Name, Func<Task> Run)[]
        {
            ("provider routes, role formats, opaque cursors and lossless fields", ProviderContracts),
            ("separate account/admin/public credentials", TokenScopes),
            ("archive metadata, uint64 history, policy-filtered export and empty clear", ArchiveContracts),
            ("admin policies, explicit false and asset variants", AdminMutations),
            ("bounded pagination, loops and consumer cancellation", Pagination),
            ("invalid selectors never reach transport", InputBounds),
            ("typed API errors, retry metadata and malformed envelopes", Errors),
            ("redirects and encrypted data fail closed", RedirectsAndEncryption),
            ("bounded declared and chunked responses", ResponseLimits),
            ("cancellation before send and during headers", RequestCancellation),
            ("cancellation and timeout during body read", BodyCancellation),
            ("caller handler lifetime and credential configuration", HandlerConfiguration)
        };
        foreach (var test in tests)
        {
            await test.Run();
            Console.WriteLine("PASS " + test.Name);
        }
        Console.WriteLine($"{tests.Length} game SDK fixture groups passed.");
    }

    private static async Task ProviderContracts()
    {
        using var handler = new FakeHandler((request, _) => Task.FromResult(Envelope(request.RequestUri!.AbsolutePath switch
        {
            "/api/account/mihoyo/status" => "{\"accountId\":12,\"region\":\"os\",\"bound\":true,\"roles\":[{\"id\":37,\"gameId\":\"genshin\",\"gameBiz\":\"hk4e_global\",\"gameUid\":\"123\"}],\"recovery\":{\"action\":\"verify\",\"url\":\"https://example.test/verify\"}}",
            "/api/account/hypergryph/status" => "{\"enabled\":true,\"account\":{\"id\":19,\"selectedRoleId\":\"arknights:cn:42\",\"discoveryStatus\":\"stale\",\"roles\":[]}}",
            "/api/account/mihoyo/games" => "[{\"id\":\"new_game\",\"provider\":\"mihoyo\",\"name\":{\"ja\":\"Game\"},\"capabilities\":[{\"id\":\"catalog\",\"name\":{},\"supported\":false,\"auth_scope\":\"public\",\"status\":\"unsupported\"}]}]",
            "/api/account/hypergryph/games" or "/api/account/hypergryph/role" => AccountGames,
            _ => Snapshot()
        })));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        var status = await client.MihoyoStatusAsync();
        Eq(12L, status.AccountId!.Value); Eq(37L, status.Roles[0].Id); Eq("verify", status.Recovery!.Action);
        Eq(19L, (await client.HypergryphStatusAsync()).Account!.Id);
        Eq("new_game", (await client.MihoyoGamesAsync())[0].Id);
        var discovery = await client.HypergryphGamesAsync();
        Eq("stale", discovery.Discovery.Status); Eq("arknights:cn:42", discovery.Roles[0].Id);
        var snapshot = await client.MihoyoSnapshotAsync("genshin", new GameSnapshotQuery
        {
            RoleId = "37", Sections = new[] { "characters" }, Cursor = "a+/=&角色", Limit = 2, Locale = "zh_CN"
        });
        var query = handler.Requests.Last().Uri.Query;
        True(query.Contains("roleId=37") && query.Contains("cursor=a%2B%2F%3D%26%E8%A7%92%E8%89%B2"));
        var fields = snapshot.Sections[0].Entities[0].Fields;
        Eq(0, fields[0].Value.GetInt32()); True(fields[0].HasValue);
        True(!fields[1].Value.GetBoolean() && fields[1].HasValue);
        Eq(JsonValueKind.Null, fields[2].Value.ValueKind); True(!fields[2].HasValue);
        Eq(JsonValueKind.Undefined, fields[3].Value.ValueKind); True(!fields[3].HasValue);
        Eq(18446744073709551615UL, fields[4].Value.GetUInt64());
        Eq("partial", snapshot.Sections[0].Status); Eq("community", snapshot.Sections[0].Source!.Kind);
        Eq("Credits", snapshot.Sections[0].Source!.Attribution); True(snapshot.Role == null);
        await client.HypergryphSnapshotAsync("arknights", new GameSnapshotQuery { RoleId = "arknights:cn:42" });
        Eq("/api/account/hypergryph/games/arknights/snapshot", handler.Requests.Last().Uri.AbsolutePath);
        await client.SelectHypergryphRoleAsync("arknights:cn:42");
        Eq("PUT", handler.Requests.Last().Method);
        using var selected = JsonDocument.Parse(handler.Requests.Last().Body!);
        Eq("arknights:cn:42", selected.RootElement.GetProperty("roleId").GetString());
    }

    private static async Task TokenScopes()
    {
        using var handler = new FakeHandler((r, _) => Task.FromResult(Envelope(r.RequestUri!.AbsolutePath switch
        {
            "/api/account/mihoyo/games" or "/api/admin/mihoyo/games" or "/api/admin/mihoyo/catalog" => "[]",
            "/api/admin/hypergryph/accounts/12/games" => AccountGames,
            _ when r.RequestUri.AbsolutePath.EndsWith("snapshot") => Snapshot(),
            _ => "{\"games\":[]}"
        })));
        using var owner = new GameDataClient(Site, "owner-jwt", Options(handler));
        using var admin = new GameDataAdminClient(Site, "admin-jwt", Options(handler));
        using var guest = new GameDataPublicClient(Site, Options(handler));
        await owner.MihoyoGamesAsync(); await admin.MihoyoGamesAsync(); await admin.MihoyoCatalogAsync();
        await admin.HypergryphGamesAsync(); await admin.HypergryphAccountGamesAsync(12);
        await admin.HypergryphAccountSnapshotAsync(12, "arknights");
        await guest.HypergryphGamesAsync(); await guest.HypergryphSnapshotAsync("exastris");
        foreach (var request in handler.Requests)
        {
            var expected = request.Uri.AbsolutePath.StartsWith("/api/account/") ? "Bearer owner-jwt" : request.Uri.AbsolutePath.StartsWith("/api/admin/") ? "Bearer admin-jwt" : null;
            Eq(expected, request.Authorization); True(!request.HasServerKey && !request.HasCookie);
        }
        var count = handler.Requests.Count;
        await Throws<ArgumentException>(() => guest.HypergryphSnapshotAsync("arknights", new GameSnapshotQuery { RoleId = "private" }));
        Eq(count, handler.Requests.Count);
        True(typeof(GameDataPublicClient).GetMethod("MihoyoSnapshotAsync") == null);
        True(typeof(GameDataClient).GetMethod("SetFieldPolicyAsync") == null);
        True(typeof(GameDataAdminClient).GetMethod("MihoyoSnapshotAsync") == null);
    }

    private static async Task ArchiveContracts()
    {
        using var handler = new FakeHandler((r, _) => Task.FromResult(Envelope(r.Method == HttpMethod.Delete ? "null" : r.RequestUri!.AbsolutePath.Split('/').Last() switch
        {
            "snapshots" when r.RequestUri.Query.Length == 0 => "[{\"game_id\":\"genshin\",\"role_id\":\"role:37\",\"version\":1,\"revision\":9,\"status\":\"stale\",\"stale\":true,\"bytes\":20,\"fetched_at\":\"" + Stamp + "\",\"expires_at\":\"" + Stamp + "\"}]",
            "snapshots" => Archived(),
            "history" => "[{\"id\":18446744073709551614,\"game_id\":\"genshin\",\"role_id\":\"role:37\",\"revision\":9,\"status\":\"partial\",\"bytes\":20,\"created_at\":\"" + Stamp + "\"}]",
            "export" => "{\"scope\":{\"site_id\":0,\"user_id\":1,\"account_id\":12},\"provider\":\"mihoyo\",\"exported_at\":\"" + Stamp + "\",\"snapshots\":[],\"history\":[],\"policies\":[],\"policy_audit\":[]}",
            _ => "[]"
        })));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        True((await client.ArchiveSnapshotsAsync(GameProvider.Mihoyo, 12))[0].Stale);
        Eq("role:37", (await client.ArchiveSnapshotAsync(GameProvider.Mihoyo, 12, "genshin", "role:37")).RoleId);
        True(handler.Requests.Last().Uri.Query.Contains("game_id=genshin") && handler.Requests.Last().Uri.Query.Contains("role_id=role%3A37"));
        var history = await client.ArchiveHistoryAsync(GameProvider.Mihoyo, 12, new GameArchivePageQuery(ulong.MaxValue, 100));
        Eq(ulong.MaxValue - 1, history[0].Id); True(handler.Requests.Last().Uri.Query.Contains("before=18446744073709551615"));
        await client.ArchivePoliciesAsync(GameProvider.Mihoyo, 12, "genshin");
        await client.ArchivePolicyAuditAsync(GameProvider.Mihoyo, 12, new GameArchivePageQuery(45, 2));
        var export = await client.ExportArchiveAsync(GameProvider.Mihoyo, 12);
        Eq(0UL, export.Scope.SiteId); Eq(0, export.Snapshots.Count);
        Eq("/api/account/mihoyo/accounts/12/archive/export", handler.Requests.Last().Uri.AbsolutePath);
        await client.ClearArchiveAsync(GameProvider.Mihoyo, 12);
        Eq("DELETE", handler.Requests.Last().Method); Eq("/api/account/mihoyo/accounts/12/archive", handler.Requests.Last().Uri.AbsolutePath);
    }

    private static async Task AdminMutations()
    {
        using var handler = new FakeHandler((r, _) => Task.FromResult(Envelope(r.RequestUri!.AbsolutePath.EndsWith("providers") ? "[]" : r.RequestUri.AbsolutePath.EndsWith("catalog")
            ? "{\"provider\":{\"id\":\"themis-public\",\"source\":{\"fetched_at\":\"" + Stamp + "\"}},\"cache\":{\"disabled\":false,\"entry_count\":0},\"status\":\"stale\",\"list\":[],\"count\":0,\"page\":2,\"limit\":20}"
            : r.RequestUri.AbsolutePath.EndsWith("policies") ? "{\"id\":1,\"game_id\":\"genshin\",\"path\":\"/sections/s/fields/k\",\"visibility\":\"owner\",\"exportable\":false,\"revision\":1,\"updated_at\":\"" + Stamp + "\"}" : "{\"provider\":\"themis-public\",\"disabled\":false}")));
        using var client = new GameDataAdminClient(Site, "admin-jwt", Options(handler));
        Eq("/sections/a~1b/entities/e~0f/fields/x~1y", GameDataFields.Path("a/b", "x/y", "e~f"));
        Eq("/sections/s/fields/k", GameDataFields.Path("s", "k", ""));
        await client.SetFieldPolicyAsync(GameProvider.Hypergryph, 19, new GameFieldPolicyInput("arknights", GameDataFields.Path("s", "k"), "owner", false, 0));
        var request = handler.Requests.Last();
        Eq("/api/admin/hypergryph/accounts/19/archive/policies", request.Uri.AbsolutePath);
        using (var body = JsonDocument.Parse(request.Body!))
        {
            True(!body.RootElement.GetProperty("exportable").GetBoolean()); Eq(0UL, body.RootElement.GetProperty("revision").GetUInt64());
            Eq(5, body.RootElement.EnumerateObject().Count()); True(!body.RootElement.TryGetProperty("actor_id", out _));
        }
        await client.AssetProvidersAsync();
        var catalog = await client.AssetCatalogAsync(new GameAssetQuery { Provider = "themis-public", Search = "a&b", Page = 2, Limit = 20, Locale = "ja" });
        Eq("stale", catalog.Status); True(handler.Requests.Last().Uri.Query.Contains("q=a%26b"));
        await client.RefreshAssetProviderAsync("themis-public", new GameAssetVariant("ja", "v1"));
        Eq("POST", handler.Requests.Last().Method);
        await client.SetAssetProviderDisabledAsync("themis-public", false);
        using var disabled = JsonDocument.Parse(handler.Requests.Last().Body!);
        True(!disabled.RootElement.GetProperty("disabled").GetBoolean()); Eq("", disabled.RootElement.GetProperty("locale").GetString());
    }

    private static async Task Pagination()
    {
        var responses = new Queue<string>(new[] { Snapshot("opaque+/=a"), Snapshot() });
        using var handler = new FakeHandler((_, _) => Task.FromResult(Envelope(responses.Dequeue())));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        var pages = 0;
        await foreach (var page in client.WalkMihoyoSnapshotsAsync("genshin", "characters", 2)) { pages++; Eq("partial", page.Sections[0].Status); }
        Eq(2, pages); True(handler.Requests[1].Uri.Query.Contains("cursor=opaque%2B%2F%3Da"));
        responses.Enqueue(Snapshot("again")); responses.Enqueue(Snapshot("again"));
        var loop = await Throws<GameDataPaginationException>(async () => { await foreach (var _ in client.WalkMihoyoSnapshotsAsync("genshin", "characters", 4)) { } });
        Eq("repeated_cursor", loop.Reason); Eq(2, loop.PagesRead);
        responses.Enqueue(Snapshot("more"));
        var limit = await Throws<GameDataPaginationException>(async () => { await foreach (var _ in client.WalkMihoyoSnapshotsAsync("genshin", "characters", 1)) { } });
        Eq("page_limit", limit.Reason);
        responses.Enqueue(Snapshot("unused"));
        var calls = handler.Requests.Count;
        await foreach (var _ in client.WalkMihoyoSnapshotsAsync("genshin", "characters", 100)) break;
        Eq(calls + 1, handler.Requests.Count);
        responses.Enqueue(Snapshot("cancel"));
        using var cancellation = new CancellationTokenSource();
        await Throws<OperationCanceledException>(async () =>
        {
            await foreach (var _ in client.WalkMihoyoSnapshotsAsync("genshin", "characters", 2).WithCancellation(cancellation.Token)) cancellation.Cancel();
        });
    }

    private static async Task InputBounds()
    {
        using var handler = new FakeHandler((_, _) => throw new Exception("Invalid inputs must not reach HTTP."));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        await Throws<ArgumentException>(() => client.MihoyoSnapshotAsync("../admin"));
        await Throws<ArgumentException>(() => client.MihoyoSnapshotAsync("genshin", new GameSnapshotQuery { RoleId = "1&roleId=2" }));
        await Throws<ArgumentException>(() => client.HypergryphSnapshotAsync("arknights", new GameSnapshotQuery { Limit = 101 }));
        await Throws<ArgumentException>(() => client.HypergryphSnapshotAsync("arknights", new GameSnapshotQuery { Cursor = new string('a', 2049) }));
        await Throws<ArgumentException>(() => client.HypergryphSnapshotAsync("arknights", new GameSnapshotQuery { Sections = Enumerable.Repeat("s", 17).ToArray() }));
        await Throws<ArgumentException>(() => client.HypergryphSnapshotAsync("arknights", new GameSnapshotQuery { Locale = "invalid" }));
        await Throws<ArgumentException>(() => client.ArchiveSnapshotsAsync(GameProvider.Mihoyo, 0));
        await Throws<ArgumentException>(() => client.ArchiveSnapshotsAsync((GameProvider)99, 1));
        await Throws<ArgumentException>(() => client.ArchiveHistoryAsync(GameProvider.Mihoyo, 1, new GameArchivePageQuery(Limit: 0)));
        await Throws<ArgumentException>(() => client.AssetCatalogAsync(new GameAssetQuery { Provider = "p", Search = new string('界', 43) }));
        await Throws<ArgumentException>(() => client.AssetCatalogAsync(new GameAssetQuery { Provider = "p", Page = 20001 }));
        using var admin = new GameDataAdminClient(Site, "admin-jwt", Options(handler));
        await Throws<ArgumentException>(() => admin.SetFieldPolicyAsync(GameProvider.Mihoyo, 1, new GameFieldPolicyInput("g", "/sections/s/fields/a~2", "owner", false, 1)));
        await Throws<ArgumentException>(async () => { await foreach (var _ in client.WalkMihoyoSnapshotsAsync("g", "s", 0)) { } });
        Eq(0, handler.Requests.Count);
    }

    private static async Task Errors()
    {
        var replies = new Queue<HttpResponseMessage>();
        var rate = Raw("{\"code\":429,\"message\":\"try later\",\"data\":{\"status\":\"challenge_required\"}}", 429);
        rate.Headers.TryAddWithoutValidation("Retry-After", "17"); replies.Enqueue(rate);
        replies.Enqueue(Raw("{\"code\":409,\"message\":\"conflict\"}"));
        foreach (var raw in new[] { "not JSON", "{}", "{\"code\":\"0\",\"data\":[]}", "{\"code\":0}", "{\"code\":0,\"data\":null}", "{\"code\":0,\"data\":{}}" }) replies.Enqueue(Raw(raw));
        replies.Enqueue(Raw("{\"code\":0,\"data\":[]}", 500));
        using var handler = new FakeHandler((_, _) => Task.FromResult(replies.Dequeue()));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        var error = await Throws<ApiException>(() => client.MihoyoGamesAsync());
        Eq(429, error.Code); Eq(429, GameDataErrors.Details(error)!.HttpStatus!.Value);
        Eq("17", GameDataErrors.Details(error)!.RetryAfter); Eq("challenge_required", GameDataErrors.Details(error)!.Data.GetProperty("status").GetString());
        Eq(1, handler.Requests.Count);
        Eq(409, (await Throws<ApiException>(() => client.MihoyoGamesAsync())).Code);
        while (replies.Count > 0) await Throws<TransportException>(() => client.MihoyoGamesAsync());
    }

    private static async Task RedirectsAndEncryption()
    {
        var redirect = Raw("", 302); redirect.Headers.Location = new Uri("https://other.example.test/");
        var replies = new Queue<HttpResponseMessage>(new[] { redirect, Envelope("{\"jwe\":\"sealed\"}") });
        using var handler = new FakeHandler((_, _) => Task.FromResult(replies.Dequeue()));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        var error = await Throws<TransportException>(() => client.MihoyoGamesAsync());
        Eq("redirect_or_not_modified", GameDataErrors.Details(error)!.Reason); Eq(1, handler.Requests.Count);
        await Throws<EncryptionException>(() => client.MihoyoGamesAsync());
        replies.Enqueue(Envelope("{}"));
        Eq("invalid_data", GameDataErrors.Details(await Throws<TransportException>(() => client.MihoyoSnapshotAsync("genshin")))!.Reason);
    }

    private static async Task ResponseLimits()
    {
        var payload = Encoding.UTF8.GetBytes("{\"code\":0,\"data\":[" + new string(' ', 200) + "]}");
        var replies = new Queue<HttpResponseMessage>(new[]
        {
            Raw(Encoding.UTF8.GetString(payload)),
            new HttpResponseMessage(HttpStatusCode.OK) { Content = new StreamContent(new NonSeekableStream(payload)) }
        });
        using var handler = new FakeHandler((_, _) => Task.FromResult(replies.Dequeue()));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler) with { MaxResponseBytes = 64 });
        for (var i = 0; i < 2; i++) Eq("response_too_large", GameDataErrors.Details(await Throws<TransportException>(() => client.MihoyoGamesAsync()))!.Reason);
        var exportBody = "{\"scope\":{\"site_id\":0,\"user_id\":1,\"account_id\":1},\"provider\":\"mihoyo\",\"exported_at\":\"" + Stamp + "\",\"snapshots\":[],\"history\":[],\"policies\":[],\"policy_audit\":[]}";
        replies.Enqueue(Envelope(exportBody));
        Eq("mihoyo", (await client.ExportArchiveAsync(GameProvider.Mihoyo, 1)).Provider);
        replies.Enqueue(Envelope(exportBody));
        using var boundedExport = new GameDataClient(Site, "site-jwt", Options(handler) with { MaxExportBytes = 64 });
        Eq("response_too_large", GameDataErrors.Details(await Throws<TransportException>(() => boundedExport.ExportArchiveAsync(GameProvider.Mihoyo, 1)))!.Reason);
    }

    private static async Task RequestCancellation()
    {
        var started = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        using var handler = new FakeHandler(async (_, ct) => { started.TrySetResult(true); await Task.Delay(Timeout.Infinite, ct); return Envelope("[]"); });
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        using var canceled = new CancellationTokenSource(); canceled.Cancel();
        await Throws<OperationCanceledException>(() => client.MihoyoGamesAsync(canceled.Token)); Eq(0, handler.Requests.Count);
        using var inflight = new CancellationTokenSource(); var task = client.MihoyoGamesAsync(inflight.Token);
        await started.Task.WaitAsync(TimeSpan.FromSeconds(2)); inflight.Cancel();
        var error = await Throws<OperationCanceledException>(() => task); Eq(inflight.Token, error.CancellationToken);
    }

    private static async Task BodyCancellation()
    {
        var blocking = new BlockingStream();
        using var handler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new StreamContent(blocking) }));
        using var client = new GameDataClient(Site, "site-jwt", Options(handler));
        using var cancellation = new CancellationTokenSource(); var request = client.MihoyoGamesAsync(cancellation.Token);
        await blocking.Started.Task.WaitAsync(TimeSpan.FromSeconds(2)); cancellation.Cancel();
        await Throws<OperationCanceledException>(() => request); True(blocking.Disposed);
        var timeoutStream = new BlockingStream();
        using var timeoutHandler = new FakeHandler((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = new StreamContent(timeoutStream) }));
        using var timed = new GameDataClient(Site, "site-jwt", Options(timeoutHandler) with { Timeout = TimeSpan.FromMilliseconds(50) });
        var error = await Throws<TransportException>(() => timed.MihoyoGamesAsync());
        Eq("timeout", GameDataErrors.Details(error)!.Reason); True(timeoutStream.Disposed);
    }

    private static Task HandlerConfiguration()
    {
        using var handler = new FakeHandler((_, _) => Task.FromResult(Envelope("[]")));
        var client = new GameDataClient(Site, "site-jwt", Options(handler)); client.Dispose(); True(!handler.Disposed);
        ThrowsSync<ArgumentException>(() => new GameDataClient("https://user:secret@example.test", "site-jwt"));
        ThrowsSync<ArgumentException>(() => new GameDataClient(Site, ""));
        ThrowsSync<ArgumentException>(() => new GameDataClient(Site, "a\r\nb"));
        ThrowsSync<ArgumentException>(() => new GameDataClient(Site, "site-jwt", new GameDataClientOptions { MaxResponseBytes = int.MaxValue }));
        using var insecure = new HttpClientHandler();
        ThrowsSync<ArgumentException>(() => new GameDataClient(Site, "site-jwt", Options(insecure)));
        return Task.CompletedTask;
    }

    private static string Snapshot(string? next = null) => "{\"game_id\":\"genshin\",\"fetched_at\":\"" + Stamp + "\",\"sections\":[{\"id\":\"characters\",\"name\":{},\"status\":\"partial\",\"fields\":[],\"policy\":{\"visibility\":\"owner\",\"exportable\":false},\"source\":{\"source_id\":\"fixture\",\"kind\":\"community\",\"name\":\"Fixture\",\"auth_scope\":\"public\",\"fetched_at\":\"" + Stamp + "\",\"attribution\":\"Credits\"},\"next_cursor\":" + JsonSerializer.Serialize(next) + ",\"entities\":[{\"id\":\"catalog-1\",\"kind\":\"character\",\"name\":{},\"policy\":{\"visibility\":\"public\",\"exportable\":true},\"fields\":[{\"key\":\"zero\",\"value\":0},{\"key\":\"false\",\"value\":false},{\"key\":\"null\",\"value\":null},{\"key\":\"missing\"},{\"key\":\"large\",\"value\":18446744073709551615}]}]}]}";
    private static string Archived() => "{\"provider\":\"mihoyo\",\"role_id\":\"role:37\",\"version\":1,\"revision\":9,\"status\":\"partial\",\"stale\":false,\"expires_at\":\"" + Stamp + "\",\"snapshot\":" + Snapshot() + "}";
    private static GameDataClientOptions Options(HttpMessageHandler handler) => new() { Handler = handler };
    private static HttpResponseMessage Envelope(string data) => Raw("{\"code\":0,\"data\":" + data + "}");
    private static HttpResponseMessage Raw(string body, int status = 200) => new((HttpStatusCode)status) { Content = new StringContent(body, Encoding.UTF8, "application/json") };
    private static void True(bool condition) { if (!condition) throw new Exception("Assertion failed."); }
    private static void Eq<T>(T expected, T actual) { if (!EqualityComparer<T>.Default.Equals(expected, actual)) throw new Exception($"Expected {expected}, got {actual}."); }
    private static async Task<T> Throws<T>(Func<Task> action) where T : Exception
    {
        try { await action(); } catch (T error) { return error; }
        throw new Exception("Expected " + typeof(T).Name);
    }
    private static void ThrowsSync<T>(Action action) where T : Exception
    {
        try { action(); } catch (T) { return; }
        throw new Exception("Expected " + typeof(T).Name);
    }

    private sealed record RecordedRequest(Uri Uri, string Method, string? Authorization, bool HasServerKey, bool HasCookie, string? Body);
    private sealed class FakeHandler : HttpMessageHandler
    {
        private readonly Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> _respond;
        internal List<RecordedRequest> Requests { get; } = new();
        internal bool Disposed { get; private set; }
        internal FakeHandler(Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>> respond) => _respond = respond;
        protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken ct)
        {
            Requests.Add(new RecordedRequest(request.RequestUri!, request.Method.Method, request.Headers.Authorization?.ToString(),
                request.Headers.Contains("X-Server-Key"), request.Headers.Contains("Cookie"), request.Content == null ? null : await request.Content.ReadAsStringAsync(ct)));
            return await _respond(request, ct);
        }
        protected override void Dispose(bool disposing) { Disposed = true; base.Dispose(disposing); }
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
        {
            Started.TrySetResult(true); await Task.Delay(Timeout.Infinite, ct); return 0;
        }
        protected override void Dispose(bool disposing) { Disposed = true; base.Dispose(disposing); }
    }
}
