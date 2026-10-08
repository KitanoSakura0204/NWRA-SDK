<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Exceptions\ApiError;
use NoWork\Sdk\Exceptions\EncryptionError;
use NoWork\Sdk\Exceptions\TransportError;
use NoWork\Sdk\Internal\Json;

/**
 * Server API client authenticated with an `X-Server-Key` (whitelist sync, join check, heartbeat, GM channel).
 *
 * Errors: network / timeout / malformed envelope -> TransportError, `code != 0` -> ApiError, an encrypted
 * reply that cannot be opened (or a plain whitelist reply while decryption keys are set) -> EncryptionError.
 */
final class NoworkClient
{
    /** Response header the backend sets on an encrypted Server API reply (DELIVERY.md section 5). */
    public const HEADER_ENCRYPTION = 'X-NoWork-Encryption';

    /** The routes the backend seals when server encryption is on. */
    private const SEALED_ROUTES = ['whitelist', 'whitelist/check'];

    private string $base;

    /** @var callable(string, string, array<string, string>, ?string, float): array */
    private $transport;

    /** @var list<array<string, string>> */
    private array $keys = [];

    /**
     * @param string        $base           user-center base URL, e.g. `https://account.example.com`
     * @param string        $serverKey      `lsk_...` issued by the admin console
     * @param float         $timeout        per-request timeout in seconds (connect + read)
     * @param callable|null $transport      custom HTTP transport, see CurlTransport for the contract
     * @param list<array<string, string>|string> $decryptionKeys private JWKs (arrays or JSON strings) that
     *        open encrypted replies (DELIVERY.md section 5); several during a key rotation. When set, the
     *        whitelist routes must come encrypted: a plain reply there is refused (no downgrade). PHP opens
     *        ECDH-ES+A256KW (P-256, X25519 with ext-sodium) only, see Delivery.
     */
    public function __construct(string $base, private string $serverKey, private float $timeout = 10.0, ?callable $transport = null, array $decryptionKeys = [])
    {
        $this->base = rtrim($base, '/');
        foreach ($decryptionKeys as $i => $k) {
            $jwk = is_string($k) ? json_decode($k, true) : $k;
            if (!is_array($jwk) || !is_string($jwk['kty'] ?? null)) {
                throw new \InvalidArgumentException(sprintf('nowork: decryption key %s is not a JWK', (string) $i));
            }
            $this->keys[] = $jwk;
        }
        $this->transport = $transport ?? new CurlTransport();
    }

    /** Fetch every approved binding of this server (full sync at start-up / on a timer). */
    public function whitelist(): Whitelist
    {
        return Whitelist::fromArray($this->call('GET', '/server/v1/whitelist'));
    }

    /** Validate a joining player by name and/or uuid (at least one is required; either match is accepted). */
    public function checkWhitelist(string $name = '', string $uuid = ''): CheckResult
    {
        return CheckResult::fromArray($this->call('GET', '/server/v1/whitelist/check', ['name' => $name, 'uuid' => $uuid]));
    }

    /**
     * Report live status (roughly every 60s; 5 minutes without a heartbeat marks the server offline).
     * $tps and $players are optional (PROTOCOL 2.3): a TPS that is not a finite number above 0 is left out,
     * and so is an empty player list.
     *
     * @param list<OnlinePlayer|array{name: string, uuid?: string}|string> $players online players (names allowed)
     */
    public function heartbeat(int $online, int $max = 0, string $version = '', ?float $tps = null, array $players = []): void
    {
        $body = ['online' => $online, 'max' => $max, 'version' => $version];
        if ($tps !== null && is_finite($tps) && $tps > 0) {
            $body['tps'] = $tps;
        }
        $listed = [];
        foreach ($players as $p) {
            $listed[] = self::player($p)->toArray();
        }
        if ($listed !== []) {
            $body['players'] = $listed;
        }
        $this->call('POST', '/server/v1/heartbeat', [], $body);
    }

    // ---- Feature modules / GM channel (PROTOCOL 2.4) ----

