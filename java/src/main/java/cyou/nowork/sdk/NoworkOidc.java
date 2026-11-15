package cyou.nowork.sdk;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
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
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Flow;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/**
 * Minimal OpenID Connect relying-party helper for the NoWork Account provider.
 * Supports authorization-code (+PKCE), refresh, client_credentials, userinfo, introspection, revocation.
 */
public class NoworkOidc {
    private static final Duration TIMEOUT = Duration.ofSeconds(10);
    private static final int MAX_METADATA_BYTES = 1024 * 1024;
    private final String issuer;
    private final String clientId;
    private final String clientSecret;
    private final String redirectUri;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(TIMEOUT)
        .followRedirects(HttpClient.Redirect.NEVER).build();

    public NoworkOidc(String issuer, String clientId, String clientSecret, String redirectUri) {
        this.issuer = issuer.endsWith("/") ? issuer.substring(0, issuer.length() - 1) : issuer;
        this.clientId = clientId;
        this.clientSecret = clientSecret == null ? "" : clientSecret;
        this.redirectUri = redirectUri;
    }

    /**
     * Reads unauthenticated discovery from the configured issuer's fixed path. The issuer must
     * match exactly; endpoint fields are returned as metadata and are never followed automatically.
     */
    public Map<String, Object> discover() throws NoWorkException {
        Map<String, Object> metadata = metadataObject(metadataGet("/.well-known/openid-configuration"));
        for (String field : List.of("issuer", "authorization_endpoint", "token_endpoint", "jwks_uri")) {
            if (!(metadata.get(field) instanceof String value) || value.isBlank()) {
                throw new NoWorkException.Transport("incomplete OIDC discovery: " + field);
            }
        }
        if (!issuer.equals(metadata.get("issuer"))) {
            throw new NoWorkException.Transport("OIDC discovery issuer mismatch");
        }
        return metadata;
    }

    /** Reads the configured issuer's fixed /jwks.json path, returning validated original JSON text. */
    public String jwks() throws NoWorkException {
        String json = metadataGet("/jwks.json");
        Map<String, Object> keys = metadataObject(json);
        if (!(keys.get("keys") instanceof List<?> list) || list.stream().anyMatch(key -> !(key instanceof Map<?, ?>))) {
            throw new NoWorkException.Transport("invalid OIDC JWKS: keys must be an array of objects");
        }
        return json;
    }

    private static Map<String, Object> metadataObject(String json) throws NoWorkException.Transport {
        try {
            // Reuse the SDK's strict reader; Json.parse accepts truncated and trailing JSON.
            return GameDataModels.object(GameDataModels.parse(json));
        } catch (IllegalArgumentException e) {
            throw new NoWorkException.Transport("invalid OIDC metadata JSON object");
        }
    }

