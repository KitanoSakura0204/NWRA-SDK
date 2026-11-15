<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Exceptions\DecryptError;

/**
 * Delivery helpers (SDK/docs/DELIVERY.md): decrypt what the user centre sends when an app registered an
 * encryption key. ECDH-ES+A256KW over P-256 (ext-openssl) or X25519 (ext-sodium), enc A256GCM.
 * RSA-OAEP-256 is not offered: ext-openssl cannot select SHA-256 OAEP on PHP 8.1 - 8.4, so register an
 * EC P-256 or X25519 key for PHP apps.
 */
final class Delivery
{
    /**
     * Open a compact JWE. $keys is one private JWK (array) or a list of them; the header kid picks one,
     * without a kid every key is tried. Returns [plaintext, protected header].
     *
     * @param array<string, string>|list<array<string, string>> $keys
     * @return array{0: string, 1: array<string, mixed>}
     */
    public static function decrypt(string $compact, array $keys): array
    {
        $parts = explode('.', $compact);
        if (count($parts) !== 5) {
            throw new DecryptError();
        }
        $seg = array_map(fn (string $p): string => self::b64($p), $parts);
        $hdr = json_decode($seg[0], true);
        if (!is_array($hdr) || ($hdr['enc'] ?? '') !== 'A256GCM' || isset($hdr['zip']) || isset($hdr['crit'])) {
            throw new DecryptError();
        }
        if (isset($keys['kty'])) {
            $keys = [$keys];
        }
        $reason = 'cannot decrypt the JWE';
        foreach ($keys as $jwk) {
            if (isset($hdr['kid'], $jwk['kid']) && $hdr['kid'] !== $jwk['kid']) {
                continue;
            }
            try {
                $cek = self::contentKey($hdr, $seg[1], $jwk);
                $plain = openssl_decrypt($seg[3], 'aes-256-gcm', $cek, OPENSSL_RAW_DATA, $seg[2], $seg[4], $parts[0]);
                if ($plain !== false) {
                    return [$plain, $hdr];
                }
            } catch (\UnexpectedValueException $e) {
                $reason = $e->getMessage();
            } catch (\Throwable) {
                // wrong key or tampered: try the next key
            }
        }
        throw new DecryptError($reason);
    }

    /**
     * @param array<string, mixed> $hdr
     * @param array<string, string> $jwk
     */
    private static function contentKey(array $hdr, string $ek, array $jwk): string
    {
        $alg = $hdr['alg'] ?? '';
        if ($alg === 'RSA-OAEP-256') {
            throw new \UnexpectedValueException('RSA-OAEP-256 is not supported by ext-openssl here: register an EC P-256 or X25519 key');
        }
        $epk = $hdr['epk'] ?? null;
        if ($alg !== 'ECDH-ES+A256KW' || !is_array($epk)) {
            throw new DecryptError();
        }
        if (($jwk['kty'] ?? '') === 'EC' && ($jwk['crv'] ?? '') === 'P-256' && ($epk['crv'] ?? '') === 'P-256') {
            $priv = openssl_pkey_get_private(self::pem('EC PRIVATE KEY', self::ecPrivateDer($jwk)));
            $pub = openssl_pkey_get_public(self::pem('PUBLIC KEY', self::ecPublicDer($epk)));
            $z = ($priv !== false && $pub !== false) ? openssl_pkey_derive($pub, $priv) : false;
        } elseif (($jwk['kty'] ?? '') === 'OKP' && ($jwk['crv'] ?? '') === 'X25519' && ($epk['crv'] ?? '') === 'X25519') {
            if (!function_exists('sodium_crypto_scalarmult')) {
                throw new \UnexpectedValueException('X25519 needs ext-sodium');
            }
            $z = sodium_crypto_scalarmult(self::b64($jwk['d']), self::b64($epk['x']));
        } else {
            throw new DecryptError();
        }
        if (!is_string($z) || strlen($z) !== 32 || $z === str_repeat("\0", 32)) {
            throw new DecryptError();
        }
        $apu = isset($hdr['apu']) ? self::b64($hdr['apu']) : '';
        $apv = isset($hdr['apv']) ? self::b64($hdr['apv']) : '';
        // Concat KDF (JWA 4.6.2) for a 256-bit key: one SHA-256 round over counter || Z || OtherInfo.
        $kek = hash('sha256', "\x00\x00\x00\x01" . $z . self::lp($alg) . self::lp($apu) . self::lp($apv) . "\x00\x00\x01\x00", true);
        return self::unwrap($kek, $ek);
    }

