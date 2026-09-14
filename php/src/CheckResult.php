<?php

declare(strict_types=1);

namespace Lounge\Sdk;

use Lounge\Sdk\Internal\Json;

/**
 * Answer of a join check (`GET /server/v1/whitelist/check`).
 *
 * `reason` is one of `not bound`, `pending review`, `banned`, `account locked`, `whitelist disabled` (or empty).
 * A server with the whitelist disabled answers `allowed = true, bound = false`.
 */
final class CheckResult
{
    public function __construct(
        public readonly bool $allowed = false,
        public readonly bool $bound = false,
        public readonly string $reason = '',
        public readonly ?WhitelistEntry $player = null,
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        $player = $d['player'] ?? null;

        return new self(
            (bool) ($d['allowed'] ?? false),
            (bool) ($d['bound'] ?? false),
            Json::str($d['reason'] ?? ''),
            is_array($player) && $player !== [] ? WhitelistEntry::fromArray($player) : null,
        );
    }
}
