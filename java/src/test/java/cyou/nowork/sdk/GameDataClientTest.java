package cyou.nowork.sdk;

import static cyou.nowork.sdk.GameDataClient.*;
import static cyou.nowork.sdk.GameDataModels.*;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.math.BigDecimal;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

/** Dependency-free executable contract checks; the lead runs this class, not a JUnit runner. */
public final class GameDataClientTest {
    private static final String POLICY = "{\"visibility\":\"owner\",\"exportable\":false}";
    private static final String SOURCE = """
            {"source_id":"fixture-source","kind":"official","name":"Source","url":"https://example.test/docs",
             "auth_scope":"role","fetched_at":"2026-10-08T01:00:00Z","updated_at":"2026-10-07T01:00:00Z",
             "license":"Reference license","attribution":"Source attribution"}
            """;
    private static final String GAME = """
            {"id":"discovered-game","provider":"hypergryph","name":{"en":"Game"},
             "capabilities":[{"id":"characters","name":{"en":"Characters"},"supported":false,
             "auth_scope":"role","status":"unsupported","reason":"no_api"}]}
            """;
    private static final String ACCOUNT_GAMES = "{\"games\":[" + GAME + "],\"roles\":[],"
            + "\"discovery\":{\"status\":\"challenge_required\",\"updatedAt\":null,\"reason\":\"verification\"},\"selectedRoleId\":\"\"}";
    private static final String ASSET_CACHE = """
            {"provider":"reference-v1","locale":"zh_CN","version":"v1","disabled":false,
             "source_url":"https://example.test/catalog","entry_count":3,"content_hash":"abc",
             "fetched_at":"2026-10-07T01:00:00Z","checked_at":"2026-10-08T01:00:00Z",
             "expires_at":"2026-10-08T02:00:00Z","attempted_at":"2026-10-08T01:00:00Z","last_error_code":"rate_limited"}
            """;
    private static final String ASSET_MANIFEST = "{\"id\":\"reference-v1\",\"game_id\":\"discovered-game\",\"name\":{\"en\":\"Reference\"},"
            + "\"source\":" + SOURCE + ",\"endpoints\":[{\"locale\":\"zh_CN\",\"version\":\"v1\",\"url\":\"https://example.test/catalog\"}],"
            + "\"media_hosts\":[\"example.test\"],\"ttl_seconds\":3600,\"max_bytes\":8388608,\"max_entries\":20000}";
    private static final String ENTITY = "{\"id\":\"catalog-entry\",\"kind\":\"character\",\"name\":{\"en\":\"Character\"},"
            + "\"fields\":[],\"policy\":" + POLICY + "}";
    private static final String LEGACY_SUMMARY = """
            {"uid":"role-uid","name":"Role","level":0,"avatarUrl":"https://example.test/avatar.png","avatarId":"a",
             "resume":"","registerTs":1,"lastOnlineTs":2,"mainStage":"","secretary":"","apCurrent":0,"apMax":0,
             "apLastAddTs":0,"apCompleteTs":0,"charCount":1,"skinCount":0,"furnitureCount":0,"medals":0,
             "daily":{"current":0,"total":5},"weekly":null,
             "campaign":{"reward":{"current":0,"total":10},"records":[{"id":"map","name":"Map","value":0,"picture":""}]},
             "tower":{"higher":{"current":0,"total":1},"lower":{"current":0,"total":1},"ts":1,"records":null},
             "rogue":[{"id":"mode","name":"Mode","picture":"","relics":0,"bank":0,"bankRecord":0}],
             "recruit":[{"state":0,"startTs":0,"finishTs":0}],
             "building":{"controlLevel":1,"labor":0,"laborMax":0,"laborRemainSec":0,"laborTs":0,"tired":0,"hireRefresh":0,
             "rooms":[{"kind":"control","level":1,"chars":0,"value":0,"limit":0}]},"assist":null,
             "roster":[{"id":"operator","name":"Operator","rarity":0,"class":"WARRIOR","elite":0,"level":0,
             "potential":0,"skillLevel":0,"skills":[{"id":"skill","spec":0}],
             "equips":[{"id":"module","name":"Module","type":"basic","level":0,"default":false}],"favor":0,"gainTs":0}],
             "rosterTrimmed":true}
            """;

    @FunctionalInterface private interface Check { void run() throws Exception; }
    @FunctionalInterface private interface Reply { void respond(HttpExchange exchange) throws IOException; }
    private record Request(String method, String path, String query, String authorization, String serverKey, String cookie, String body) {}
    private final List<Request> requests = Collections.synchronizedList(new ArrayList<>());
    private final HttpServer http;
    private final ExecutorService executor = Executors.newCachedThreadPool();
    private volatile Reply reply = exchange -> send(exchange, 200, "{\"code\":0,\"data\":[]}");
    private final String base;
    private int checks;

