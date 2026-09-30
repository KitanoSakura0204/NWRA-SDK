<?php

declare(strict_types=1);

namespace Lounge\Sdk;

/**
 * A GM action the plugin offers to the admin panel (PROTOCOL 2.4).
 * key: no colon, no whitespace, at most 64 bytes. danger: `normal` | `confirm` | `critical` (the last two
 * need a confirmation before the command is sent).
 */
final class GmAction implements \JsonSerializable
{
    /**
     * @param list<GmActionParam|array<string, mixed>> $params
     * @param array<string, string>                    $labels per-locale titles, e.g. `['en' => 'Kick']`
     * @param string                                   $category catalogue group
     */
    public function __construct(
        public readonly string $key,
        public readonly string $title,
        public readonly string $description = '',
        public readonly array $params = [],
        public readonly array $labels = [],
        public readonly string $category = '',
        public readonly string $danger = '',
    ) {
    }

    /** @return array<string, mixed> */
    public function toArray(): array
    {
        $out = ['key' => $this->key, 'title' => $this->title];
        if ($this->description !== '') {
            $out['description'] = $this->description;
        }
        if ($this->params !== []) {
            $out['params'] = array_map(static fn ($p) => $p instanceof GmActionParam ? $p->toArray() : $p, array_values($this->params));
        }
        if ($this->labels !== []) {
            $out['labels'] = $this->labels;
        }
        if ($this->category !== '') {
            $out['category'] = $this->category;
        }
        if ($this->danger !== '') {
            $out['danger'] = $this->danger;
        }

        return $out;
    }

    /** @return array<string, mixed> */
    public function jsonSerialize(): array
    {
        return $this->toArray();
    }
}
