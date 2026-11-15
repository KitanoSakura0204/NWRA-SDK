package cyou.nowork.sdk;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyFactory;
import java.security.SecureRandom;
import java.security.Signature;
import java.security.spec.MGF1ParameterSpec;
import java.security.spec.RSAPrivateCrtKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.time.Duration;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import javax.crypto.Cipher;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.OAEPParameterSpec;
import javax.crypto.spec.PSource;
import javax.crypto.spec.SecretKeySpec;

/** Executable offline OIDC contracts; uses loopback HTTP and committed test-only delivery keys. */
public final class OidcTest implements AutoCloseable {
    private static final int LIMIT = 1024 * 1024;
    @FunctionalInterface private interface Call { void run() throws Exception; }
    @FunctionalInterface private interface Reply { void send(HttpExchange exchange) throws Exception; }
    private record Request(String method, String path, String auth, String serverKey, String cookie, String body) {}
    private final HttpServer server;
    private final ExecutorService executor = Executors.newCachedThreadPool();
    private final List<Request> requests = Collections.synchronizedList(new java.util.ArrayList<>());
    private volatile Reply reply = exchange -> send(exchange, 200, "{}");
    private final String issuer;
    private final NoworkOidc oidc;
    private final Map<String, Object> rsa;
    private final Map<String, Object> vectors;
    private final NoWorkDelivery.PrivateJwk privateKey;
    private final String jwks;
    private int checks;

