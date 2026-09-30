<?php

declare(strict_types=1);

namespace Lounge\Sdk;

/** One entry of the heartbeat player list (PROTOCOL 2.3): name required, uuid optional. */
final class OnlinePlayer implements \JsonSerializable
{
    public function __construct(
        public readonly string $name,
        public readonly string $uuid = '',
    ) {
    }

    /** @return array{name: string, uuid?: string} */
    public function toArray(): array
    {
        return $this->uuid !== '' ? ['name' => $this->name, 'uuid' => $this->uuid] : ['name' => $this->name];
    }

    /** @return array{name: string, uuid?: string} */
    public function jsonSerialize(): array
    {
        return $this->toArray();
    }
}
