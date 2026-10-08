<?php

declare(strict_types=1);

namespace NoWork\Sdk;

use NoWork\Sdk\Exceptions\ApiError;
use NoWork\Sdk\Exceptions\EncryptionError;
use NoWork\Sdk\Exceptions\TransportError;

/**
 * Site game data API, independent of the signed Server API client.
 *
 * Construct with forPublic(), forAccount() or forAdmin(). Account/admin tokens
 * are site JWTs; server keys and upstream provider credentials are never used.
 * Results retain the server's JSON field names, source/policy/status metadata,
 * nulls and unknown extension fields as PHP arrays. Large JSON integers remain
 * decimal strings. An unavailable/private section is a valid result.
 *
 * The default cURL transport refuses redirects and bounds the downloaded body.
 * Injected transports use the CurlTransport callable signature and must enforce
 * the supplied timeout, refuse redirects, and bound buffering themselves. The
 * client additionally checks their returned body before decoding it.
 */
final class GameDataClient
{
    private string $base;
    /** @var callable */
    private $transport;

    private function __construct(
        string $base,
        private string $scope,
        private ?string $token,
        private float $timeout,
        ?callable $transport,
        private int $maxResponseBytes,
    ) {
        $url = parse_url($base);
        if ($url === false || !in_array($url['scheme'] ?? '', ['http', 'https'], true)
            || empty($url['host']) || isset($url['user']) || isset($url['pass'])
            || isset($url['query']) || isset($url['fragment'])
            || preg_match('/[\x00-\x20\x7f\\\\]/', $base)
            || filter_var($base, FILTER_VALIDATE_URL) === false) {
            throw new \InvalidArgumentException('base must be an HTTP(S) URL without credentials, query or fragment');
        }
        if (!is_finite($timeout) || $timeout <= 0 || $timeout > 60) {
            throw new \InvalidArgumentException('timeout must be greater than zero and at most 60 seconds');
        }
        if ($maxResponseBytes < 1024 || $maxResponseBytes > 32 * 1024 * 1024) {
            throw new \InvalidArgumentException('maxResponseBytes must be between 1024 and 33554432');
        }
        if ($token !== null && ($token === '' || strlen($token) > 16384 || preg_match('/[^\x21-\x7e]/', $token))) {
            throw new \InvalidArgumentException('a nonempty site JWT without whitespace is required');
        }
        $this->base = rtrim($base, '/');
        $this->transport = $transport ?? $this->boundedTransport();
    }

    public static function forPublic(string $base, float $timeout = 10.0, ?callable $transport = null, int $maxResponseBytes = 20971520): self
    {
        return new self($base, 'public', null, $timeout, $transport, $maxResponseBytes);
    }

    public static function forAccount(string $base, string $token, float $timeout = 10.0, ?callable $transport = null, int $maxResponseBytes = 20971520): self
    {
        return new self($base, 'account', $token, $timeout, $transport, $maxResponseBytes);
    }

    public static function forAdmin(string $base, string $token, float $timeout = 10.0, ?callable $transport = null, int $maxResponseBytes = 20971520): self
    {
        return new self($base, 'admin', $token, $timeout, $transport, $maxResponseBytes);
    }

    public function __debugInfo(): array
    {
        return ['scope' => $this->scope, 'timeout' => $this->timeout, 'maxResponseBytes' => $this->maxResponseBytes];
    }

    /** @return list<array<string, mixed>> */
    public function games(string $provider): array
    {
        self::provider($provider);
        if ($this->scope === 'public' && $provider !== 'hypergryph') {
            throw new \LogicException('public game routes are available only for hypergryph');
        }
        $data = $this->call('GET', $this->prefix() . '/' . $provider . '/games');
        if ($provider === 'hypergryph') {
            $data = self::object($data)->games ?? null;
        }
        return self::rows($data);
    }

    /** Full games/roles/discovery/selectedRoleId view. Admin requires a binding ID. */
    public function hypergryphGames(?int $accountId = null): array
    {
        $this->requireScope('account', 'admin');
        $path = $this->prefix() . '/hypergryph';
        if ($this->scope === 'admin') {
            $path .= '/accounts/' . self::positive($accountId, 'accountId');
        } elseif ($accountId !== null) {
            throw new \InvalidArgumentException('accountId is only accepted by the admin client');
        }
        $data = self::object($this->call('GET', $path . '/games'));
        self::objectList($data->games ?? null);
        self::objectList($data->roles ?? null);
        return self::arrayValue($data);
    }

