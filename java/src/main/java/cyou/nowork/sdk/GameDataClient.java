package cyou.nowork.sdk;

import static cyou.nowork.sdk.GameDataModels.*;

import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.nio.ByteBuffer;
import java.nio.charset.CharacterCodingException;
import java.nio.charset.CodingErrorAction;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Flow;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/**
 * Java 17, dependency-free clients for the site's game APIs. Construct exactly one
 * credential scope: {@link #account}, {@link #admin}, {@link #publicData}, or {@link #server}.
 * Rich account/admin routes take a site login JWT; an OAuth token or server key is not
 * a substitute. The server enforces account ownership, permissions and source grants.
 * No method calls publisher APIs directly, downloads icons, retries challenges or
 * converts public catalogs into owned inventory. Interrupt the calling thread to cancel.
 */
public final class GameDataClient {
    private GameDataClient() {}

    public enum Provider {
        MIHOYO("mihoyo"), HYPERGRYPH("hypergryph");
        private final String path;
        Provider(String path) { this.path = path; }
        public String id() { return path; }
    }

    /** Timeout covers headers and the entire bounded response body; no automatic retry. */
    public record Options(Duration timeout, int maxResponseBytes) {
        public Options {
            if (timeout == null || timeout.compareTo(Duration.ofMillis(1)) < 0
                    || timeout.compareTo(Duration.ofSeconds(60)) > 0) {
                throw new IllegalArgumentException("timeout must be between 1 millisecond and 60 seconds");
            }
            if (maxResponseBytes < 1 || maxResponseBytes > 32 * 1024 * 1024) {
                throw new IllegalArgumentException("maxResponseBytes must be between 1 and 33554432");
            }
        }
        /** Archive exports can contain 16 MiB of snapshots plus metadata. */
        public static Options defaults() { return new Options(Duration.ofSeconds(10), 24 * 1024 * 1024); }
    }

    public record SnapshotQuery(String roleId, List<String> sections, String cursor, Integer limit, String locale) {
        public SnapshotQuery { sections = sections == null ? List.of() : List.copyOf(sections); }
        public static SnapshotQuery defaults() { return new SnapshotQuery(null, List.of(), null, null, null); }
        public SnapshotQuery withCursor(String next) { return new SnapshotQuery(roleId, sections, next, limit, locale); }
    }
    public record AssetQuery(String provider, String locale, String version, String search, String kind,
            Integer page, Integer limit) {
        public static AssetQuery of(String provider) { return new AssetQuery(provider, null, null, null, null, null, null); }
        public AssetQuery withPage(int next) { return new AssetQuery(provider, locale, version, search, kind, next, limit); }
    }
    public record AssetVariant(String locale, String version) {
        public static AssetVariant defaults() { return new AssetVariant(null, null); }
    }
    public record ArchiveQuery(BigInteger before, Integer limit) {
        public static ArchiveQuery defaults() { return new ArchiveQuery(null, null); }
    }
    /** revision=0 creates a policy; updates require the revision returned by the last read. */
    public record PolicyInput(String path, String visibility, boolean exportable, BigInteger revision) {}

    public static Account account(String base, String siteJwt) { return account(base, siteJwt, Options.defaults()); }
    public static Account account(String base, String siteJwt, Options options) {
        return new Account(new Transport(base, "Authorization", "Bearer " + credential(siteJwt), options));
    }
    public static Admin admin(String base, String siteJwt) { return admin(base, siteJwt, Options.defaults()); }
    public static Admin admin(String base, String siteJwt, Options options) {
        return new Admin(new Transport(base, "Authorization", "Bearer " + credential(siteJwt), options));
    }
    public static Public publicData(String base) { return publicData(base, Options.defaults()); }
    public static Public publicData(String base, Options options) { return new Public(new Transport(base, null, null, options)); }
    public static Server server(String base, String serverKey) { return server(base, serverKey, Options.defaults()); }
    public static Server server(String base, String serverKey, Options options) {
        return new Server(new Transport(base, "X-Server-Key", credential(serverKey), options));
    }

