<?php

declare(strict_types=1);

namespace Lounge\Sdk\Exceptions;

/** Webhook signature, timestamp or body verification failed. Answer such requests with HTTP 400. */
class WebhookError extends LoungeException
{
    public function __construct(string $message, ?\Throwable $previous = null)
    {
        parent::__construct('lounge: ' . $message, 0, $previous);
    }
}
