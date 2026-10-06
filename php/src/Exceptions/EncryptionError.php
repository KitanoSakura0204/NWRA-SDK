<?php

declare(strict_types=1);

namespace NoWork\Sdk\Exceptions;

/**
 * A Server API reply came encrypted (`X-NoWork-Encryption`, or `data` is exactly `{"jwe": "..."}`) and could
 * not be opened: no decryption key configured, the JWE did not open, its `ctx` names another route, or a
 * whitelist reply came plain while decryption keys are configured (no downgrade). Fail closed: treat it
 * like a refused check, never as "allowed". Not a TransportError, so a fail-open network policy does not
 * swallow it by accident.
 */
class EncryptionError extends NoWorkException
{
    public function __construct(private string $route, private string $reason, ?\Throwable $previous = null)
    {
        parent::__construct(sprintf('nowork: encrypted response on %s: %s', $route, $reason), 0, $previous);
    }

    /** Server API route without the `/server/v1/` prefix, e.g. `whitelist/check`. */
    public function getRoute(): string
    {
        return $this->route;
    }

    public function getReason(): string
    {
        return $this->reason;
    }
}