    /** Public Hypergryph capabilities/reference snapshots. No role selector or credentials are sent. */
    public static final class Public {
        private final Transport transport;
        private Public(Transport transport) { this.transport = transport; }
        public GameList hypergryphGames() throws NoWorkException {
            return transport.get("/api/hypergryph/games", Map.of(), GameList.class);
        }
        public Snapshot hypergryphSnapshot(String gameId, SnapshotQuery query) throws NoWorkException {
            return transport.get("/api/hypergryph/games/" + segment(gameId, 64) + "/snapshot",
                    snapshotQuery(query, false, false), Snapshot.class);
        }
        public void walkHypergryphSnapshots(String gameId, SnapshotQuery query, int maxPages,
                PageVisitor<Snapshot> visitor) throws NoWorkException {
            walkSnapshots(next -> hypergryphSnapshot(gameId, next), query, maxPages, visitor);
        }
    }

    /** Only the real legacy Arknights sharing endpoint accepts a server key. */
    public static final class Server {
        private final Transport transport;
        private Server(Transport transport) { this.transport = transport; }
        /** uid is the site's public player UID; sharing and an approved server role are required. */
        public SharedRoles sharedHypergryphRoles(long uid) throws NoWorkException {
            return transport.get("/server/v1/hypergryph/roles", Map.of("uid", positive(uid)), SharedRoles.class);
        }
    }

    /** Common scoped archive/catalog reads. Owner clients expose no policy-writing method. */
    public abstract static class Authenticated {
        final Transport transport;
        private final String scope;
        private Authenticated(Transport transport, String scope) { this.transport = transport; this.scope = scope; }
        final String archivePath(Provider provider, long accountId) {
            return "/api/" + scope + "/" + Objects.requireNonNull(provider).id()
                    + "/accounts/" + positive(accountId) + "/archive";
        }
        public List<SnapshotMetadata> archiveSnapshots(Provider provider, long accountId) throws NoWorkException {
            return transport.list(archivePath(provider, accountId) + "/snapshots", Map.of(), SnapshotMetadata.class);
        }
        public ArchivedSnapshot archivedSnapshot(Provider provider, long accountId, String gameId, String roleId)
                throws NoWorkException {
            return transport.get(archivePath(provider, accountId) + "/snapshots",
                    Map.of("game_id", text(gameId, 64, false), "role_id", text(roleId == null ? "" : roleId, 128, true)),
                    ArchivedSnapshot.class);
        }
        public List<StoredPolicy> fieldPolicies(Provider provider, long accountId, String gameId) throws NoWorkException {
            return transport.list(archivePath(provider, accountId) + "/policies",
                    Map.of("game_id", text(gameId, 64, false)), StoredPolicy.class);
        }
        /** Newest first; continue using the last returned ID as before. */
        public List<History> syncHistory(Provider provider, long accountId, ArchiveQuery query) throws NoWorkException {
            return transport.list(archivePath(provider, accountId) + "/history", archiveQuery(query), History.class);
        }
        public List<PolicyAudit> policyAudit(Provider provider, long accountId, ArchiveQuery query) throws NoWorkException {
            return transport.list(archivePath(provider, accountId) + "/policy-audit", archiveQuery(query), PolicyAudit.class);
        }
        /** Uses the server's policy-filtered export; serializing an owner snapshot is not an export. */
        public ArchiveExport exportArchive(Provider provider, long accountId) throws NoWorkException {
            return transport.get(archivePath(provider, accountId) + "/export", Map.of(), ArchiveExport.class);
        }
        /** Clears cached snapshots/history; the server retains administrator field restrictions. */
        public void clearArchive(Provider provider, long accountId) throws NoWorkException {
            transport.call("DELETE", archivePath(provider, accountId), Map.of(), null, true);
        }
        public List<AssetProvider> assetProviders() throws NoWorkException {
            return transport.list("/api/" + scope + "/game-assets/providers", Map.of(), AssetProvider.class);
        }
        public AssetPage assets(AssetQuery query) throws NoWorkException {
            return transport.get("/api/" + scope + "/game-assets/catalog", assetQuery(query), AssetPage.class);
        }
        /** Streams bounded pages to the visitor; false stops without another request. */
        public void walkAssets(AssetQuery query, int maxPages, PageVisitor<AssetPage> visitor) throws NoWorkException {
            pageBudget(maxPages); Objects.requireNonNull(visitor); assetQuery(query);
            int page = query.page() == null ? 1 : query.page();
            Integer expectedLimit = query.limit();
            for (int seen = 0; seen < maxPages; seen++) {
                AssetPage data = assets(query.withPage(page));
                if (data.page() != page || data.limit() < 1 || data.limit() > 100 || data.count() < 0
                        || expectedLimit != null && data.limit() != expectedLimit || data.list().size() > data.limit()) {
                    throw new PaginationException("invalid catalog pagination", null);
                }
                expectedLimit = data.limit();
                if (!visitor.visit(data)) return;
                if ((long) page * data.limit() >= data.count()) return;
                if (data.list().isEmpty()) throw new PaginationException("catalog ended before its reported count", null);
                if (seen + 1 == maxPages || page == 20000) throw new PaginationException("catalog page budget exhausted", null);
                page++;
            }
        }
    }