    public function mihoyoStatus(): array
    {
        $this->requireScope('account');
        return self::arrayValue(self::object($this->call('GET', '/api/account/mihoyo/status')));
    }

    /** Administrator product catalog, including supported adapters and evidence. */
    public function mihoyoCatalog(): array
    {
        $this->requireScope('admin');
        return self::rows($this->call('GET', '/api/admin/mihoyo/catalog'));
    }

    public function selectHypergryphRole(string $roleId): array
    {
        $this->requireScope('account');
        self::text($roleId, 'roleId', 128, false);
        return self::arrayValue(self::object($this->call('PUT', '/api/account/hypergryph/role', [], ['roleId' => $roleId])));
    }

    /**
     * miHoYo roleId is the stored numeric role ID, Hypergryph uses the returned
     * bound-role string ID. Neither is an upstream UID. Admin live snapshots
     * exist only for Hypergryph and require its binding accountId.
     *
     * @param array{roleId?: int|string, sections?: list<string>, cursor?: string, limit?: int, locale?: string, accountId?: int} $options
     * @return array<string, mixed>
     */
    public function snapshot(string $provider, string $gameId, array $options = []): array
    {
        self::provider($provider);
        self::segment($gameId, 'gameId', 64);
        self::keys($options, ['roleId', 'sections', 'cursor', 'limit', 'locale', 'accountId']);
        $path = $this->prefix() . '/' . $provider;
        if ($this->scope === 'public' && $provider !== 'hypergryph') {
            throw new \LogicException('public game routes are available only for hypergryph');
        }
        if ($this->scope === 'admin') {
            if ($provider !== 'hypergryph') {
                throw new \LogicException('admin live snapshots are available only for hypergryph; use the archive for saved miHoYo data');
            }
            $path .= '/accounts/' . self::positive($options['accountId'] ?? null, 'accountId');
        } elseif (array_key_exists('accountId', $options)) {
            throw new \InvalidArgumentException('accountId is only accepted by the admin client');
        }
        $query = ['limit' => self::positive($options['limit'] ?? 100, 'limit', 100)];
        if (array_key_exists('roleId', $options)) {
            $query['roleId'] = $provider === 'mihoyo'
                ? self::positive($options['roleId'], 'roleId')
                : self::text($options['roleId'], 'roleId', 128, false);
        }
        $sections = $options['sections'] ?? [];
        if (!is_array($sections) || !array_is_list($sections) || count($sections) > 32) {
            throw new \InvalidArgumentException('sections must be a list of at most 32 identifiers');
        }
        foreach ($sections as $section) {
            self::segment($section, 'section', 64);
        }
        if ($sections !== []) {
            $query['sections'] = implode(',', $sections);
        }
        $cursor = self::text($options['cursor'] ?? '', 'cursor', 2048);
        $locale = self::text($options['locale'] ?? '', 'locale', 16);
        if ($locale !== '' && !in_array($locale, ['en', 'zh_CN', 'zh_TW', 'ja'], true)) {
            throw new \InvalidArgumentException('unsupported snapshot locale');
        }
        if ($cursor !== '') {
            $query['cursor'] = $cursor;
        }
        if ($locale !== '') {
            $query['locale'] = $locale;
        }
        $path .= '/games/' . rawurlencode($gameId) . ($provider === 'mihoyo' ? '/data' : '/snapshot');
        return self::snapshotValue($this->call('GET', $path, $query), $gameId);
    }

    /** Bounded lazy page stream; exhaustion with more data throws TransportError. */
    public function snapshotPages(string $provider, string $gameId, array $options = [], int $maxPages = 10): \Generator
    {
        self::positive($maxPages, 'maxPages', 100);
        $seen = [];
        $initial = self::text($options['cursor'] ?? '', 'cursor', 2048);
        if ($initial !== '') {
            $seen[$initial] = true;
        }
        for ($i = 0; $i < $maxPages; $i++) {
            $page = $this->snapshot($provider, $gameId, $options);
            $next = $page['next_cursor'] ?? '';
            if ($next !== '' && isset($seen[$next])) {
                throw new TransportError('snapshot pagination repeated a cursor');
            }
            yield $page;
            if ($next === '') {
                return;
            }
            $seen[$next] = true;
            $options['cursor'] = $next;
        }
        throw new TransportError('snapshot page budget exhausted; resume using the last next_cursor');
    }

