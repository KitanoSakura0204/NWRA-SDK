<?php

declare(strict_types=1);

namespace Lounge\Sdk\Internal;

use Lounge\Sdk\Exceptions\LoungeException;
use Lounge\Sdk\Exceptions\TransportError;

/**
 * JSON / transport plumbing shared by LoungeClient and LoungeOidc.
 *
 * @internal not part of the public API
 */
final class Json
{
    private function __construct()
    {
    }

    /**
     * Invoke a transport callable and normalise its `[status, body]` answer. Any exception that is not
     * already a LoungeException is reported as a TransportError so callers only ever see the SDK's types.
     *
     * @param callable(string, string, array<string, string>, ?string, float): array{int, string} $transport
     * @param array<string, string>                                                                 $headers
     *
     * @return array{int, string}
     */
    public static function send(callable $transport, string $method, string $url, array $headers, ?string $body, float $timeout): array
    {
        try {
            $res = $transport($method, $url, $headers, $body, $timeout);
        } catch (LoungeException $e) {
            throw $e;
        } catch (\Exception $e) {
            throw new TransportError($e->getMessage(), 0, $e);
        }
        if (!is_array($res) || !array_key_exists(0, $res) || !array_key_exists(1, $res)) {
            throw new TransportError('transport returned an invalid response');
        }

        return [(int) $res[0], (string) $res[1]];
    }

    /**
     * Decode a JSON object into an associative array; a JSON `{}` decodes to `[]`.
     *
     * @return array<string, mixed>
     *
     * @throws TransportError when the body is not valid JSON or not an object
     */
    public static function decodeObject(string $raw, int $status): array
    {
        $value = json_decode($raw, true);
        if (json_last_error() !== JSON_ERROR_NONE) {
            throw new TransportError(sprintf('invalid json (http %d): %s', $status, json_last_error_msg()));
        }
        if (!self::isObject($value)) {
            throw new TransportError(sprintf('invalid response (http %d): not an object', $status));
        }

        /** @var array<string, mixed> $value */
        return $value;
    }

    /**
     * Whether a decoded value came from a JSON object. PHP decodes both `{}` and `[]` to an empty array,
     * so an empty array is accepted; non-empty lists are rejected.
     */
    public static function isObject(mixed $value): bool
    {
        return is_array($value) && ($value === [] || !array_is_list($value));
    }

    public static function str(mixed $value, string $default = ''): string
    {
        return is_scalar($value) ? (string) $value : $default;
    }

    public static function int(mixed $value, int $default = 0): int
    {
        if (is_int($value)) {
            return $value;
        }
        if (is_bool($value) || is_float($value) || (is_string($value) && is_numeric($value))) {
            return (int) $value;
        }

        return $default;
    }
}
