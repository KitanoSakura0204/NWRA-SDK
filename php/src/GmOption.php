<?php

declare(strict_types=1);

namespace NoWork\Sdk;

/** One choice of a select parameter or of a named option list (PROTOCOL 2.4). */
final class GmOption implements \JsonSerializable
{
    public function __construct(
        public readonly string $value,
        public readonly string $label = '',
    ) {
    }

    /** @return array{value: string, label?: string} */
    public function toArray(): array
    {
        return $this->label !== '' ? ['value' => $this->value, 'label' => $this->label] : ['value' => $this->value];
    }

    /** @return array{value: string, label?: string} */
    public function jsonSerialize(): array
    {
        return $this->toArray();
    }
}
