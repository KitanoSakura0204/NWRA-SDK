<?php

declare(strict_types=1);

/*
 * Protocol regression tests for the PHP SDK, runnable without PHPUnit:
 *
 *     php tests/run.php
 *
 * Uses vendor/autoload.php when Composer has been run, otherwise a tiny PSR-4 loader. Unit tests inject a fake
 * transport (a callable returning [status, body]); the end-to-end section starts `php -S` with tests/server.php
 * and exercises the real CurlTransport (skipped when the built-in server cannot start). Exit code 1 on failure.
 */

use Lounge\Sdk\CurlTransport;
use Lounge\Sdk\Exceptions\ApiError;
use Lounge\Sdk\Exceptions\EncryptionError;
use Lounge\Sdk\Exceptions\LoungeException;
use Lounge\Sdk\Exceptions\OAuthError;
use Lounge\Sdk\Exceptions\TransportError;
use Lounge\Sdk\Exceptions\WebhookError;
use Lounge\Sdk\GmAction;
use Lounge\Sdk\GmActionParam;
use Lounge\Sdk\GmOption;
use Lounge\Sdk\LoungeClient;
use Lounge\Sdk\LoungeOidc;
use Lounge\Sdk\OnlinePlayer;
use Lounge\Sdk\Webhook;
use Lounge\Sdk\WebhookEvent;
use Lounge\Sdk\Whitelist;
use Lounge\Sdk\WhitelistEntry;

$vendor = __DIR__ . '/../vendor/autoload.php';
if (is_file($vendor)) {
    require $vendor;
} else {
    spl_autoload_register(static function (string $class): void {
        $prefix = 'Lounge\\Sdk\\';
        if (str_starts_with($class, $prefix)) {
            $file = __DIR__ . '/../src/' . str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
            if (is_file($file)) {
                require $file;
            }
        }
    });
}

/* ------------------------------------------------------------------ micro test harness */

final class T
{
    public static int $passed = 0;
    public static int $skipped = 0;
    /** @var string[] */
    public static array $failed = [];

    public static function test(string $name, callable $fn): void
    {
        try {
            $fn();
            self::$passed++;
            echo "  ok    $name\n";
        } catch (\Throwable $e) {
            self::$failed[] = $name;
            printf("  FAIL  %s\n        %s: %s\n        at %s:%d\n", $name, $e::class, $e->getMessage(), $e->getFile(), $e->getLine());
        }
    }

    public static function skip(string $name, string $why): void
    {
        self::$skipped++;
        echo "  skip  $name ($why)\n";
    }

    public static function same(mixed $expected, mixed $actual, string $what = 'value'): void
    {
        if ($expected !== $actual) {
            throw new \AssertionError(sprintf('%s: expected %s, got %s', $what, var_export($expected, true), var_export($actual, true)));
        }
    }

    public static function true(bool $condition, string $what): void
    {
        if (!$condition) {
            throw new \AssertionError($what);
        }
    }

    /**
     * @template E of \Throwable
     *
     * @param class-string<E> $class
     *
     * @return E
     */
    public static function throws(string $class, callable $fn, string $messagePart = ''): \Throwable
    {
        try {
            $fn();
        } catch (\Throwable $e) {
            if (!$e instanceof $class) {
                throw new \AssertionError(sprintf('expected %s, got %s: %s', $class, $e::class, $e->getMessage()));
            }
            if ($messagePart !== '' && !str_contains($e->getMessage(), $messagePart)) {
                throw new \AssertionError(sprintf('expected message containing "%s", got "%s"', $messagePart, $e->getMessage()));
            }

            return $e;
        }
        throw new \AssertionError("expected $class, nothing was thrown");
    }
}

/** Records every request and answers from a queue of [status, body] pairs. */
final class FakeTransport
{
    /** @var array<int, array{method: string, url: string, headers: array<string, string>, body: ?string, timeout: float}> */
    public array $requests = [];
    /** @var array<int, array{int, string}> */
    private array $queue;

    public function __construct(array ...$responses)
    {
        $this->queue = $responses;
    }

    /** @return array{int, string} */
    public static function json(mixed $body, int $status = 200): array
    {
        return [$status, (string) json_encode($body)];
    }

    /** @param array<string, string> $headers */
    public function __invoke(string $method, string $url, array $headers, ?string $body, float $timeout): array
    {
        $this->requests[] = ['method' => $method, 'url' => $url, 'headers' => $headers, 'body' => $body, 'timeout' => $timeout];
        if ($this->queue === []) {
            throw new \LogicException('FakeTransport: no response queued');
        }

        return array_shift($this->queue);
    }

    /** @return array{method: string, url: string, headers: array<string, string>, body: ?string, timeout: float} */
    public function last(): array
    {
        return $this->requests[array_key_last($this->requests)];
    }
}

/** @return array<string, mixed> */
function query(string $url): array
{
    parse_str((string) parse_url($url, PHP_URL_QUERY), $q);

    return $q;
}

/** @return array<string, mixed> */
function form(?string $body): array
{
    parse_str((string) $body, $q);

    return $q;
}

