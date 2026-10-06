<?php

declare(strict_types=1);

namespace NoWork\Sdk\Exceptions;

/**
 * Network, timeout or response-decoding failure (including malformed success envelopes).
 * The caller decides fail-open vs fail-closed; getCode() carries the cURL errno when one is known.
 */
class TransportError extends NoWorkException
{
    public function __construct(string $message, int $code = 0, ?\Throwable $previous = null)
    {
        parent::__construct('nowork: transport error: ' . $message, $code, $previous);
    }
}
