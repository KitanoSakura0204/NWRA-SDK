<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Internal\Json;

/**
 * What a GM publish stored; entries the server could not accept are skipped and listed, never a failure for
 * the whole call.
 */
final class GmPublishResult
{
    /** @param list<array{key: string, reason: string}> $skipped */
    public function __construct(
        public readonly int $count = 0,
        public readonly array $skipped = [],
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        $skipped = [];
        foreach (is_array($d['skipped'] ?? null) ? $d['skipped'] : [] as $s) {
            if (is_array($s)) {
                $skipped[] = ['key' => Json::str($s['key'] ?? ''), 'reason' => Json::str($s['reason'] ?? '')];
            }
        }

        return new self(Json::int($d['count'] ?? 0), $skipped);
    }
}
