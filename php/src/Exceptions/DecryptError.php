<?php

declare(strict_types=1);

namespace NoWork\Sdk\Exceptions;

/** A delivery JWE could not be opened (wrong key, tampered, unsupported) or a check on it failed. */
class DecryptError extends NoWorkException
{
    public function __construct(string $message = 'cannot decrypt the JWE', ?\Throwable $previous = null)
    {
        parent::__construct('nowork: ' . $message, 0, $previous);
    }
}