    /** Backend-only character SSO exchange; check callback state against the GM session first.
     * @return array{serverId:int,serverCode:string,subject:string,character:array,authenticatedAt:int,scope:string}
     */
    public function exchangeGmCode(string $code, string $codeVerifier, string $redirectUri): array
    {
        return $this->call('POST', '/server/v1/gm/sso/exchange', [], [
            'code' => $code, 'codeVerifier' => $codeVerifier, 'redirectUri' => $redirectUri,
        ]);
    }

    /** Input: exactly one of accessToken or uid; optional playerName/playerUuid/channels. */
    public function issueChatTicket(array $input): array
    {
        return $this->call('POST', '/server/v1/chat/tickets', [], $input);
    }

    public function pollChatRelay(): array
    {
        return $this->call('GET', '/server/v1/chat/relay')['items'] ?? [];
    }

    /** Failed entries are permanent {id,error} failures. Leave transient failures pending. */
    public function ackChatRelay(array $done, array $failed = []): int
    {
        return Json::int($this->call('POST', '/server/v1/chat/relay/ack', [], ['done' => array_values($done), 'failed' => array_values($failed)])['updated'] ?? 0);
    }

    /** Input: player/text plus optional playerUuid/uid/kind (chat or system). */
    public function postChatMessage(array $input): array
    {
        return $this->call('POST', '/server/v1/chat/messages', [], $input);
    }

    /**
     * Modules enabled for this server, keyed by module name, with their per-server config.
     *
     * @return array<string, mixed>
     */
    public function modules(): array
    {
        $m = $this->call('GET', '/server/v1/modules')['modules'] ?? [];

        return is_array($m) ? $m : [];
    }

    /**
     * Replace the GM action catalogue shown in the admin panel; returns the stored count.
     *
     * @param list<GmAction|array<string, mixed>> $actions GmAction or arrays with key / title / description / params
     */
    public function publishGmActions(array $actions): int
    {
        return Json::int($this->call('PUT', '/server/v1/gm/actions', [], self::actions($actions), true)['count'] ?? 0);
    }

    /**
     * Replace the actions and the named option lists (select / suggestion sources such as worlds or kits) in
     * one call; null actions or options keep what is stored (an empty list clears). Entries the server cannot
     * accept are skipped and listed in the result, never a failure for the whole call.
     *
     * @param list<GmAction|array<string, mixed>>|null                      $actions
     * @param array<string, list<GmOption|array<string, mixed>|string>>|null $options list name => choices
     */
    public function publishGmCatalog(?array $actions, ?array $options = null): GmPublishResult
    {
        $body = [];
        if ($actions !== null) {
            $body['actions'] = self::actions($actions);
        }
        if ($options !== null) {
            $lists = self::optionLists($options);
            $body['options'] = $lists === [] ? new \stdClass() : $lists; // {} clears, never a JSON list
        }

        return GmPublishResult::fromArray($this->call('PUT', '/server/v1/gm/actions', [], $body === [] ? new \stdClass() : $body));
    }

    /**
     * Replace only the named option lists (the published actions stay), e.g. when a world loads.
     *
     * @param array<string, list<GmOption|array<string, mixed>|string>> $options list name => choices
     */
    public function publishGmOptions(array $options): GmPublishResult
    {
        $lists = self::optionLists($options);

        return GmPublishResult::fromArray($this->call('PUT', '/server/v1/gm/options', [], $lists === [] ? new \stdClass() : $lists));
    }

    /**
     * Queued GM commands (each becomes "sent"); poll alongside the heartbeat. Each command carries `id`,
     * `action`, `params`, `status`, and since 2026-09-28 `source` (admin | owner | grant) and `operator`
     * ("staff" for admin commands on owner-run servers, the login name otherwise).
     *
     * @return list<array<string, mixed>>
     */
    public function pollGmCommands(): array
    {
        $data = $this->call('GET', '/server/v1/gm/commands', [], null, false, true);

        return array_values(array_filter($data, 'is_array'));
    }

    /** Report how a command went; result is free text (at most 4000 characters). */
    public function reportGmResult(int $commandId, bool $ok, string $result = ''): void
    {
        $this->call('POST', sprintf('/server/v1/gm/commands/%d/result', $commandId), [], ['ok' => $ok, 'result' => $result]);
    }

