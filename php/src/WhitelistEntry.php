<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Internal\Json;

/** One approved binding between a user-center account and an in-game identity. */
final class WhitelistEntry
{
    public function __construct(
        public readonly string $name = '',
        public readonly string $uuid = '',
        public readonly int $uid = 0,
        public readonly string $nickname = '',
        public readonly int $securityLevel = 0,
        public readonly string $since = '',
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        return new self(
            Json::str($d['name'] ?? ''),
            Json::str($d['uuid'] ?? ''),
            Json::int($d['uid'] ?? 0),
            Json::str($d['nickname'] ?? ''),
            Json::int($d['securityLevel'] ?? 0),
            Json::str($d['since'] ?? ''),
        );
    }
}
