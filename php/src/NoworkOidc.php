<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Exceptions\OAuthError;
use NoWork\Sdk\Exceptions\TransportError;
use NoWork\Sdk\Internal\Json;

/**
 * Minimal OpenID Connect relying-party helper: authorization code (+ PKCE S256), refresh, client credentials,
 * userinfo, introspection, revocation and RP-initiated logout.
 *
 * Token / userinfo / introspection responses are returned as associative arrays with the wire field names
 * (`access_token`, `expires_in`, `sub`, `uid`, `security_level`, `game_roles`, ...).
 * Errors: RFC 6749 error object -> OAuthError, anything else that fails -> TransportError.
 */
final class NoworkOidc
{
    private const MAX_METADATA_BYTES = 1048576;

    private string $issuer;

    /** @var callable(string, string, array<string, string>, ?string, float): array{int, string} */
    private $transport;

    /**
     * @param string        $issuer       user-center base URL (the OIDC issuer)
     * @param string        $clientId     `lc_...` from the admin console
     * @param string        $clientSecret empty for public clients (use PKCE)
     * @param string        $redirectUri  registered callback URL
     * @param float         $timeout      per-request timeout in seconds
     * @param callable|null $transport    custom HTTP transport, see CurlTransport for the contract
     */
    public function __construct(
        string $issuer,
        private string $clientId,
        private string $clientSecret = '',
        private string $redirectUri = '',
        private float $timeout = 10.0,
        ?callable $transport = null,
    ) {
        $this->issuer = rtrim($issuer, '/');
        $this->transport = $transport ?? new CurlTransport();
    }

    /**
     * Read discovery from the configured issuer's fixed path; advertised endpoints are never fetched.
     *
     * @return array<string, mixed> complete metadata, including unknown fields
     */
    public function discover(): array
    {
        [$metadata] = $this->metadata('/.well-known/openid-configuration');
        foreach (['issuer', 'authorization_endpoint', 'token_endpoint', 'jwks_uri'] as $field) {
            if (!is_string($metadata[$field] ?? null) || trim($metadata[$field]) === '') {
                throw new TransportError('incomplete discovery: ' . $field . ' must be a nonempty string');
            }
        }
        if ($metadata['issuer'] !== $this->issuer) {
            throw new TransportError('discovery issuer mismatch');
        }

        return $metadata;
    }

    /**
     * Read the provider's fixed /jwks.json path, independently of any discovery document.
     *
     * @return array<string, mixed> JWKS with a keys array of objects, preserving unknown fields
     */
    public function jwks(): array
    {
        [$metadata, $object] = $this->metadata('/jwks.json');
        if (!is_array($object->keys ?? null)) {
            throw new TransportError('invalid JWKS: keys must be an array of objects');
        }
        foreach ($object->keys as $key) {
            if (!$key instanceof \stdClass) {
                throw new TransportError('invalid JWKS: keys must be an array of objects');
            }
        }

        return $metadata;
    }

    /** Build the authorization request URL; `$codeChallenge` (S256) is required for public clients. */
    public function authorizeUrl(string $state, string $nonce = '', string $scope = 'openid profile email', string $codeChallenge = ''): string
    {
        $q = [
            'response_type' => 'code',
            'client_id' => $this->clientId,
            'redirect_uri' => $this->redirectUri,
            'scope' => $scope,
            'state' => $state,
        ];
        if ($nonce !== '') {
            $q['nonce'] = $nonce;
        }
        if ($codeChallenge !== '') {
            $q['code_challenge'] = $codeChallenge;
            $q['code_challenge_method'] = 'S256';
        }

        return $this->issuer . '/oauth2/authorize?' . http_build_query($q);
    }

    /**
     * Generate a PKCE verifier / S256 challenge pair. Keep the verifier in the session and pass it to exchange().
     *
     * @return array{verifier: string, challenge: string}
     */
    public static function pkce(): array
    {
        $verifier = self::pkceVerifier();

        return ['verifier' => $verifier, 'challenge' => self::pkceChallenge($verifier)];
    }

    /** Random RFC 7636 code_verifier (base64url, 43-128 characters; 32 random bytes by default). */
    public static function pkceVerifier(int $bytes = 32): string
    {
        return self::base64url(random_bytes(max(32, min(96, $bytes))));
    }

    /** S256 code_challenge for a verifier: base64url(sha256(verifier)). */
    public static function pkceChallenge(string $verifier): string
    {
        return self::base64url(hash('sha256', $verifier, true));
    }

    /**
     * Trade an authorization code for tokens; `$codeVerifier` is required when PKCE was used.
     *
     * @return array<string, mixed> `access_token`, `token_type`, `expires_in`, `refresh_token`, `id_token`, `scope`
     */
    public function exchange(string $code, string $codeVerifier = ''): array
    {
        return $this->post('/oauth2/token', [
            'grant_type' => 'authorization_code',
            'code' => $code,
            'redirect_uri' => $this->redirectUri,
            'code_verifier' => $codeVerifier !== '' ? $codeVerifier : null,
        ]);
    }

    /**
     * Obtain a new token pair from a refresh token (the old refresh token is rotated).
     *
     * @return array<string, mixed>
     */
    public function refresh(string $refreshToken): array
    {
        return $this->post('/oauth2/token', ['grant_type' => 'refresh_token', 'refresh_token' => $refreshToken]);
    }

