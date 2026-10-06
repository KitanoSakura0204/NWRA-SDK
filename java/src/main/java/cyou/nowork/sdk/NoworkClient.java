package cyou.nowork.sdk;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * NoWork Account Server API client (whitelist sync / join check / heartbeat).
 * Zero dependencies: uses java.net.http and a tiny built-in JSON parser.
 *
 * <pre>{@code
 * NoworkClient client = new NoworkClient("https://account.example.com", System.getenv("NOWORK_SERVER_KEY"));
 * CheckResult r = client.checkWhitelist(player.getName(), player.getUniqueId().toString());
 * if (!r.allowed) event.disallow(Result.KICK_WHITELIST, "未绑定账号: " + r.reason);
 * }</pre>
 */
public class NoworkClient {
    /** Marks a Server API reply whose data is the {"jwe": "..."} envelope (DELIVERY.md section 5). */
    public static final String HEADER_ENCRYPTION = "X-NoWork-Encryption";

    private final String base;
    private final String serverKey;
    private final HttpClient http;
    private final Duration timeout;

    public NoworkClient(String base, String serverKey) {
        this(base, serverKey, Duration.ofSeconds(10));
    }

    public NoworkClient(String base, String serverKey, Duration timeout) {
        this.base = base.endsWith("/") ? base.substring(0, base.length() - 1) : base;
        this.serverKey = Objects.requireNonNull(serverKey, "serverKey");
        this.timeout = timeout;
        this.http = HttpClient.newBuilder().connectTimeout(timeout).build();
    }

    /** Full whitelist for this server. */
    public Whitelist whitelist() throws NoWorkException {
        Map<String, Object> data = call("GET", "/server/v1/whitelist", null);
        Whitelist w = new Whitelist();
        w.server = Json.str(data.get("server"));
        w.mode = Json.num(data.get("mode")).intValue();
        w.count = Json.num(data.get("count")).intValue();
        w.entries = new ArrayList<>();
        Object list = data.get("entries");
        if (list instanceof List<?>) {
            for (Object o : (List<?>) list) {
                if (o instanceof Map<?, ?>) {
                    w.entries.add(WhitelistEntry.from((Map<?, ?>) o));
                }
            }
        }
        return w;
    }

    /** Join check by name and/or uuid (either may be null). */
    public CheckResult checkWhitelist(String name, String uuid) throws NoWorkException {
        StringBuilder q = new StringBuilder();
        if (name != null && !name.isEmpty()) q.append("name=").append(enc(name));
        if (uuid != null && !uuid.isEmpty()) q.append(q.length() > 0 ? "&" : "").append("uuid=").append(enc(uuid));
        Map<String, Object> data = call("GET", "/server/v1/whitelist/check?" + q, null);
        CheckResult r = new CheckResult();
        r.allowed = Boolean.TRUE.equals(data.get("allowed"));
        r.bound = Boolean.TRUE.equals(data.get("bound"));
        r.reason = Json.str(data.get("reason"));
        r.message = Json.str(data.get("message"));
        Object p = data.get("player");
        if (p instanceof Map<?, ?>) r.player = WhitelistEntry.from((Map<?, ?>) p);
        return r;
    }

    /** Report live status; call about every 60 seconds. */
    public void heartbeat(int online, int max, String version) throws NoWorkException {
        heartbeat(online, max, version, Double.NaN, null);
    }

    /**
     * Report live status with the optional TPS and player list (PROTOCOL 2.3). A tps that is not a
     * finite number above 0 is left out (pass Double.NaN for "unknown"); players may be null.
     */
    public void heartbeat(int online, int max, String version, double tps, List<OnlinePlayer> players) throws NoWorkException {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("online", online);
        body.put("max", max);
        body.put("version", version == null ? "" : version);
        if (Double.isFinite(tps) && tps > 0) body.put("tps", tps);
        List<Object> listed = new ArrayList<>();
        if (players != null) for (OnlinePlayer p : players) if (p != null) listed.add(p.toMap());
        if (!listed.isEmpty()) body.put("players", listed);
        call("POST", "/server/v1/heartbeat", encode(body));
    }

    // ---- Feature modules / GM channel (PROTOCOL 2.4) ----

    /** Modules enabled for this server, keyed by module name, with their per-server config. */
    @SuppressWarnings("unchecked")
    public Map<String, Object> modules() throws NoWorkException {
        Object m = call("GET", "/server/v1/modules", null).get("modules");
        return m instanceof Map<?, ?> ? (Map<String, Object>) m : Map.of();
    }