    public function assetProviders(): array
    {
        $this->requireScope('account', 'admin');
        return self::rows($this->call('GET', $this->prefix() . '/game-assets/providers'));
    }

    /** Blank locale/version select the provider manifest default, not English/latest. */
    public function assetCatalog(string $provider, array $options = []): array
    {
        $this->requireScope('account', 'admin');
        self::segment($provider, 'asset provider', 96);
        self::keys($options, ['locale', 'version', 'q', 'kind', 'page', 'limit']);
        $query = self::variant($options);
        $query['provider'] = $provider;
        $query['q'] = self::text($options['q'] ?? '', 'q', 128);
        $query['kind'] = self::text($options['kind'] ?? '', 'kind', 32);
        if (!in_array($query['kind'], ['', 'character', 'item', 'weapon', 'equipment', 'quest', 'achievement', 'other'], true)) {
            throw new \InvalidArgumentException('unsupported catalog entity kind');
        }
        $query['page'] = self::positive($options['page'] ?? 1, 'page', 20000);
        $query['limit'] = self::positive($options['limit'] ?? 50, 'limit', 100);
        $data = self::object($this->call('GET', $this->prefix() . '/game-assets/catalog', $query));
        $rows = self::objectList($data->list ?? null);
        if (($data->page ?? null) !== $query['page'] || ($data->limit ?? null) !== $query['limit']
            || !is_int($data->count ?? null) || $data->count < 0 || count($rows) > $query['limit']) {
            throw new TransportError('invalid catalog pagination');
        }
        self::object($data->provider ?? null);
        self::object($data->cache ?? null);
        if (($data->provider->id ?? null) !== $provider) {
            throw new TransportError('catalog provider does not match the request');
        }
        return self::arrayValue($data);
    }

    public function assetCatalogPages(string $provider, array $options = [], int $maxPages = 10): \Generator
    {
        self::positive($maxPages, 'maxPages', 100);
        for ($i = 0; $i < $maxPages; $i++) {
            $page = $this->assetCatalog($provider, $options);
            $offset = ($page['page'] - 1) * $page['limit'];
            if (count($page['list']) !== min($page['limit'], max(0, $page['count'] - $offset))) {
                throw new TransportError('catalog returned an incomplete page');
            }
            yield $page;
            if ($page['page'] * $page['limit'] >= $page['count']) {
                return;
            }
            $options['page'] = $page['page'] + 1;
        }
        throw new TransportError('catalog page budget exhausted; resume using the next page number');
    }

    public function refreshAssetProvider(string $provider, string $locale = '', string $version = ''): array
    {
        $this->requireScope('admin');
        self::segment($provider, 'asset provider', 96);
        return self::arrayValue(self::object($this->call('POST', '/api/admin/game-assets/providers/' . rawurlencode($provider) . '/refresh', [], self::variant(compact('locale', 'version')))));
    }

    public function setAssetDisabled(string $provider, bool $disabled, string $locale = '', string $version = ''): array
    {
        $this->requireScope('admin');
        self::segment($provider, 'asset provider', 96);
        return self::arrayValue(self::object($this->call('PUT', '/api/admin/game-assets/providers/' . rawurlencode($provider) . '/state', [], self::variant(compact('locale', 'version')) + ['disabled' => $disabled])));
    }

    /** accountId always identifies the provider binding, never the site user or game UID. */
    public function archiveSnapshots(string $provider, int $accountId): array
    {
        return self::rows($this->call('GET', $this->archivePath($provider, $accountId) . '/snapshots'));
    }

    /** Returns Entry metadata plus its normalized snapshot, including stale/revision. */
    public function archiveSnapshot(string $provider, int $accountId, string $gameId, string $roleId = ''): array
    {
        self::segment($gameId, 'gameId', 64);
        self::text($roleId, 'roleId', 128);
        $data = self::object($this->call('GET', $this->archivePath($provider, $accountId) . '/snapshots', ['game_id' => $gameId, 'role_id' => $roleId]));
        self::snapshotValue($data->snapshot ?? null, $gameId);
        return self::arrayValue($data);
    }