function client(callable $t, string $base = 'https://account.example/', string $key = 'test-key', float $timeout = 10.0): LoungeClient
{
    return new LoungeClient($base, $key, $timeout, $t);
}

function oidc(callable $t, string $secret = 'secret', string $redirect = 'https://app.example/callback'): LoungeOidc
{
    return new LoungeOidc('https://account.example/', 'client', $secret, $redirect, 10.0, $t);
}

/* ------------------------------------------------------------------ Server API */

echo "Server API\n";

T::test('join check sends X-Server-Key, escapes the player name and maps the player entry', function (): void {
    $t = new FakeTransport(FakeTransport::json(['code' => 0, 'data' => [
        'allowed' => true, 'bound' => true, 'reason' => '',
        'player' => ['name' => 'Player & One', 'uid' => 100001, 'securityLevel' => 2],
    ]]));
    $result = client($t)->checkWhitelist('Player & One');
    $req = $t->last();
    T::same('GET', $req['method']);
    T::same('test-key', $req['headers']['X-Server-Key'] ?? null, 'X-Server-Key');
    T::same('application/json', $req['headers']['Accept'] ?? null, 'Accept');
    T::true(!isset($req['headers']['Content-Type']), 'GET must not carry Content-Type');
    T::same(null, $req['body'], 'GET body');
    T::same(10.0, $req['timeout'], 'timeout');
    T::same('/server/v1/whitelist/check', parse_url($req['url'], PHP_URL_PATH), 'path');
    T::same('account.example', parse_url($req['url'], PHP_URL_HOST), 'host (trailing slash trimmed)');
    T::same(['name' => 'Player & One'], query($req['url']), 'query (empty uuid omitted)');
    T::same(true, $result->allowed);
    T::same(true, $result->bound);
    T::same('', $result->reason);
    T::true($result->player instanceof WhitelistEntry, 'player DTO');
    T::same('Player & One', $result->player->name);
    T::same(100001, $result->player->uid);
    T::same(2, $result->player->securityLevel);
    T::same('', $result->player->uuid, 'missing uuid defaults to empty string');
});

T::test('join check with uuid only; disabled whitelist has no player', function (): void {
    $t = new FakeTransport(FakeTransport::json(['code' => 0, 'data' => ['allowed' => true, 'bound' => false, 'reason' => 'whitelist disabled', 'player' => null]]));
    $result = client($t)->checkWhitelist('', '069a79f4-44e9-4726-a5be-fca90e38aaf5');
    T::same(['uuid' => '069a79f4-44e9-4726-a5be-fca90e38aaf5'], query($t->last()['url']));
    T::same(true, $result->allowed);
    T::same(false, $result->bound);
    T::same('whitelist disabled', $result->reason);
    T::same(null, $result->player);
});

T::test('whitelist maps server / mode / count / entries', function (): void {
    $t = new FakeTransport(FakeTransport::json(['code' => 0, 'message' => '操作成功', 'data' => [
        'server' => 'survival', 'mode' => 1, 'count' => 2,
        'entries' => [
            ['name' => 'Steve', 'uuid' => '069a79f4-44e9-4726-a5be-fca90e38aaf5', 'uid' => 100001, 'nickname' => '旅行者', 'securityLevel' => 2, 'since' => '2026-09-13 10:00:00'],
            ['name' => 'Alex', 'uuid' => null, 'uid' => '100002', 'securityLevel' => 1],
        ],
    ]]));
    $list = client($t)->whitelist();
    T::same('/server/v1/whitelist', parse_url($t->last()['url'], PHP_URL_PATH));
    T::same(null, parse_url($t->last()['url'], PHP_URL_QUERY), 'no query string');
    T::same('survival', $list->server);
    T::same(Whitelist::MODE_MANUAL, $list->mode);
    T::same(2, $list->count);
    T::same(2, count($list->entries));
    T::same('Steve', $list->entries[0]->name);
    T::same('069a79f4-44e9-4726-a5be-fca90e38aaf5', $list->entries[0]->uuid);
    T::same('旅行者', $list->entries[0]->nickname);
    T::same('2026-09-13 10:00:00', $list->entries[0]->since);
    T::same('', $list->entries[1]->uuid, 'null uuid');
    T::same(100002, $list->entries[1]->uid, 'numeric string uid');
    T::same(1, $list->entries[1]->securityLevel);
});

T::test('heartbeat POSTs a JSON body', function (): void {
    $t = new FakeTransport(FakeTransport::json(['code' => 0, 'data' => ['server' => 'survival', 'status' => 'ok']]));
    client($t)->heartbeat(2, 20);
    $req = $t->last();
    T::same('POST', $req['method']);
    T::same('https://account.example/server/v1/heartbeat', $req['url']);
    T::same('application/json', $req['headers']['Content-Type'] ?? null, 'Content-Type');
    T::same(['online' => 2, 'max' => 20, 'version' => ''], json_decode((string) $req['body'], true), 'body');
});

