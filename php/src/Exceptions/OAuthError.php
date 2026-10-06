<?php

declare(strict_types=1);

namespace NoWork\Sdk\Exceptions;

/** RFC 6749 error object returned by the OIDC endpoints (`{"error": "...", "error_description": "..."}`). */
class OAuthError extends NoWorkException
{
    public function __construct(private string $error, private string $description = '', private int $status = 0)
    {
        parent::__construct(sprintf('nowork: oauth error %s: %s', $error, $description), $status);
    }

    /** The `error` code, e.g. `invalid_grant`, `invalid_client`, `invalid_token`. */
    public function getError(): string
    {
        return $this->error;
    }

    /** The optional `error_description`. */
    public function getDescription(): string
    {
        return $this->description;
    }

    /** HTTP status of the failing response (also returned by getCode()). */
    public function getStatus(): int
    {
        return $this->status;
    }
}