    public function archiveHistory(string $provider, int $accountId, int|string $before = 0, int $limit = 50): array
    {
        return $this->archiveRows($provider, $accountId, 'history', $before, $limit);
    }

    public function archivePolicyAudit(string $provider, int $accountId, int|string $before = 0, int $limit = 50): array
    {
        return $this->archiveRows($provider, $accountId, 'policy-audit', $before, $limit);
    }

    public function archiveHistoryPages(string $provider, int $accountId, int|string $before = 0, int $limit = 50, int $maxPages = 10): \Generator
    {
        yield from $this->archivePages($provider, $accountId, 'history', $before, $limit, $maxPages);
    }

    public function archivePolicyAuditPages(string $provider, int $accountId, int|string $before = 0, int $limit = 50, int $maxPages = 10): \Generator
    {
        yield from $this->archivePages($provider, $accountId, 'policy-audit', $before, $limit, $maxPages);
    }

    public function archivePolicies(string $provider, int $accountId, string $gameId): array
    {
        self::segment($gameId, 'gameId', 64);
        return self::rows($this->call('GET', $this->archivePath($provider, $accountId) . '/policies', ['game_id' => $gameId]));
    }

    /** Export obeys server field policies and includes history/policies/audit. */
    public function exportArchive(string $provider, int $accountId): array
    {
        return self::arrayValue(self::object($this->call('GET', $this->archivePath($provider, $accountId) . '/export')));
    }

    /** Explicit destructive operation; does not remove the provider binding. */
    public function clearArchive(string $provider, int $accountId): void
    {
        $this->call('DELETE', $this->archivePath($provider, $accountId));
    }

    /** Revision is the current field-policy revision (0 when creating one). */
    public function setArchivePolicy(string $provider, int $accountId, string $gameId, string $path, string $visibility, bool $exportable, int $revision): array
    {
        $this->requireScope('admin');
        self::segment($gameId, 'gameId', 64);
        self::text($path, 'path', 320, false);
        $parts = explode('/', $path);
        if (!(count($parts) === 5 && $parts[1] === 'sections' && $parts[3] === 'fields'
            || count($parts) === 7 && $parts[1] === 'sections' && $parts[3] === 'entities' && $parts[5] === 'fields')
            || $parts[0] !== '') {
            throw new \InvalidArgumentException('invalid field policy path');
        }
        foreach (array_slice($parts, 1) as $part) {
            if ($part === '' || preg_match('/~(?![01])/', $part)) {
                throw new \InvalidArgumentException('invalid field policy path');
            }
        }
        if (!in_array($visibility, ['owner', 'shared', 'public', 'secret'], true) || $revision < 0) {
            throw new \InvalidArgumentException('invalid field policy visibility or revision');
        }
        $body = ['game_id' => $gameId, 'path' => $path, 'visibility' => $visibility, 'exportable' => $exportable, 'revision' => $revision];
        return self::arrayValue(self::object($this->call('PUT', $this->archivePath($provider, $accountId) . '/policies', [], $body)));
    }

    private function archivePath(string $provider, int $accountId): string
    {
        $this->requireScope('account', 'admin');
        self::provider($provider);
        return $this->prefix() . '/' . $provider . '/accounts/' . self::positive($accountId, 'accountId') . '/archive';
    }

    private function archiveRows(string $provider, int $accountId, string $kind, int|string $before, int $limit): array
    {
        $before = self::decimal($before);
        self::positive($limit, 'limit', 100);
        $rows = self::rows($this->call('GET', $this->archivePath($provider, $accountId) . '/' . $kind, ['before' => $before, 'limit' => $limit]));
        if (count($rows) > $limit) {
            throw new TransportError('archive page exceeds requested limit');
        }
        $last = $before;
        foreach ($rows as $row) {
            try {
                $id = self::decimal($row['id'] ?? '');
            } catch (\InvalidArgumentException $e) {
                throw new TransportError('archive row has an invalid ID');
            }
            if ($id === '0' || ($last !== '0' && self::compareDecimal($id, $last) >= 0)) {
                throw new TransportError('archive pagination IDs must strictly decrease');
            }
            $last = $id;
        }
        return $rows;
    }

