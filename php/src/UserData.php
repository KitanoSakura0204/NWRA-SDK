<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Internal\Json;

/**
 * One account's data on this server (SERVER_USERS.md): every schema field with its stored value, else the
 * field default, else null. `$version` is the optimistic lock (0 = nothing stored yet).
 */
final class UserData
{
    /** A data write was refused because the data changed since the version you read (ApiError code). */
    public const CODE_VERSION_CONFLICT = 409;
    /** The uid has no binding and was never seen on this server (ApiError code). */
    public const CODE_NOT_ON_ROSTER = 404;

    /**
     * @param array<string, mixed> $data
     * @param string|null          $updatedAt "2006-01-02 15:04:05" server time; null before the first write
     */
    public function __construct(
        public readonly int $uid = 0,
        public readonly array $data = [],
        public readonly int $version = 0,
        public readonly ?string $updatedAt = null,
        public readonly string $updatedBy = '',
        public readonly int $size = 0,
    ) {
    }

    /** @param array<string, mixed> $d */
    public static function fromArray(array $d): self
    {
        $data = is_array($d['data'] ?? null) ? $d['data'] : [];

        return new self(
            Json::int($d['uid'] ?? 0),
            $data,
            Json::int($d['version'] ?? 0),
            isset($d['updatedAt']) ? Json::str($d['updatedAt']) : null,
            Json::str($d['updatedBy'] ?? ''),
            Json::int($d['size'] ?? 0),
        );
    }

    /** Whether a write failed because of a stale version: read again, then write. */
    public static function isVersionConflict(\Throwable $e): bool
    {
        return $e instanceof Exceptions\ApiError && $e->getCode() === self::CODE_VERSION_CONFLICT;
    }
}