T::test('malformed success envelopes are TransportError', function (): void {
    $cases = [
        'empty object' => [200, '{}'],
        'null' => [200, 'null'],
        'list' => [200, '[]'],
        'non-empty list' => [200, '[{"code":0,"data":{}}]'],
        'missing code' => [200, '{"data":{"allowed":true}}'],
        'boolean code' => [200, '{"code":true,"data":{}}'],
        'string code' => [200, '{"code":"0","data":{}}'],
        'float code' => [200, '{"code":0.0,"data":{}}'],
        'missing data' => [200, '{"code":0}'],
        'null data' => [200, '{"code":0,"data":null}'],
        'list data' => [200, '{"code":0,"data":[1,2]}'],
        'scalar data' => [200, '{"code":0,"data":"ok"}'],
        'HTTP failure with success envelope' => [503, '{"code":0,"data":{"allowed":true}}'],
        'HTML gateway page' => [502, '<html>upstream unavailable</html>'],
        'empty body' => [200, ''],
        'truncated json' => [200, '{"code":0,"data":{'],
    ];
    foreach ($cases as $name => $response) {
        $e = T::throws(TransportError::class, fn () => client(new FakeTransport($response))->whitelist());
        T::true(str_starts_with($e->getMessage(), 'lounge: transport error: '), "$name: message prefix");
    }
    $e = T::throws(TransportError::class, fn () => client(new FakeTransport([503, '{"code":0,"data":{}}']))->checkWhitelist('Steve'), 'http 503');
    T::true($e instanceof LoungeException, 'TransportError extends LoungeException');
});

T::test('business errors are ApiError with code and message', function (): void {
    $e = T::throws(ApiError::class, fn () => client(new FakeTransport([401, '{"code":401,"message":"invalid server key"}']))->whitelist());
    T::same(401, $e->getCode());
    T::same('invalid server key', $e->getApiMessage());
    T::same('lounge: api error 401: invalid server key', $e->getMessage());
    $e = T::throws(ApiError::class, fn () => client(new FakeTransport([200, '{"code":1001,"message":"server disabled","data":null}']))->heartbeat(1));
    T::same(1001, $e->getCode(), 'code != 0 wins over missing data');
    $e = T::throws(ApiError::class, fn () => client(new FakeTransport([200, '{"code":7}']))->heartbeat(1));
    T::same('', $e->getApiMessage(), 'missing message defaults to empty');
});

T::test('transport exceptions are wrapped, SDK exceptions pass through', function (): void {
    $e = T::throws(TransportError::class, fn () => client(fn () => throw new \RuntimeException('connection reset'))->whitelist(), 'connection reset');
    T::true($e->getPrevious() instanceof \RuntimeException, 'previous exception kept');
    $own = new TransportError('curl error 28: timed out', 28);
    $e = T::throws(TransportError::class, fn () => client(fn () => throw $own)->whitelist());
    T::true($e === $own, 'same instance rethrown');
    T::same(28, $e->getCode());
    T::throws(TransportError::class, fn () => client(fn () => 'not a pair')->whitelist(), 'invalid response');
});

T::test('timeout and base URL normalisation reach the transport', function (): void {
    $t = new FakeTransport(FakeTransport::json(['code' => 0, 'data' => []]));
    client($t, 'https://account.example///', 'k', 2.5)->heartbeat(0);
    T::same(2.5, $t->last()['timeout']);
    T::same('https://account.example/server/v1/heartbeat', $t->last()['url']);
});

T::test('heartbeat sends tps and players; non-finite or non-positive tps and empty lists are left out', function (): void {
    $ok = FakeTransport::json(['code' => 0, 'data' => ['status' => 'ok']]);
    $t = new FakeTransport($ok, $ok, $ok, $ok, $ok);
    client($t)->heartbeat(2, 20, '1.21.1', 19.8, [new OnlinePlayer('Steve', '069a79f4'), ['name' => 'Alex'], 'Notch']);
    T::same(
        ['online' => 2, 'max' => 20, 'version' => '1.21.1', 'tps' => 19.8, 'players' => [['name' => 'Steve', 'uuid' => '069a79f4'], ['name' => 'Alex'], ['name' => 'Notch']]],
        json_decode((string) $t->last()['body'], true),
        'body with tps and players',
    );
    foreach ([NAN, INF, 0.0, -1.0] as $tps) {
        client($t)->heartbeat(1, 0, '', $tps, []);
        T::same(['online' => 1, 'max' => 0, 'version' => ''], json_decode((string) $t->last()['body'], true), "tps $tps left out");
    }
});