    private function archivePages(string $provider, int $accountId, string $kind, int|string $before, int $limit, int $maxPages): \Generator
    {
        self::positive($maxPages, 'maxPages', 100);
        for ($i = 0; $i < $maxPages; $i++) {
            $rows = $this->archiveRows($provider, $accountId, $kind, $before, $limit);
            yield $rows;
            if (count($rows) < $limit) {
                return;
            }
            $before = $rows[count($rows) - 1]['id'];
        }
        throw new TransportError('archive page budget exhausted; resume before the last row ID');
    }

    private function prefix(): string
    {
        return $this->scope === 'public' ? '/api' : '/api/' . $this->scope;
    }

    private function requireScope(string ...$scopes): void
    {
        if (!in_array($this->scope, $scopes, true)) {
            throw new \LogicException('operation is unavailable for this client scope');
        }
    }

    private function call(string $method, string $path, array $query = [], ?array $body = null): mixed
    {
        $url = $this->base . $path . ($query === [] ? '' : '?' . http_build_query($query, '', '&', PHP_QUERY_RFC3986));
        $headers = ['Accept' => 'application/json'];
        if ($this->token !== null) {
            $headers['Authorization'] = 'Bearer ' . $this->token;
        }
        $payload = null;
        if ($body !== null) {
            $headers['Content-Type'] = 'application/json';
            $payload = json_encode((object) $body, JSON_THROW_ON_ERROR);
        }
        try {
            $result = ($this->transport)($method, $url, $headers, $payload, $this->timeout);
        } catch (\Throwable $e) {
            // Transport messages and previous exceptions can contain URLs,
            // authorization headers or bodies. Never attach them to this error.
            throw new TransportError('game data request failed', $e instanceof TransportError ? $e->getCode() : 0);
        }
        if (!is_array($result) || !isset($result[0], $result[1]) || !is_int($result[0]) || !is_string($result[1])
            || $result[0] < 100 || $result[0] > 599) {
            throw new TransportError('transport returned an invalid response');
        }
        [$status, $raw] = $result;
        if ($status >= 300 && $status < 400) {
            throw new TransportError('game data redirects and bodyless cache replies are not supported');
        }
        if (strlen($raw) > $this->maxResponseBytes) {
            throw new TransportError('game data response exceeds the byte limit');
        }
        if (isset($result[2]) && !is_array($result[2])) {
            throw new TransportError('transport returned invalid response headers');
        }
        foreach (($result[2] ?? []) as $name => $value) {
            if (is_string($name) && strcasecmp($name, 'X-NoWork-Encryption') === 0 && $value !== '') {
                throw new EncryptionError('game-data', 'site API response must be unencrypted');
            }
        }
        try {
            $envelope = json_decode($raw, false, 64, JSON_THROW_ON_ERROR | JSON_BIGINT_AS_STRING);
        } catch (\JsonException $e) {
            throw new TransportError('game data response is not valid bounded JSON');
        }
        if ($envelope instanceof \stdClass && (isset($envelope->ciphertext) || isset($envelope->encrypted)
            || ($envelope->data ?? null) instanceof \stdClass && isset($envelope->data->jwe))) {
            throw new EncryptionError('game-data', 'site API response must be unencrypted');
        }
        if (!$envelope instanceof \stdClass || !isset($envelope->code) || !is_int($envelope->code)) {
            throw new TransportError('invalid game data API envelope');
        }
        if ($envelope->code !== 0) {
            // Keep stable business codes without exposing arbitrary echoed data.
            throw new ApiError($envelope->code, 'game data request was rejected');
        }
        if ($status < 200 || $status >= 300 || (!property_exists($envelope, 'data') && $method !== 'DELETE')) {
            throw new TransportError('invalid game data success response');
        }
        return $envelope->data ?? null;
    }

    private function boundedTransport(): callable
    {
        $maxBytes = $this->maxResponseBytes;
        return static function (string $method, string $url, array $headers, ?string $body, float $timeout) use ($maxBytes): array {
            $raw = '';
            $transport = new CurlTransport([
                CURLOPT_FOLLOWLOCATION => false,
                CURLOPT_MAXREDIRS => 0,
                CURLOPT_WRITEFUNCTION => static function ($handle, string $chunk) use (&$raw, $maxBytes): int {
                    if (strlen($chunk) > $maxBytes - strlen($raw)) {
                        return 0;
                    }
                    $raw .= $chunk;
                    return strlen($chunk);
                },
            ]);
            $response = $transport($method, $url, $headers, $body, $timeout);
            return [$response[0], $raw, $response[2]];
        };
    }