    /**
     * Replace the GM action catalogue shown in the admin panel. Each action is a {@link GMAction} or
     * a map with {@code key}, {@code title}, optional {@code description} and {@code params} (list of
     * maps with {@code name}, {@code type}, {@code required}, {@code label}). Returns the stored count.
     */
    public int publishGMActions(List<?> actions) throws NoWorkException {
        return Json.num(call("PUT", "/server/v1/gm/actions", encode(actions)).get("count")).intValue();
    }

    /**
     * Replace the actions and the named option lists (select / suggestion sources such as worlds or
     * kits) in one call; {@code null} actions or options keep what is stored (an empty list clears).
     * Entries the server cannot accept are skipped and listed in the result, never a failure for the
     * whole call.
     */
    public GMPublishResult publishGMCatalog(List<?> actions, Map<String, ? extends List<?>> options) throws NoWorkException {
        Map<String, Object> body = new LinkedHashMap<>();
        if (actions != null) body.put("actions", actions);
        if (options != null) body.put("options", options);
        return GMPublishResult.from(call("PUT", "/server/v1/gm/actions", encode(body)));
    }

    /** Replace only the named option lists (the published actions stay), e.g. when a world loads. */
    public GMPublishResult publishGMOptions(Map<String, ? extends List<?>> options) throws NoWorkException {
        return GMPublishResult.from(call("PUT", "/server/v1/gm/options", encode(options == null ? Map.of() : options)));
    }

    /** Queued GM commands (each becomes "sent"); poll alongside the heartbeat. */
    @SuppressWarnings("unchecked")
    public List<Map<String, Object>> pollGMCommands() throws NoWorkException {
        Object data = callRaw("GET", "/server/v1/gm/commands", null);
        List<Map<String, Object>> out = new ArrayList<>();
        if (data instanceof List<?>) {
            for (Object o : (List<?>) data) if (o instanceof Map<?, ?>) out.add((Map<String, Object>) o);
        }
        return out;
    }

    /** Report how a command went; result is free text (at most 4000 characters). */
    public void reportGMResult(long commandId, boolean ok, String result) throws NoWorkException {
        String body = "{\"ok\":" + ok + ",\"result\":" + Json.quote(result == null ? "" : result) + "}";
        call("POST", "/server/v1/gm/commands/" + commandId + "/result", body);
    }

    // ---- Player data and per-server bans (SERVER_USERS.md) ----

    /** The data schema the server owner defined (needs the module on for this server). */
    public ServerUsers.UserSchema userSchema() throws NoWorkException {
        return ServerUsers.UserSchema.from(call("GET", "/server/v1/users/schema", null));
    }

    /** One account's data (uid from the join check or the whitelist). */
    public ServerUsers.UserData userData(long uid) throws NoWorkException {
        return ServerUsers.UserData.from(call("GET", userPath(uid), null));
    }

    /**
     * Replace the account's stored values (fields left out are unset). version is the one you last
     * read, or -1 to write unconditionally; a stale version throws Api 409
     * ({@link ServerUsers#isVersionConflict}). Returns the data as stored.
     */
    public ServerUsers.UserData putUserData(long uid, Map<String, ?> data, long version) throws NoWorkException {
        return ServerUsers.UserData.from(call("PUT", userPath(uid), writeBody(data, version)));
    }

    /** Merge values into the stored ones; a null value deletes that field. version as for putUserData. */
    public ServerUsers.UserData patchUserData(long uid, Map<String, ?> data, long version) throws NoWorkException {
        return ServerUsers.UserData.from(call("PATCH", userPath(uid), writeBody(data, version)));
    }

    /**
     * Active bans on this server: all (only == null; at most 5000, newest first) or those of the
     * matching accounts. Works with the module off too.
     */
    public List<ServerUsers.ServerBan> bans(ServerUsers.Target only) throws NoWorkException {
        StringBuilder q = new StringBuilder();
        if (only != null) {
            for (Map.Entry<String, Object> e : only.toMap().entrySet()) {
                q.append(q.length() == 0 ? "?" : "&").append(e.getKey()).append('=').append(enc(String.valueOf(e.getValue())));
            }
        }
        List<ServerUsers.ServerBan> out = new ArrayList<>();
        Object list = call("GET", "/server/v1/bans" + q, null).get("bans");
        if (list instanceof List<?>) for (Object o : (List<?>) list) if (o instanceof Map<?, ?>) out.add(ServerUsers.ServerBan.from((Map<?, ?>) o));
        return out;
    }