    // ---- Player data and per-server bans (SERVER_USERS.md) ----

    /**
     * The data schema the server owner defined (needs the module on for this server).
     *
     * @return array{fields: list<array<string, mixed>>, maxUserBytes: int} each field has key / label / type
     *         (text | number | bool | select | json) / visible and optionally description, default,
     *         maxLength, min, max, integer, options
     */
    public function userSchema(): array
    {
        $d = $this->call('GET', '/server/v1/users/schema');
        $fields = is_array($d['fields'] ?? null) ? array_values(array_filter($d['fields'], 'is_array')) : [];

        return ['fields' => $fields, 'maxUserBytes' => Json::int($d['maxUserBytes'] ?? 0)];
    }

    /** One account's data (uid from the join check or the whitelist). */
    public function userData(int $uid): UserData
    {
        return UserData::fromArray($this->call('GET', self::userPath($uid)));
    }

    /**
     * Replace the account's stored values (fields left out are unset). Pass the version you last read to
     * refuse a concurrent change (ApiError 409, see UserData::isVersionConflict); null writes unconditionally.
     *
     * @param array<string, mixed> $data
     */
    public function putUserData(int $uid, array $data, ?int $version = null): UserData
    {
        return UserData::fromArray($this->call('PUT', self::userPath($uid), [], self::writeBody($data, $version)));
    }

    /**
     * Merge values into the stored ones; a null value deletes that field.
     *
     * @param array<string, mixed> $data
     */
    public function patchUserData(int $uid, array $data, ?int $version = null): UserData
    {
        return UserData::fromArray($this->call('PATCH', self::userPath($uid), [], self::writeBody($data, $version)));
    }

    /**
     * Active bans on this server: all (no argument; at most 5000, newest first) or those of the matching
     * accounts. Works with the module off too.
     *
     * @return list<ServerBan>
     */
    public function bans(int $uid = 0, string $name = '', string $uuid = ''): array
    {
        $q = self::target($uid, $name, $uuid);
        $list = $this->call('GET', '/server/v1/bans', array_map('strval', $q))['bans'] ?? [];

        return array_values(array_map(static fn (array $b): ServerBan => ServerBan::fromArray($b), array_filter(is_array($list) ? $list : [], 'is_array')));
    }

    /**
     * Online players (as of the last heartbeat) whose account is banned here: disconnect them.
     *
     * @return list<array{name: string, uuid: string, uid: int, reason: string, expireTime: ?string}>
     */
    public function kickList(): array
    {
        $out = [];
        $list = $this->call('GET', '/server/v1/bans/kick')['players'] ?? [];
        foreach (is_array($list) ? $list : [] as $p) {
            if (is_array($p)) {
                $out[] = [
                    'name' => Json::str($p['name'] ?? ''),
                    'uuid' => Json::str($p['uuid'] ?? ''),
                    'uid' => Json::int($p['uid'] ?? 0),
                    'reason' => Json::str($p['reason'] ?? ''),
                    'expireTime' => isset($p['expireTime']) ? Json::str($p['expireTime']) : null,
                ];
            }
        }

        return $out;
    }

    /**
     * Ban an account on this server (module on, pluginBans allowed), named by uid or by the name / uuid of a
     * binding. $expireTime ("2006-01-02 15:04:05" or RFC 3339) or $durationMinutes; neither = permanent. A ban
     * an administrator placed cannot be replaced (ApiError 403). Returns the ban now in force.
     */
    public function ban(int $uid = 0, string $name = '', string $uuid = '', string $reason = '', string $expireTime = '', int $durationMinutes = 0): ?ServerBan
    {
        $body = self::target($uid, $name, $uuid);
        if ($reason !== '') {
            $body['reason'] = $reason;
        }
        if ($expireTime !== '') {
            $body['expireTime'] = $expireTime;
        }
        if ($durationMinutes > 0) {
            $body['durationMinutes'] = $durationMinutes;
        }
        $ban = $this->call('POST', '/server/v1/bans', [], $body === [] ? new \stdClass() : $body)['ban'] ?? null;

        return is_array($ban) ? ServerBan::fromArray($ban) : null;
    }