T::test('GM catalogue: typed schema, {actions, options} publish, options-only publish, skipped entries', function (): void {
    $res = FakeTransport::json(['code' => 0, 'data' => ['count' => 1, 'skipped' => [['key' => 'bad key', 'reason' => 'invalid key']]]]);
    $t = new FakeTransport($res, $res, FakeTransport::json(['code' => 0, 'data' => ['count' => 3]]));
    $kick = new GmAction('kick', 'Kick', params: [
        new GmActionParam('player', 'player', required: true, label: 'Player'),
        new GmActionParam('world', 'select', source: 'worlds'),
        new GmActionParam('minutes', 'number', min: 0, max: 60, integer: true, default: 0),
    ], category: 'players', danger: 'confirm');
    $r = client($t)->publishGmCatalog([$kick], ['worlds' => [new GmOption('world', 'Overworld'), 'nether']]);
    T::same(1, $r->count);
    T::same([['key' => 'bad key', 'reason' => 'invalid key']], $r->skipped);
    T::same('PUT', $t->last()['method']);
    T::same('https://account.example/server/v1/gm/actions', $t->last()['url']);
    T::same([
        'actions' => [[
            'key' => 'kick', 'title' => 'Kick',
            'params' => [
                ['name' => 'player', 'type' => 'player', 'required' => true, 'label' => 'Player'],
                ['name' => 'world', 'type' => 'select', 'required' => false, 'label' => '', 'source' => 'worlds'],
                ['name' => 'minutes', 'type' => 'number', 'required' => false, 'label' => '', 'default' => 0, 'min' => 0, 'max' => 60, 'integer' => true],
            ],
            'category' => 'players', 'danger' => 'confirm',
        ]],
        'options' => ['worlds' => [['value' => 'world', 'label' => 'Overworld'], ['value' => 'nether']]],
    ], json_decode((string) $t->last()['body'], true), 'catalog body');
    client($t)->publishGmOptions(['kits' => ['starter']]);
    T::same('https://account.example/server/v1/gm/options', $t->last()['url']);
    T::same(['kits' => [['value' => 'starter']]], json_decode((string) $t->last()['body'], true), 'options body');
    T::same(3, client($t)->publishGmActions([$kick]));
    T::same('kick', json_decode((string) $t->last()['body'], true)[0]['key'], 'legacy list body');
});

T::test('encrypted replies fail closed without keys (header or {"jwe"} data) and raw() returns them as is', function (): void {
    $sealed = ['code' => 0, 'data' => ['jwe' => 'a.b.c.d.e']];
    $e = T::throws(EncryptionError::class, fn () => client(new FakeTransport(FakeTransport::json($sealed)))->checkWhitelist('Steve'), 'no decryption key');
    T::same('whitelist/check', $e->getRoute());
    T::true(!$e instanceof TransportError, 'EncryptionError is not a TransportError');
    $byHeader = [200, '{"code":0,"data":{"allowed":true}}', ['X-Lounge-Encryption' => 'JWE']];
    T::throws(EncryptionError::class, fn () => client(new FakeTransport($byHeader))->checkWhitelist('Steve'));
    $raw = client(new FakeTransport(FakeTransport::json($sealed)))->raw('GET', '/server/v1/whitelist');
    T::true($raw->encrypted, 'raw marks the reply encrypted');
    T::same('a.b.c.d.e', $raw->data['jwe']);
    $raw = client(new FakeTransport($byHeader))->raw('GET', '/server/v1/whitelist/check', ['name' => 'Steve']);
    T::same('JWE', $raw->header('x-lounge-encryption'));
    T::true($raw->encrypted, 'header marks the reply encrypted');
});

/** @return array<string, mixed> the shared delivery vectors (SDK/go/testdata/vectors.json, generated by the backend) */
function vectors(): array
{
    return json_decode((string) file_get_contents(__DIR__ . '/../../go/testdata/vectors.json'), true, 512, JSON_THROW_ON_ERROR);
}

if (!\function_exists('openssl_decrypt')) {
    T::skip('encrypted whitelist opens with the decryption keys (shared vector)', 'ext-openssl missing');
} else {
    T::test('encrypted whitelist opens with the decryption keys; wrong route, wrong key and plain replies are refused', function (): void {
        $set = vectors();
        $v = array_values(array_filter($set['vectors'], static fn (array $x): bool => $x['name'] === 'server-envelope'))[0];
        $sealed = [200, $v['response'], ['X-Lounge-Encryption' => 'JWE']];
        $keyed = static fn (FakeTransport $t, array $keys): LoungeClient => new LoungeClient('https://account.example', 'k', 10.0, $t, $keys);
        $w = $keyed(new FakeTransport($sealed), [$set['keys']['ec']])->whitelist();
        T::same('survival', $w->server);
        T::same(1, $w->count);
        T::same('Steve', $w->entries[0]->name);
        $w = $keyed(new FakeTransport($sealed), [json_encode($set['keys']['ec'])])->whitelist();
        T::same('survival', $w->server, 'JWK given as a JSON string');
        // the ctx says server:survival:whitelist, so the same ciphertext on the join check is refused
        T::throws(EncryptionError::class, fn () => $keyed(new FakeTransport($sealed), [$set['keys']['ec']])->checkWhitelist('Steve'), 'ctx names another route');
        T::throws(EncryptionError::class, fn () => $keyed(new FakeTransport($sealed), [$set['keys']['rsa']])->whitelist(), 'did not open');
        $plain = FakeTransport::json(['code' => 0, 'data' => ['allowed' => true, 'bound' => true]]);
        T::throws(EncryptionError::class, fn () => $keyed(new FakeTransport($plain), [$set['keys']['ec']])->checkWhitelist('Steve'), 'plain reply');
        $ok = FakeTransport::json(['code' => 0, 'data' => ['status' => 'ok']]);
        $keyed(new FakeTransport($ok), [$set['keys']['ec']])->heartbeat(1); // routes the backend never seals stay plain
        T::throws(\InvalidArgumentException::class, fn () => new LoungeClient('https://a.example', 'k', 10.0, null, ['not a jwk']));
    });
}


