<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Internal\Json;

/** An active ban on this server with the account's bindings there (SERVER_USERS.md). */
final class ServerBan
{
    /** A ban an administrator placed: only an administrator can change it (ApiError code). */
    public const CODE_ADMIN_BAN = 403;

    /**
     * @param list<string> $names
     * @param list<string> $uuids
     * @param string|null  $expireTime null = permanent
     * @param string       $source     admin | owner | plugin
     */
    public function __construct(
        public readonly int $uid = 0,
        public readonly array $names = [],
        public readonly array $uuids = [],
        public readonly string $reason = '',
        public readonly ?string $expireTime = null,
        public readonly string $createTime = '',
        public readonly string $source = '',
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        $list = static fn (mixed $v): array => is_array($v) ? array_values(array_map(static fn ($x): string => Json::str($x), $v)) : [];

        return new self(
            Json::int($d['uid'] ?? 0),
            $list($d['names'] ?? null),
            $list($d['uuids'] ?? null),
            Json::str($d['reason'] ?? ''),
            isset($d['expireTime']) ? Json::str($d['expireTime']) : null,
            Json::str($d['createTime'] ?? ''),
            Json::str($d['source'] ?? ''),
        );
    }
}