    /** Lift the account's active ban on this server (not one an administrator placed); returns the uid. */
    public function unban(int $uid = 0, string $name = '', string $uuid = '', string $reason = ''): int
    {
        $body = self::target($uid, $name, $uuid);
        if ($reason !== '') {
            $body['reason'] = $reason;
        }

        return Json::int($this->call('POST', '/server/v1/bans/lift', [], $body === [] ? new \stdClass() : $body)['uid'] ?? 0);
    }

    private static function userPath(int $uid): string
    {
        if ($uid <= 0) {
            throw new \InvalidArgumentException(sprintf('nowork: uid must be positive, got %d', $uid));
        }

        return sprintf('/server/v1/users/%d/data', $uid);
    }

    /**
     * @param array<string, mixed> $data
     *
     * @return array<string, mixed>
     */
    private static function writeBody(array $data, ?int $version): array
    {
        // the server wants a data object, even an empty one ([] would encode as a JSON list)
        $body = ['data' => $data === [] ? new \stdClass() : $data];
        if ($version !== null) {
            $body['version'] = $version;
        }

        return $body;
    }

    /** @return array<string, int|string> */
    private static function target(int $uid, string $name, string $uuid): array
    {
        return array_filter(['uid' => $uid, 'name' => $name, 'uuid' => $uuid], static fn ($v): bool => $v !== 0 && $v !== '');
    }

    /**
     * Call a Server API route (e.g. `/server/v1/whitelist`) and return the reply without decoding `data`.
     * Transport failures, unreadable replies (missing code, HTTP >= 400) and business errors (code != 0)
     * still throw; an encrypted reply is returned as is (`$encrypted` set).
     *
     * @param array<string, string|null>   $query
     * @param array<mixed>|\stdClass|null  $body
     */
    public function raw(string $method, string $path, array $query = [], array|\stdClass|null $body = null): RawResponse
    {
        return $this->send($method, $path, $query, $body);
    }

    /**
     * Decoded `data` of a successful envelope; an encrypted reply is opened with the decryption keys or
     * refused with EncryptionError (fail closed).
     *
     * @param array<string, string|null>  $query
     * @param array<mixed>|\stdClass|null $body      object body, or a list when $bodyIsList is true
     * @param bool                        $allowList accept a JSON array as `data` (returned as a list)
     *
     * @return array<string, mixed> the `data` object (or list)
     */
    private function call(string $method, string $path, array $query = [], array|\stdClass|null $body = null, bool $bodyIsList = false, bool $allowList = false): array
    {
        $res = $this->send($method, $path, $query, $bodyIsList && is_array($body) ? array_values($body) : $body);
        $route = self::route($path);
        $data = $res->data;
        if ($res->encrypted) {
            if ($this->keys === []) {
                throw new EncryptionError($route, 'no decryption key configured');
            }
            $data = $this->open($data, $route);
        } elseif ($this->keys !== [] && in_array($route, self::SEALED_ROUTES, true)) {
            throw new EncryptionError($route, 'plain reply while decryption keys are configured');
        }
        if ($allowList && is_array($data) && array_is_list($data)) {
            return $data;
        }
        if (!Json::isObject($data)) {
            throw new TransportError(sprintf('invalid response (http %d): missing data', $res->status));
        }

        /** @var array<string, mixed> $data */
        return $data;
    }

