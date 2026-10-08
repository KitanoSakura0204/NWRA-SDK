using System.Globalization;
using NoWork.Sdk;

// All credentials stay in the environment. The example performs read operations only.
var site = Required("NOWORK_BASE_URL");
var mode = args.Length == 0 ? "public" : args[0];
var providerName = Environment.GetEnvironmentVariable("NOWORK_GAME_PROVIDER") ?? "hypergryph";
var provider = providerName switch
{
    "mihoyo" => GameProvider.Mihoyo,
    "hypergryph" => GameProvider.Hypergryph,
    _ => throw new ArgumentException("NOWORK_GAME_PROVIDER must be mihoyo or hypergryph.")
};
var game = Environment.GetEnvironmentVariable("NOWORK_GAME_ID");
var section = Environment.GetEnvironmentVariable("NOWORK_GAME_SECTION");
var query = new GameSnapshotQuery
{
    RoleId = Environment.GetEnvironmentVariable("NOWORK_GAME_ROLE_ID"),
    Limit = 25,
    Locale = Environment.GetEnvironmentVariable("NOWORK_GAME_LOCALE") ?? "en"
};
using var stop = new CancellationTokenSource(TimeSpan.FromSeconds(60));
Console.CancelKeyPress += (_, e) => { e.Cancel = true; stop.Cancel(); };
try
{
    switch (mode)
    {
        case "public":
            if (provider != GameProvider.Hypergryph) throw new ArgumentException("The public game API is Hypergryph only.");
            using (var client = new GameDataPublicClient(site))
            {
                PrintGames((await client.HypergryphGamesAsync(stop.Token)).Games);
                if (game != null && section != null)
                    await PrintPages(client.WalkHypergryphSnapshotsAsync(game, section, 3, query, stop.Token));
            }
            break;
        case "account":
            using (var client = new GameDataClient(site, Required("NOWORK_SITE_JWT")))
            {
                if (provider == GameProvider.Mihoyo)
                {
                    var status = await client.MihoyoStatusAsync(stop.Token);
                    Console.WriteLine($"Bound: {status.Bound}; role count: {status.Roles.Count}; recovery: {status.Recovery?.Action}");
                    PrintGames(await client.MihoyoGamesAsync(stop.Token));
                    if (game != null && section != null) await PrintPages(client.WalkMihoyoSnapshotsAsync(game, section, 3, query, stop.Token));
                }
                else
                {
                    var discovery = await client.HypergryphGamesAsync(stop.Token);
                    Console.WriteLine($"Discovery: {discovery.Discovery.Status}; role count: {discovery.Roles.Count}");
                    PrintGames(discovery.Games);
                    if (game != null && section != null) await PrintPages(client.WalkHypergryphSnapshotsAsync(game, section, 3, query, stop.Token));
                }
                await PrintArchive(client, provider, stop.Token);
            }
            break;
        case "admin":
            using (var client = new GameDataAdminClient(site, Required("NOWORK_ADMIN_JWT")))
            {
                PrintGames(provider == GameProvider.Mihoyo ? await client.MihoyoGamesAsync(stop.Token) : (await client.HypergryphGamesAsync(stop.Token)).Games);
                var account = Environment.GetEnvironmentVariable("NOWORK_GAME_ACCOUNT_ID");
                if (provider == GameProvider.Hypergryph && account != null && game != null && section != null)
                    await PrintPages(client.WalkHypergryphAccountSnapshotsAsync(ParseAccount(account), game, section, 3, query, stop.Token));
                await PrintArchive(client, provider, stop.Token);
                var assets = await client.AssetProvidersAsync(stop.Token);
                Console.WriteLine($"Public asset providers: {assets.Count}");
            }
            break;
        default: throw new ArgumentException("Mode must be public, account, or admin.");
    }
}
catch (OperationCanceledException) { Console.Error.WriteLine("Canceled or example's total time budget reached."); Environment.ExitCode = 2; }
catch (ApiException e)
{
    var details = GameDataErrors.Details(e);
    Console.Error.WriteLine($"Game API refused the request: code {e.Code}, HTTP {details?.HttpStatus}, retry-after {details?.RetryAfter ?? "unspecified"}.");
    Environment.ExitCode = 3;
}
catch (TransportException e)
{
    Console.Error.WriteLine("Game transport failed: " + (GameDataErrors.Details(e)?.Reason ?? "transport")); Environment.ExitCode = 4;
}
catch (EncryptionException) { Console.Error.WriteLine("Unexpected encrypted data on a game API route."); Environment.ExitCode = 5; }
catch (GameDataPaginationException e)
{
    Console.Error.WriteLine($"Stopped after {e.PagesRead} pages: {e.Reason}. Results may be incomplete."); Environment.ExitCode = 6;
}

static string Required(string name) => Environment.GetEnvironmentVariable(name) is { Length: > 0 } value ? value : throw new ArgumentException("Set " + name + ".");
static long ParseAccount(string value) => long.TryParse(value, NumberStyles.None, CultureInfo.InvariantCulture, out var id) && id > 0 ? id : throw new ArgumentException("Invalid NOWORK_GAME_ACCOUNT_ID.");
static void PrintGames(IReadOnlyList<GameDescriptor> games)
{
    foreach (var game in games)
        Console.WriteLine($"{game.Id}: " + string.Join(", ", game.Capabilities.Select(c => $"{c.Id}={c.Status} ({c.AuthScope})")));
}
static async Task PrintPages(IAsyncEnumerable<GameSnapshot> pages)
{
    await foreach (var page in pages)
        foreach (var section in page.Sections)
            Console.WriteLine($"{page.GameId}/{section.Id}: {section.Status}; entities={section.Entities.Count}; source={section.Source?.SourceId ?? "unspecified"}; fetched={page.FetchedAt:O}");
}
static async Task PrintArchive(GameDataArchiveClient client, GameProvider provider, CancellationToken ct)
{
    var account = Environment.GetEnvironmentVariable("NOWORK_GAME_ACCOUNT_ID");
    if (account == null) return;
    var snapshots = await client.ArchiveSnapshotsAsync(provider, ParseAccount(account), ct);
    Console.WriteLine($"Archived snapshots: {snapshots.Count}; stale: {snapshots.Count(s => s.Stale)}");
}
