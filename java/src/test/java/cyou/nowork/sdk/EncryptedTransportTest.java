package cyou.nowork.sdk;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.SecureRandom;
import java.security.spec.MGF1ParameterSpec;
import java.security.spec.RSAPublicKeySpec;
import java.time.Duration;
import java.util.Arrays;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import javax.crypto.Cipher;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.OAEPParameterSpec;
import javax.crypto.spec.PSource;
import javax.crypto.spec.SecretKeySpec;

/**
 * Offline Server API transport contracts. Uses the backend's committed delivery vectors and a
 * loopback HTTP fixture; no provider, credentials or Paper runtime. Run from SDK/java, optionally
 * passing the path to SDK/go/testdata/vectors.json as the first argument. No JUnit dependency.
 */
public final class EncryptedTransportTest implements AutoCloseable {
    private static final String SERVER = "survival";
    private static final String CHECK = "{\"allowed\":true,\"bound\":true,\"reason\":\"\",\"player\":{\"name\":\"Steve Two\"}}";
    private static final String WHITELIST = "{\"server\":\"survival\",\"mode\":1,\"count\":0,\"entries\":[]}";
    private static final Duration TIMEOUT = Duration.ofSeconds(2);

    private final HttpServer http;
    private final String base;
    private final Map<String, Object> rsaJwk;
    private final NoWorkDelivery.PrivateJwk ec;
    private final NoWorkDelivery.PrivateJwk rsa;
    private final String sharedEnvelope;
    private final NoworkClient keyed;
    private final NoworkClient unkeyed;
    private volatile Reply reply = new Reply(200, "{\"code\":0,\"data\":{}}", null);
    private volatile String requestPath;
    private volatile String requestKey;
    private int checks;
    private boolean closed;

    private record Reply(int status, String body, String encryption) {}
    @FunctionalInterface private interface Call { void run() throws Exception; }

    private EncryptedTransportTest(Path vectors) throws IOException {
        Map<String, Object> set = object(GameDataModels.parse(Files.readString(vectors)));
        Map<String, Object> keys = object(set.get("keys"));
        rsaJwk = object(keys.get("rsa"));
        ec = NoWorkDelivery.PrivateJwk.parse(NoworkClient.encode(keys.get("ec")));
        rsa = NoWorkDelivery.PrivateJwk.parse(NoworkClient.encode(rsaJwk));
        sharedEnvelope = ((List<?>) set.get("vectors")).stream().map(EncryptedTransportTest::object)
                .filter(v -> "server-envelope".equals(v.get("name"))).map(v -> (String) v.get("response"))
                .findFirst().orElseThrow(() -> new AssertionError("missing backend server-envelope vector"));
        http = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        http.createContext("/", exchange -> {
            Reply current = reply;
            requestPath = exchange.getRequestURI().toASCIIString();
            requestKey = exchange.getRequestHeaders().getFirst("X-Server-Key");
            exchange.getRequestBody().readAllBytes();
            if (current.encryption != null) exchange.getResponseHeaders().set(NoworkClient.HEADER_ENCRYPTION, current.encryption);
            byte[] bytes = current.body.getBytes(StandardCharsets.UTF_8);
            exchange.sendResponseHeaders(current.status, bytes.length);
            try (var out = exchange.getResponseBody()) { out.write(bytes); }
            finally { exchange.close(); }
        });
        http.start();
        base = "http://127.0.0.1:" + http.getAddress().getPort();
        NoWorkDelivery.PrivateJwk[] rotationKeys = {rsa, ec};
        keyed = client(SERVER, rotationKeys);
        // Callers cannot later mutate the configured key array into a missing or different key set.
        Arrays.fill(rotationKeys, null);
        unkeyed = new NoworkClient(base, "test-server-key", TIMEOUT);
    }

    public static void main(String[] args) throws Exception {
        Path vectors = Path.of(args.length == 0 ? "../go/testdata/vectors.json" : args[0]);
        try (EncryptedTransportTest test = new EncryptedTransportTest(vectors)) {
            test.run();
            System.out.println("EncryptedTransportTest: " + test.checks + " checks passed");
        }
    }

    private NoworkClient client(String serverCode, NoWorkDelivery.PrivateJwk... keys) {
        return new NoworkClient(base, "test-server-key", TIMEOUT, serverCode, keys);
    }

    private void use(String body, String encryption) { reply = new Reply(200, body, encryption); }
    private static String plain(String data) { return "{\"code\":0,\"data\":" + data + "}"; }