echo "\nOIDC\n";

T::test('authorizeUrl encodes state, nonce and PKCE S256', function (): void {
    $o = oidc(new FakeTransport());
    $url = $o->authorizeUrl('state & +', 'nonce', 'openid security_level', 'challenge');
    T::same('https://account.example/oauth2/authorize', strtok($url, '?'));
    T::same([
        'response_type' => 'code', 'client_id' => 'client', 'redirect_uri' => 'https://app.example/callback',
        'scope' => 'openid security_level', 'state' => 'state & +', 'nonce' => 'nonce',
        'code_challenge' => 'challenge', 'code_challenge_method' => 'S256',
    ], query($url));
    $q = query($o->authorizeUrl('s'));
    T::same('openid profile email', $q['scope'], 'default scope');
    T::true(!isset($q['nonce'], $q['code_challenge'], $q['code_challenge_method']), 'optional params omitted');
});

T::test('PKCE helpers follow RFC 7636', function (): void {
    ['verifier' => $v, 'challenge' => $c] = LoungeOidc::pkce();
    T::same(1, preg_match('/^[A-Za-z0-9_-]{43}$/', $v), 'verifier is 43 base64url chars');
    T::same(LoungeOidc::pkceChallenge($v), $c);
    T::same('E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM', LoungeOidc::pkceChallenge('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk'), 'RFC 7636 appendix B vector');
    T::same(128, strlen(LoungeOidc::pkceVerifier(96)), 'longest verifier');
    T::same(43, strlen(LoungeOidc::pkceVerifier(1)), 'never shorter than 43');
    T::true(LoungeOidc::pkce()['verifier'] !== $v, 'random');
});

T::test('exchange posts client_secret_post authentication and the PKCE verifier', function (): void {
    $t = new FakeTransport(FakeTransport::json(['access_token' => 'access', 'token_type' => 'Bearer', 'expires_in' => 3600]));
    $token = oidc($t)->exchange('code', 'verifier');
    T::same('access', $token['access_token']);
    $req = $t->last();
    T::same('POST', $req['method']);
    T::same('https://account.example/oauth2/token', $req['url']);
    T::same('application/x-www-form-urlencoded', $req['headers']['Content-Type'] ?? null);
    T::same('application/json', $req['headers']['Accept'] ?? null);
    T::same([
        'grant_type' => 'authorization_code', 'code' => 'code', 'redirect_uri' => 'https://app.example/callback',
        'code_verifier' => 'verifier', 'client_id' => 'client', 'client_secret' => 'secret',
    ], form($req['body']));
});

T::test('public clients omit client_secret and code_verifier when empty', function (): void {
    $t = new FakeTransport(FakeTransport::json(['access_token' => 'a']));
    oidc($t, '')->exchange('code');
    $f = form($t->last()['body']);
    T::true(!isset($f['client_secret']), 'no client_secret');
    T::true(!isset($f['code_verifier']), 'no code_verifier');
    T::same('client', $f['client_id']);
});

T::test('refresh / clientCredentials / introspect / revoke form bodies', function (): void {
    $t = new FakeTransport(
        FakeTransport::json(['access_token' => 'a2', 'refresh_token' => 'r2']),
        FakeTransport::json(['access_token' => 'm']),
        FakeTransport::json(['active' => true, 'sub' => 'subject', 'scope' => 'openid security_level']),
        [200, ''],
    );
    $o = oidc($t);
    T::same('r2', $o->refresh('r1')['refresh_token']);
    T::same(['grant_type' => 'refresh_token', 'refresh_token' => 'r1', 'client_id' => 'client', 'client_secret' => 'secret'], form($t->last()['body']));
    T::same('m', $o->clientCredentials()['access_token']);
    T::same(['grant_type' => 'client_credentials', 'scope' => '', 'client_id' => 'client', 'client_secret' => 'secret'], form($t->last()['body']), 'empty scope is still sent');
    T::same(true, $o->introspect('a2')['active']);
    T::same('https://account.example/oauth2/introspect', $t->last()['url']);
    T::same(['token' => 'a2', 'client_id' => 'client', 'client_secret' => 'secret'], form($t->last()['body']));
    $o->revoke('a2');
    T::same('https://account.example/oauth2/revoke', $t->last()['url']);
    T::same(['token' => 'a2', 'client_id' => 'client', 'client_secret' => 'secret'], form($t->last()['body']));
});

T::test('only revoke may answer with an empty body; non-JSON is TransportError', function (): void {
    oidc(new FakeTransport([200, '']))->revoke('access');
    oidc(new FakeTransport([200, '{}']))->revoke('access');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([200, '']))->exchange('code'), 'empty response');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([200, '']))->introspect('t'), 'empty response');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([200, '<html>unavailable</html>']))->userInfo('access'), 'invalid json');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([200, '[1,2]']))->exchange('code'), 'not an object');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([200, '"token"']))->exchange('code'), 'not an object');
});