    private OidcTest(Path vectorPath) throws Exception {
        vectors = GameDataModels.object(GameDataModels.parse(Files.readString(vectorPath)));
        rsa = GameDataModels.object(GameDataModels.object(vectors.get("keys")).get("rsa"));
        privateKey = NoWorkDelivery.PrivateJwk.parse(NoworkClient.encode(rsa));
        jwks = NoworkClient.encode(Map.of("keys", List.of(Map.of("kty", "RSA", "kid", rsa.get("kid"), "n", rsa.get("n"), "e", rsa.get("e")))));
        server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
        server.setExecutor(executor);
        server.createContext("/", exchange -> {
            requests.add(new Request(exchange.getRequestMethod(), exchange.getRequestURI().toString(),
                    exchange.getRequestHeaders().getFirst("Authorization"), exchange.getRequestHeaders().getFirst("X-Server-Key"),
                    exchange.getRequestHeaders().getFirst("Cookie"), new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8)));
            try { reply.send(exchange); }
            catch (Exception ignored) { /* Cancellation closes the response under the fixture writer. */ }
            finally { exchange.close(); }
        });
        server.start();
        issuer = "http://127.0.0.1:" + server.getAddress().getPort() + "/tenant";
        oidc = new NoworkOidc(issuer + "/", "test-client", "secret-must-not-leak", "https://app.example/callback");
    }

    public static void main(String[] args) throws Exception {
        try (OidcTest test = new OidcTest(Path.of(args.length == 0 ? "../go/testdata/vectors.json" : args[0]))) {
            test.unicodeEscapes(); test.metadata(); test.nonces();
            test.server.stop(0);
            test.expect(NoWorkException.Transport.class, test.oidc::discover);
            test.expect(NoWorkException.Transport.class, test.oidc::jwks);
            System.out.println("OidcTest: " + test.checks + " checks passed");
        }
    }

    private Map<String, Object> document() {
        Map<String, Object> metadata = new LinkedHashMap<>();
        metadata.put("issuer", issuer);
        metadata.put("authorization_endpoint", "https://other.example/authorize");
        metadata.put("token_endpoint", "https://other.example/token");
        metadata.put("jwks_uri", "https://other.example/keys");
        metadata.put("extension", Map.of("unicode", "元数据", "list", List.of(1, true)));
        return metadata;
    }
    private void use(int status, String body) { reply = exchange -> send(exchange, status, body); }
    private static void send(HttpExchange exchange, int status, String body) throws IOException {
        byte[] bytes = body.getBytes(StandardCharsets.UTF_8);
        exchange.sendResponseHeaders(status, bytes.length); exchange.getResponseBody().write(bytes);
    }
    private void check(boolean condition, String message) { if (!condition) throw new AssertionError(message); checks++; }
    private <T extends Throwable> T expect(Class<T> type, Call call) throws Exception {
        try { call.run(); }
        catch (Throwable failure) {
            if (type.isInstance(failure)) { checks++; return type.cast(failure); }
            throw new AssertionError("expected " + type.getName(), failure);
        }
        throw new AssertionError("expected " + type.getName());
    }

    private void unicodeEscapes() throws Exception {
        String escape = "\\" + "u";
        for (String malformed : List.of(escape + "\uff10\uff10\uff14\uff11", escape + "00\uff26F", escape + "00\uff10a")) {
            use(200, "{\"keys\":[],\"x\":\"" + malformed + "\"}");
            expect(NoWorkException.Transport.class, oidc::jwks);
            Map<String, Object> metadata = document(); metadata.put("extension", "MARKER");
            use(200, NoworkClient.encode(metadata).replace("\"MARKER\"", "\"" + malformed + "\""));
            expect(NoWorkException.Transport.class, oidc::discover);
        }
        String valid = "{\"keys\":[],\"x\":\"" + escape + "0041" + escape + "00eF" + escape + "00Af\"}";
        use(200, valid); check(valid.equals(oidc.jwks()), "ASCII hexadecimal Unicode escapes stay valid");
        check(GameDataModels.object(GameDataModels.parse(valid)).get("x").equals("A\u00ef\u00af"), "ASCII Unicode escapes decode correctly");
        // A literal backslash followed by u and fullwidth text is valid JSON, not an escape.
        String literal = escape + "\uff10\uff10\uff14\uff11";
        Map<String, Object> metadata = document(); metadata.put("extension", literal);
        use(200, NoworkClient.encode(metadata));
        check(literal.equals(oidc.discover().get("extension")), "escaped literal backslash and fullwidth text are preserved");
    }

    private void metadata() throws Exception {
        requests.clear();
        use(200, NoworkClient.encode(document()));
        Map<String, Object> found = oidc.discover();
        check(issuer.equals(found.get("issuer")) && found.get("extension") instanceof Map, "exact normalized issuer and unknown fields");
        String keys = " {\"keys\":[{\"kty\":\"RSA\",\"extension\":{\"preserved\":true}}],\"extra\":null}\n";
        use(201, keys);
        check(keys.equals(oidc.jwks()), "JWKS preserves the validated original JSON text");
        check(requests.size() == 2 && requests.get(0).path.equals("/tenant/.well-known/openid-configuration")
                && requests.get(1).path.equals("/tenant/jwks.json"), "fixed issuer paths, never advertised endpoints");
        for (Request request : requests) check(request.method.equals("GET") && request.auth == null && request.serverKey == null
                && request.cookie == null && request.body.isEmpty(), "metadata is unauthenticated and has no credential body");
        for (String invalidIssuer : List.of(issuer + "?query=", issuer + "#fragment", issuer + "/../bad",
                issuer.replace("http://", "http://user:secret@"), "file:///tmp/issuer", "not a URI")) {
            NoworkOidc invalid = new NoworkOidc(invalidIssuer, "client", "secret", "https://app.example/callback");
            int before = requests.size();
            expect(NoWorkException.Transport.class, invalid::discover); expect(NoWorkException.Transport.class, invalid::jwks);
            check(requests.size() == before, "invalid issuer cannot redirect fixed paths or carry credentials");
        }
        for (String field : List.of("issuer", "authorization_endpoint", "token_endpoint", "jwks_uri")) {
            for (Object invalid : new Object[] {null, "", " ", 1, List.of()}) {
                Map<String, Object> bad = document(); bad.put(field, invalid);
                use(200, NoworkClient.encode(bad)); expect(NoWorkException.Transport.class, oidc::discover);
            }
            Map<String, Object> bad = document(); bad.remove(field);
            use(200, NoworkClient.encode(bad)); expect(NoWorkException.Transport.class, oidc::discover);
        }
        for (String mismatch : List.of(issuer + "/", issuer + "/other", "https://wrong.example")) {
            Map<String, Object> bad = document(); bad.put("issuer", mismatch);
            use(200, NoworkClient.encode(bad)); expect(NoWorkException.Transport.class, oidc::discover);
        }
        for (String bad : List.of("", " ", "[]", "null", "true", "<html>", "{", "{\"keys\":[]", "{\"keys\":[]}garbage",
                "{\"keys\":[] \"x\":1}", "{\"keys\":[],}", "{\"keys\":[],\"n\":01}", "{\"keys\":[],\"x\":\"\\x\"}",
                "{\"keys\":[],\"keys\":[]}", "{\"keys\":[],\"depth\":" + "[".repeat(66) + "0" + "]".repeat(66) + "}")) {
            use(200, bad); expect(NoWorkException.Transport.class, oidc::discover); expect(NoWorkException.Transport.class, oidc::jwks);
        }
        for (String bad : List.of("{}", "{\"keys\":null}", "{\"keys\":{}}", "{\"keys\":[null]}", "{\"keys\":[1]}", "{\"keys\":[{},[]]}")) {
            use(200, bad); expect(NoWorkException.Transport.class, oidc::jwks);
        }
        use(200, "{\"keys\":[]}"); check(oidc.jwks().equals("{\"keys\":[]}"), "empty key array is a valid JWKS shape");
        for (int status : new int[] {302, 401, 500}) {
            reply = exchange -> { exchange.getResponseHeaders().set("Location", issuer + "/advertised"); send(exchange, status, NoworkClient.encode(document())); };
            int before = requests.size(); expect(NoWorkException.Transport.class, oidc::discover);
            check(requests.size() == before + 1, "non-2xx does not redirect");
            expect(NoWorkException.Transport.class, oidc::jwks);
        }
        String exactly = "{\"keys\":[]}" + " ".repeat(LIMIT - "{\"keys\":[]}".length());
        use(200, exactly); check(oidc.jwks().length() == LIMIT, "one MiB is accepted");
        use(200, exactly + " "); expect(NoWorkException.Transport.class, oidc::jwks);
        use(200, NoworkClient.encode(document()) + " ".repeat(LIMIT)); expect(NoWorkException.Transport.class, oidc::discover);
        use(200, "{\"keys\":[],\"x\":\"" + "元".repeat(LIMIT / 2) + "\"}"); expect(NoWorkException.Transport.class, oidc::jwks);
        reply = exchange -> {
            exchange.sendResponseHeaders(200, 0);
            exchange.getResponseBody().write(new byte[] {'{', '"', 'x', '"', ':', '"', (byte) 0xc3, '"', '}'});
        };
        expect(NoWorkException.Transport.class, oidc::jwks);
        // The caller must stop while a chunked server remains open after exceeding the byte cap.
        CountDownLatch release = new CountDownLatch(1);
        reply = exchange -> {
            exchange.sendResponseHeaders(200, 0);
            exchange.getResponseBody().write((exactly + " ").getBytes(StandardCharsets.UTF_8)); exchange.getResponseBody().flush();
            release.await(15, TimeUnit.SECONDS);
        };
        long start = System.nanoTime();
        try { expect(NoWorkException.Transport.class, oidc::jwks); check(Duration.ofNanos(System.nanoTime() - start).toSeconds() < 5, "chunked byte limit cancels before EOF"); }
        finally { release.countDown(); }
        // Preserve interruption and cancel the in-flight response-body subscription.
        CountDownLatch bodyStarted = new CountDownLatch(1), interruptedRelease = new CountDownLatch(1);
        reply = exchange -> {
            exchange.sendResponseHeaders(200, 0); exchange.getResponseBody().write('{'); exchange.getResponseBody().flush();
            bodyStarted.countDown(); interruptedRelease.await(15, TimeUnit.SECONDS);
        };
        AtomicReference<Throwable> error = new AtomicReference<>(); AtomicReference<Boolean> interrupted = new AtomicReference<>();
        Thread caller = new Thread(() -> { try { oidc.discover(); } catch (Throwable e) { error.set(e); interrupted.set(Thread.currentThread().isInterrupted()); } });
        caller.start();
        try {
            check(bodyStarted.await(3, TimeUnit.SECONDS), "body began"); caller.interrupt(); caller.join(3000);
            check(!caller.isAlive() && error.get() instanceof NoWorkException.Transport && Boolean.TRUE.equals(interrupted.get()), "body interruption returns Transport and keeps interrupt");
        } finally { interruptedRelease.countDown(); caller.interrupt(); caller.join(3000); }
        CountDownLatch timeoutRelease = new CountDownLatch(1);
        reply = exchange -> {
            exchange.sendResponseHeaders(200, 0); exchange.getResponseBody().write('{'); exchange.getResponseBody().flush();
            timeoutRelease.await(15, TimeUnit.SECONDS);
        };
        start = System.nanoTime();
        try {
            NoWorkException.Transport timeout = expect(NoWorkException.Transport.class, oidc::discover);
            long millis = Duration.ofNanos(System.nanoTime() - start).toMillis();
            check(timeout.getMessage().contains("timed out") && millis >= 9000 && millis < 13000, "existing ten-second timeout includes response body");
        } finally { timeoutRelease.countDown(); }
    }

    private Map<String, Object> claims() {
        return new LinkedHashMap<>(Map.of("iss", issuer, "aud", "test-client", "exp", System.currentTimeMillis() / 1000 + 600,
                "sub", "fixture-user", "nonce", "expected-nonce"));
    }
    private void nonces() throws Exception {
        String token = sign(claims());
        check(NoWorkDelivery.verifyIdToken(token, jwks, issuer, "test-client", "expected-nonce").get("sub").equals("fixture-user"), "signed nonce match");
        for (String nonce : new String[] {null, "", "wrong", "Expected-nonce"})
            expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.verifyIdToken(token, jwks, issuer, "test-client", nonce));
        for (Object value : new Object[] {null, 123, List.of("expected-nonce"), "wrong", ""}) {
            Map<String, Object> bad = claims(); bad.put("nonce", value); String signed = sign(bad);
            expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.verifyIdToken(signed, jwks, issuer, "test-client", "expected-nonce"));
        }
        Map<String, Object> absent = claims(); absent.remove("nonce"); String noNonce = sign(absent);
        check(NoWorkDelivery.verifyIdToken(noNonce, jwks, issuer, "test-client").get("sub").equals("fixture-user"), "old signed signature keeps nonce-optional behavior");
        expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.verifyIdToken(noNonce, jwks, issuer, "test-client", "expected-nonce"));
        for (String field : List.of("iss", "aud", "exp", "nbf")) {
            Map<String, Object> bad = claims();
            bad.put(field, field.equals("exp") ? 1 : field.equals("nbf") ? System.currentTimeMillis() / 1000 + 600 : "wrong");
            String signed = sign(bad);
            NoWorkDelivery.DecryptException failed = expect(NoWorkDelivery.DecryptException.class,
                    () -> NoWorkDelivery.verifyIdToken(signed, jwks, issuer, "test-client", ""));
            check(!failed.getMessage().contains("nonce"), "issuer/audience/time checks precede nonce");
        }
        String[] parts = token.split("\\."); byte[] signature = Base64.getUrlDecoder().decode(parts[2]); signature[0] ^= 1;
        String tampered = parts[0] + "." + parts[1] + "." + b64(signature);
        NoWorkDelivery.DecryptException failed = expect(NoWorkDelivery.DecryptException.class,
                () -> NoWorkDelivery.verifyIdToken(tampered, jwks, issuer, "test-client", ""));
        check(failed.getMessage().contains("signature"), "signature precedes nonce");
        String encrypted = seal(token);
        check(NoWorkDelivery.decryptIdToken(encrypted, jwks, issuer, "test-client", null, privateKey).get("nonce").equals("expected-nonce"), "legacy null-plus-key call compiles and skips the null key");
        check(NoWorkDelivery.decryptIdTokenWithNonce(encrypted, jwks, issuer, "test-client", "expected-nonce", privateKey).get("nonce").equals("expected-nonce"), "encrypted nonce match");
        for (String nonce : new String[] {null, "", "wrong"})
            expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.decryptIdTokenWithNonce(encrypted, jwks, issuer, "test-client", nonce, privateKey));
        check(NoWorkDelivery.decryptIdToken(seal(noNonce), jwks, issuer, "test-client", privateKey).get("sub").equals("fixture-user"), "old encrypted signature stays usable");
        expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.decryptIdTokenWithNonce(seal(noNonce), jwks, issuer, "test-client", "expected-nonce", privateKey));
        Map<String, Object> badNonce = claims(); badNonce.put("nonce", 123);
        expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.decryptIdTokenWithNonce(seal(sign(badNonce)), jwks, issuer, "test-client", "expected-nonce", privateKey));
        Map<String, Object> badIssuer = claims(); badIssuer.put("iss", "https://wrong.example");
        expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.decryptIdTokenWithNonce(seal(sign(badIssuer)), jwks, issuer, "test-client", "expected-nonce", privateKey));
        expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.decryptIdTokenWithNonce(seal(tampered), jwks, issuer, "test-client", "expected-nonce", privateKey));
        expect(NoWorkDelivery.DecryptException.class, () -> NoWorkDelivery.decryptIdTokenWithNonce(seal(NoworkClient.encode(claims())), jwks, issuer, "test-client", "expected-nonce", privateKey));
        Map<String, Object> nested = ((List<?>) vectors.get("vectors")).stream().map(GameDataModels::object)
                .filter(v -> "nested-id-token".equals(v.get("name"))).findFirst().orElseThrow();
        check(NoWorkDelivery.decryptIdTokenWithNonce((String) nested.get("compact"), NoworkClient.encode(vectors.get("signing")),
                "https://account.example.com", "vector-app", "n-vector", privateKey).get("nonce").equals("n-vector"), "committed backend nested ID-token vector");
    }

    private String sign(Map<String, Object> claims) throws Exception {
        Signature signer = Signature.getInstance("SHA256withRSA");
        signer.initSign(KeyFactory.getInstance("RSA").generatePrivate(new RSAPrivateCrtKeySpec(uint("n"), uint("e"), uint("d"),
                uint("p"), uint("q"), uint("dp"), uint("dq"), uint("qi"))));
        String content = b64(NoworkClient.encode(Map.of("alg", "RS256", "kid", rsa.get("kid"))).getBytes(StandardCharsets.UTF_8))
                + "." + b64(NoworkClient.encode(claims).getBytes(StandardCharsets.UTF_8));
        signer.update(content.getBytes(StandardCharsets.US_ASCII)); return content + "." + b64(signer.sign());
    }
    private String seal(String plaintext) throws Exception {
        String header = b64(NoworkClient.encode(Map.of("alg", "RSA-OAEP-256", "enc", "A256GCM", "cty", "JWT", "kid", rsa.get("kid"))).getBytes(StandardCharsets.UTF_8));
        SecureRandom random = new SecureRandom(); byte[] cek = new byte[32], iv = new byte[12]; random.nextBytes(cek); random.nextBytes(iv);
        Cipher oaep = Cipher.getInstance("RSA/ECB/OAEPPadding");
        oaep.init(Cipher.ENCRYPT_MODE, KeyFactory.getInstance("RSA").generatePublic(new RSAPublicKeySpec(uint("n"), uint("e"))),
                new OAEPParameterSpec("SHA-256", "MGF1", MGF1ParameterSpec.SHA256, PSource.PSpecified.DEFAULT));
        Cipher gcm = Cipher.getInstance("AES/GCM/NoPadding");
        gcm.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(cek, "AES"), new GCMParameterSpec(128, iv)); gcm.updateAAD(header.getBytes(StandardCharsets.US_ASCII));
        byte[] encrypted = gcm.doFinal(plaintext.getBytes(StandardCharsets.UTF_8)); int end = encrypted.length - 16;
        return String.join(".", header, b64(oaep.doFinal(cek)), b64(iv), b64(Arrays.copyOfRange(encrypted, 0, end)), b64(Arrays.copyOfRange(encrypted, end, encrypted.length)));
    }
    private BigInteger uint(String field) { return new BigInteger(1, Base64.getUrlDecoder().decode((String) rsa.get(field))); }
    private static String b64(byte[] bytes) { return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes); }
    @Override public void close() { server.stop(0); executor.shutdownNow(); }
}