    public static final class Account extends Authenticated {
        private Account(Transport transport) { super(transport, "account"); }
        public MihoyoStatus mihoyoStatus() throws NoWorkException {
            return transport.get("/api/account/mihoyo/status", Map.of(), MihoyoStatus.class);
        }
        public HypergryphStatus hypergryphStatus() throws NoWorkException {
            return transport.get("/api/account/hypergryph/status", Map.of(), HypergryphStatus.class);
        }
        public List<Game> mihoyoGames() throws NoWorkException {
            return transport.list("/api/account/mihoyo/games", Map.of(), Game.class);
        }
        public AccountGames hypergryphGames() throws NoWorkException {
            return transport.get("/api/account/hypergryph/games", Map.of(), AccountGames.class);
        }
        public AccountGames selectHypergryphRole(String roleId) throws NoWorkException {
            return transport.mutate("PUT", "/api/account/hypergryph/role",
                    Map.of("roleId", text(roleId, 128, false)), AccountGames.class);
        }
        /** miHoYo roleId must be a stored numeric MihoyoRole.id, never its upstream gameUid. */
        public Snapshot snapshot(Provider provider, String gameId, SnapshotQuery query) throws NoWorkException {
            Objects.requireNonNull(provider);
            return transport.get("/api/account/" + provider.id() + "/games/" + segment(gameId, 64)
                    + (provider == Provider.MIHOYO ? "/data" : "/snapshot"),
                    snapshotQuery(query, true, provider == Provider.MIHOYO), Snapshot.class);
        }
        public void walkSnapshots(Provider provider, String gameId, SnapshotQuery query, int maxPages,
                PageVisitor<Snapshot> visitor) throws NoWorkException {
            GameDataClient.walkSnapshots(next -> snapshot(provider, gameId, next), query, maxPages, visitor);
        }
    }

    public static final class Admin extends Authenticated {
        private Admin(Transport transport) { super(transport, "admin"); }
        public List<Game> mihoyoGames() throws NoWorkException {
            return transport.list("/api/admin/mihoyo/games", Map.of(), Game.class);
        }
        public GameList hypergryphGames() throws NoWorkException {
            return transport.get("/api/admin/hypergryph/games", Map.of(), GameList.class);
        }
        public AccountGames hypergryphAccountGames(long accountId) throws NoWorkException {
            return transport.get("/api/admin/hypergryph/accounts/" + positive(accountId) + "/games", Map.of(), AccountGames.class);
        }
        public Snapshot hypergryphSnapshot(long accountId, String gameId, SnapshotQuery query) throws NoWorkException {
            return transport.get("/api/admin/hypergryph/accounts/" + positive(accountId) + "/games/"
                    + segment(gameId, 64) + "/snapshot", snapshotQuery(query, true, false), Snapshot.class);
        }
        public void walkHypergryphSnapshots(long accountId, String gameId, SnapshotQuery query, int maxPages,
                PageVisitor<Snapshot> visitor) throws NoWorkException {
            walkSnapshots(next -> hypergryphSnapshot(accountId, gameId, next), query, maxPages, visitor);
        }
        public AssetCache refreshAssets(String provider, AssetVariant variant) throws NoWorkException {
            return transport.mutate("POST", "/api/admin/game-assets/providers/" + segment(provider, 96) + "/refresh",
                    assetVariant(variant), AssetCache.class);
        }
        public AssetCache setAssetDisabled(String provider, boolean disabled, AssetVariant variant) throws NoWorkException {
            Map<String, Object> body = new LinkedHashMap<>(assetVariant(variant));
            body.put("disabled", disabled);
            return transport.mutate("PUT", "/api/admin/game-assets/providers/" + segment(provider, 96) + "/state", body, AssetCache.class);
        }
        /** Optimistic concurrency; conflicts propagate and no broader source grant is invented. */
        public StoredPolicy setFieldPolicy(Provider provider, long accountId, String gameId, PolicyInput input)
                throws NoWorkException {
            Objects.requireNonNull(input);
            String path = validFieldPath(input.path());
            if (!Set.of("owner", "shared", "public", "secret").contains(input.visibility())) {
                throw new IllegalArgumentException("invalid field visibility");
            }
            unsigned(input.revision());
            Map<String, Object> body = new LinkedHashMap<>();
            body.put("game_id", text(gameId, 64, false)); body.put("path", path);
            body.put("visibility", input.visibility()); body.put("exportable", input.exportable()); body.put("revision", input.revision());
            return transport.mutate("PUT", super.archivePath(provider, accountId) + "/policies", body, StoredPolicy.class);
        }
    }

