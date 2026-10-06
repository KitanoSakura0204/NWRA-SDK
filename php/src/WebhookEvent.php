<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Internal\Json;

/** A verified webhook envelope: `{ id, event, timestamp, data }`. Deliveries are retried, so de-duplicate on `id`. */
final class WebhookEvent
{
    /** @param array<string, mixed> $data event payload, e.g. `userId`, `uid`, `username`, `ip` for `user.login` */
    public function __construct(
        public readonly string $id,
        public readonly string $event,
        public readonly int $timestamp,
        public readonly array $data,
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        $data = $d['data'] ?? null;

        return new self(
            Json::str($d['id'] ?? ''),
            Json::str($d['event'] ?? ''),
            Json::int($d['timestamp'] ?? 0),
            is_array($data) ? $data : [],
        );
    }
}