    /**
     * Obtain a machine token (confidential clients only).
     *
     * @return array<string, mixed>
     */
    public function clientCredentials(string $scope = ''): array
    {
        return $this->post('/oauth2/token', ['grant_type' => 'client_credentials', 'scope' => $scope]);
    }

    /**
     * RFC 7662 introspection of an access or refresh token.
     *
     * @return array<string, mixed> `active`, `sub`, `uid`, `scope`, `client_id`, `exp`, ...
     */
    public function introspect(string $token): array
    {
        return $this->post('/oauth2/introspect', ['token' => $token]);
    }

    /** Invalidate a token. The endpoint may answer with an empty body. */
    public function revoke(string $token): void
    {
        $this->post('/oauth2/revoke', ['token' => $token], true);
    }

    /**
     * Fetch the profile for an access token.
     *
     * @return array<string, mixed> see docs/PROTOCOL.md section 3 for the released claims
     */
    public function userInfo(string $accessToken): array
    {
        return $this->send('GET', '/oauth2/userinfo', ['Authorization' => 'Bearer ' . $accessToken, 'Accept' => 'application/json'], null, false);
    }

    /** Build the RP-initiated logout URL (end session endpoint). */
    public function logoutUrl(string $idToken = '', string $postLogoutRedirectUri = '', string $state = ''): string
    {
        $q = ['client_id' => $this->clientId];
        if ($idToken !== '') {
            $q['id_token_hint'] = $idToken;
        }
        if ($postLogoutRedirectUri !== '') {
            $q['post_logout_redirect_uri'] = $postLogoutRedirectUri;
        }
        if ($state !== '') {
            $q['state'] = $state;
        }

        return $this->issuer . '/oauth2/logout?' . http_build_query($q);
    }

    /**
     * POST an `application/x-www-form-urlencoded` body with client_secret_post authentication.
     *
     * @param array<string, string|null> $form null values are omitted
     *
     * @return array<string, mixed>
     */
    private function post(string $path, array $form, bool $allowEmpty = false): array
    {
        $form = array_filter($form, static fn ($v): bool => $v !== null);
        $form['client_id'] = $this->clientId;
        if ($this->clientSecret !== '') {
            $form['client_secret'] = $this->clientSecret;
        }
        $headers = ['Content-Type' => 'application/x-www-form-urlencoded', 'Accept' => 'application/json'];

        return $this->send('POST', $path, $headers, http_build_query($form), $allowEmpty);
    }

    /**
     * Keep JSON object/array distinctions for validation while returning the usual associative arrays.
     * Direct CurlTransport instances bound downloads; custom callables still receive exactly five arguments.
     *
     * @return array{array<string, mixed>, \stdClass}
     */
    private function metadata(string $path): array
    {
        $url = parse_url($this->issuer);
        if (!is_array($url) || !in_array($url['scheme'] ?? '', ['http', 'https'], true) || empty($url['host'])
            || isset($url['user']) || isset($url['pass']) || isset($url['query']) || isset($url['fragment'])
            || preg_match('/[\x00-\x20\x7f]/', $this->issuer)) {
            throw new TransportError('invalid metadata issuer URL');
        }
        $transport = $this->transport;
        if ($transport instanceof CurlTransport) {
            $curl = $transport;
            $transport = static fn (string $method, string $url, array $headers, ?string $body, float $timeout): array =>
                $curl->sendBounded($method, $url, $headers, $body, $timeout, self::MAX_METADATA_BYTES);
        }
        [$status, $raw] = Json::send($transport, 'GET', $this->issuer . $path, ['Accept' => 'application/json'], null, $this->timeout);
        if (strlen($raw) > self::MAX_METADATA_BYTES) {
            throw new TransportError('metadata response exceeds 1 MiB');
        }
        if ($status < 200 || $status >= 300) {
            throw new TransportError(sprintf('http %d', $status));
        }
        try {
            $object = json_decode($raw, false, 512, JSON_THROW_ON_ERROR);
            if (!$object instanceof \stdClass) {
                throw new TransportError('invalid metadata response: not an object');
            }
            $metadata = json_decode($raw, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new TransportError('invalid metadata json: ' . $e->getMessage());
        }

        return [$metadata, $object];
    }

    /**
     * HTTP >= 400 with an RFC 6749 error object -> OAuthError, otherwise TransportError. A 2xx body must be a
     * JSON object; only revocation (`$allowEmpty`) may answer with an empty body.
     *
     * @param array<string, string> $headers
     *
     * @return array<string, mixed>
     */
    private function send(string $method, string $path, array $headers, ?string $body, bool $allowEmpty): array
    {
        [$status, $raw] = Json::send($this->transport, $method, $this->issuer . $path, $headers, $body, $this->timeout);
        if ($status >= 400) {
            $err = $raw !== '' ? json_decode($raw, true) : null;
            if (is_array($err) && isset($err['error']) && Json::str($err['error']) !== '') {
                throw new OAuthError(Json::str($err['error']), Json::str($err['error_description'] ?? ''), $status);
            }
            throw new TransportError(sprintf('http %d', $status));
        }
        if ($raw === '') {
            if ($allowEmpty) {
                return [];
            }
            throw new TransportError(sprintf('empty response (http %d)', $status));
        }

        return Json::decodeObject($raw, $status);
    }

    private static function base64url(string $binary): string
    {
        return rtrim(strtr(base64_encode($binary), '+/', '-_'), '=');
    }
}