T::test('RFC 6749 error objects are OAuthError, other failures TransportError', function (): void {
    $e = T::throws(OAuthError::class, fn () => oidc(new FakeTransport([400, '{"error":"invalid_grant","error_description":"expired"}']))->exchange('code'));
    T::same('invalid_grant', $e->getError());
    T::same('expired', $e->getDescription());
    T::same(400, $e->getStatus());
    T::same(400, $e->getCode());
    T::same('lounge: oauth error invalid_grant: expired', $e->getMessage());
    $e = T::throws(OAuthError::class, fn () => oidc(new FakeTransport([401, '{"error":"invalid_token"}']))->userInfo('bad'));
    T::same('invalid_token', $e->getError());
    T::same('', $e->getDescription());
    T::same(401, $e->getStatus());
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([502, '<html>bad gateway</html>']))->exchange('code'), 'http 502');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([500, '']))->refresh('r'), 'http 500');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([400, '{"message":"no error field"}']))->exchange('code'), 'http 400');
    T::throws(TransportError::class, fn () => oidc(new FakeTransport([400, '{"error":""}']))->exchange('code'), 'http 400');
    $e = T::throws(OAuthError::class, fn () => oidc(new FakeTransport([503, '{"error":"temporarily_unavailable"}']))->userInfo('x'));
    T::same(503, $e->getStatus(), 'any status >= 400 with an error object is OAuthError');
    T::true($e instanceof LoungeException, 'OAuthError extends LoungeException');
});

T::test('userInfo sends the bearer token', function (): void {
    $t = new FakeTransport(FakeTransport::json(['sub' => 'subject', 'uid' => 100001, 'security_level' => 2, 'game_roles' => []]));
    $me = oidc($t)->userInfo('access');
    $req = $t->last();
    T::same('GET', $req['method']);
    T::same('https://account.example/oauth2/userinfo', $req['url']);
    T::same('Bearer access', $req['headers']['Authorization'] ?? null);
    T::same(null, $req['body']);
    T::same(2, $me['security_level']);
    T::same([], $me['game_roles']);
});

T::test('logoutUrl builds the end-session request', function (): void {
    $o = oidc(new FakeTransport());
    T::same(['client_id' => 'client', 'id_token_hint' => 'idt', 'post_logout_redirect_uri' => 'https://app.example/', 'state' => 's'], query($o->logoutUrl('idt', 'https://app.example/', 's')));
    T::same('https://account.example/oauth2/logout?client_id=client', $o->logoutUrl());
});

T::test('OIDC transport exceptions are wrapped as TransportError', function (): void {
    $o = new LoungeOidc('https://account.example', 'client', '', '', 10.0, fn () => throw new \RuntimeException('dns failure'));
    $e = T::throws(TransportError::class, fn () => $o->userInfo('a'), 'dns failure');
    T::true($e->getPrevious() instanceof \RuntimeException, 'previous kept');
});

/* ------------------------------------------------------------------ Webhook */

echo "\nWebhook\n";

$secret = 'whsec_test';
$body = '{"id":"e1","event":"user.login","timestamp":1,"data":{"userId":7}}';
$now = time();
$headers = ['X-Lounge-Timestamp' => (string) $now, 'X-Lounge-Signature' => Webhook::signature($secret, $now, $body), 'X-Lounge-Event' => 'user.login'];

T::test('signature matches the reference vector (shared with the other SDKs)', function (): void {
    T::same('sha256=f409921f3a33654b38b4798557621d1c8274b3d0c4e4a39fb1bd3ee27aa1d5c0', Webhook::signature('whsec_test', 1757800000, '{"id":"e1"}'));
    T::same(Webhook::signature('s', 5, 'b'), Webhook::signature('s', '5', 'b'), 'int and string timestamps agree');
});

T::test('verify accepts a fresh, correctly signed delivery', function () use ($secret, $body, $headers): void {
    $ev = Webhook::verify($secret, $headers, $body);
    T::true($ev instanceof WebhookEvent, 'DTO');
    T::same('e1', $ev->id);
    T::same('user.login', $ev->event);
    T::same(1, $ev->timestamp);
    T::same(['userId' => 7], $ev->data);
    $lower = array_change_key_case($headers, CASE_LOWER);
    T::same('e1', Webhook::verify($secret, $lower, $body)->id, 'header names are case-insensitive');
    $multi = ['x-lounge-timestamp' => [$headers['X-Lounge-Timestamp']], 'x-lounge-signature' => [$headers['X-Lounge-Signature']]];
    T::same('e1', Webhook::verify($secret, $multi, $body)->id, 'multi-value headers (framework style)');
});

T::test('verifyRequest reads $_SERVER-style headers', function () use ($secret, $body, $headers): void {
    $server = ['REQUEST_METHOD' => 'POST', 'HTTP_X_LOUNGE_TIMESTAMP' => $headers['X-Lounge-Timestamp'], 'HTTP_X_LOUNGE_SIGNATURE' => $headers['X-Lounge-Signature'], 'HTTP_X_LOUNGE_DELIVERY' => 'd1', 'argv' => []];
    T::same('user.login', Webhook::verifyRequest($secret, 300, $server, $body)->event);
});

