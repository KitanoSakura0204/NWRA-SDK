<?php

declare(strict_types=1);

namespace NoWork\Sdk;

/**
 * A Server API reply as received, `data` left undecoded (see NoworkClient::raw). Pass `$body` or `$envelope`
 * to Delivery::openServerData to open an encrypted one yourself.
 */
final class RawResponse
{
    /**
     * @param array<string, string> $headers  response headers, names lower-cased (empty when the transport
     *                                        does not report headers)
     * @param array<string, mixed>  $envelope the whole parsed body `{code, message, data}`
     * @param bool                  $encrypted `X-NoWork-Encryption` is set or `data` is exactly `{"jwe": "<string>"}`
     */
    public function __construct(
        public readonly int $status,
        public readonly array $headers,
        public readonly string $message,
        public readonly mixed $data,
        public readonly array $envelope,
        public readonly string $body,
        public readonly bool $encrypted,
    ) {
    }

    /** A response header by name (case-insensitive), '' when absent. */
    public function header(string $name): string
    {
        return $this->headers[strtolower($name)] ?? '';
    }
}