    /** RFC 3394 AES key unwrap over raw AES-256-ECB blocks. */
    private static function unwrap(string $kek, string $wrapped): string
    {
        $n = intdiv(strlen($wrapped), 8) - 1;
        if (strlen($wrapped) % 8 !== 0 || $n < 2) {
            throw new DecryptError();
        }
        $a = substr($wrapped, 0, 8);
        $r = str_split(substr($wrapped, 8), 8);
        for ($j = 5; $j >= 0; $j--) {
            for ($i = $n; $i >= 1; $i--) {
                $b = openssl_decrypt(($a ^ pack('J', $n * $j + $i)) . $r[$i - 1], 'aes-256-ecb', $kek, OPENSSL_RAW_DATA | OPENSSL_ZERO_PADDING);
                if ($b === false) {
                    throw new DecryptError();
                }
                $a = substr($b, 0, 8);
                $r[$i - 1] = substr($b, 8, 8);
            }
        }
        if (!hash_equals("\xA6\xA6\xA6\xA6\xA6\xA6\xA6\xA6", $a)) {
            throw new DecryptError();
        }
        return implode('', $r);
    }

    /**
     * Verify an RS256 ID token against the provider JWKS (GET BASE/jwks.json): signature, iss, aud and
     * exp / nbf with 60 s leeway. An expected nonce must be nonempty and match the signed string claim.
     * Returns the claims unchanged; omit $expectedNonce to retain verification without nonce binding.
     *
     * @param array<string, mixed>|string $jwks
     * @return array<string, mixed>
     */
    public static function verifyIdToken(string $token, array|string $jwks, ?string $issuer = null, ?string $audience = null, ?string $expectedNonce = null): array
    {
        $parts = explode('.', $token);
        if (count($parts) !== 3) {
            throw new DecryptError('not a signed JWT');
        }
        $hdr = json_decode(self::b64($parts[0]), true);
        if (!is_array($hdr) || ($hdr['alg'] ?? '') !== 'RS256') {
            throw new DecryptError('ID token must be RS256');
        }
        $set = is_string($jwks) ? json_decode($jwks, true) : $jwks;
        $jwk = null;
        foreach ($set['keys'] ?? [] as $k) {
            if (($k['kty'] ?? '') === 'RSA' && (!isset($hdr['kid']) || ($k['kid'] ?? null) === $hdr['kid'])) {
                $jwk = $k;
                break;
            }
        }
        if ($jwk === null) {
            throw new DecryptError('signing key not in the JWKS');
        }
        $rsa = self::der(0x30, self::derInt(self::b64($jwk['n'])) . self::derInt(self::b64($jwk['e'])));
        $spki = self::der(0x30, self::der(0x30, "\x06\x09\x2a\x86\x48\x86\xf7\x0d\x01\x01\x01\x05\x00") . self::der(0x03, "\x00" . $rsa));
        if (openssl_verify($parts[0] . '.' . $parts[1], self::b64($parts[2]), self::pem('PUBLIC KEY', $spki), OPENSSL_ALGO_SHA256) !== 1) {
            throw new DecryptError('invalid ID token signature');
        }
        $claims = json_decode(self::b64($parts[1]), true);
        $aud = $claims['aud'] ?? null;
        $now = time();
        if (!is_array($claims) || ($issuer !== null && ($claims['iss'] ?? null) !== $issuer)) {
            throw new DecryptError('ID token issuer mismatch');
        }
        if ($audience !== null && $aud !== $audience && !(is_array($aud) && in_array($audience, $aud, true))) {
            throw new DecryptError('ID token audience mismatch');
        }
        if (!is_int($claims['exp'] ?? null) || $claims['exp'] + 60 < $now) {
            throw new DecryptError('ID token expired');
        }
        if (is_int($claims['nbf'] ?? null) && $claims['nbf'] - 60 > $now) {
            throw new DecryptError('ID token not valid yet');
        }
        if (func_num_args() >= 5 && ($expectedNonce === null || $expectedNonce === '' || !is_string($claims['nonce'] ?? null)
            || !hash_equals($expectedNonce, $claims['nonce']))) {
            throw new DecryptError('ID token nonce mismatch');
        }
        return $claims;
    }

    /**
     * Open a signed-then-encrypted ID token (id_token_encrypted_response_alg) and verify the inner token,
     * including the optional expected nonce after signature, issuer, audience and time validation.
     *
     * @param array<string, mixed>|string $jwks
     * @return array<string, mixed>
     */
    public static function decryptIdToken(string $token, array $keys, array|string $jwks, ?string $issuer = null, ?string $audience = null, ?string $expectedNonce = null): array
    {
        [$inner] = self::decrypt($token, $keys);
        if (func_num_args() < 6) {
            return self::verifyIdToken($inner, $jwks, $issuer, $audience);
        }
        return self::verifyIdToken($inner, $jwks, $issuer, $audience, $expectedNonce);
    }

