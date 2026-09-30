<?php

declare(strict_types=1);

namespace Lounge\Sdk;

/**
 * One input of a GM action (PROTOCOL 2.4). Only name / type / required / label existed before 2026-09-28;
 * every other field is optional and left out of the wire form when unset.
 *
 * type: `string` | `text` | `number` | `boolean` | `select` | `player` (aliases `int` / `integer` / `bool` / `textarea`).
 */
final class GmActionParam implements \JsonSerializable
{
    /**
     * @param mixed                            $default   ignored by the server when it does not fit the definition
     * @param float|int|null                   $min       number only
     * @param int                              $maxLength text length limit; 0 = server default (1000, text 4000, player 64)
     * @param list<GmOption|array<string, mixed>> $options inline select choices (1-200, unique values)
     * @param string                           $source    names an option list published with publishGmCatalog / publishGmOptions
     */
    public function __construct(
        public readonly string $name,
        public readonly string $type = 'string',
        public readonly bool $required = false,
        public readonly string $label = '',
        public readonly string $description = '',
        public readonly string $placeholder = '',
        public readonly mixed $default = null,
        public readonly float|int|null $min = null,
        public readonly float|int|null $max = null,
        public readonly float|int|null $step = null,
        public readonly bool $integer = false,
        public readonly int $maxLength = 0,
        public readonly array $options = [],
        public readonly string $source = '',
    ) {
    }

    /** @return array<string, mixed> */
    public function toArray(): array
    {
        $out = ['name' => $this->name, 'type' => $this->type, 'required' => $this->required, 'label' => $this->label];
        foreach (['description' => $this->description, 'placeholder' => $this->placeholder, 'source' => $this->source] as $k => $v) {
            if ($v !== '') {
                $out[$k] = $v;
            }
        }
        // 0 and false are real values for these, so only null means "unset"
        foreach (['default' => $this->default, 'min' => $this->min, 'max' => $this->max, 'step' => $this->step] as $k => $v) {
            if ($v !== null) {
                $out[$k] = $v;
            }
        }
        if ($this->integer) {
            $out['integer'] = true;
        }
        if ($this->maxLength > 0) {
            $out['maxLength'] = $this->maxLength;
        }
        if ($this->options !== []) {
            $out['options'] = array_map(static fn ($o) => $o instanceof GmOption ? $o->toArray() : $o, array_values($this->options));
        }

        return $out;
    }

    /** @return array<string, mixed> */
    public function jsonSerialize(): array
    {
        return $this->toArray();
    }
}
