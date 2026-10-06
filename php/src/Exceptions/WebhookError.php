<?php

declare(strict_types=1);

namespace NoWork\Sdk\Exceptions;

/** Webhook signature, timestamp or body verification failed. Answer such requests with HTTP 400. */
class WebhookError extends NoWorkException
{
    public function __construct(string $message, ?\Throwable $previous = null)
    {
        parent::__construct('nowork: ' . $message, 0, $previous);
    }
}