    private GameDataClientTest() throws IOException {
        http = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        http.setExecutor(executor);
        http.createContext("/", exchange -> {
            requests.add(new Request(exchange.getRequestMethod(), exchange.getRequestURI().getRawPath(),
                    exchange.getRequestURI().getRawQuery(), exchange.getRequestHeaders().getFirst("Authorization"),
                    exchange.getRequestHeaders().getFirst("X-Server-Key"), exchange.getRequestHeaders().getFirst("Cookie"),
                    new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8)));
            try { reply.respond(exchange); }
            finally { exchange.close(); }
        });
        http.start();
        base = "http://127.0.0.1:" + http.getAddress().getPort();
    }

    public static void main(String[] args) throws Exception {
        GameDataClientTest fixture = new GameDataClientTest();
        try { fixture.run(); }
        finally { fixture.http.stop(0); fixture.executor.shutdownNow(); }
        System.out.println("GameDataClientTest: " + fixture.checks + " checks passed");
    }

    private void test(Check check) throws Exception { requests.clear(); check.run(); checks++; }
    private void data(String json) { reply = exchange -> send(exchange, 200, "{\"code\":0,\"data\":" + json + "}"); }
    private Request last() { return requests.get(requests.size() - 1); }
    private static void send(HttpExchange exchange, int status, String body) throws IOException {
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        exchange.sendResponseHeaders(status, bytes.length);
        exchange.getResponseBody().write(bytes);
    }
    private static void check(boolean condition, String message) { if (!condition) throw new AssertionError(message); }
    private static <T extends Throwable> T expect(Class<T> type, Check check) throws Exception {
        try { check.run(); }
        catch (Throwable error) {
            if (type.isInstance(error)) return type.cast(error);
            throw new AssertionError("expected " + type.getSimpleName() + ", got " + error.getClass().getSimpleName(), error);
        }
        throw new AssertionError("expected " + type.getSimpleName());
    }
    private static String field(String key, String valuePart) {
        return field(key, "number", valuePart);
    }
    private static String field(String key, String kind, String valuePart) {
        return "{\"key\":\"" + key + "\",\"name\":{\"en\":\"Field\"},\"kind\":" + Json.quote(kind) + ","
                + valuePart + "\"status\":\"stale\",\"policy\":" + POLICY + ",\"source\":" + SOURCE + "}";
    }
    private static String snapshot(String cursor) {
        String fields = String.join(",", field("missing", ""), field("null", "\"value\":null,"),
                field("zero", "\"value\":0,"), field("false", "boolean", "\"value\":false,"),
                field("large", "\"value\":18446744073709551615,"), field("decimal", "\"value\":0.1000000000000000001,"));
        return "{\"game_id\":\"discovered-game\",\"sections\":[{\"id\":\"characters\",\"name\":{\"en\":\"Characters\"},"
                + "\"status\":\"partial\",\"reason\":\"private\",\"fields\":[" + fields + "],\"entities\":["
                + "{\"id\":\"observed\",\"kind\":\"character\",\"name\":{\"en\":\"Observed\"},\"icon\":{\"url\":\"https://example.test/icon.png\","
                + "\"alt\":{\"en\":\"Icon\"},\"source_id\":\"fixture-source\",\"width\":64,\"height\":64},\"fields\":[],\"policy\":" + POLICY + "}],"
                + "\"policy\":" + POLICY + ",\"source\":" + SOURCE
                + (cursor == null ? "" : ",\"next_cursor\":" + Json.quote(cursor))
                + "}],\"fetched_at\":\"2026-10-08T01:00:00Z\",\"expires_at\":\"2026-10-08T02:00:00Z\"}";
    }
    private static String assetPage(int page, int limit, int count, String entities) {
        return "{\"provider\":" + ASSET_MANIFEST + ",\"cache\":" + ASSET_CACHE + ",\"status\":\"stale\","
                + "\"list\":[" + entities + "],\"count\":" + count + ",\"page\":" + page + ",\"limit\":" + limit + "}";
    }

    private void run() throws Exception {
        Account account = account(base, "owner-jwt");
        Admin admin = admin(base, "admin-jwt");
        Public publicClient = publicData(base);
        Server server = server(base, "server-key");
        SnapshotQuery oneSection = new SnapshotQuery(null, List.of("characters"), null, 2, "en");

        test(() -> {
            data(snapshot(null));
            Snapshot value = account.snapshot(Provider.HYPERGRYPH, "discovered-game",
                    new SnapshotQuery("owned:role", List.of("characters"), "cursor+/=?&x=1", 100, "zh_CN"));
            check(last().path().equals("/api/account/hypergryph/games/discovered-game/snapshot"), "Hypergryph snapshot path");
            check(last().query().contains("cursor=cursor%2B%2F%3D%3F%26x%3D1"), "cursor is an opaque query value");
            check(last().query().contains("roleId=owned%3Arole"), "owned role selector");
            check(last().authorization().equals("Bearer owner-jwt") && last().serverKey() == null && last().cookie() == null, "JWT-only authentication");
            List<Field> fields = value.sections().get(0).fields();
            check(!fields.get(0).hasValue() && fields.get(1).value() == NullValue.INSTANCE, "missing and null differ");
            check(((NumberValue) fields.get(2).value()).value().signum() == 0, "observed zero retained");
            check(!((BooleanValue) fields.get(3).value()).value(), "observed false retained");
            check(((NumberValue) fields.get(4).value()).value().toBigIntegerExact().equals(new BigInteger("18446744073709551615")), "uint64 field precision");
            check(((NumberValue) fields.get(5).value()).value().equals(new BigDecimal("0.1000000000000000001")), "decimal precision");
            check(value.role() == null && value.sections().get(0).entities().get(0).fields().isEmpty(), "no inferred role or ownership");
            check(value.sections().get(0).source().attribution().equals("Source attribution"), "source attribution");
            check(fields.get(0).source().updated_at().equals("2026-10-07T01:00:00Z") && !fields.get(0).policy().exportable(), "timestamp and false policy retained");
            check(value.sections().get(0).entities().get(0).icon().width() == 64, "typed icon metadata");
            expect(UnsupportedOperationException.class, () -> value.sections().clear());
        });
        test(() -> {
            data("[" + GAME + "]");
            check(account.mihoyoGames().size() == 1, "miHoYo array response");
            check(last().path().equals("/api/account/mihoyo/games"), "miHoYo games route");
            data(snapshot(null));
            account.snapshot(Provider.MIHOYO, "genshin", new SnapshotQuery("42", List.of("characters"), null, 10, "en"));
            check(last().path().endsWith("/mihoyo/games/genshin/data") && last().query().contains("roleId=42"), "miHoYo data route/numeric binding");
            int sent = requests.size();
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.MIHOYO, "genshin", new SnapshotQuery("uid:42", List.of(), null, null, null)));
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.MIHOYO, "genshin", new SnapshotQuery("9223372036854775808", List.of(), null, null, null)));
            check(requests.size() == sent, "bad miHoYo selectors rejected before send");
        });
        test(() -> {
            data(ACCOUNT_GAMES);
            check(account.hypergryphGames().discovery().status().equals("challenge_required"), "discovery status retained");
            check(account.selectHypergryphRole("saved-role").selectedRoleId().isEmpty(), "selection response retained");
            check(last().method().equals("PUT") && last().path().equals("/api/account/hypergryph/role")
                    && last().body().equals("{\"roleId\":\"saved-role\"}"), "selection route and body allowlist");
            admin.hypergryphAccountGames(17);
            check(last().path().equals("/api/admin/hypergryph/accounts/17/games") && last().authorization().equals("Bearer admin-jwt"), "admin account scope");
            data(snapshot(null)); admin.hypergryphSnapshot(17, "discovered-game", oneSection);
            check(last().path().equals("/api/admin/hypergryph/accounts/17/games/discovered-game/snapshot"), "admin snapshot scope");
        });
        test(() -> {
            data("{\"games\":[" + GAME + "]}");
            check(publicClient.hypergryphGames().games().get(0).id().equals("discovered-game"), "unknown product identifiers retained");
            check(last().authorization() == null && last().serverKey() == null && last().cookie() == null, "public scope is credential-free");
            data(snapshot(null)); publicClient.hypergryphSnapshot("discovered-game", oneSection);
            check(last().path().equals("/api/hypergryph/games/discovered-game/snapshot"), "public snapshot route");
            int sent = requests.size();
            expect(IllegalArgumentException.class, () -> publicClient.hypergryphSnapshot("discovered-game", new SnapshotQuery("private", List.of(), null, 1, "en")));
            check(sent == requests.size(), "public private-role selector rejected");
        });
        test(() -> {
            data("""
                    {"uid":123,"status":"expired","lastSync":null,"serverTime":1791421200,
                     "roles":[{"uid":"role-uid","nickname":"Role","channelName":"Official","isOfficial":true,
                     "isDefault":false,"level":0,"apNow":0,"summary":null,"lastSync":null}]}
                    """);
            SharedRoles shared = server.sharedHypergryphRoles(123);
            check(shared.roles().get(0).summary() == null && shared.status().equals("expired"), "legacy null summary/status retained");
            check(last().path().equals("/server/v1/hypergryph/roles") && last().query().equals("uid=123"), "real sharing route");
            check(last().serverKey().equals("server-key") && last().authorization() == null, "server key never accompanies JWT");
        });
        test(() -> {
            data("""
                    {"accountId":9,"region":"os","regions":[{"id":"os","name":"HoYoLAB","qr":false,"cookie":true}],
                     "recovery":{"action":"rebind","url":"https://example.test/rebind"},"gameCatalog":[],
                     "enabled":false,"allowQr":false,"allowCookie":true,"recordEnabled":false,"bound":true,
                     "profile":null,"roles":[{"id":42,"gameBiz":"hk4e_global","gameId":"genshin","gameName":"Genshin",
                     "region":"os","regionName":"Global","gameUid":"upstream-uid","nickname":"Role","level":0,
                     "isChosen":false,"syncTime":null}],"lastSync":null,"nextSyncAt":null,"games":[],
                     "method":"cookie","state":"expired","stateMessage":"Rebind"}
                    """);
            MihoyoStatus status = account.mihoyoStatus();
            check(status.accountId() == 9 && status.roles().get(0).id() == 42 && status.recovery().action().equals("rebind"), "binding/recovery contract");
            check(status.roles().get(0).gameUid().equals("upstream-uid") && status.roles().get(0).syncTime() == null, "role ID distinct from upstream UID");
            data("{\"enabled\":true,\"allowSms\":true,\"allowToken\":true,\"serverApi\":false,\"tokenHelpUrl\":\"https://example.test/help\",\"account\":null}");
            check(account.hypergryphStatus().account() == null, "unbound Hypergryph status preserved");
        });
        test(() -> {
            data("{\"uid\":123,\"status\":\"ok\",\"lastSync\":null,\"serverTime\":1,\"roles\":[{\"uid\":\"role-uid\","
                    + "\"nickname\":\"Role\",\"channelName\":\"Official\",\"isOfficial\":true,\"isDefault\":false,\"level\":0,\"apNow\":0,"
                    + "\"lastSync\":null,\"summary\":" + LEGACY_SUMMARY + "}]}");
            ArknightsSummary summary = server.sharedHypergryphRoles(123).roles().get(0).summary();
            check(summary.rosterTrimmed() && summary.weekly() == null && summary.assist() == null, "summary keeps truncation and nullable sections");
            check(summary.roster().get(0).profession().equals("WARRIOR") && !summary.roster().get(0).equips().get(0).equipped(), "reserved Java names map to actual class/default wire keys");
            check(summary.roster().get(0).skills().get(0).spec() == 0 && summary.building().rooms().get(0).kind().equals("control"), "typed skills/modules/base data");
            check(summary.campaign().records().get(0).value() == 0 && summary.tower().records().isEmpty()
                    && summary.rogue().get(0).bank() == 0 && summary.recruit().get(0).state() == 0, "typed mode/task/recruit data");
        });
        test(() -> {
            data("""
                    {"enabled":true,"allowSms":true,"allowToken":true,"serverApi":false,"tokenHelpUrl":"https://example.test/help",
                     "account":{"id":9,"userId":8,"sklandUid":"binding-uid","hgUid":"hg-uid","nickname":"Binding","avatar":"",
                     "method":"token","share":false,"status":"verify","lastError":"verify","lastSync":null,"selectedRoleId":"saved-role",
                     "discoveryStatus":"challenge_required","discoveryAt":null,"createTime":"2026-10-08 01:00:00","updateTime":"2026-10-08 01:00:00",
                     "roles":[{"id":17,"accountId":9,"userId":8,"uid":"role-uid","nickname":"Role","channelName":"Official","channelId":"1",
                     "isOfficial":true,"isDefault":false,"level":0,"syncError":"","lastSync":null,"createTime":"2026-10-08 01:00:00",
                     "updateTime":"2026-10-08 01:00:00","summary":null,"apNow":0}]}}
                    """);
            HypergryphAccount binding = account.hypergryphStatus().account();
            check(binding.id() == 9 && binding.roles().get(0).accountId() == 9 && binding.discoveryAt() == null, "Hypergryph owned archive binding ID and discovery metadata");
            String role = "{\"id\":\"saved-role\",\"game_id\":\"discovered-game\",\"uid\":\"role-uid\",\"region\":\"cn\",\"level\":0,\"policy\":" + POLICY + "}";
            data("{\"games\":[],\"roles\":[" + role + "],\"discovery\":{\"status\":\"ok\"},\"selectedRoleId\":\"saved-role\"}");
            check(account.hypergryphGames().roles().get(0).level() == 0, "normalized role retains observed zero level");
        });
        test(() -> {
            AtomicInteger pages = new AtomicInteger();
            reply = exchange -> send(exchange, 200, "{\"code\":0,\"data\":" + snapshot(pages.getAndIncrement() == 0 ? "next+/=?&" : null) + "}");
            List<Snapshot> received = new ArrayList<>();
            account.walkSnapshots(Provider.HYPERGRYPH, "discovered-game", oneSection, 3, page -> { received.add(page); return true; });
            check(received.size() == 2 && requests.size() == 2 && last().query().contains("cursor=next%2B%2F%3D%3F%26"), "section cursor continues exactly once");
        });
        test(() -> {
            data(snapshot("again"));
            PaginationException budget = expect(PaginationException.class, () -> account.walkSnapshots(Provider.HYPERGRYPH,
                    "discovered-game", oneSection, 1, page -> true));
            check(budget.nextCursor.equals("again") && requests.size() == 1, "budget stops before another request");
            requests.clear();
            expect(PaginationException.class, () -> account.walkSnapshots(Provider.HYPERGRYPH, "discovered-game", oneSection, 5, page -> true));
            check(requests.size() == 2, "cursor loop detected");
            requests.clear();
            account.walkSnapshots(Provider.HYPERGRYPH, "discovered-game", oneSection, 5, page -> false);
            check(requests.size() == 1, "visitor cancellation prevents next request");
        });
        test(() -> {
            data(assetPage(1, 2, 3, ENTITY + "," + ENTITY));
            AssetPage assets = account.assets(new AssetQuery("reference-v1", "", "", "刀 & shield", "character", 1, 2));
            check(assets.status().equals("stale") && assets.cache().last_error_code().equals("rate_limited"), "stale cache metadata retained");
            check(assets.list().get(0).fields().isEmpty() && assets.provider().endpoints().get(0).locale().equals("zh_CN"), "catalog remains public reference data");
            check(last().query().contains("locale=&version=&q=%E5%88%80%20%26%20shield"), "blank variant and search encoding");
            data("[{\"manifest\":" + ASSET_MANIFEST + ",\"caches\":[" + ASSET_CACHE + "]}]");
            check(admin.assetProviders().get(0).caches().size() == 1 && last().path().equals("/api/admin/game-assets/providers"), "asset provider metadata");
        });
        test(() -> {
            data(ASSET_CACHE);
            admin.refreshAssets("reference-v1", new AssetVariant("zh_CN", "v1"));
            check(last().method().equals("POST") && last().path().endsWith("/reference-v1/refresh"), "asset refresh route");
            admin.setAssetDisabled("reference-v1", false, AssetVariant.defaults());
            check(last().method().equals("PUT") && last().body().equals("{\"disabled\":false}"), "explicit false state write");
        });
        test(() -> {
            AtomicInteger pages = new AtomicInteger();
            reply = exchange -> send(exchange, 200, "{\"code\":0,\"data\":"
                    + (pages.getAndIncrement() == 0 ? assetPage(1, 2, 3, ENTITY + "," + ENTITY) : assetPage(2, 2, 3, ENTITY)) + "}");
            account.walkAssets(new AssetQuery("reference-v1", null, null, null, null, 1, 2), 3, page -> true);
            check(requests.size() == 2 && last().query().contains("page=2"), "catalog bounded iteration");
            data(assetPage(1, 2, 3, ""));
            expect(PaginationException.class, () -> account.walkAssets(AssetQuery.of("reference-v1"), 3, page -> true));
            data(assetPage(1, 2, 3, ENTITY + "," + ENTITY));
            expect(PaginationException.class, () -> account.walkAssets(AssetQuery.of("reference-v1"), 1, page -> true));
            data(assetPage(2, 2, 3, ENTITY));
            expect(PaginationException.class, () -> account.walkAssets(AssetQuery.of("reference-v1"), 3, page -> true));
        });
        test(() -> {
            String revision = "18446744073709551615";
            data("[{\"game_id\":\"genshin\",\"role_id\":\"bound-role\",\"version\":1,\"revision\":" + revision
                    + ",\"status\":\"stale\",\"stale\":true,\"bytes\":12,\"fetched_at\":\"2026-10-07T01:00:00Z\",\"expires_at\":\"2026-10-08T01:00:00Z\"}]");
            SnapshotMetadata metadata = account.archiveSnapshots(Provider.MIHOYO, 9).get(0);
            check(metadata.revision().equals(new BigInteger(revision)) && last().path().equals("/api/account/mihoyo/accounts/9/archive/snapshots"), "exact archive revision/path");
            data("{\"provider\":\"mihoyo\",\"role_id\":\"bound-role\",\"version\":1,\"revision\":2,\"status\":\"partial\",\"stale\":false,"
                    + "\"expires_at\":\"2026-10-08T02:00:00Z\",\"snapshot\":" + snapshot(null) + "}");
            check(admin.archivedSnapshot(Provider.MIHOYO, 9, "genshin", "role &/").snapshot().sections().size() == 1, "typed archived detail");
            check(last().path().startsWith("/api/admin/") && last().query().contains("role_id=role%20%26%2F") && last().query().contains("game_id=genshin"), "archive uses snake-case selectors");
        });
        test(() -> {
            BigInteger cursor = new BigInteger("18446744073709551615");
            data("[{\"id\":" + cursor + ",\"game_id\":\"genshin\",\"role_id\":\"\",\"revision\":0,\"status\":\"unavailable\",\"bytes\":0,\"created_at\":\"2026-10-08T01:00:00Z\"}]");
            check(account.syncHistory(Provider.MIHOYO, 9, new ArchiveQuery(cursor, 100)).get(0).id().equals(cursor), "history uint64 precision");
            check(last().query().contains("before=" + cursor) && last().query().contains("limit=100"), "exact history continuation");
            data("[{\"id\":1,\"game_id\":\"genshin\",\"path\":\"/sections/profile/fields/level\",\"revision\":0,\"previous\":\"null\",\"current\":\"{}\",\"created_at\":\"2026-10-08T01:00:00Z\"}]");
            check(account.policyAudit(Provider.MIHOYO, 9, ArchiveQuery.defaults()).get(0).previous().equals("null"), "policy audit encoded JSON");
        });
        test(() -> {
            String path = fieldPath("characters/a", "level~current", "entity/a~b");
            check(path.equals("/sections/characters~1a/entities/entity~1a~0b/fields/level~0current"), "Go pointer escaping parity");
            String policy = "{\"id\":1,\"game_id\":\"genshin\",\"path\":" + Json.quote(path)
                    + ",\"visibility\":\"owner\",\"exportable\":false,\"revision\":0,\"updated_at\":\"2026-10-08T01:00:00Z\"}";
            data(policy);
            StoredPolicy stored = admin.setFieldPolicy(Provider.MIHOYO, 9, "genshin", new PolicyInput(path, "owner", false, BigInteger.ZERO));
            check(!stored.exportable() && last().body().contains("\"revision\":0") && last().body().contains("\"exportable\":false"), "revision zero and false are explicit");
            Map<String, Object> body = GameDataModels.object(GameDataModels.parse(last().body()));
            check(body.size() == 5 && !body.containsKey("actor_id") && !body.containsKey("site_id"), "policy mutation allowlist");
            data("[" + policy + "]");
            check(account.fieldPolicies(Provider.MIHOYO, 9, "genshin").size() == 1, "owner reads policy without write interface");
            check(java.util.Arrays.stream(Account.class.getMethods()).noneMatch(method -> method.getName().equals("setFieldPolicy")), "owner has no policy mutator");
        });
        test(() -> {
            data("{\"scope\":{\"site_id\":0,\"user_id\":8,\"account_id\":9},\"provider\":\"mihoyo\","
                    + "\"exported_at\":\"2026-10-08T01:00:00Z\",\"snapshots\":[],\"history\":[],\"policies\":[],\"policy_audit\":[]}");
            ArchiveExport exported = account.exportArchive(Provider.MIHOYO, 9);
            check(exported.scope().site_id().signum() == 0 && last().path().endsWith("/archive/export"), "real policy-filtered export route");
            reply = exchange -> send(exchange, 200, "{\"code\":0}");
            account.clearArchive(Provider.MIHOYO, 9);
            check(last().method().equals("DELETE") && last().path().endsWith("/accounts/9/archive"), "empty-success cache clear");
            expect(NoWorkException.Transport.class, account::mihoyoGames);
        });
        test(() -> {
            reply = exchange -> {
                exchange.getResponseHeaders().set("Retry-After", "13");
                send(exchange, 429, "{\"code\":429,\"message\":\"challenge\",\"data\":{\"status\":\"challenge_required\"}}");
            };
            ApiException error = expect(ApiException.class, account::hypergryphGames);
            check(error instanceof NoWorkException.Api && error.code == 429 && error.status == 429 && error.retryAfter.equals("13"), "API identity/status/retry metadata");
            check(GameDataModels.object(error.data).get("status").equals("challenge_required") && requests.size() == 1, "challenge details and no retry");
            reply = exchange -> send(exchange, 200, "{\"code\":409,\"message\":\"conflict\"}");
            check(expect(ApiException.class, () -> admin.setFieldPolicy(Provider.MIHOYO, 9, "genshin",
                    new PolicyInput(fieldPath("p", "level", null), "owner", false, BigInteger.ZERO))).code == 409, "policy conflict is not overwritten");
        });
        test(() -> {
            for (String malformed : List.of("{\"code\":0}", "{\"code\":0,\"data\":null}", "{\"data\":[]}",
                    "{\"code\":0.0,\"data\":[]}", "{\"code\":0,\"data\":[]} trailing", "{\"code\":0,\"code\":0,\"data\":[]}",
                    "{\"code\":0,\"data\":[}", "{\"code\":0,\"data\":[]", "{\"code\":0,\"data\":[NaN]}",
                    "{\"code\":0,\"data\":{},}", "{\"code\":0,\"data\":[{\"id\":\"bad\"}]}", "{\"code\":0,\"data\":[]}[]")) {
                reply = exchange -> send(exchange, 200, malformed);
                expect(NoWorkException.Transport.class, account::mihoyoGames);
            }
            reply = exchange -> send(exchange, 503, "{\"code\":0,\"data\":[]}");
            expect(NoWorkException.Transport.class, account::mihoyoGames);
            reply = exchange -> {
                byte[] invalidUtf8 = {(byte) 0xC3, (byte) 0x28};
                exchange.sendResponseHeaders(200, invalidUtf8.length); exchange.getResponseBody().write(invalidUtf8);
            };
            expect(NoWorkException.Transport.class, account::mihoyoGames);
        });
        test(() -> {
            data("{\"jwe\":\"a.b.c.d.e\"}");
            expect(NoWorkException.Encryption.class, () -> server.sharedHypergryphRoles(123));
            reply = exchange -> { exchange.getResponseHeaders().set(NoworkClient.HEADER_ENCRYPTION, "JWE"); send(exchange, 200, "encrypted"); };
            expect(NoWorkException.Encryption.class, account::mihoyoGames);
        });
        test(() -> {
            for (String invalidBase : List.of("ftp://example.test", "https://user:password@example.test", "https://example.test?x=1",
                    "https://example.test#fragment", "https://example.test/a/../b", "https://example.test\r\nX: y")) {
                expect(IllegalArgumentException.class, () -> account(invalidBase, "token"));
            }
            expect(IllegalArgumentException.class, () -> account(base, ""));
            expect(IllegalArgumentException.class, () -> account(base, "x\nheader"));
            expect(IllegalArgumentException.class, () -> new Options(Duration.ZERO, 1));
            expect(IllegalArgumentException.class, () -> new Options(Duration.ofSeconds(61), 1));
            expect(IllegalArgumentException.class, () -> new Options(Duration.ofSeconds(1), 33 * 1024 * 1024));
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.HYPERGRYPH, "../private", oneSection));
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.HYPERGRYPH, "%2e%2e", oneSection));
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.HYPERGRYPH, "game", new SnapshotQuery(null, List.of("a,b"), null, 1, "en")));
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.HYPERGRYPH, "game", new SnapshotQuery(null, List.of(), "x".repeat(2049), 1, "en")));
            expect(IllegalArgumentException.class, () -> account.snapshot(Provider.HYPERGRYPH, "game", new SnapshotQuery(null, List.of(), null, 101, "en")));
            expect(IllegalArgumentException.class, () -> account.assets(new AssetQuery("reference-v1", null, null, "刀".repeat(43), null, 1, 1)));
            expect(IllegalArgumentException.class, () -> account.assets(new AssetQuery("reference-v1", null, null, null, null, 20001, 1)));
            expect(IllegalArgumentException.class, () -> account.syncHistory(Provider.MIHOYO, 9, new ArchiveQuery(BigInteger.ONE.shiftLeft(64), 1)));
            expect(IllegalArgumentException.class, () -> admin.hypergryphAccountGames(0));
            expect(IllegalArgumentException.class, () -> server.sharedHypergryphRoles(-1));
            expect(IllegalArgumentException.class, () -> admin.setFieldPolicy(Provider.MIHOYO, 9, "genshin", new PolicyInput("/sections/a/fields/~9", "owner", false, BigInteger.ZERO)));
            expect(IllegalArgumentException.class, () -> account.walkSnapshots(Provider.HYPERGRYPH, "game", SnapshotQuery.defaults(), 1, page -> true));
            expect(IllegalArgumentException.class, () -> account.walkSnapshots(Provider.HYPERGRYPH, "game", oneSection, 101, page -> true));
            check(requests.isEmpty(), "invalid inputs never send credentials");
        });
        test(() -> {
            AtomicInteger redirected = new AtomicInteger();
            HttpServer target = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            target.createContext("/", exchange -> { redirected.incrementAndGet(); exchange.close(); }); target.start();
            try {
                reply = exchange -> { exchange.getResponseHeaders().set("Location", "http://127.0.0.1:" + target.getAddress().getPort() + "/private"); send(exchange, 302, "{}"); };
                expect(NoWorkException.Transport.class, account::mihoyoGames);
                check(redirected.get() == 0, "JWT redirect never reaches target");
            } finally { target.stop(0); }
        });
        test(() -> {
            Account small = account(base, "owner-jwt", new Options(Duration.ofSeconds(3), 64));
            reply = exchange -> {
                exchange.sendResponseHeaders(200, 0); // Chunked: no Content-Length shortcut.
                exchange.getResponseBody().write(("{\"code\":0,\"data\":[]}" + " ".repeat(200)).getBytes(StandardCharsets.UTF_8));
            };
            expect(NoWorkException.Transport.class, small::mihoyoGames);
        });
        test(() -> {
            data("[]");
            Thread.currentThread().interrupt();
            try {
                expect(NoWorkException.Transport.class, account::mihoyoGames);
                check(Thread.currentThread().isInterrupted() && requests.isEmpty(), "pre-interrupted caller sends nothing and keeps flag");
            } finally { Thread.interrupted(); }
        });
        test(() -> {
            CountDownLatch bodyStarted = new CountDownLatch(1);
            CountDownLatch release = new CountDownLatch(1);
            reply = exchange -> {
                exchange.sendResponseHeaders(200, 0);
                exchange.getResponseBody().write('{'); exchange.getResponseBody().flush(); bodyStarted.countDown();
                try { release.await(5, TimeUnit.SECONDS); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
            };
            AtomicReference<Throwable> failure = new AtomicReference<>();
            AtomicBoolean keptInterrupt = new AtomicBoolean();
            Thread caller = new Thread(() -> {
                try { account.mihoyoGames(); } catch (Throwable error) { failure.set(error); keptInterrupt.set(Thread.currentThread().isInterrupted()); }
            });
            caller.start();
            try {
                check(bodyStarted.await(3, TimeUnit.SECONDS), "body started");
                caller.interrupt(); caller.join(3000);
                check(!caller.isAlive() && failure.get() instanceof NoWorkException.Transport && keptInterrupt.get(), "in-flight body cancellation");
            } finally { release.countDown(); caller.interrupt(); caller.join(3000); }
        });
        test(() -> {
            CountDownLatch release = new CountDownLatch(1);
            reply = exchange -> {
                exchange.sendResponseHeaders(200, 0); exchange.getResponseBody().write('{'); exchange.getResponseBody().flush();
                try { release.await(5, TimeUnit.SECONDS); } catch (InterruptedException e) { Thread.currentThread().interrupt(); }
            };
            Account shortTimeout = account(base, "owner-jwt", new Options(Duration.ofMillis(200), 1024));
            long start = System.nanoTime();
            try {
                NoWorkException.Transport error = expect(NoWorkException.Transport.class, shortTimeout::mihoyoGames);
                check(error.getMessage().contains("timed out") && Duration.ofNanos(System.nanoTime() - start).toMillis() < 3000, "timeout includes stalled body");
            } finally { release.countDown(); }
        });
        test(() -> {
            String deep = "[".repeat(66) + "0" + "]".repeat(66);
            expect(IllegalArgumentException.class, () -> GameDataModels.parse(deep));
            expect(IllegalArgumentException.class, () -> GameDataModels.parse("[01]"));
            expect(IllegalArgumentException.class, () -> GameDataModels.parse("[1.]"));
            expect(IllegalArgumentException.class, () -> GameDataModels.parse("[1e]"));
            expect(IllegalArgumentException.class, () -> GameDataModels.parse("[\"\\x\"]"));
            expect(IllegalArgumentException.class, () -> GameDataModels.decode(GameDataModels.parse(field("map", "\"value\":{},")), Field.class));
        });
    }
}