T::test('wrong secret, tampered body or bad signature header are WebhookError', function () use ($secret, $body, $headers): void {
    $e = T::throws(WebhookError::class, fn () => Webhook::verify('wrong', $headers, $body), 'invalid webhook signature');
    T::true($e instanceof LoungeException, 'WebhookError extends LoungeException');
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, $headers, '{"id":"e1","event":"user.deleted","timestamp":1,"data":{}}'), 'invalid webhook signature');
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, ['X-Lounge-Timestamp' => $headers['X-Lounge-Timestamp'], 'X-Lounge-Signature' => 'sha256=deadbeef'], $body), 'invalid webhook signature');
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, ['X-Lounge-Timestamp' => $headers['X-Lounge-Timestamp']], $body), 'invalid webhook signature');
});

T::test('stale or future deliveries are rejected unless tolerance is disabled', function () use ($secret, $body, $now): void {
    $old = $now - 3600;
    $stale = ['X-Lounge-Timestamp' => (string) $old, 'X-Lounge-Signature' => Webhook::signature($secret, $old, $body)];
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, $stale, $body), 'outside tolerance');
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, $stale, $body, 300), 'outside tolerance');
    T::same('e1', Webhook::verify($secret, $stale, $body, null)->id, 'null disables');
    T::same('e1', Webhook::verify($secret, $stale, $body, 0)->id, '0 disables');
    T::same('e1', Webhook::verify($secret, $stale, $body, 7200)->id, 'inside a wider window');
    $future = $now + 3600;
    $ahead = ['X-Lounge-Timestamp' => (string) $future, 'X-Lounge-Signature' => Webhook::signature($secret, $future, $body)];
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, $ahead, $body), 'outside tolerance');
});

T::test('missing or malformed timestamps are rejected before any HMAC work', function () use ($secret, $body): void {
    foreach (['', 'abc', '12.5', '1e9', '   ', '99999999999999999999'] as $ts) {
        T::throws(WebhookError::class, fn () => Webhook::verify($secret, ['X-Lounge-Timestamp' => $ts, 'X-Lounge-Signature' => Webhook::signature($secret, $ts, $body)], $body), 'invalid X-Lounge-Timestamp');
    }
    T::throws(WebhookError::class, fn () => Webhook::verify($secret, [], $body), 'invalid X-Lounge-Timestamp');
    $padded = ['X-Lounge-Timestamp' => ' ' . time() . ' ', 'X-Lounge-Signature' => ' ' . Webhook::signature($secret, time(), $body) . ' '];
    T::same('e1', Webhook::verify($secret, $padded, $body)->id, 'surrounding whitespace is trimmed');
});

T::test('a signed but non-object body is WebhookError; loose payload fields are normalised', function () use ($secret, $now): void {
    foreach (['not json', '[1,2]', '"str"', 'null', ''] as $bad) {
        $h = ['X-Lounge-Timestamp' => (string) $now, 'X-Lounge-Signature' => Webhook::signature($secret, $now, $bad)];
        T::throws(WebhookError::class, fn () => Webhook::verify($secret, $h, $bad), 'invalid webhook body');
    }
    $loose = '{"id":42,"event":null,"timestamp":"1757800000","data":[1,2]}';
    $ev = Webhook::verify($secret, ['X-Lounge-Timestamp' => (string) $now, 'X-Lounge-Signature' => Webhook::signature($secret, $now, $loose)], $loose);
    T::same('42', $ev->id);
    T::same('', $ev->event);
    T::same(1757800000, $ev->timestamp);
    T::same([1, 2], $ev->data);
    $ev = WebhookEvent::fromArray(['data' => 'oops', 'timestamp' => 'x']);
    T::same([], $ev->data, 'non-array data becomes []');
    T::same(0, $ev->timestamp, 'non-numeric timestamp becomes 0');
});

/* ------------------------------------------------------------------ end-to-end: php -S + CurlTransport */

echo "\nEnd-to-end (php -S + CurlTransport)\n";

final class BuiltinServer
{
    /** @param resource $proc */
    private function __construct(private $proc, public readonly string $base, private string $log)
    {
    }

    public static function start(string $router): self
    {
        if (!extension_loaded('curl')) {
            throw new \RuntimeException('ext-curl not loaded');
        }
        if (!function_exists('proc_open')) {
            throw new \RuntimeException('proc_open is disabled');
        }
        $port = 0;
        for ($attempt = 0; $attempt < 10; $attempt++) {
            $candidate = random_int(20000, 40000);
            $probe = @fsockopen('127.0.0.1', $candidate, $errno, $errstr, 0.2);
            if ($probe === false) {
                $port = $candidate;
                break;
            }
            fclose($probe);
        }
        if ($port === 0) {
            throw new \RuntimeException('no free port found');
        }
        $log = (string) tempnam(sys_get_temp_dir(), 'lounge-php-s-');
        $proc = proc_open(
            [PHP_BINARY, '-n', '-d', 'display_errors=stderr', '-S', "127.0.0.1:$port", '-t', dirname($router), $router],
            [0 => ['pipe', 'r'], 1 => ['file', $log, 'a'], 2 => ['file', $log, 'a']],
            $pipes,
            dirname($router),
        );
        if (!is_resource($proc)) {
            throw new \RuntimeException('proc_open failed');
        }
        fclose($pipes[0]);
        $deadline = microtime(true) + 8.0;
        while (microtime(true) < $deadline) {
            $s = @fsockopen('127.0.0.1', $port, $errno, $errstr, 0.2);
            if ($s !== false) {
                fclose($s);

                return new self($proc, "http://127.0.0.1:$port", $log);
            }
            if (!proc_get_status($proc)['running']) {
                break;
            }
            usleep(50_000);
        }
        proc_terminate($proc);
        proc_close($proc);
        throw new \RuntimeException('server did not start: ' . trim((string) @file_get_contents($log)));
    }