    private String metadataGet(String path) throws NoWorkException.Transport {
        if (Thread.currentThread().isInterrupted()) {
            throw new NoWorkException.Transport("OIDC metadata request interrupted");
        }
        CompletableFuture<HttpResponse<byte[]>> pending;
        try {
            URI configured = URI.create(issuer);
            if (!("https".equalsIgnoreCase(configured.getScheme()) || "http".equalsIgnoreCase(configured.getScheme()))
                || configured.getHost() == null || configured.getRawUserInfo() != null || configured.getRawQuery() != null
                || configured.getRawFragment() != null || !configured.normalize().equals(configured)) {
                throw new IllegalArgumentException("invalid OIDC issuer URL");
            }
            HttpRequest request = HttpRequest.newBuilder(URI.create(issuer + path)).timeout(TIMEOUT)
                .header("Accept", "application/json").GET().build();
            pending = http.sendAsync(request, ignored -> new MetadataBody());
        } catch (IllegalArgumentException e) {
            throw new NoWorkException.Transport(e);
        }
        HttpResponse<byte[]> response;
        try {
            // HttpRequest.timeout alone need not cover a body stalled after the response headers.
            response = pending.get(TIMEOUT.toNanos(), TimeUnit.NANOSECONDS);
        } catch (InterruptedException e) {
            pending.cancel(true);
            Thread.currentThread().interrupt();
            throw new NoWorkException.Transport(e);
        } catch (TimeoutException e) {
            pending.cancel(true);
            throw new NoWorkException.Transport("OIDC metadata request timed out");
        } catch (ExecutionException e) {
            if (e.getCause() instanceof HttpTimeoutException) {
                throw new NoWorkException.Transport("OIDC metadata request timed out");
            }
            throw new NoWorkException.Transport(e.getCause());
        }
        if (response.statusCode() < 200 || response.statusCode() >= 300) {
            throw new NoWorkException.Transport("OIDC metadata HTTP status " + response.statusCode());
        }
        try {
            return StandardCharsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT)
                .onUnmappableCharacter(CodingErrorAction.REPORT).decode(ByteBuffer.wrap(response.body())).toString();
        } catch (CharacterCodingException e) {
            throw new NoWorkException.Transport(e);
        }
    }

    /** Caps actual bytes received, including chunked bodies and misleading Content-Length headers. */
    private static final class MetadataBody implements HttpResponse.BodySubscriber<byte[]> {
        private final ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        private final CompletableFuture<byte[]> result = new CompletableFuture<>();
        private Flow.Subscription subscription;

        @Override public CompletionStage<byte[]> getBody() { return result; }
        @Override public void onSubscribe(Flow.Subscription value) {
            if (subscription != null) { value.cancel(); return; }
            subscription = value;
            subscription.request(1);
        }
        @Override public void onNext(List<ByteBuffer> buffers) {
            if (result.isDone()) return;
            for (ByteBuffer buffer : buffers) {
                int count = buffer.remaining();
                if (count > MAX_METADATA_BYTES - bytes.size()) {
                    subscription.cancel();
                    result.completeExceptionally(new IOException("OIDC metadata exceeds 1 MiB"));
                    return;
                }
                byte[] chunk = new byte[count];
                buffer.get(chunk);
                bytes.writeBytes(chunk);
            }
            subscription.request(1);
        }
        @Override public void onError(Throwable error) { result.completeExceptionally(error); }
        @Override public void onComplete() { result.complete(bytes.toByteArray()); }
    }

    /** Build the authorization URL. codeChallenge may be null for confidential clients. */
    public String authorizeUrl(String state, String nonce, String scope, String codeChallenge) {
        StringBuilder q = new StringBuilder()
            .append("response_type=code")
            .append("&client_id=").append(enc(clientId))
            .append("&redirect_uri=").append(enc(redirectUri))
            .append("&scope=").append(enc(scope))
            .append("&state=").append(enc(state));
        if (nonce != null) q.append("&nonce=").append(enc(nonce));
        if (codeChallenge != null) q.append("&code_challenge=").append(enc(codeChallenge)).append("&code_challenge_method=S256");
        return issuer + "/oauth2/authorize?" + q;
    }

    public Map<String, Object> exchange(String code, String codeVerifier) throws NoWorkException {
        Map<String, String> f = new LinkedHashMap<>();
        f.put("grant_type", "authorization_code");
        f.put("code", code);
        f.put("redirect_uri", redirectUri);
        if (codeVerifier != null) f.put("code_verifier", codeVerifier);
        return post("/oauth2/token", f);
    }

    public Map<String, Object> refresh(String refreshToken) throws NoWorkException {
        Map<String, String> f = new LinkedHashMap<>();
        f.put("grant_type", "refresh_token");
        f.put("refresh_token", refreshToken);
        return post("/oauth2/token", f);
    }

    public Map<String, Object> clientCredentials(String scope) throws NoWorkException {
        Map<String, String> f = new LinkedHashMap<>();
        f.put("grant_type", "client_credentials");
        f.put("scope", scope == null ? "" : scope);
        return post("/oauth2/token", f);
    }

    public Map<String, Object> introspect(String token) throws NoWorkException {
        Map<String, String> f = new LinkedHashMap<>();
        f.put("token", token);
        return post("/oauth2/introspect", f);
    }

    public void revoke(String token) throws NoWorkException {
        Map<String, String> f = new LinkedHashMap<>();
        f.put("token", token);
        post("/oauth2/revoke", f);
    }

    @SuppressWarnings("unchecked")
    public Map<String, Object> userInfo(String accessToken) throws NoWorkException {
        HttpRequest req = HttpRequest.newBuilder(URI.create(issuer + "/oauth2/userinfo"))
            .timeout(Duration.ofSeconds(10))
            .header("Authorization", "Bearer " + accessToken)
            .header("Accept", "application/json")
            .GET().build();
        try {
            HttpResponse<String> res = http.send(req, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
            Object parsed = Json.parse(res.body());
            Map<String, Object> m = parsed instanceof Map<?, ?> ? (Map<String, Object>) parsed : Map.of();
            if (res.statusCode() >= 400) throw new NoWorkException.OAuth(Json.str(m.get("error")), Json.str(m.get("error_description")));
            return m;
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) Thread.currentThread().interrupt();
            throw new NoWorkException.Transport(e);
        }
    }

    public String logoutUrl(String idToken, String postLogoutRedirectUri, String state) {
        StringBuilder q = new StringBuilder("client_id=").append(enc(clientId));
        if (idToken != null) q.append("&id_token_hint=").append(enc(idToken));
        if (postLogoutRedirectUri != null) q.append("&post_logout_redirect_uri=").append(enc(postLogoutRedirectUri));
        if (state != null) q.append("&state=").append(enc(state));
        return issuer + "/oauth2/logout?" + q;
    }

    @SuppressWarnings("unchecked")
    private Map<String, Object> post(String path, Map<String, String> form) throws NoWorkException {
        form.put("client_id", clientId);
        if (!clientSecret.isEmpty()) form.put("client_secret", clientSecret);
        StringBuilder body = new StringBuilder();
        form.forEach((k, v) -> body.append(body.length() > 0 ? "&" : "").append(enc(k)).append('=').append(enc(v)));
        HttpRequest req = HttpRequest.newBuilder(URI.create(issuer + path))
            .timeout(Duration.ofSeconds(10))
            .header("Content-Type", "application/x-www-form-urlencoded")
            .header("Accept", "application/json")
            .POST(HttpRequest.BodyPublishers.ofString(body.toString(), StandardCharsets.UTF_8)).build();
        try {
            HttpResponse<String> res = http.send(req, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
            Object parsed = res.body() == null || res.body().isBlank() ? Map.of() : Json.parse(res.body());
            Map<String, Object> m = parsed instanceof Map<?, ?> ? (Map<String, Object>) parsed : Map.of();
            if (res.statusCode() >= 400) throw new NoWorkException.OAuth(Json.str(m.get("error")), Json.str(m.get("error_description")));
            return m;
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) Thread.currentThread().interrupt();
            throw new NoWorkException.Transport(e);
        }
    }

    private static String enc(String s) {
        return URLEncoder.encode(s, StandardCharsets.UTF_8);
    }
}