    private void run() throws Exception {
        vectorsAndRotation();
        contextsAndMalformedData();
        downgradeAndRaw();
        errorPrecedence();
        configuration();
        close();
        expectTransport(() -> keyed.checkWhitelist("Steve", null));
    }

    private void vectorsAndRotation() throws Exception {
        for (String header : new String[] {"JWE", null}) {
            use(sharedEnvelope, header);
            NoworkClient.Whitelist whitelist = keyed.whitelist();
            check(whitelist.mode == 1 && whitelist.server.equals(SERVER)
                    && whitelist.entries.size() == 1 && whitelist.entries.get(0).name.equals("Steve"),
                    "backend EC server vector decodes with and without encryption header");
            expectEncryption("whitelist", unkeyed::whitelist);
            NoworkClient.RawResponse raw = keyed.raw("GET", "/server/v1/whitelist", null);
            check(raw.encrypted && raw.body.equals(sharedEnvelope) && object(raw.data).containsKey("jwe"),
                    "keyed raw keeps the original ciphertext and envelope");
        }
        String active = seal("server:survival:whitelist/check", CHECK, "vec-rsa");
        use(active, "JWE");
        NoworkClient.CheckResult result = keyed.checkWhitelist("Steve Two", "uuid-value");
        check(result.allowed && result.bound && result.player.name.equals("Steve Two"), "active RSA key joins during EC/RSA rotation");
        check(requestPath.equals("/server/v1/whitelist/check?name=Steve+Two&uuid=uuid-value")
                && requestKey.equals("test-server-key"), "query encoding and server-key authentication survive decryption");
        expectEncryption("whitelist/check", () -> client(SERVER, ec).checkWhitelist("Steve", null));
        check(client(SERVER, rsa).checkWhitelist("Steve", null).allowed, "new key alone opens new reply");
        use(sharedEnvelope, null);
        check(client(SERVER, ec).whitelist().mode == 1, "retiring key opens its existing reply");
        expectEncryption("whitelist", () -> client(SERVER, rsa).whitelist());
        use(seal("server:survival:whitelist", WHITELIST, "unknown-kid"), "JWE");
        expectEncryption("whitelist", keyed::whitelist);
        String sealed = seal("server:survival:whitelist", WHITELIST, "vec-rsa");
        Map<String, Object> data = object(object(GameDataModels.parse(sealed)).get("data"));
        String[] parts = ((String) data.get("jwe")).split("\\.", -1);
        byte[] tag = Base64.getUrlDecoder().decode(parts[4]);
        tag[0] ^= 1;
        parts[4] = b64(tag);
        use(plain(NoworkClient.encode(Map.of("jwe", String.join(".", parts)))), "JWE");
        expectEncryption("whitelist", keyed::whitelist);
    }

