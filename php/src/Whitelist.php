<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Internal\Json;

/** The full whitelist of a server (`GET /server/v1/whitelist`). */
final class Whitelist
{
    /** Whitelist disabled: anyone may join. */
    public const MODE_OFF = 0;
    /** Bindings are approved manually by an administrator. */
    public const MODE_MANUAL = 1;
    /** Bindings are approved automatically once the account reaches the required security level. */
    public const MODE_AUTO = 2;

    /** @param WhitelistEntry[] $entries */
    public function __construct(
        public readonly string $server = '',
        public readonly int $mode = self::MODE_OFF,
        public readonly int $count = 0,
        public readonly array $entries = [],
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        $entries = [];
        foreach (is_array($d['entries'] ?? null) ? $d['entries'] : [] as $entry) {
            if (is_array($entry)) {
                $entries[] = WhitelistEntry::fromArray($entry);
            }
        }

        return new self(
            Json::str($d['server'] ?? ''),
            Json::int($d['mode'] ?? self::MODE_OFF),
            Json::int($d['count'] ?? 0),
            $entries,
        );
    }
}
