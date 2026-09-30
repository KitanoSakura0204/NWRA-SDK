package cyou.lounge.sdk;

import com.sun.net.httpserver.HttpServer;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;

/**
 * Dependency-free checks of the Server API client against an in-process HTTP server (the repo has
 * no JUnit). Run from SDK/java:
 * javac -d out src/main/java/cyou/lounge/sdk/*.java src/test/java/cyou/lounge/sdk/SelfCheck.java
 * java -cp out cyou.lounge.sdk.SelfCheck
 */
public final class SelfCheck {
    private static volatile String reply = "";
    private static volatile String header = null;
    private static final List<String> bodies = Collections.synchronizedList(new ArrayList<>());
    private static final List<String> requests = Collections.synchronizedList(new ArrayList<>());

    interface Call {
        void run() throws Exception;
    }

    public static void main(String[] args) throws Exception {
        HttpServer server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.createContext("/", ex -> {
            bodies.add(new String(ex.getRequestBody().readAllBytes(), StandardCharsets.UTF_8));
            requests.add(ex.getRequestMethod() + " " + ex.getRequestURI());
            if (header != null) ex.getResponseHeaders().set(LoungeClient.HEADER_ENCRYPTION, header);
            byte[] out = reply.getBytes(StandardCharsets.UTF_8);
            ex.sendResponseHeaders(200, out.length);
            try (OutputStream os = ex.getResponseBody()) {
                os.write(out);
            }
        });
        server.start();
        try {
            run(new LoungeClient("http://127.0.0.1:" + server.getAddress().getPort(), "k"));
        } finally {
            server.stop(0);
        }
        System.out.println("SelfCheck: all checks passed");
    }

    private static void run(LoungeClient c) throws Exception {
        // Encrypted replies fail closed, with and without the header (a proxy may drop it).
        String sealed = "{\"code\":0,\"message\":\"ok\",\"data\":{\"jwe\":\"aGVhZA.a2V5.aXY.Y2lwaGVy.dGFn\"}}";
        for (String h : new String[] {"JWE", null}) {
            reply = sealed;
            header = h;
            expectEncryption("whitelist", c::whitelist);
            expectEncryption("whitelist/check", () -> c.checkWhitelist("Steve", null));
            expectEncryption("gm/commands", c::pollGMCommands);
            LoungeClient.RawResponse raw = c.raw("GET", "/server/v1/whitelist", null);
            check(raw.encrypted && raw.body.equals(sealed) && raw.data instanceof Map, "raw keeps the envelope");
        }
        header = null;
        reply = "{\"code\":0,\"data\":{\"jwe\":\"x\",\"mode\":1,\"count\":0,\"entries\":[]}}";
        check(c.whitelist().mode == 1, "plain data with a jwe field is not encrypted");
        // Unreadable replies are Transport errors, never an empty (mode 0) whitelist.
        for (String bad : new String[] {"<html>bad gateway</html>", "{\"code\":0}", "{\"data\":{\"mode\":1}}", "{\"code\":0,\"data\":{\"mode\""}) {
            reply = bad;
            try {
                c.whitelist();
                throw new AssertionError("expected Transport for " + bad);
            } catch (LoungeException.Transport expected) {
                // ok
            }
        }

        // Heartbeat: tps only when finite and > 0, players only when non-empty.
        reply = "{\"code\":0,\"data\":{\"status\":\"ok\"}}";
        bodies.clear();
        c.heartbeat(2, 20, "1.21.1");
        c.heartbeat(2, 20, "1.21.1", 19.8, List.of(new LoungeClient.OnlinePlayer("Steve", "069a79f4-44e9-4726-a5be-fca90e38aaf5"), new LoungeClient.OnlinePlayer("Alex")));
        c.heartbeat(0, 0, null, Double.NaN, List.of());
        c.heartbeat(0, 0, "", Double.POSITIVE_INFINITY, null);
        check(bodies.equals(List.of(
            "{\"online\":2,\"max\":20,\"version\":\"1.21.1\"}",
            "{\"online\":2,\"max\":20,\"version\":\"1.21.1\",\"tps\":19.8,\"players\":[{\"name\":\"Steve\",\"uuid\":\"069a79f4-44e9-4726-a5be-fca90e38aaf5\"},{\"name\":\"Alex\"}]}",
            "{\"online\":0,\"max\":0,\"version\":\"\"}",
            "{\"online\":0,\"max\":0,\"version\":\"\"}")), "heartbeat bodies: " + bodies);

        // GM catalogue: object form with typed actions and named lists; skipped entries are listed.
        reply = "{\"code\":0,\"data\":{\"count\":1,\"skipped\":[{\"key\":\"bad key\",\"reason\":\"invalid key\"}]}}";
        bodies.clear();
        LoungeClient.GMAction kick = new LoungeClient.GMAction("kick", "Kick").category("players").danger("confirm")
            .param(new LoungeClient.GMActionParam("player", "player").required())
            .param(new LoungeClient.GMActionParam("world", "select").source("worlds").defaultValue("overworld"))
            .param(new LoungeClient.GMActionParam("amount", "number").range(0, 64));
        LoungeClient.GMPublishResult res = c.publishGMCatalog(List.of(kick), Map.of("worlds", List.of(new LoungeClient.GMOption("overworld", "Overworld"))));
        check(res.count == 1 && res.skipped.size() == 1 && res.skipped.get(0).get("key").equals("bad key"), "publish result");
        check(bodies.get(0).equals("{\"actions\":[{\"key\":\"kick\",\"title\":\"Kick\",\"params\":["
            + "{\"name\":\"player\",\"type\":\"player\",\"required\":true,\"label\":\"\"},"
            + "{\"name\":\"world\",\"type\":\"select\",\"required\":false,\"label\":\"\",\"default\":\"overworld\",\"source\":\"worlds\"},"
            + "{\"name\":\"amount\",\"type\":\"number\",\"required\":false,\"label\":\"\",\"min\":0.0,\"max\":64.0}],"
            + "\"category\":\"players\",\"danger\":\"confirm\"}],"
            + "\"options\":{\"worlds\":[{\"value\":\"overworld\",\"label\":\"Overworld\"}]}}"), "catalogue body: " + bodies.get(0));
        reply = "{\"code\":0,\"data\":{\"count\":1}}";
        check(c.publishGMOptions(Map.of("worlds", List.of(new LoungeClient.GMOption("overworld")))).skipped.isEmpty(), "options result");
        check(bodies.get(1).equals("{\"worlds\":[{\"value\":\"overworld\"}]}"), "options body: " + bodies.get(1));
        check(c.publishGMCatalog(null, null).count == 1 && bodies.get(2).equals("{}"), "null keeps both: " + bodies.get(2));

        serverUsers(c);
    }