    public function stop(): void
    {
        proc_terminate($this->proc);
        proc_close($this->proc);
        @unlink($this->log);
    }
}

$server = null;
try {
    $server = BuiltinServer::start(__DIR__ . '/server.php');
} catch (\Throwable $e) {
    T::skip('end-to-end suite', $e->getMessage());
}

if ($server !== null) {
    $base = $server->base;

    T::test('e2e: whitelist, escaped join check and JSON heartbeat over real cURL', function () use ($base): void {
        $c = new LoungeClient($base, 'test-key', 5.0);
        $list = $c->whitelist();
        T::same(1, $list->count);
        T::same('Steve', $list->entries[0]->name);
        T::same(2, $list->entries[0]->securityLevel);
        $r = $c->checkWhitelist('Player & One');
        T::same(true, $r->allowed, 'server saw the decoded player name');
        T::same(false, $r->bound);
        T::same('whitelist disabled', $r->reason);
        T::same(null, $r->player);
        $c->heartbeat(2, 20); // server answers code=3 when method / Content-Type / body are wrong
    });

    T::test('e2e: 401 envelope is ApiError, 503 / HTML answers are TransportError', function () use ($base): void {
        $e = T::throws(ApiError::class, fn () => (new LoungeClient($base, 'wrong-key', 5.0))->whitelist());
        T::same(401, $e->getCode());
        T::same('invalid server key', $e->getApiMessage());
        T::throws(TransportError::class, fn () => (new LoungeClient($base, 'fail-503', 5.0))->checkWhitelist('Steve'), 'http 503');
        T::throws(TransportError::class, fn () => (new LoungeClient($base, 'html', 5.0))->checkWhitelist('Steve'), 'invalid json');
    });

    T::test('e2e: timeout and connection refusal map to TransportError with the cURL errno', function () use ($base): void {
        $e = T::throws(TransportError::class, fn () => (new LoungeClient($base, 'slow', 0.5))->whitelist(), 'curl error');
        T::same(CURLE_OPERATION_TIMEDOUT, $e->getCode(), 'errno 28');
        $e = T::throws(TransportError::class, fn () => (new LoungeClient('http://127.0.0.1:1', 'k', 2.0))->whitelist(), 'curl error');
        T::true(in_array($e->getCode(), [CURLE_COULDNT_CONNECT, CURLE_OPERATION_TIMEDOUT], true), 'errno 7 (or 28 behind a firewall)');
    });

    T::test('e2e: OIDC exchange / userinfo / introspect / revoke and OAuthError', function () use ($base): void {
        $o = new LoungeOidc($base, 'client', 'secret', 'https://app.example/callback', 5.0);
        $token = $o->exchange('code', 'verifier');
        T::same('access', $token['access_token']);
        T::same(3600, $token['expires_in']);
        T::same(2, $o->userInfo('access')['security_level']);
        T::same(true, $o->introspect('access')['active']);
        T::same('r2', $o->refresh('refresh')['refresh_token']);
        $o->revoke('access');
        $e = T::throws(OAuthError::class, fn () => $o->exchange('expired', 'verifier'));
        T::same('invalid_grant', $e->getError());
        T::same('expired code', $e->getDescription());
        T::same(400, $e->getStatus());
        $e = T::throws(OAuthError::class, fn () => $o->userInfo('bad'));
        T::same('invalid_token', $e->getError());
        T::same(401, $e->getStatus());
        $e = T::throws(OAuthError::class, fn () => (new LoungeOidc($base, 'client', 'nope', '', 5.0))->clientCredentials('openid'));
        T::same('invalid_client', $e->getError());
    });

    T::test('e2e: CurlTransport caller options are applied under the required ones', function () use ($base): void {
        $transport = new CurlTransport([CURLOPT_USERAGENT => 'lounge-sdk-test', CURLOPT_URL => 'http://127.0.0.1:1/ignored']);
        $c = new LoungeClient($base, 'test-key', 5.0, $transport);
        T::same('survival', $c->whitelist()->server, 'required CURLOPT_URL wins over the caller option');
    });

    $server->stop();
}

/* ------------------------------------------------------------------ summary */

printf("\n%d passed, %d failed, %d skipped\n", T::$passed, count(T::$failed), T::$skipped);
if (T::$failed !== []) {
    echo "Failed:\n  - " . implode("\n  - ", T::$failed) . "\n";
    exit(1);
}
