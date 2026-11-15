<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Exceptions\TransportError;

/**
 * Default HTTP transport built on ext-curl.
 *
 * A transport is any callable with the signature
 * `fn(string $method, string $url, array<string,string> $headers, ?string $body, float $timeout): array{int, string}`
 * returning the HTTP status code and the raw response body (also for 4xx/5xx answers). A transport may add the
 * response headers as a third element (`array<string, string>`, any name case): the Server API client reads
 * `X-NoWork-Encryption` from them, and falls back to the body shape without them. Network failures must be
 * thrown as TransportError; other exceptions are wrapped into TransportError by the SDK. Inject your own
 * (Guzzle, Symfony HttpClient, a fake in tests) through the `$transport` constructor argument of
 * NoworkClient / NoworkOidc.
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
            throw new \RuntimeException('nowork: ext-curl is required by the default transport; install it or inject another transport');
        }
    }

    /**
     * @param array<string, string> $headers
     *
     * @return array{int, string, array<string, string>} HTTP status, raw body and response headers (names lower-cased)
     *
     * @throws TransportError with the cURL errno as code (e.g. 6 DNS, 7 connect, 28 timeout, 60 TLS)
     */
    public function __invoke(string $method, string $url, array $headers, ?string $body, float $timeout): array
    {
        return $this->request($method, $url, $headers, $body, $timeout, null);
    }

    /**
     * Bound a metadata download while reading and disable redirects / cURL origin authentication.
     * Caller timeout/proxy/TLS options remain in effect, as does an existing write callback's stricter limit.
     * Custom transport callables retain the five-argument __invoke contract.
     *
     * @param array<string, string> $headers
     * @return array{int, string, array<string, string>}
     */
    public function sendBounded(string $method, string $url, array $headers, ?string $body, float $timeout, int $maxResponseBytes): array
    {
        if ($maxResponseBytes < 1) {
            throw new \InvalidArgumentException('response byte limit must be positive');
        }

        return $this->request($method, $url, $headers, $body, $timeout, $maxResponseBytes);
    }

    /** @param array<string, string> $headers
     *  @return array{int, string, array<string, string>}
     */
    private function request(string $method, string $url, array $headers, ?string $body, float $timeout, ?int $maxResponseBytes): array
    {
        $ch = curl_init();
        if ($ch === false) {
            throw new TransportError('curl_init failed');
        }
        $received = [];
        $onHeader = static function ($handle, string $line) use (&$received): int {
            if (str_starts_with($line, 'HTTP/')) {
                $received = []; // a new status line (redirect / 100 Continue): keep only the final headers
            } elseif (($p = strpos($line, ':')) !== false) {
                $received[strtolower(trim(substr($line, 0, $p)))] = trim(substr($line, $p + 1));
            }

            return strlen($line);
        };
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
            CURLOPT_HEADERFUNCTION => $onHeader,
        ];
        if ($body !== null) {
            $required[CURLOPT_POSTFIELDS] = $body;
        }
        $download = '';
        $exceeded = false;
        if ($maxResponseBytes !== null) {
            $onWrite = $this->options[CURLOPT_WRITEFUNCTION] ?? null;
            $required[CURLOPT_RETURNTRANSFER] = false;
            $required[CURLOPT_FOLLOWLOCATION] = false;
            $required[CURLOPT_HTTPAUTH] = CURLAUTH_NONE;
            $required[CURLOPT_USERPWD] = null;
            $required[CURLOPT_USERNAME] = '';
            $required[CURLOPT_PASSWORD] = '';
            if (defined('CURLOPT_XOAUTH2_BEARER')) {
                $required[CURLOPT_XOAUTH2_BEARER] = null;
            }
            $required[CURLOPT_NETRC] = CURL_NETRC_IGNORED;
            if ($method === 'GET' && $body === null) {
                $required[CURLOPT_POSTFIELDS] = null;
                $required[CURLOPT_HTTPGET] = true;
            }
            $required[CURLOPT_WRITEFUNCTION] = static function ($handle, string $chunk) use (&$download, &$exceeded, $maxResponseBytes, $onWrite): int {
                $length = strlen($chunk);
                if ($length > $maxResponseBytes - strlen($download)) {
                    $exceeded = true;
                    return 0;
                }
                if ($onWrite !== null) {
                    $accepted = $onWrite($handle, $chunk);
                    if ($accepted !== $length) {
                        return $accepted;
                    }
                }
                $download .= $chunk;
                return $length;
            };
        }
        $defaults = [
            CURLOPT_CONNECTTIMEOUT_MS => $ms,
            CURLOPT_TIMEOUT_MS => $ms,
            CURLOPT_FOLLOWLOCATION => false,
            CURLOPT_FAILONERROR => false,
        ];
        try {
            // Bounded calls set required options last: CURLOPT_FILE and other caller options can otherwise
            // replace PHP's write callback as a side effect and bypass the download limit.
            // Effective precedence remains required > caller options > defaults.
            $configured = $maxResponseBytes === null
                ? curl_setopt_array($ch, $required + $this->options + $defaults)
                : curl_setopt_array($ch, $this->options + $defaults) && curl_setopt_array($ch, $required);
            if (!$configured) {
                throw new TransportError('invalid curl option: ' . curl_error($ch), curl_errno($ch));
            }
            $raw = curl_exec($ch);
            if ($raw === false) {
                $errno = curl_errno($ch);
                if ($exceeded) {
                    throw new TransportError('metadata response exceeds ' . $maxResponseBytes . ' bytes', $errno);
                }
                throw new TransportError(sprintf('curl error %d: %s', $errno, curl_error($ch)), $errno);
            }
            $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);

            return [$status, $maxResponseBytes !== null ? $download : (is_string($raw) ? $raw : ''), $received];
        } finally {
            curl_close($ch);
        }
    }
}