    // Player data and per-server bans (SERVER_USERS.md): paths, bodies and decoding.
    private static void serverUsers(LoungeClient c) throws Exception {
        bodies.clear();
        requests.clear();
        reply = "{\"code\":0,\"data\":{\"fields\":[{\"key\":\"coins\",\"label\":\"Coins\",\"type\":\"number\",\"visible\":true,\"integer\":true,\"min\":0}],\"maxUserBytes\":8192}}";
        ServerUsers.UserSchema schema = c.userSchema();
        check(schema.maxUserBytes == 8192 && schema.fields.get(0).integer && schema.fields.get(0).min == 0.0, "schema");
        reply = "{\"code\":0,\"data\":{\"uid\":100001,\"data\":{\"coins\":12,\"rank\":null},\"version\":2,\"updatedAt\":\"2026-09-29 10:00:00\",\"size\":11}}";
        ServerUsers.UserData d = c.userData(100001);
        check(d.version == 2 && ((Number) d.data.get("coins")).intValue() == 12 && d.data.containsKey("rank") && "2026-09-29 10:00:00".equals(d.updatedAt), "user data");
        c.putUserData(100001, Map.of("coins", 1), 2);
        Map<String, Object> patch = new java.util.LinkedHashMap<>();
        patch.put("rank", null);
        c.patchUserData(100001, patch, -1);
        check(requests.subList(1, 4).equals(List.of("GET /server/v1/users/100001/data", "PUT /server/v1/users/100001/data", "PATCH /server/v1/users/100001/data")), "data paths: " + requests);
        check(bodies.get(2).equals("{\"data\":{\"coins\":1},\"version\":2}") && bodies.get(3).equals("{\"data\":{\"rank\":null}}"), "data bodies: " + bodies);
        reply = "{\"code\":409,\"message\":\"stale\"}";
        try {
            c.putUserData(100001, Map.of("coins", 1), 1);
            throw new AssertionError("expected a version conflict");
        } catch (LoungeException.Api e) {
            check(ServerUsers.isVersionConflict(e), "409 is a version conflict");
        }

        bodies.clear();
        requests.clear();
        String ban = "{\"uid\":100001,\"names\":[\"Steve\"],\"uuids\":[],\"reason\":\"x-ray\",\"expireTime\":null,\"createTime\":\"2026-09-29 10:00:00\",\"source\":\"plugin\"}";
        reply = "{\"code\":0,\"data\":{\"count\":1,\"bans\":[" + ban + "],\"players\":[{\"name\":\"Steve\",\"uid\":100001,\"reason\":\"x-ray\",\"expireTime\":null}],\"uid\":100001,\"ban\":" + ban + "}}";
        List<ServerUsers.ServerBan> bans = c.bans(ServerUsers.Target.name("Steve Two"));
        check(bans.size() == 1 && bans.get(0).expireTime == null && bans.get(0).names.get(0).equals("Steve"), "bans");
        check(c.kickList().get(0).uid == 100001, "kick list");
        check(c.ban(ServerUsers.Target.name("Steve"), "x-ray", null, 1440).reason.equals("x-ray"), "ban");
        check(c.unban(ServerUsers.Target.uid(100001), "appeal") == 100001, "unban");
        check(requests.equals(List.of("GET /server/v1/bans?name=Steve+Two", "GET /server/v1/bans/kick", "POST /server/v1/bans", "POST /server/v1/bans/lift")), "ban paths: " + requests);
        check(bodies.get(2).equals("{\"name\":\"Steve\",\"reason\":\"x-ray\",\"durationMinutes\":1440}") && bodies.get(3).equals("{\"uid\":100001,\"reason\":\"appeal\"}"), "ban bodies: " + bodies);

        reply = "{\"code\":0,\"data\":{\"allowed\":false,\"bound\":true,\"reason\":\"banned\",\"message\":\"x-ray\",\"player\":null}}";
        LoungeClient.CheckResult r = c.checkWhitelist("Steve", null);
        check(!r.allowed && r.reason.equals("banned") && r.message.equals("x-ray"), "ban message on the join check");
    }

    private static void expectEncryption(String route, Call call) throws Exception {
        try {
            call.run();
        } catch (LoungeException.Encryption e) {
            check(e.route.equals(route), "route " + e.route + " != " + route);
            return;
        }
        throw new AssertionError("expected Encryption on " + route);
    }

    private static void check(boolean ok, String what) {
        if (!ok) throw new AssertionError(what);
    }
}