    /**
     * Open the plugin-API envelope (data {"jwe"}, or the whole response body) and return the decoded
     * plaintext. $ctx, when set, must equal the header ctx. A plain body is refused (fail closed).
     *
     * @param array<string, mixed>|string $response
     */
    public static function openServerData(array|string $response, array $keys, ?string $ctx = null): mixed
    {
        $obj = is_string($response) ? json_decode($response, true) : $response;
        $jwe = is_array($obj) ? ($obj['jwe'] ?? ($obj['data']['jwe'] ?? null)) : null;
        if (!is_string($jwe)) {
            throw new DecryptError('response is not encrypted');
        }
        [$plain, $hdr] = self::decrypt($jwe, $keys);
        if ($ctx !== null && ($hdr['ctx'] ?? null) !== $ctx) {
            throw new DecryptError('envelope ctx mismatch');
        }
        return json_decode($plain, true, 512, JSON_THROW_ON_ERROR);
    }

    /**
     * Open an encrypted webhook body ({"id","event","timestamp","jwe"}); verify the signature with
     * Webhook::verify first. Returns the inner document, which must repeat id, event and timestamp.
     *
     * @return array<string, mixed>
     */
    public static function openWebhook(string $body, array $keys): array
    {
        $outer = json_decode($body, true);
        if (!is_array($outer) || !is_string($outer['jwe'] ?? null)) {
            throw new DecryptError('webhook is not encrypted');
        }
        [$plain, $hdr] = self::decrypt($outer['jwe'], $keys);
        if (!str_starts_with((string) ($hdr['ctx'] ?? ''), 'webhook:')) {
            throw new DecryptError('envelope ctx mismatch');
        }
        $doc = json_decode($plain, true);
        foreach (['id', 'event', 'timestamp'] as $k) {
            if (!is_array($doc) || ($doc[$k] ?? null) !== ($outer[$k] ?? null)) {
                throw new DecryptError('webhook envelope does not match its content');
            }
        }
        return $doc;
    }

    /** @param array<string, string> $jwk SEC1 ECPrivateKey (RFC 5915) for P-256. */
    private static function ecPrivateDer(array $jwk): string
    {
        $point = "\x04" . self::pad(self::b64($jwk['x'])) . self::pad(self::b64($jwk['y']));
        return self::der(0x30, "\x02\x01\x01" . self::der(0x04, self::pad(self::b64($jwk['d'])))
            . "\xa0\x0a\x06\x08\x2a\x86\x48\xce\x3d\x03\x01\x07" . self::der(0xa1, self::der(0x03, "\x00" . $point)));
    }

    /** @param array<string, mixed> $epk SubjectPublicKeyInfo of a P-256 point. */
    private static function ecPublicDer(array $epk): string
    {
        $point = "\x04" . self::pad(self::b64((string) $epk['x'])) . self::pad(self::b64((string) $epk['y']));
        return self::der(0x30, "\x30\x13\x06\x07\x2a\x86\x48\xce\x3d\x02\x01\x06\x08\x2a\x86\x48\xce\x3d\x03\x01\x07" . self::der(0x03, "\x00" . $point));
    }

    private static function der(int $tag, string $body): string
    {
        $len = strlen($body);
        if ($len < 0x80) {
            return chr($tag) . chr($len) . $body;
        }
        $bytes = ltrim(pack('N', $len), "\0");
        return chr($tag) . chr(0x80 | strlen($bytes)) . $bytes . $body;
    }

    private static function derInt(string $unsigned): string
    {
        $v = ltrim($unsigned, "\0");
        if ($v === '' || (ord($v[0]) & 0x80) !== 0) {
            $v = "\0" . $v;
        }
        return self::der(0x02, $v);
    }

    private static function pad(string $b): string
    {
        return str_pad($b, 32, "\0", STR_PAD_LEFT);
    }

    private static function pem(string $label, string $der): string
    {
        return "-----BEGIN {$label}-----\n" . chunk_split(base64_encode($der), 64, "\n") . "-----END {$label}-----\n";
    }

    private static function lp(string $b): string
    {
        return pack('N', strlen($b)) . $b;
    }

    private static function b64(string $s): string
    {
        $v = base64_decode(strtr($s, '-_', '+/') . str_repeat('=', (4 - strlen($s) % 4) % 4), true);
        if ($v === false) {
            throw new DecryptError();
        }
        return $v;
    }
}