    /** Online players (as of the last heartbeat) whose account is banned here: disconnect them. */
    public List<ServerUsers.KickEntry> kickList() throws NoWorkException {
        List<ServerUsers.KickEntry> out = new ArrayList<>();
        Object list = call("GET", "/server/v1/bans/kick", null).get("players");
        if (list instanceof List<?>) for (Object o : (List<?>) list) if (o instanceof Map<?, ?>) out.add(ServerUsers.KickEntry.from((Map<?, ?>) o));
        return out;
    }

    /**
     * Ban an account on this server (module on, pluginBans allowed). Give expireTime
     * ("2006-01-02 15:04:05" or RFC 3339) or durationMinutes &gt; 0; neither (null / 0) = permanent.
     * A ban an administrator placed cannot be replaced (Api 403). Returns the ban now in force.
     */
    public ServerUsers.ServerBan ban(ServerUsers.Target target, String reason, String expireTime, long durationMinutes) throws NoWorkException {
        Map<String, Object> body = target.toMap();
        putText(body, "reason", reason);
        putText(body, "expireTime", expireTime);
        if (durationMinutes > 0) body.put("durationMinutes", durationMinutes);
        Object b = call("POST", "/server/v1/bans", encode(body)).get("ban");
        return b instanceof Map<?, ?> ? ServerUsers.ServerBan.from((Map<?, ?>) b) : null;
    }

    /** Lift the account's active ban on this server (not one an administrator placed); returns the uid. */
    public long unban(ServerUsers.Target target, String reason) throws NoWorkException {
        Map<String, Object> body = target.toMap();
        putText(body, "reason", reason);
        return Json.num(call("POST", "/server/v1/bans/lift", encode(body)).get("uid")).longValue();
    }

    private static String userPath(long uid) {
        if (uid <= 0) throw new IllegalArgumentException("nowork: uid must be positive, got " + uid);
        return "/server/v1/users/" + uid + "/data";
    }

