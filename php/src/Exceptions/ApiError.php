<?php

declare(strict_types=1);

namespace NoWork\Sdk\Exceptions;

/** Business error returned by the user center (`code != 0`, e.g. 401 for an invalid server key). */
class ApiError extends NoWorkException
{
    public function __construct(int $code, private string $apiMessage)
    {
        parent::__construct(sprintf('nowork: api error %d: %s', $code, $apiMessage), $code);
    }

    /** Human-readable reason from the response envelope (`message`); the code is available via getCode(). */
    public function getApiMessage(): string
    {
        return $this->apiMessage;
    }
}
