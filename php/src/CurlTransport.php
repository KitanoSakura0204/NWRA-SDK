<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Exceptions\TransportError;

/**
 * Default HTTP transport built on ext-curl.
 *
 * A transport is any callable with the signature
 * `fn(string $method, string $url, array<string,string> $headers, ?string $body, float $timeout): array{int, string}`
 * returning the HTTP status code and the raw response body (also for 4xx/5xx answers). Network failures must be
 * thrown as TransportError; other exceptions are wrapped into TransportError by the SDK. Inject your own
 * (Guzzle, Symfony HttpClient, a fake in tests) through the `$transport` constructor argument of
 * LoungeClient / LoungeOidc.
 */
final class CurlTransport
{
    /**
     * @param array<int, mixed> $options extra `CURLOPT_*` values applied to every request
     *                                   (e.g. `CURLOPT_PROXY`, `CURLOPT_CAINFO`, `CURLOPT_USERAGENT`)
     */
    public function __construct(private array $options = [])
    {
        if (!\function_exists('curl_init')) {
            throw new \RuntimeException('lounge: ext-curl is required by the default transport; install it or inject another transport');
        }
    }

    /**
     * @param array<string, string> $headers
     *
     * @return array{int, string} HTTP status and raw body
     *
     * @throws TransportError with the cURL errno as code (e.g. 6 DNS, 7 connect, 28 timeout, 60 TLS)
     */
    public function __invoke(string $method, string $url, array $headers, ?string $body, float $timeout): array
    {
        $ch = curl_init();
        if ($ch === false) {
            throw new TransportError('curl_init failed');
        }
        $lines = [];
        foreach ($headers as $name => $value) {
            $lines[] = $name . ': ' . $value;
        }
        $lines[] = 'Expect:'; // never wait for a 100-continue on request bodies
        $ms = max(1, (int) round($timeout * 1000));
        $required = [
            CURLOPT_URL => $url,
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HEADER => false,
            CURLOPT_HTTPHEADER => $lines,
        ];
        if ($body !== null) {
            $required[CURLOPT_POSTFIELDS] = $body;
        }
        $defaults = [
            CURLOPT_CONNECTTIMEOUT_MS => $ms,
            CURLOPT_TIMEOUT_MS => $ms,
            CURLOPT_FOLLOWLOCATION => false,
            CURLOPT_FAILONERROR => false,
        ];
        try {
            // precedence: required > caller options > defaults
            if (!curl_setopt_array($ch, $required + $this->options + $defaults)) {
                throw new TransportError('invalid curl option: ' . curl_error($ch), curl_errno($ch));
            }
            $raw = curl_exec($ch);
            if ($raw === false) {
                $errno = curl_errno($ch);
                throw new TransportError(sprintf('curl error %d: %s', $errno, curl_error($ch)), $errno);
            }
            $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);

            return [$status, is_string($raw) ? $raw : ''];
        } finally {
            curl_close($ch);
        }
    }
}