    private void contextsAndMalformedData() throws Exception {
        use(sharedEnvelope, null);
        expectEncryption("whitelist/check", () -> keyed.checkWhitelist("Steve", null));
        expectEncryption("whitelist", () -> client("another-server", ec).whitelist());
        for (String context : new String[] {null, "webhook:7", "server::whitelist", "server:other:whitelist",
                "server:survival:whitelist/check", "server:survival:whitelist/"}) {
            use(seal(context, WHITELIST, "vec-rsa"), "JWE");
            expectEncryption("whitelist", keyed::whitelist);
        }
        for (String bad : List.of("", "{", "null", "[]", "\"text\"", "{}", "{\"mode\":null}",
                "{\"mode\":-1}", "{\"mode\":3}", "{\"mode\":1.5}", "{\"mode\":1e-1000}",
                "{\"mode\":1.0000000000000000000001}", "{\"mode\":4294967296}", "{\"mode\":\"1\"}",
                "{\"mode\":true}", "{\"mode\":1} trailing", "{\"mode\":1,\"mode\":0}")) {
            use(seal("server:survival:whitelist", bad, "vec-rsa"), "JWE");
            check(expectEncryption("whitelist", keyed::whitelist).getCause() != null,
                    "invalid decrypted JSON/schema keeps an encryption failure cause");
        }
        for (int mode : new int[] {0, 1, 2}) {
            use(seal("server:survival:whitelist", NoworkClient.encode(Map.of(
                    "server", SERVER, "mode", mode, "count", 0, "entries", List.of())), "vec-rsa"), null);
            check(keyed.whitelist().mode == mode, "explicit backend whitelist mode is accepted exactly");
        }
        for (String bad : List.of("{}", "{\"allowed\":null}", "{\"allowed\":\"true\"}", "{\"allowed\":1}")) {
            use(seal("server:survival:whitelist/check", bad, "vec-rsa"), null);
            expectEncryption("whitelist/check", () -> keyed.checkWhitelist("Steve", null));
        }
        use(seal("server:survival:whitelist/check", "{\"allowed\":false,\"bound\":true,\"reason\":\"banned\"}", "vec-rsa"), null);
        NoworkClient.CheckResult denied = keyed.checkWhitelist("Steve", null);
        check(!denied.allowed && denied.bound && denied.reason.equals("banned"), "explicit boolean denial remains a valid reply");
        for (String jwe : List.of("null", "17", "{}", "\"\"", "\"not.a.jwe\"")) {
            use(plain("{\"jwe\":" + jwe + "}"), null);
            expectEncryption("whitelist", unkeyed::whitelist);
            expectEncryption("whitelist", keyed::whitelist);
        }
        String validJwe = (String) object(object(GameDataModels.parse(sharedEnvelope)).get("data")).get("jwe");
        use(plain(NoworkClient.encode(Map.of("jwe", validJwe, "mode", 0))), "JWE");
        expectEncryption("whitelist", keyed::whitelist);
        // Future/other marked routes still pass through authenticated context checks before decoding.
        use(seal("server:survival:gm/commands", "[{\"id\":9,\"action\":\"broadcast\",\"params\":{}}]", "vec-rsa"), null);
        check(keyed.pollGMCommands().size() == 1, "marked non-whitelist list reply opens with its exact route");
        expectEncryption("gm/commands", unkeyed::pollGMCommands);
        use(seal("server:survival:heartbeat", "{\"status\":\"ok\"}", "vec-rsa"), "JWE");
        keyed.heartbeat(0, 20, "1.21.1");
        expectEncryption("heartbeat", () -> unkeyed.heartbeat(0, 20, "1.21.1"));
        use(seal("server:other:heartbeat", "{\"status\":\"ok\"}", "vec-rsa"), "JWE");
        expectEncryption("heartbeat", () -> keyed.heartbeat(0, 20, "1.21.1"));
    }

    private void downgradeAndRaw() throws Exception {
        use(plain(WHITELIST), null);
        expectEncryption("whitelist", keyed::whitelist);
        check(unkeyed.whitelist().mode == 1, "existing unkeyed constructor still accepts ordinary plaintext");
        NoworkClient.RawResponse raw = keyed.raw("GET", "/server/v1/whitelist", null);
        check(!raw.encrypted && raw.body.equals(reply.body), "raw remains available for plaintext inspection with keys configured");
        use(plain(CHECK), null);
        expectEncryption("whitelist/check", () -> keyed.checkWhitelist("Steve", null));
        check(unkeyed.checkWhitelist("Steve", null).allowed, "existing join check remains usable without encryption");
        use(plain("{\"status\":\"ok\"}"), null);
        keyed.heartbeat(0, 20, "1.21.1");
        check(requestPath.equals("/server/v1/heartbeat"), "key configuration does not require encrypted heartbeats");
        use(plain("{\"server\":\"survival\",\"modules\":{\"server-gm\":{}}}"), null);
        check(keyed.modules().containsKey("server-gm"), "normal module reply stays plaintext");
        use(plain("[]"), null);
        check(keyed.pollGMCommands().isEmpty(), "normal GM poll stays plaintext");
        use(plain("{\"status\":\"ok\"}"), null);
        keyed.reportGMResult(9, true, "sent");
        check(requestPath.equals("/server/v1/gm/commands/9/result"), "normal GM acknowledgement stays plaintext");
        use(plain("{\"jwe\":\"user-field\",\"mode\":1,\"count\":0,\"entries\":[]}"), null);
        check(unkeyed.whitelist().mode == 1, "a plain object's additional jwe field is not itself an envelope");
    }

    private void errorPrecedence() throws Exception {
        reply = new Reply(403, "{\"code\":403,\"message\":\"server disabled\"}", "JWE");
        try { keyed.whitelist(); throw new AssertionError("expected business refusal"); }
        catch (NoWorkException.Api e) { check(e.code == 403, "business error precedes encrypted-body handling"); }
        for (String bad : List.of("<html>bad gateway</html>", "[]", "{}", "{\"code\":0}")) {
            use(bad, "JWE");
            expectEncryption("whitelist", keyed::whitelist);
            expectEncryption("whitelist", unkeyed::whitelist);
        }
        use("{\"data\":{\"jwe\":\"broken\"}}", null);
        expectEncryption("whitelist", keyed::whitelist);
        reply = new Reply(503, sharedEnvelope, "JWE");
        expectEncryption("whitelist", keyed::whitelist);
        reply = new Reply(503, plain(WHITELIST), null);
        expectTransport(keyed::whitelist);
        use("<html>bad gateway</html>", null);
        expectTransport(keyed::whitelist);
    }

