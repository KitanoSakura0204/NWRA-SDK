<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Exceptions\ApiError;
use Lounge\Sdk\Exceptions\TransportError;
use Lounge\Sdk\Internal\Json;

/**
 * Server API client authenticated with an `X-Server-Key` (whitelist sync, join check, heartbeat).
 *
 * Errors: network / timeout / malformed envelope -> TransportError, `code != 0` -> ApiError.
 */
final class LoungeClient
{
    private string $base;

    /** @var callable(string, string, array<string, string>, ?string, float): array{int, string} */
    private $transport;

    /**
     * @param string        $base      user-center base URL, e.g. `https://account.example.com`
     * @param string        $serverKey `lsk_...` issued by the admin console
     * @param float         $timeout   per-request timeout in seconds (connect + read)
     * @param callable|null $transport custom HTTP transport, see CurlTransport for the contract
     */
    public function __construct(string $base, private string $serverKey, private float $timeout = 10.0, ?callable $transport = null)
    {
        $this->base = rtrim($base, '/');
        $this->transport = $transport ?? new CurlTransport();
    }

    /** Fetch every approved binding of this server (full sync at start-up / on a timer). */
    public function whitelist(): Whitelist
    {
        return Whitelist::fromArray($this->call('GET', '/server/v1/whitelist'));
    }

    /** Validate a joining player by name and/or uuid (at least one is required; either match is accepted). */
    public function checkWhitelist(string $name = '', string $uuid = ''): CheckResult
    {
        return CheckResult::fromArray($this->call('GET', '/server/v1/whitelist/check', ['name' => $name, 'uuid' => $uuid]));
    }

    /** Report live status (roughly every 60s; 5 minutes without a heartbeat marks the server offline). */
    public function heartbeat(int $online, int $max = 0, string $version = ''): void
    {
        $this->call('POST', '/server/v1/heartbeat', [], ['online' => $online, 'max' => $max, 'version' => $version]);
    }

    /**
     * @param array<string, string|null> $query
     * @param array<string, mixed>|null  $body
     *
     * @return array<string, mixed> the `data` object of a successful envelope
     */
    private function call(string $method, string $path, array $query = [], ?array $body = null): array
    {
        $url = $this->base . $path;
        $query = array_filter($query, static fn ($v): bool => $v !== null && $v !== '');
        if ($query !== []) {
            $url .= '?' . http_build_query($query);
        }
        $headers = ['Accept' => 'application/json', 'X-Server-Key' => $this->serverKey];
        $raw = null;
        if ($body !== null) {
            $raw = json_encode($body, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE);
            if ($raw === false) {
                throw new TransportError('cannot encode request body: ' . json_last_error_msg());
            }
            $headers['Content-Type'] = 'application/json';
        }
        [$status, $text] = Json::send($this->transport, $method, $url, $headers, $raw, $this->timeout);

        return self::unwrap($status, $text);
    }

    /**
     * Validate the `{code, message, data}` envelope: missing / non-integer `code`, missing or null `data`
     * and HTTP >= 400 with `code = 0` are transport problems; `code != 0` is a business error.
     *
     * @return array<string, mixed>
     */
    private static function unwrap(int $status, string $raw): array
    {
        $env = Json::decodeObject($raw, $status);
        $code = $env['code'] ?? null;
        if (!is_int($code)) {
            throw new TransportError(sprintf('invalid response (http %d): missing code', $status));
        }
        if ($code !== 0) {
            throw new ApiError($code, Json::str($env['message'] ?? ''));
        }
        if ($status >= 400) {
            throw new TransportError(sprintf('http %d', $status));
        }
        $data = $env['data'] ?? null;
        if (!Json::isObject($data)) {
            throw new TransportError(sprintf('invalid response (http %d): missing data', $status));
        }

        /** @var array<string, mixed> $data */
        return $data;
    }
}
