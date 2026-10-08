using System.Globalization;
using System.Runtime.CompilerServices;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace NoWork.Sdk;

/// <summary>Signed-in account access. Supply a site login JWT; OAuth access tokens, chat tokens and server keys are not substitutes.</summary>
public sealed class GameDataClient : GameDataArchiveClient
{
    public GameDataClient(string baseUrl, string siteJwt, GameDataClientOptions? options = null)
        : base(baseUrl, "/api/account/", siteJwt, options) { }

    public Task<GameMihoyoStatus> MihoyoStatusAsync(CancellationToken ct = default) => Get<GameMihoyoStatus>("mihoyo/status", ct);
    public Task<IReadOnlyList<GameDescriptor>> MihoyoGamesAsync(CancellationToken ct = default) => Get<IReadOnlyList<GameDescriptor>>("mihoyo/games", ct);
    public Task<GameSnapshot> MihoyoSnapshotAsync(string gameId, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        Get<GameSnapshot>("mihoyo/games/" + GameDataInput.Id(gameId) + "/data" + GameDataInput.Snapshot(query, GameProvider.Mihoyo), ct);
    public Task<GameHypergryphStatus> HypergryphStatusAsync(CancellationToken ct = default) => Get<GameHypergryphStatus>("hypergryph/status", ct);
    public Task<GameAccountData> HypergryphGamesAsync(CancellationToken ct = default) => Get<GameAccountData>("hypergryph/games", ct);
    public Task<GameSnapshot> HypergryphSnapshotAsync(string gameId, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        Get<GameSnapshot>("hypergryph/games/" + GameDataInput.Id(gameId) + "/snapshot" + GameDataInput.Snapshot(query, GameProvider.Hypergryph), ct);
    public Task<GameAccountData> SelectHypergryphRoleAsync(string roleId, CancellationToken ct = default) =>
        Send<GameAccountData>(HttpMethod.Put, "hypergryph/role", new { roleId = GameDataInput.Text(roleId, 128, false) }, ct);

    /// <summary>Visits one section with a mandatory page budget. Breaking enumeration stops further requests.</summary>
    public IAsyncEnumerable<GameSnapshot> WalkMihoyoSnapshotsAsync(string gameId, string section, int maxPages, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        GameDataPages.Walk((q, c) => MihoyoSnapshotAsync(gameId, q, c), section, maxPages, query, ct);
    public IAsyncEnumerable<GameSnapshot> WalkHypergryphSnapshotsAsync(string gameId, string section, int maxPages, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        GameDataPages.Walk((q, c) => HypergryphSnapshotAsync(gameId, q, c), section, maxPages, query, ct);
}

/// <summary>Unauthenticated Hypergryph reference data. No token constructor or private archive/asset API is exposed.</summary>
public sealed class GameDataPublicClient : IDisposable
{
    private readonly GameDataTransport _transport;
    public GameDataPublicClient(string baseUrl, GameDataClientOptions? options = null) =>
        _transport = new GameDataTransport(baseUrl, "/api/hypergryph/", null, options);
    public Task<GameList> HypergryphGamesAsync(CancellationToken ct = default) =>
        _transport.SendAsync<GameList>(HttpMethod.Get, "/api/hypergryph/games", null, ct);
    /// <summary>RoleId must be absent. Account-only capabilities are returned with their true availability status.</summary>
    public Task<GameSnapshot> HypergryphSnapshotAsync(string gameId, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        _transport.SendAsync<GameSnapshot>(HttpMethod.Get, "/api/hypergryph/games/" + GameDataInput.Id(gameId) + "/snapshot" +
            GameDataInput.Snapshot(query, GameProvider.Hypergryph, publicOnly: true), null, ct);
    public IAsyncEnumerable<GameSnapshot> WalkHypergryphSnapshotsAsync(string gameId, string section, int maxPages, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        GameDataPages.Walk((q, c) => HypergryphSnapshotAsync(gameId, q, c), section, maxPages, query, ct);
    public void Dispose() => _transport.Dispose();
}

/// <summary>Site JWT administrative game APIs. The server enforces per-provider permissions and account data scope on every call.</summary>
public sealed class GameDataAdminClient : GameDataArchiveClient
{
    public GameDataAdminClient(string baseUrl, string siteJwt, GameDataClientOptions? options = null)
        : base(baseUrl, "/api/admin/", siteJwt, options) { }
    public Task<IReadOnlyList<GameDescriptor>> MihoyoGamesAsync(CancellationToken ct = default) => Get<IReadOnlyList<GameDescriptor>>("mihoyo/games", ct);
    public Task<IReadOnlyList<GameMihoyoCatalogEntry>> MihoyoCatalogAsync(CancellationToken ct = default) => Get<IReadOnlyList<GameMihoyoCatalogEntry>>("mihoyo/catalog", ct);
    public Task<GameList> HypergryphGamesAsync(CancellationToken ct = default) => Get<GameList>("hypergryph/games", ct);
    public Task<GameAccountData> HypergryphAccountGamesAsync(long accountId, CancellationToken ct = default) =>
        Get<GameAccountData>("hypergryph/accounts/" + GameDataInput.Account(accountId) + "/games", ct);
    public Task<GameSnapshot> HypergryphAccountSnapshotAsync(long accountId, string gameId, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        Get<GameSnapshot>("hypergryph/accounts/" + GameDataInput.Account(accountId) + "/games/" + GameDataInput.Id(gameId) + "/snapshot" +
            GameDataInput.Snapshot(query, GameProvider.Hypergryph), ct);
    public IAsyncEnumerable<GameSnapshot> WalkHypergryphAccountSnapshotsAsync(long accountId, string gameId, string section, int maxPages, GameSnapshotQuery? query = null, CancellationToken ct = default) =>
        GameDataPages.Walk((q, c) => HypergryphAccountSnapshotAsync(accountId, gameId, q, c), section, maxPages, query, ct);

    public Task<GameStoredFieldPolicy> SetFieldPolicyAsync(GameProvider provider, long accountId, GameFieldPolicyInput input, CancellationToken ct = default)
    {
        ArgumentNullException.ThrowIfNull(input);
        GameDataInput.Id(input.GameId, 64);
        GameDataInput.PolicyPath(input.Path);
        if (input.Visibility is not ("owner" or "shared" or "public" or "secret")) throw new ArgumentException("Unknown field visibility.", nameof(input));
        // Deliberate allowlist: actor, account scope, source grants and other fields are server-owned.
        return Send<GameStoredFieldPolicy>(HttpMethod.Put, Archive(provider, accountId) + "/policies",
            new { game_id = input.GameId, path = input.Path, visibility = input.Visibility, exportable = input.Exportable, revision = input.Revision }, ct);
    }

    public Task<GameAssetCache> RefreshAssetProviderAsync(string providerId, GameAssetVariant? variant = null, CancellationToken ct = default)
    {
        var v = GameDataInput.Variant(variant);
        return Send<GameAssetCache>(HttpMethod.Post, "game-assets/providers/" + GameDataInput.AssetId(providerId) + "/refresh", new { locale = v.Locale, version = v.Version }, ct);
    }
    public Task<GameAssetCache> SetAssetProviderDisabledAsync(string providerId, bool disabled, GameAssetVariant? variant = null, CancellationToken ct = default)
    {
        var v = GameDataInput.Variant(variant);
        return Send<GameAssetCache>(HttpMethod.Put, "game-assets/providers/" + GameDataInput.AssetId(providerId) + "/state", new { locale = v.Locale, version = v.Version, disabled }, ct);
    }
}

/// <summary>Account/admin archive and public-reference asset methods; the constructor's token scope cannot be changed.</summary>
public abstract class GameDataArchiveClient : IDisposable
{
    private readonly GameDataTransport _transport;
    private readonly string _prefix;
    private protected GameDataArchiveClient(string baseUrl, string prefix, string token, GameDataClientOptions? options)
    {
        _prefix = prefix;
        _transport = new GameDataTransport(baseUrl, prefix, token, options);
    }
    private protected Task<T> Get<T>(string path, CancellationToken ct, bool export = false) => _transport.SendAsync<T>(HttpMethod.Get, _prefix + path, null, ct, export);
    private protected Task<T> Send<T>(HttpMethod method, string path, object body, CancellationToken ct) => _transport.SendAsync<T>(method, _prefix + path, body, ct);
    private protected static string Archive(GameProvider provider, long accountId) => GameDataInput.Provider(provider) + "/accounts/" + GameDataInput.Account(accountId) + "/archive";

    public Task<IReadOnlyList<GameSnapshotMetadata>> ArchiveSnapshotsAsync(GameProvider provider, long accountId, CancellationToken ct = default) =>
        Get<IReadOnlyList<GameSnapshotMetadata>>(Archive(provider, accountId) + "/snapshots", ct);
    public Task<GameArchivedSnapshot> ArchiveSnapshotAsync(GameProvider provider, long accountId, string gameId, string? roleId = null, CancellationToken ct = default) =>
        Get<GameArchivedSnapshot>(Archive(provider, accountId) + "/snapshots" + GameDataInput.Query(
            ("game_id", GameDataInput.Id(gameId, 64)), ("role_id", string.IsNullOrEmpty(roleId) ? null : GameDataInput.ArchiveRole(roleId))), ct);
    public Task<IReadOnlyList<GameArchiveHistory>> ArchiveHistoryAsync(GameProvider provider, long accountId, GameArchivePageQuery? query = null, CancellationToken ct = default) =>
        Get<IReadOnlyList<GameArchiveHistory>>(Archive(provider, accountId) + "/history" + GameDataInput.ArchivePage(query), ct);
    public Task<IReadOnlyList<GameStoredFieldPolicy>> ArchivePoliciesAsync(GameProvider provider, long accountId, string gameId, CancellationToken ct = default) =>
        Get<IReadOnlyList<GameStoredFieldPolicy>>(Archive(provider, accountId) + "/policies" + GameDataInput.Query(("game_id", GameDataInput.Id(gameId, 64))), ct);
    public Task<IReadOnlyList<GamePolicyAudit>> ArchivePolicyAuditAsync(GameProvider provider, long accountId, GameArchivePageQuery? query = null, CancellationToken ct = default) =>
        Get<IReadOnlyList<GamePolicyAudit>>(Archive(provider, accountId) + "/policy-audit" + GameDataInput.ArchivePage(query), ct);
    /// <summary>Reads the server's policy-filtered export with MaxExportBytes as a hard response limit.</summary>
    public Task<GameArchiveExport> ExportArchiveAsync(GameProvider provider, long accountId, CancellationToken ct = default) =>
        Get<GameArchiveExport>(Archive(provider, accountId) + "/export", ct, export: true);
    /// <summary>Clears snapshots/history. The backend retains administrator field restrictions and policy audit.</summary>
    public async Task ClearArchiveAsync(GameProvider provider, long accountId, CancellationToken ct = default) =>
        _ = await _transport.SendAsync<JsonElement>(HttpMethod.Delete, _prefix + Archive(provider, accountId), null, ct, allowEmpty: true).ConfigureAwait(false);
    public Task<IReadOnlyList<GameAssetProvider>> AssetProvidersAsync(CancellationToken ct = default) => Get<IReadOnlyList<GameAssetProvider>>("game-assets/providers", ct);
    public Task<GameAssetPage> AssetCatalogAsync(GameAssetQuery query, CancellationToken ct = default) =>
        Get<GameAssetPage>("game-assets/catalog" + GameDataInput.AssetQuery(query), ct);
    public void Dispose() => _transport.Dispose();
}

/// <summary>Policy selectors use the backend's JSON pointer escaping, not URI escaping.</summary>
public static class GameDataFields
{
    public static string Path(string sectionId, string fieldKey, string? entityId = null)
    {
        static string Escape(string value) => value.Replace("~", "~0").Replace("/", "~1");
        GameDataInput.Text(sectionId, 320, false);
        GameDataInput.Text(fieldKey, 320, false);
        if (!string.IsNullOrEmpty(entityId)) GameDataInput.Text(entityId, 320, false);
        var result = "/sections/" + Escape(sectionId) + (string.IsNullOrEmpty(entityId) ? "" : "/entities/" + Escape(entityId)) + "/fields/" + Escape(fieldKey);
        GameDataInput.PolicyPath(result);
        return result;
    }
}

internal static class GameDataPages
{
    internal static async IAsyncEnumerable<GameSnapshot> Walk(Func<GameSnapshotQuery, CancellationToken, Task<GameSnapshot>> fetch,
        string section, int maxPages, GameSnapshotQuery? query, [EnumeratorCancellation] CancellationToken ct)
    {
        GameDataInput.Id(section);
        if (maxPages < 1 || maxPages > 1000) throw new ArgumentOutOfRangeException(nameof(maxPages), "Page budget must be 1..1000.");
        query ??= new GameSnapshotQuery();
        if (query.Sections is { Count: > 0 } && (query.Sections.Count != 1 || query.Sections[0] != section))
            throw new ArgumentException("Iteration selects exactly one section.", nameof(query));
        var request = query with { Sections = new[] { section } };
        var seen = new HashSet<string>(StringComparer.Ordinal);
        if (!string.IsNullOrEmpty(query.Cursor)) seen.Add(query.Cursor);
        for (var page = 1; page <= maxPages; page++)
        {
            ct.ThrowIfCancellationRequested();
            var snapshot = await fetch(request, ct).ConfigureAwait(false);
            if (snapshot.Sections == null) throw new TransportException("game snapshot is missing sections");
            var selected = snapshot.Sections.Where(s => s.Id == section).ToArray();
            if (selected.Length > 1) throw new GameDataPaginationException("ambiguous_section", page);
            var sectionCursor = selected.SingleOrDefault()?.NextCursor;
            var next = !string.IsNullOrEmpty(snapshot.NextCursor) ? snapshot.NextCursor : sectionCursor;
            // Two different continuation tokens cannot be safely combined into a single query.
            if (!string.IsNullOrEmpty(snapshot.NextCursor) && !string.IsNullOrEmpty(sectionCursor) && snapshot.NextCursor != sectionCursor)
                throw new GameDataPaginationException("ambiguous_cursor", page);
            yield return snapshot;
            ct.ThrowIfCancellationRequested();
            if (string.IsNullOrEmpty(next)) yield break;
            GameDataInput.Text(next, 2048, false);
            if (!seen.Add(next)) throw new GameDataPaginationException("repeated_cursor", page);
            if (page == maxPages) throw new GameDataPaginationException("page_limit", page);
            request = request with { Cursor = next };
        }
    }
}

internal static class GameDataInput
{
    private static readonly Regex Identifier = new("^[a-zA-Z0-9][a-zA-Z0-9_.:-]*$", RegexOptions.CultureInvariant);
    private static readonly Regex AssetToken = new("^[a-zA-Z0-9][a-zA-Z0-9_.-]*$", RegexOptions.CultureInvariant);
    internal static string Provider(GameProvider provider) => provider switch
    {
        GameProvider.Mihoyo => "mihoyo", GameProvider.Hypergryph => "hypergryph", _ => throw new ArgumentOutOfRangeException(nameof(provider))
    };
    internal static string Account(long accountId) => accountId > 0 ? accountId.ToString(CultureInfo.InvariantCulture) : throw new ArgumentOutOfRangeException(nameof(accountId));
    internal static string Text(string value, int maxBytes, bool allowEmpty = true)
    {
        if (value == null || !allowEmpty && value.Length == 0 || Encoding.UTF8.GetByteCount(value) > maxBytes || value.Any(char.IsControl))
            throw new ArgumentException("Selector has an invalid length or control character.");
        return value;
    }
    internal static string Id(string value, int max = 128)
    {
        Text(value, max, false);
        if (!Identifier.IsMatch(value)) throw new ArgumentException("Invalid game identifier.");
        return value;
    }
    internal static string AssetId(string value)
    {
        Text(value, 96, false);
        if (!AssetToken.IsMatch(value)) throw new ArgumentException("Invalid asset provider identifier.");
        return value;
    }
    internal static string ArchiveRole(string value) => Id(value, 128);
    internal static void PageSize(int? limit)
    {
        if (limit is < 1 or > 100) throw new ArgumentOutOfRangeException(nameof(limit), "Page size must be 1..100.");
    }
    private static string? Number<T>(T? value) where T : struct, IFormattable => value?.ToString(null, CultureInfo.InvariantCulture);
    internal static string Query(params (string Key, string? Value)[] fields)
    {
        var parts = fields.Where(f => f.Value != null).Select(f => Uri.EscapeDataString(f.Key) + "=" + Uri.EscapeDataString(f.Value!));
        var query = string.Join("&", parts);
        return query.Length == 0 ? "" : "?" + query;
    }
    internal static string Snapshot(GameSnapshotQuery? query, GameProvider provider, bool publicOnly = false)
    {
        query ??= new GameSnapshotQuery();
        PageSize(query.Limit);
        Locale(query.Locale);
        if (query.Sections is { Count: > 16 }) throw new ArgumentException("At most sixteen sections are allowed.", nameof(query));
        if (query.Sections != null) foreach (var section in query.Sections) Id(section);
        if (query.Cursor != null) Text(query.Cursor, 2048);
        if (!string.IsNullOrEmpty(query.RoleId))
        {
            if (publicOnly) throw new ArgumentException("Public requests cannot select an account role.", nameof(query));
            Text(query.RoleId, 128, false);
            if (provider == GameProvider.Mihoyo && (!long.TryParse(query.RoleId, NumberStyles.None, CultureInfo.InvariantCulture, out var id) || id <= 0))
                throw new ArgumentException("miHoYo roleId must be a positive binding row ID.", nameof(query));
        }
        return Query(("roleId", string.IsNullOrEmpty(query.RoleId) ? null : query.RoleId),
            ("sections", query.Sections is { Count: > 0 } ? string.Join(",", query.Sections) : null),
            ("cursor", query.Cursor), ("limit", Number(query.Limit)), ("locale", EmptyToNull(query.Locale)));
    }
    internal static string ArchivePage(GameArchivePageQuery? query)
    {
        query ??= new GameArchivePageQuery();
        PageSize(query.Limit);
        return Query(("before", Number(query.Before)), ("limit", Number(query.Limit)));
    }
    private static string? EmptyToNull(string? value) => string.IsNullOrEmpty(value) ? null : value;
    private static void Locale(string? locale)
    {
        if (!string.IsNullOrEmpty(locale) && locale is not ("en" or "ja" or "zh_CN" or "zh_TW")) throw new ArgumentException("Unsupported locale.", nameof(locale));
    }
    internal static GameAssetVariant Variant(GameAssetVariant? variant)
    {
        variant ??= new GameAssetVariant();
        Locale(variant.Locale);
        if (!string.IsNullOrEmpty(variant.Version))
        {
            Text(variant.Version, 64, false);
            if (!AssetToken.IsMatch(variant.Version)) throw new ArgumentException("Invalid asset version.");
        }
        // Empty strings request the backend's first supported variant.
        return variant with { Locale = variant.Locale ?? "", Version = variant.Version ?? "" };
    }
    internal static string AssetQuery(GameAssetQuery query)
    {
        ArgumentNullException.ThrowIfNull(query);
        AssetId(query.Provider);
        var variant = Variant(new GameAssetVariant(query.Locale, query.Version));
        PageSize(query.Limit);
        if (query.Page is < 1 or > 20000) throw new ArgumentOutOfRangeException(nameof(query), "Catalog page must be 1..20000.");
        if (query.Search != null) Text(query.Search, 128);
        if (!string.IsNullOrEmpty(query.Kind) && query.Kind is not ("character" or "item" or "weapon" or "equipment" or "quest" or "achievement" or "other"))
            throw new ArgumentException("Unknown catalog entity kind.", nameof(query));
        return Query(("provider", query.Provider), ("locale", EmptyToNull(variant.Locale)), ("version", EmptyToNull(variant.Version)),
            ("q", query.Search), ("kind", EmptyToNull(query.Kind)), ("page", Number(query.Page)), ("limit", Number(query.Limit)));
    }
    internal static void PolicyPath(string path)
    {
        Text(path, 320, false);
        var parts = path.Split('/');
        if (!path.StartsWith("/sections/", StringComparison.Ordinal) ||
            !(parts.Length == 5 && parts[3] == "fields" || parts.Length == 7 && parts[3] == "entities" && parts[5] == "fields"))
            throw new ArgumentException("Invalid archive field path.", nameof(path));
        foreach (var part in parts.Skip(1))
        {
            if (part.Length == 0) throw new ArgumentException("Empty archive path component.", nameof(path));
            for (var i = 0; i < part.Length; i++)
                if (part[i] == '~' && (++i == part.Length || part[i] is not ('0' or '1')))
                    throw new ArgumentException("Invalid JSON pointer escape.", nameof(path));
        }
    }
}