    private void configuration() throws Exception {
        expectConfiguration(() -> client(null, rsa));
        expectConfiguration(() -> client(" ", rsa));
        expectConfiguration(() -> client(SERVER));
        expectConfiguration(() -> client(SERVER, (NoWorkDelivery.PrivateJwk[]) null));
        expectConfiguration(() -> client(SERVER, new NoWorkDelivery.PrivateJwk[] {null}));
        use(sharedEnvelope, null);
        expectEncryption("whitelist", new NoworkClient(base, "test-server-key")::whitelist);
    }

    /** Test-only RFC 7516 RSA-OAEP-256/A256GCM fixture; never part of SDK production encryption. */
    private String seal(String context, String plaintext, String kid) throws Exception {
        Map<String, Object> header = new LinkedHashMap<>();
        header.put("alg", "RSA-OAEP-256");
        header.put("enc", "A256GCM");
        header.put("kid", kid);
        if (context != null) header.put("ctx", context);
        String protectedHeader = b64(NoworkClient.encode(header).getBytes(StandardCharsets.UTF_8));
        byte[] cek = new byte[32];
        byte[] iv = new byte[12];
        SecureRandom random = new SecureRandom();
        random.nextBytes(cek);
        random.nextBytes(iv);
        var publicKey = KeyFactory.getInstance("RSA").generatePublic(new RSAPublicKeySpec(
                uint((String) rsaJwk.get("n")), uint((String) rsaJwk.get("e"))));
        Cipher rsaCipher = Cipher.getInstance("RSA/ECB/OAEPPadding");
        rsaCipher.init(Cipher.ENCRYPT_MODE, publicKey,
                new OAEPParameterSpec("SHA-256", "MGF1", MGF1ParameterSpec.SHA256, PSource.PSpecified.DEFAULT));
        byte[] wrapped = rsaCipher.doFinal(cek);
        Cipher gcm = Cipher.getInstance("AES/GCM/NoPadding");
        gcm.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(cek, "AES"), new GCMParameterSpec(128, iv));
        gcm.updateAAD(protectedHeader.getBytes(StandardCharsets.US_ASCII));
        byte[] ciphertextAndTag = gcm.doFinal(plaintext.getBytes(StandardCharsets.UTF_8));
        int boundary = ciphertextAndTag.length - 16;
        String compact = String.join(".", protectedHeader, b64(wrapped), b64(iv),
                b64(Arrays.copyOfRange(ciphertextAndTag, 0, boundary)), b64(Arrays.copyOfRange(ciphertextAndTag, boundary, ciphertextAndTag.length)));
        return plain(NoworkClient.encode(Map.of("jwe", compact)));
    }

    private static BigInteger uint(String value) { return new BigInteger(1, Base64.getUrlDecoder().decode(value)); }
    private static String b64(byte[] value) { return Base64.getUrlEncoder().withoutPadding().encodeToString(value); }
    @SuppressWarnings("unchecked") private static Map<String, Object> object(Object value) {
        if (!(value instanceof Map<?, ?>)) throw new AssertionError("expected fixture object");
        return (Map<String, Object>) value;
    }

    private void check(boolean condition, String message) {
        if (!condition) throw new AssertionError(message);
        checks++;
    }

    private NoWorkException.Encryption expectEncryption(String route, Call call) throws Exception {
        try { call.run(); }
        catch (NoWorkException.Encryption e) { check(e.route.equals(route), "encryption error names the exact route"); return e; }
        throw new AssertionError("expected Encryption on " + route);
    }

    private void expectTransport(Call call) throws Exception {
        try { call.run(); }
        catch (NoWorkException.Transport expected) { checks++; return; }
        throw new AssertionError("expected a genuine Transport error");
    }

    private void expectConfiguration(Call call) throws Exception {
        try { call.run(); }
        catch (IllegalArgumentException expected) { checks++; return; }
        throw new AssertionError("expected invalid encryption configuration to be rejected");
    }

    @Override public void close() {
        if (!closed) { closed = true; http.stop(0); }
    }
}