    @FunctionalInterface public interface PageVisitor<T> { boolean visit(T page) throws NoWorkException; }
    @FunctionalInterface private interface SnapshotFetcher { Snapshot fetch(SnapshotQuery query) throws NoWorkException; }
    public static final class PaginationException extends NoWorkException {
        /** Explicit resume cursor, if the snapshot budget ended before the final page. */
        public final String nextCursor;
        private PaginationException(String message, String nextCursor) { super("nowork: " + message); this.nextCursor = nextCursor; }
    }
    /** Preserves the existing SDK business-error identity plus HTTP/challenge/rate-limit metadata. */
    public static final class ApiException extends NoWorkException.Api {
        public final int status;
        public final Object data;
        public final String retryAfter;
        private ApiException(int code, String message, int status, Object data, String retryAfter) {
            super(code, message); this.status = status; this.data = data; this.retryAfter = retryAfter;
        }
    }

    private static void walkSnapshots(SnapshotFetcher fetch, SnapshotQuery query, int maxPages,
            PageVisitor<Snapshot> visitor) throws NoWorkException {
        pageBudget(maxPages); Objects.requireNonNull(visitor); Objects.requireNonNull(query);
        if (query.sections().size() != 1) throw new IllegalArgumentException("snapshot walking requires exactly one section");
        Set<String> cursors = new HashSet<>();
        if (query.cursor() != null && !query.cursor().isEmpty()) cursors.add(query.cursor());
        for (int page = 0; page < maxPages; page++) {
            Snapshot data = fetch.fetch(query);
            if (!visitor.visit(data)) return;
            String cursor = data.next_cursor();
            for (Section section : data.sections()) {
                if (section.id().equals(query.sections().get(0)) && section.next_cursor() != null && !section.next_cursor().isEmpty()) {
                    if (cursor != null && !cursor.isEmpty() && !cursor.equals(section.next_cursor())) {
                        throw new PaginationException("conflicting snapshot cursors", null);
                    }
                    cursor = section.next_cursor();
                }
            }
            if (cursor == null || cursor.isEmpty()) return;
            text(cursor, 2048, false);
            if (!cursors.add(cursor)) throw new PaginationException("snapshot cursor repeated", cursor);
            if (page + 1 == maxPages) throw new PaginationException("snapshot page budget exhausted", cursor);
            query = query.withCursor(cursor);
        }
    }

