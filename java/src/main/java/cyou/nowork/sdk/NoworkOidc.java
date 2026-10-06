package cyou.nowork.sdk;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Minimal OpenID Connect relying-party helper for the NoWork Account provider.
 * Supports authorization-code (+PKCE), refresh, client_credentials, userinfo, introspection, revocation.
 */
public class NoworkOidc {
    private final String issuer;
    private final String clientId;
    private final String clientSecret;
    private final String redirectUri;
    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build();

    public NoworkOidc(String issuer, String clientId, String clientSecret, String redirectUri) {
        this.issuer = issuer.endsWith("/") ? issuer.substring(0, issuer.length() - 1) : issuer;
        this.clientId = clientId;
        this.clientSecret = clientSecret == null ? "" : clientSecret;
        this.redirectUri = redirectUri;
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
