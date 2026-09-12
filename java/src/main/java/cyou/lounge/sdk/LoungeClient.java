package cyou.lounge.sdk;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * Lounge Account Server API client (whitelist sync / join check / heartbeat).
 * Zero dependencies: uses java.net.http and a tiny built-in JSON parser.
 *
 * <pre>{@code
 * LoungeClient client = new LoungeClient("https://account.example.com", System.getenv("LOUNGE_SERVER_KEY"));
 * CheckResult r = client.checkWhitelist(player.getName(), player.getUniqueId().toString());
 * if (!r.allowed) event.disallow(Result.KICK_WHITELIST, "未绑定账号: " + r.reason);
 * }</pre>
 */
public class LoungeClient {
    private final String base;
    private final String serverKey;
    private final HttpClient http;
    private final Duration timeout;

    public LoungeClient(String base, String serverKey) {
        this(base, serverKey, Duration.ofSeconds(10));
    }

    public LoungeClient(String base, String serverKey, Duration timeout) {
        this.base = base.endsWith("/") ? base.substring(0, base.length() - 1) : base;
        this.serverKey = Objects.requireNonNull(serverKey, "serverKey");
        this.timeout = timeout;
        this.http = HttpClient.newBuilder().connectTimeout(timeout).build();
    }

    /** Full whitelist for this server. */
    public Whitelist whitelist() throws LoungeException {
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
    public CheckResult checkWhitelist(String name, String uuid) throws LoungeException {
        StringBuilder q = new StringBuilder();
        if (name != null && !name.isEmpty()) q.append("name=").append(enc(name));
        if (uuid != null && !uuid.isEmpty()) q.append(q.length() > 0 ? "&" : "").append("uuid=").append(enc(uuid));
        Map<String, Object> data = call("GET", "/server/v1/whitelist/check?" + q, null);
        CheckResult r = new CheckResult();
        r.allowed = Boolean.TRUE.equals(data.get("allowed"));
        r.bound = Boolean.TRUE.equals(data.get("bound"));
        r.reason = Json.str(data.get("reason"));
        Object p = data.get("player");
        if (p instanceof Map<?, ?>) r.player = WhitelistEntry.from((Map<?, ?>) p);
        return r;
    }

    /** Report live status; call about every 60 seconds. */
    public void heartbeat(int online, int max, String version) throws LoungeException {
        String body = "{\"online\":" + online + ",\"max\":" + max + ",\"version\":" + Json.quote(version == null ? "" : version) + "}";
        call("POST", "/server/v1/heartbeat", body);
    }

    @SuppressWarnings("unchecked")
    private Map<String, Object> call(String method, String path, String body) throws LoungeException {
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
            throw new LoungeException.Transport(e);
        }
        Object parsed = Json.parse(res.body());
        if (!(parsed instanceof Map<?, ?>)) throw new LoungeException.Transport("invalid response (" + res.statusCode() + ")");
        Map<String, Object> env = (Map<String, Object>) parsed;
        int code = Json.num(env.get("code")).intValue();
        if (code != 0) throw new LoungeException.Api(code, Json.str(env.get("message")));
        Object data = env.get("data");
        return data instanceof Map<?, ?> ? (Map<String, Object>) data : Map.of();
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
        public WhitelistEntry player;
    }
}