    /** Produces gamearchive.FieldPath's JSON-pointer escaping, including slash/tilde in entity IDs. */
    public static String fieldPath(String sectionId, String fieldKey, String entityId) {
        String path = "/sections/" + pointer(sectionId) + (entityId == null ? "" : "/entities/" + pointer(entityId))
                + "/fields/" + pointer(fieldKey);
        return validFieldPath(path);
    }
    private static String pointer(String value) { return text(value, 320, false).replace("~", "~0").replace("/", "~1"); }
    private static String validFieldPath(String value) {
        text(value, 320, false);
        if (!value.matches("/sections/(?:[^/~]|~[01])+/(?:entities/(?:[^/~]|~[01])+/)?fields/(?:[^/~]|~[01])+")) {
            throw new IllegalArgumentException("invalid field path");
        }
        return value;
    }
    private static void pageBudget(int pages) {
        if (pages < 1 || pages > 100) throw new IllegalArgumentException("maxPages must be between 1 and 100");
    }
    private static String positive(long value) {
        if (value <= 0) throw new IllegalArgumentException("ID must be positive");
        return Long.toString(value);
    }
    private static String unsigned(BigInteger value) {
        if (value == null || value.signum() < 0 || value.bitLength() > 64) throw new IllegalArgumentException("invalid uint64 value");
        return value.toString();
    }
    private static String text(String value, int maxBytes, boolean empty) {
        if (value == null || !empty && value.isEmpty() || value.length() > maxBytes
                || value.getBytes(StandardCharsets.UTF_8).length > maxBytes) {
            throw new IllegalArgumentException("invalid game API parameter");
        }
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            if (c < 32 || c == 127) throw new IllegalArgumentException("invalid game API parameter");
            if (Character.isHighSurrogate(c)) {
                if (++i >= value.length() || !Character.isLowSurrogate(value.charAt(i))) throw new IllegalArgumentException("invalid Unicode parameter");
            } else if (Character.isLowSurrogate(c)) throw new IllegalArgumentException("invalid Unicode parameter");
        }
        return value;
    }
    private static String credential(String value) {
        text(value, 16384, false);
        if (!value.equals(value.trim()) || value.isBlank() || value.indexOf(' ') >= 0) {
            throw new IllegalArgumentException("invalid credential");
        }
        return value;
    }
    private static String segment(String value, int maxBytes) {
        text(value, maxBytes, false);
        if (!value.matches("[A-Za-z0-9][A-Za-z0-9_.:-]*") || value.equals(".") || value.equals("..")) {
            throw new IllegalArgumentException("invalid game/provider identifier");
        }
        return encode(value);
    }
    private static String encode(String value) { return URLEncoder.encode(value, StandardCharsets.UTF_8).replace("+", "%20"); }
    private static Map<String, String> snapshotQuery(SnapshotQuery input, boolean roleAllowed, boolean numericRole) {
        Objects.requireNonNull(input);
        Map<String, String> result = new LinkedHashMap<>();
        if (input.roleId() != null) {
            if (!roleAllowed) throw new IllegalArgumentException("public snapshots do not accept roleId");
            text(input.roleId(), 128, false);
            if (numericRole) {
                if (!input.roleId().matches("[1-9][0-9]*")) throw new IllegalArgumentException("miHoYo roleId must be a positive binding ID");
                try { positive(Long.parseLong(input.roleId())); }
                catch (NumberFormatException e) { throw new IllegalArgumentException("invalid miHoYo binding ID"); }
            }
            result.put("roleId", input.roleId());
        }
        if (input.sections().size() > 16) throw new IllegalArgumentException("at most 16 sections are allowed");
        if (!input.sections().isEmpty()) {
            for (String section : input.sections()) {
                text(section, 64, false);
                if (section.contains(",")) throw new IllegalArgumentException("section cannot contain a comma");
            }
            result.put("sections", String.join(",", input.sections()));
        }
        if (input.cursor() != null) result.put("cursor", text(input.cursor(), 2048, true));
        if (input.limit() != null) result.put("limit", bounded(input.limit(), 100));
        if (input.locale() != null) {
            if (!Set.of("zh_CN", "zh_TW", "en", "ja").contains(input.locale())) throw new IllegalArgumentException("invalid locale");
            result.put("locale", input.locale());
        }
        return result;
    }
    private static String bounded(int value, int max) {
        if (value < 1 || value > max) throw new IllegalArgumentException("pagination value is out of range");
        return Integer.toString(value);
    }
    private static Map<String, String> assetQuery(AssetQuery input) {
        Objects.requireNonNull(input);
        Map<String, String> result = new LinkedHashMap<>();
        result.put("provider", text(input.provider(), 96, false));
        if (input.locale() != null) result.put("locale", text(input.locale(), 16, true));
        if (input.version() != null) result.put("version", text(input.version(), 64, true));
        if (input.search() != null) result.put("q", text(input.search(), 128, true));
        if (input.kind() != null) {
            if (!Set.of("character", "item", "weapon", "equipment", "quest", "achievement", "other").contains(input.kind())) {
                throw new IllegalArgumentException("invalid entity kind");
            }
            result.put("kind", input.kind());
        }
        if (input.page() != null) result.put("page", bounded(input.page(), 20000));
        if (input.limit() != null) result.put("limit", bounded(input.limit(), 100));
        return result;
    }
    private static Map<String, Object> assetVariant(AssetVariant input) {
        Objects.requireNonNull(input);
        Map<String, Object> result = new LinkedHashMap<>();
        if (input.locale() != null) result.put("locale", text(input.locale(), 16, true));
        if (input.version() != null) result.put("version", text(input.version(), 64, true));
        return result;
    }
    private static Map<String, String> archiveQuery(ArchiveQuery input) {
        Objects.requireNonNull(input);
        Map<String, String> result = new LinkedHashMap<>();
        if (input.before() != null) result.put("before", unsigned(input.before()));
        if (input.limit() != null) result.put("limit", bounded(input.limit(), 100));
        return result;
    }

    private static final class Transport {
        private final String base;
        private final String header;
        private final String credential;
        private final Options options;
        private final HttpClient http;
        Transport(String base, String header, String credential, Options options) {
            text(base, 4096, false);
            URI uri;
            try { uri = URI.create(base); }
            catch (IllegalArgumentException e) { throw new IllegalArgumentException("invalid base URL"); }
            if (!("https".equalsIgnoreCase(uri.getScheme()) || "http".equalsIgnoreCase(uri.getScheme()))
                    || uri.getHost() == null || uri.getRawUserInfo() != null || uri.getRawQuery() != null
                    || uri.getRawFragment() != null || !uri.normalize().equals(uri)) {
                throw new IllegalArgumentException("base must be an HTTP(S) URL without credentials, query, fragment or dot segments");
            }
            this.base = base.replaceAll("/+$", ""); this.header = header; this.credential = credential;
            this.options = Objects.requireNonNull(options);
            this.http = HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NEVER)
                    .connectTimeout(options.timeout()).build();
        }
        <T> T get(String path, Map<String, String> query, Class<T> type) throws NoWorkException {
            return decoded(call("GET", path, query, null, false), type);
        }
        <T> List<T> list(String path, Map<String, String> query, Class<T> type) throws NoWorkException {
            Object raw = call("GET", path, query, null, false);
            try { return GameDataModels.decodeList(raw, type); }
            catch (IllegalArgumentException e) { throw new NoWorkException.Transport("invalid game API payload"); }
        }
        <T> T mutate(String method, String path, Map<String, ?> body, Class<T> type) throws NoWorkException {
            return decoded(call(method, path, Map.of(), body, false), type);
        }
        <T> T decoded(Object raw, Class<T> type) throws NoWorkException.Transport {
            try { return GameDataModels.decode(raw, type); }
            catch (IllegalArgumentException e) { throw new NoWorkException.Transport("invalid game API payload"); }
        }
        Object call(String method, String path, Map<String, String> query, Map<String, ?> body, boolean emptyAllowed)
                throws NoWorkException {
            if (Thread.currentThread().isInterrupted()) throw new NoWorkException.Transport("game request interrupted");
            StringBuilder url = new StringBuilder(base).append(path);
            for (var entry : query.entrySet()) url.append(url.indexOf("?") < 0 ? '?' : '&')
                    .append(encode(entry.getKey())).append('=').append(encode(entry.getValue()));
            HttpRequest.Builder request = HttpRequest.newBuilder(URI.create(url.toString()))
                    .timeout(options.timeout()).header("Accept", "application/json");
            if (header != null) request.header(header, credential);
            if (body == null) request.method(method, HttpRequest.BodyPublishers.noBody());
            else request.header("Content-Type", "application/json")
                    .method(method, HttpRequest.BodyPublishers.ofString(jsonBody(body), StandardCharsets.UTF_8));
            CompletableFuture<HttpResponse<byte[]>> pending = http.sendAsync(request.build(),
                    ignored -> new LimitedBody(options.maxResponseBytes()));
            HttpResponse<byte[]> response;
            try { response = pending.get(options.timeout().toNanos(), TimeUnit.NANOSECONDS); }
            catch (InterruptedException e) {
                pending.cancel(true); Thread.currentThread().interrupt();
                throw new NoWorkException.Transport("game request interrupted");
            } catch (TimeoutException e) {
                pending.cancel(true); throw new NoWorkException.Transport("game request timed out");
            } catch (ExecutionException e) {
                if (e.getCause() instanceof HttpTimeoutException) throw new NoWorkException.Transport("game request timed out");
                throw new NoWorkException.Transport("game request failed or response exceeded the configured bound");
            }
            if (response.statusCode() >= 300 && response.statusCode() < 400) {
                throw new NoWorkException.Transport("game API redirects are not permitted");
            }
            if (response.headers().firstValue(NoworkClient.HEADER_ENCRYPTION).isPresent()) {
                throw new NoWorkException.Encryption(path, "game client requires a readable response");
            }
            Map<String, Object> envelope;
            try {
                String json = StandardCharsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT)
                        .onUnmappableCharacter(CodingErrorAction.REPORT).decode(ByteBuffer.wrap(response.body())).toString();
                envelope = GameDataModels.object(GameDataModels.parse(json));
            } catch (IllegalArgumentException | CharacterCodingException e) {
                throw new NoWorkException.Transport("invalid game API JSON envelope");
            }
            int code;
            try {
                if (!(envelope.get("code") instanceof BigInteger value)) throw new IllegalArgumentException();
                code = value.intValueExact();
            } catch (IllegalArgumentException | ArithmeticException e) {
                throw new NoWorkException.Transport("invalid game API envelope code");
            }
            Object data = envelope.get("data");
            if (code != 0) {
                String message = envelope.get("message") instanceof String value ? value : "";
                throw new ApiException(code, message, response.statusCode(), data,
                        response.headers().firstValue("Retry-After").orElse(null));
            }
            if (response.statusCode() < 200 || response.statusCode() >= 300 || !emptyAllowed && data == null) {
                throw new NoWorkException.Transport("invalid game API success response");
            }
            if (data instanceof Map<?, ?> value && value.size() == 1 && value.get("jwe") instanceof String) {
                throw new NoWorkException.Encryption(path, "game client requires a readable response");
            }
            return data;
        }
    }

    /** Mutation bodies contain only the typed allowlisted scalar fields above. */
    private static String jsonBody(Map<String, ?> body) {
        List<String> fields = new ArrayList<>();
        for (var entry : body.entrySet()) {
            Object value = entry.getValue();
            String encoded;
            if (value instanceof String string) encoded = Json.quote(string);
            else if (value instanceof Boolean || value instanceof BigInteger) encoded = value.toString();
            else throw new IllegalArgumentException("invalid mutation field");
            fields.add(Json.quote(entry.getKey()) + ":" + encoded);
        }
        return "{" + String.join(",", fields) + "}";
    }

    /** Enforces the byte cap during download, including chunked bodies. */
    private static final class LimitedBody implements HttpResponse.BodySubscriber<byte[]> {
        private final int limit;
        private final ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        private final CompletableFuture<byte[]> result = new CompletableFuture<>();
        private Flow.Subscription subscription;
        LimitedBody(int limit) { this.limit = limit; }
        @Override public CompletionStage<byte[]> getBody() { return result; }
        @Override public void onSubscribe(Flow.Subscription value) {
            if (subscription != null) { value.cancel(); return; }
            subscription = value; subscription.request(1);
        }
        @Override public void onNext(List<ByteBuffer> buffers) {
            if (result.isDone()) return;
            for (ByteBuffer buffer : buffers) {
                int count = buffer.remaining();
                if (count > limit - bytes.size()) {
                    subscription.cancel(); result.completeExceptionally(new IllegalArgumentException("response byte limit exceeded")); return;
                }
                byte[] chunk = new byte[count]; buffer.get(chunk); bytes.writeBytes(chunk);
            }
            subscription.request(1);
        }
        @Override public void onError(Throwable error) { result.completeExceptionally(error); }
        @Override public void onComplete() { result.complete(bytes.toByteArray()); }
    }
}