    private static String writeBody(Map<String, ?> data, long version) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("data", data == null ? Map.of() : data); // the server wants a data object, even an empty one
        if (version >= 0) body.put("version", version);
        return encode(body);
    }

    /**
     * Minimal JSON encoder for maps / lists / strings / numbers / booleans and the typed models
     * ({@link Wire}); NaN and infinities, which JSON cannot carry, become null.
     */
    static String encode(Object v) {
        if (v == null) return "null";
        if (v instanceof Wire) return encode(((Wire) v).toMap());
        if (v instanceof String) return Json.quote((String) v);
        if (v instanceof Double || v instanceof Float) {
            double d = ((Number) v).doubleValue();
            return Double.isFinite(d) ? String.valueOf(v) : "null";
        }
        if (v instanceof Number || v instanceof Boolean) return String.valueOf(v);
        if (v instanceof Map<?, ?>) {
            StringBuilder b = new StringBuilder("{");
            boolean first = true;
            for (Map.Entry<?, ?> e : ((Map<?, ?>) v).entrySet()) {
                if (!first) b.append(',');
                first = false;
                b.append(Json.quote(String.valueOf(e.getKey()))).append(':').append(encode(e.getValue()));
            }
            return b.append('}').toString();
        }
        if (v instanceof Iterable<?>) {
            StringBuilder b = new StringBuilder("[");
            boolean first = true;
            for (Object o : (Iterable<?>) v) {
                if (!first) b.append(',');
                first = false;
                b.append(encode(o));
            }
            return b.append(']').toString();
        }
        return Json.quote(String.valueOf(v));
    }

    @SuppressWarnings("unchecked")
    private Map<String, Object> call(String method, String path, String body) throws NoWorkException {
        Object data = callRaw(method, path, body);
        return data instanceof Map<?, ?> ? (Map<String, Object>) data : Map.of();
    }

    /** Decoded data; an encrypted reply throws {@link NoWorkException.Encryption} instead (fail closed). */
    private Object callRaw(String method, String path, String body) throws NoWorkException {
        RawResponse res = raw(method, path, body);
        if (res.encrypted) throw new NoWorkException.Encryption(route(path), "no decryption key configured");
        if (res.data == null) throw new NoWorkException.Transport("invalid response (" + res.status + "): missing data");
        return res.data;
    }

    /**
     * Call a Server API route (e.g. "/server/v1/whitelist", query included) and return the reply
     * without decoding data, e.g. to hand {@code body} to NoWorkDelivery.openServerData. Transport
     * failures and business errors (code != 0) still throw.
     */
    @SuppressWarnings("unchecked")
    public RawResponse raw(String method, String path, String body) throws NoWorkException {
        HttpRequest.Builder b = HttpRequest.newBuilder(URI.create(base + path))
            .timeout(timeout)
            .header("Accept", "application/json")
            .header("X-Server-Key", serverKey);
        if (body != null) {
            b.header("Content-Type", "application/json").method(method, HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8));
        } else {
            b.method(method, HttpRequest.BodyPublishers.noBody());
        }
        HttpResponse<String> res;
        try {
            res = http.send(b.build(), HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) Thread.currentThread().interrupt();
            throw new NoWorkException.Transport(e);
        }
        Object parsed;
        try {
            parsed = Json.parse(res.body());
        } catch (RuntimeException e) { // an HTML error page or a cut-off body must not escape unchecked
            throw new NoWorkException.Transport("invalid json (" + res.statusCode() + ")");
        }
        if (!(parsed instanceof Map<?, ?>)) throw new NoWorkException.Transport("invalid response (" + res.statusCode() + ")");
        Map<String, Object> env = (Map<String, Object>) parsed;
        if (!(env.get("code") instanceof Number)) {
            throw new NoWorkException.Transport("invalid response (" + res.statusCode() + "): missing code");
        }
        int code = ((Number) env.get("code")).intValue();
        if (code != 0) throw new NoWorkException.Api(code, Json.str(env.get("message")));
        if (res.statusCode() >= 400) throw new NoWorkException.Transport("http " + res.statusCode());
        RawResponse out = new RawResponse();
        out.status = res.statusCode();
        out.headers = res.headers().map();
        out.message = Json.str(env.get("message"));
        out.data = env.get("data");
        out.body = res.body();
        out.encrypted = res.headers().firstValue(HEADER_ENCRYPTION).filter(v -> !v.isEmpty()).isPresent() || isJweData(out.data);
        return out;
    }

    /** "whitelist/check" for "/server/v1/whitelist/check?name=..." */
    private static String route(String path) {
        int q = path.indexOf('?');
        String p = q >= 0 ? path.substring(0, q) : path;
        return p.startsWith("/server/v1/") ? p.substring("/server/v1/".length()) : p;
    }

    private static boolean isJweData(Object data) {
        return data instanceof Map<?, ?> && ((Map<?, ?>) data).size() == 1 && ((Map<?, ?>) data).get("jwe") instanceof String;
    }

    private static String enc(String s) {
        return URLEncoder.encode(s, StandardCharsets.UTF_8);
    }

    // ---- models ----

    public static class WhitelistEntry {
        public String name;
        public String uuid;
        public long uid;
        public String nickname;
        public int securityLevel;
        public String since;

        static WhitelistEntry from(Map<?, ?> m) {
            WhitelistEntry e = new WhitelistEntry();
            e.name = Json.str(m.get("name"));
            e.uuid = Json.str(m.get("uuid"));
            e.uid = Json.num(m.get("uid")).longValue();
            e.nickname = Json.str(m.get("nickname"));
            e.securityLevel = Json.num(m.get("securityLevel")).intValue();
            e.since = Json.str(m.get("since"));
            return e;
        }
    }

    public static class Whitelist {
        public String server;
        public int mode;
        public int count;
        public List<WhitelistEntry> entries;
    }

    public static class CheckResult {
        public boolean allowed;
        public boolean bound;
        public String reason;
        /** Extra text for some refusals, e.g. the reason of a per-server ban (reason "banned"); "" when none. */
        public String message;
        public WhitelistEntry player;
    }

    /** A Server API reply as received, data left undecoded (see {@link NoworkClient#raw}). */
    public static class RawResponse {
        public int status;
        public Map<String, List<String>> headers;
        public String message;
        /** The envelope's data as parsed ({"jwe": "..."} when encrypted). */
        public Object data;
        /** The whole response body. */
        public String body;
        /** X-NoWork-Encryption is set or data is exactly {"jwe": "<string>"}. */
        public boolean encrypted;
    }

    /** A typed model that is sent as a JSON object by the client's encoder. */
    public interface Wire {
        Map<String, Object> toMap();
    }

    /** One entry of the heartbeat player list (name required, uuid optional). */
    public static class OnlinePlayer implements Wire {
        public String name;
        public String uuid;

        public OnlinePlayer(String name) {
            this(name, null);
        }

        public OnlinePlayer(String name, String uuid) {
            this.name = name;
            this.uuid = uuid;
        }

        @Override
        public Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("name", name == null ? "" : name);
            if (uuid != null && !uuid.isEmpty()) m.put("uuid", uuid);
            return m;
        }
    }

    /** One choice of a select parameter or of a named option list. */
    public static class GMOption implements Wire {
        public String value;
        public String label;

        public GMOption(String value) {
            this(value, null);
        }

        public GMOption(String value, String label) {
            this.value = value;
            this.label = label;
        }

        @Override
        public Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("value", value == null ? "" : value);
            if (label != null && !label.isEmpty()) m.put("label", label);
            return m;
        }
    }

    /**
     * One input of a GM action (PROTOCOL 2.4). Only name / type / required / label existed before
     * 2026-09-28; every other field is optional. type: string | text | number | boolean | select |
     * player (aliases int / integer / bool / textarea).
     */
    public static class GMActionParam implements Wire {
        public String name;
        public String type;
        public boolean required;
        public String label;
        public String description;
        public String placeholder;
        /** Sent as "default"; ignored by the server when it does not fit the definition. */
        public Object defaultValue;
        /** number only */
        public Double min, max, step;
        public boolean integer;
        /** Text length limit; 0 = server default (1000, text 4000, player 64). */
        public int maxLength;
        /** Inline select choices (1-200, unique values). */
        public List<GMOption> options = new ArrayList<>();
        /** Names an option list published with publishGMCatalog / publishGMOptions. */
        public String source;

        public GMActionParam(String name, String type) {
            this.name = name;
            this.type = type;
        }

        public GMActionParam required() { this.required = true; return this; }
        public GMActionParam label(String v) { this.label = v; return this; }
        public GMActionParam source(String v) { this.source = v; return this; }
        public GMActionParam defaultValue(Object v) { this.defaultValue = v; return this; }
        public GMActionParam range(double min, double max) { this.min = min; this.max = max; return this; }
        public GMActionParam option(String value, String label) { this.options.add(new GMOption(value, label)); return this; }

        @Override
        public Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("name", name == null ? "" : name);
            m.put("type", type == null || type.isEmpty() ? "string" : type);
            m.put("required", required);
            m.put("label", label == null ? "" : label);
            putText(m, "description", description);
            putText(m, "placeholder", placeholder);
            if (defaultValue != null) m.put("default", defaultValue);
            if (min != null) m.put("min", min);
            if (max != null) m.put("max", max);
            if (step != null) m.put("step", step);
            if (integer) m.put("integer", true);
            if (maxLength > 0) m.put("maxLength", maxLength);
            if (options != null && !options.isEmpty()) m.put("options", options);
            putText(m, "source", source);
            return m;
        }
    }

    /**
     * A GM action the plugin offers to the admin panel. key: no colon, no whitespace, at most 64
     * bytes. danger: normal | confirm | critical (the last two need a confirmation before sending).
     */
    public static class GMAction implements Wire {
        public String key;
        public String title;
        public String description;
        public List<GMActionParam> params = new ArrayList<>();
        public Map<String, String> labels = new LinkedHashMap<>();
        /** Catalogue group. */
        public String category;
        public String danger;

        public GMAction(String key, String title) {
            this.key = key;
            this.title = title;
        }

        public GMAction description(String v) { this.description = v; return this; }
        public GMAction param(GMActionParam p) { this.params.add(p); return this; }
        public GMAction category(String v) { this.category = v; return this; }
        public GMAction danger(String v) { this.danger = v; return this; }

        @Override
        public Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            m.put("key", key == null ? "" : key);
            m.put("title", title == null ? "" : title);
            putText(m, "description", description);
            if (params != null && !params.isEmpty()) m.put("params", params);
            if (labels != null && !labels.isEmpty()) m.put("labels", labels);
            putText(m, "category", category);
            putText(m, "danger", danger);
            return m;
        }
    }

    /** What a publish stored; entries the server could not accept are skipped and listed. */
    public static class GMPublishResult {
        public int count;
        /** Each entry has "key" and "reason". */
        public List<Map<String, String>> skipped = new ArrayList<>();

        static GMPublishResult from(Map<String, Object> data) {
            GMPublishResult r = new GMPublishResult();
            r.count = Json.num(data.get("count")).intValue();
            Object list = data.get("skipped");
            if (list instanceof List<?>) {
                for (Object o : (List<?>) list) {
                    if (!(o instanceof Map<?, ?>)) continue;
                    Map<String, String> s = new LinkedHashMap<>();
                    s.put("key", Json.str(((Map<?, ?>) o).get("key")));
                    s.put("reason", Json.str(((Map<?, ?>) o).get("reason")));
                    r.skipped.add(s);
                }
            }
            return r;
        }
    }

    private static void putText(Map<String, Object> m, String key, String v) {
        if (v != null && !v.isEmpty()) m.put(key, v);
    }
}
