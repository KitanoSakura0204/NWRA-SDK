<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Exceptions\WebhookError;
use NoWork\Sdk\Internal\Json;

/**
 * Verification of webhook deliveries from the user center (docs/PROTOCOL.md section 5).
 *
 * `X-NoWork-Signature` = `sha256=` + hex(HMAC-SHA256(secret, "<X-NoWork-Timestamp>.<raw body>")).
 * The comparison is constant-time and deliveries outside the replay window are rejected.
 */
final class Webhook
{
    /** Default replay window in seconds. */
    public const DEFAULT_TOLERANCE = 300;

    private function __construct()
    {
    }

    /** sha256=HMAC_SHA256(secret, "<ts>.<body>") */
    public static function signature(string $secret, int|string $timestamp, string $body): string
    {
        return 'sha256=' . hash_hmac('sha256', $timestamp . '.' . $body, $secret);
    }

    /**
     * Verify the signature and timestamp of a delivery and parse the event.
     *
     * @param array<string, string|string[]|mixed> $headers   request headers, case-insensitive; both `X-NoWork-Timestamp`
     *                                                        and `$_SERVER`-style `HTTP_X_NOWORK_TIMESTAMP` keys are accepted
     * @param string                               $body      the raw request body, exactly as received
     * @param int|null                             $tolerance replay window in seconds; null or 0 disables the check
     *
     * @throws WebhookError when the timestamp is missing / stale, the signature does not match, or the body is not a JSON object
     */
    public static function verify(string $secret, array $headers, string $body, ?int $tolerance = self::DEFAULT_TOLERANCE): WebhookEvent
    {
        $lookup = self::normalize($headers);
        $tsRaw = trim($lookup['x-nowork-timestamp'] ?? '');
        if (preg_match('/^[+-]?\d{1,18}$/', $tsRaw) !== 1) {
            throw new WebhookError('missing or invalid X-NoWork-Timestamp');
        }
        $ts = (int) $tsRaw;
        if ($tolerance !== null && $tolerance > 0 && abs(time() - $ts) > $tolerance) {
            throw new WebhookError('webhook timestamp outside tolerance');
        }
        $expected = self::signature($secret, $ts, $body);
        $given = trim($lookup['x-nowork-signature'] ?? '');
        if (!hash_equals($expected, $given)) {
            throw new WebhookError('invalid webhook signature');
        }
        $payload = json_decode($body, true);
        if (json_last_error() !== JSON_ERROR_NONE || !Json::isObject($payload)) {
            throw new WebhookError('invalid webhook body');
        }

        /** @var array<string, mixed> $payload */
        return WebhookEvent::fromArray($payload);
    }

    /**
     * Verify the request currently being handled by PHP (`$_SERVER` headers + `php://input`).
     *
     * @param array<string, mixed>|null $server defaults to `$_SERVER`
     * @param string|null               $body   defaults to the raw request body
     */
    public static function verifyRequest(string $secret, ?int $tolerance = self::DEFAULT_TOLERANCE, ?array $server = null, ?string $body = null): WebhookEvent
    {
        $server ??= $_SERVER;
        $body ??= (string) file_get_contents('php://input');

        return self::verify($secret, $server, $body, $tolerance);
    }

    /**
     * Lower-case header names, map `HTTP_X_NOWORK_*` (CGI style) to `x-nowork-*`, take the first value of multi-value headers.
     *
     * @param array<mixed, mixed> $headers
     *
     * @return array<string, string>
     */
    private static function normalize(array $headers): array
    {
        $out = [];
        foreach ($headers as $name => $value) {
            if (!is_string($name)) {
                continue;
            }
            if (is_array($value)) {
                $value = $value === [] ? '' : reset($value);
            }
            if (!is_scalar($value)) {
                continue;
            }
            $key = strtolower(str_replace('_', '-', $name));
            if (str_starts_with($key, 'http-')) {
                $key = substr($key, 5);
            }
            $out[$key] = (string) $value;
        }

        return $out;
    }
}