    private static function provider(string $provider): void
    {
        if (!in_array($provider, ['mihoyo', 'hypergryph'], true)) {
            throw new \InvalidArgumentException('provider must be mihoyo or hypergryph');
        }
    }

    private static function text(mixed $value, string $name, int $maxBytes, bool $empty = true): string
    {
        if (!is_string($value) || (!$empty && $value === '') || strlen($value) > $maxBytes
            || preg_match('/[\x00-\x1f\x7f]/', $value) || preg_match('//u', $value) !== 1) {
            throw new \InvalidArgumentException('invalid ' . $name);
        }
        return $value;
    }

    private static function segment(mixed $value, string $name, int $maxBytes): string
    {
        self::text($value, $name, $maxBytes, false);
        if (!preg_match('/^[A-Za-z0-9][A-Za-z0-9_.:-]*$/D', $value)) {
            throw new \InvalidArgumentException('invalid ' . $name);
        }
        return $value;
    }

    private static function positive(mixed $value, string $name, int $maximum = PHP_INT_MAX): int
    {
        if (!is_int($value) || $value < 1 || $value > $maximum) {
            throw new \InvalidArgumentException('invalid ' . $name);
        }
        return $value;
    }

    private static function decimal(mixed $value): string
    {
        if ((!is_int($value) && !is_string($value)) || !preg_match('/^(0|[1-9][0-9]{0,19})$/D', (string) $value)
            || self::compareDecimal((string) $value, '18446744073709551615') > 0) {
            throw new \InvalidArgumentException('invalid unsigned archive cursor');
        }
        return (string) $value;
    }

    private static function compareDecimal(string $left, string $right): int
    {
        return strlen($left) <=> strlen($right) ?: strcmp($left, $right);
    }

    private static function keys(array $options, array $allowed): void
    {
        if (array_diff(array_keys($options), $allowed) !== []) {
            throw new \InvalidArgumentException('unknown game data option');
        }
    }

    private static function variant(array $options): array
    {
        return ['locale' => self::text($options['locale'] ?? '', 'locale', 16), 'version' => self::text($options['version'] ?? '', 'version', 64)];
    }

    private static function object(mixed $value): \stdClass
    {
        if (!$value instanceof \stdClass) {
            throw new TransportError('game data response must be an object');
        }
        return $value;
    }

    private static function rows(mixed $value): array
    {
        return self::arrayValue(self::objectList($value));
    }

    /** Validate without copying a potentially large nested snapshot tree. */
    private static function objectList(mixed $value): array
    {
        if (!is_array($value) || !array_is_list($value)) {
            throw new TransportError('game data response must be a list');
        }
        foreach ($value as $row) {
            self::object($row);
        }
        return $value;
    }

    private static function arrayValue(mixed $value): mixed
    {
        if (is_float($value) && !is_finite($value)) {
            throw new TransportError('game data response contains a nonfinite number');
        }
        if ($value instanceof \stdClass) {
            $value = get_object_vars($value);
        }
        if (is_array($value)) {
            foreach ($value as $key => $item) {
                $value[$key] = self::arrayValue($item);
            }
        }
        return $value;
    }

    private static function snapshotValue(mixed $value, string $gameId): array
    {
        $data = self::object($value);
        if (($data->game_id ?? null) !== $gameId || !is_string($data->fetched_at ?? null)) {
            throw new TransportError('invalid snapshot identity or timestamp');
        }
        self::objectList($data->sections ?? null);
        self::responseCursor($data->next_cursor ?? '');
        foreach ($data->sections as $section) {
            if (!is_string($section->id ?? null) || !is_string($section->status ?? null)) {
                throw new TransportError('invalid snapshot section');
            }
            self::objectList($section->fields ?? []);
            self::objectList($section->entities ?? []);
            self::responseCursor($section->next_cursor ?? '');
        }
        return self::arrayValue($data);
    }

    private static function responseCursor(mixed $value): void
    {
        try {
            self::text($value, 'cursor', 2048);
        } catch (\InvalidArgumentException $e) {
            throw new TransportError('invalid snapshot response cursor');
        }
    }
}