    /**
     * Send a request and validate the `{code, message, data}` envelope: missing / non-integer `code` and
     * HTTP >= 400 with `code = 0` are transport problems; `code != 0` is a business error.
     *
     * @param array<string, string|null>  $query
     * @param array<mixed>|\stdClass|null $body
     */
    private function send(string $method, string $path, array $query, array|\stdClass|null $body): RawResponse
    {
        $url = $this->base . $path;
        $query = array_filter($query, static fn ($v): bool => $v !== null && $v !== '');
        if ($query !== []) {
            $url .= '?' . http_build_query($query);
        }
        $headers = ['Accept' => 'application/json', 'X-Server-Key' => $this->serverKey];
        $raw = null;
        if ($body !== null) {
            // no JSON_PARTIAL_OUTPUT_ON_ERROR: NaN / INF fail here instead of reaching the server as 0
            $raw = json_encode($body, JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES | JSON_INVALID_UTF8_SUBSTITUTE);
            if ($raw === false) {
                throw new TransportError('cannot encode request body: ' . json_last_error_msg());
            }
            $headers['Content-Type'] = 'application/json';
        }
        [$status, $text, $received] = Json::send($this->transport, $method, $url, $headers, $raw, $this->timeout);
        $env = Json::decodeObject($text, $status);
        $code = $env['code'] ?? null;
        if (!is_int($code)) {
            throw new TransportError(sprintf('invalid response (http %d): missing code', $status));
        }
        $message = Json::str($env['message'] ?? '');
        if ($code !== 0) {
            throw new ApiError($code, $message);
        }
        if ($status >= 400) {
            throw new TransportError(sprintf('http %d', $status));
        }
        $data = $env['data'] ?? null;
        $encrypted = ($received[strtolower(self::HEADER_ENCRYPTION)] ?? '') !== '' || self::isJweData($data);

        return new RawResponse($status, $received, $message, $data, $env, $text, $encrypted);
    }

    /** Opens the `{"jwe": "..."}` data and checks its ctx names this route (`server:<code>:<route>`). */
    private function open(mixed $data, string $route): mixed
    {
        $jwe = is_array($data) ? ($data['jwe'] ?? null) : null;
        if (!is_string($jwe) || $jwe === '') {
            throw new EncryptionError($route, 'encrypted reply without a JWE');
        }
        try {
            [$plain, $hdr] = Delivery::decrypt($jwe, $this->keys);
        } catch (\Throwable $e) {
            throw new EncryptionError($route, 'the JWE did not open', $e);
        }
        $ctx = is_string($hdr['ctx'] ?? null) ? $hdr['ctx'] : '';
        if (!self::ctxMatches($ctx, $route)) {
            throw new EncryptionError($route, 'envelope ctx names another route: ' . $ctx);
        }
        try {
            return json_decode($plain, true, 512, JSON_THROW_ON_ERROR);
        } catch (\JsonException $e) {
            throw new EncryptionError($route, 'decrypted data is not JSON', $e);
        }
    }

    // The server code is not known here, so the kind and the route are checked.
    private static function ctxMatches(string $ctx, string $route): bool
    {
        return str_starts_with($ctx, 'server:') && str_ends_with($ctx, ':' . $route)
            && strlen($ctx) > strlen('server:') + strlen($route) + 1;
    }

    /** `whitelist/check` for `/server/v1/whitelist/check`. */
    private static function route(string $path): string
    {
        $p = explode('?', $path, 2)[0];

        return str_starts_with($p, '/server/v1/') ? substr($p, strlen('/server/v1/')) : $p;
    }

    private static function isJweData(mixed $data): bool
    {
        return is_array($data) && count($data) === 1 && is_string($data['jwe'] ?? null);
    }

    /** @param OnlinePlayer|array<string, mixed>|string $p */
    private static function player(mixed $p): OnlinePlayer
    {
        if ($p instanceof OnlinePlayer) {
            return $p;
        }
        if (is_array($p)) {
            return new OnlinePlayer(Json::str($p['name'] ?? ''), Json::str($p['uuid'] ?? ''));
        }

        return new OnlinePlayer(Json::str($p));
    }

    /**
     * @param list<GmAction|array<string, mixed>> $actions
     *
     * @return list<array<string, mixed>>
     */
    private static function actions(array $actions): array
    {
        return array_map(static fn ($a) => $a instanceof GmAction ? $a->toArray() : $a, array_values($actions));
    }

    /**
     * @param array<string, list<GmOption|array<string, mixed>|string>> $options
     *
     * @return array<string, list<array<string, mixed>>>
     */
    private static function optionLists(array $options): array
    {
        $out = [];
        foreach ($options as $name => $list) {
            $choices = [];
            foreach (is_array($list) ? $list : [] as $o) {
                $choices[] = match (true) {
                    $o instanceof GmOption => $o->toArray(),
                    is_array($o) => $o,
                    default => ['value' => Json::str($o)],
                };
            }
            $out[(string) $name] = $choices;
        }

        return $out;
    }
}
