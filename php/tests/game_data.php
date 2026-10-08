<?php

declare(strict_types=1);

// Root/lead runs: php tests/game_data.php. No provider or production requests.
// The optional real-transport section starts this same file on loopback only.
if (PHP_SAPI === 'cli-server') {
    $path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
    header('Content-Type: application/json');
    if (str_starts_with($path, '/redirect/')) {
        header('Location: http://127.0.0.1:1/credential-sink', true, 302);
    } elseif (str_starts_with($path, '/large/')) {
        echo str_repeat(' ', 4096);
    } elseif (str_starts_with($path, '/slow/')) {
        usleep(500000);
        echo '{"code":0,"data":{"games":[]}}';
    } else {
        echo '{"code":0,"data":{"games":[{"id":"arknights"}]}}';
    }
    return;
}

use NoWork\Sdk\Exceptions\ApiError;
use NoWork\Sdk\Exceptions\EncryptionError;
use NoWork\Sdk\Exceptions\TransportError;
use NoWork\Sdk\GameDataClient;

spl_autoload_register(static function (string $class): void {
    $prefix = 'NoWork\\Sdk\\';
    if (str_starts_with($class, $prefix)) {
        $file = __DIR__ . '/../src/' . str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
        if (is_file($file)) {
            require_once $file;
        }
    }
});

function gd_assert(bool $condition, string $message = 'assertion failed'): void
{
    if (!$condition) {
        throw new RuntimeException($message);
    }
}

function gd_throws(string $class, callable $fn): Throwable
{
    try {
        $fn();
    } catch (Throwable $e) {
        gd_assert($e instanceof $class, 'expected ' . $class . ', received ' . $e::class);
        return $e;
    }
    throw new RuntimeException('expected ' . $class);
}

function gd_transport(array &$requests, mixed $data): callable
{
    return static function (string $method, string $url, array $headers, ?string $body, float $timeout) use (&$requests, $data): array {
        $requests[] = compact('method', 'url', 'headers', 'body', 'timeout');
        return [200, json_encode(['code' => 0, 'data' => $data], JSON_THROW_ON_ERROR)];
    };
}

function gd_snapshot(string $game = 'arknights', string $cursor = ''): array
{
    return [
        'game_id' => $game,
        'next_cursor' => $cursor,
        'fetched_at' => '2026-10-08T00:00:00Z',
        'sections' => [
            [
                'id' => 'characters',
                'status' => 'partial',
                'fields' => [
                    ['key' => 'zero', 'value' => 0],
                    ['key' => 'false', 'value' => false],
                    ['key' => 'null', 'value' => null],
                ],
                'entities' => [
                    [
                        'id' => 'char_1',
                        'icon' => ['url' => 'https://assets.example/icon.png'],
                        'source' => ['auth_scope' => 'public', 'attribution' => 'catalog'],
                        'policy' => ['visibility' => 'public', 'exportable' => false],
                    ],
                ],
            ],
        ],
        'extra' => ['future' => 'preserved'],
    ];
}

$tests = [];
$tests['credential boundaries and distinct game list shapes'] = static function (): void {
    $requests = [];
    $public = GameDataClient::forPublic('https://site.example/prefix/', transport: gd_transport($requests, ['games' => [['id' => 'arknights']]]));
    gd_assert($public->games('hypergryph')[0]['id'] === 'arknights');
    gd_assert($requests[0]['url'] === 'https://site.example/prefix/api/hypergryph/games');
    gd_assert(!isset($requests[0]['headers']['Authorization']) && !isset($requests[0]['headers']['X-Server-Key']));
    $account = GameDataClient::forAccount('https://site.example', 'site-jwt', 2.5, gd_transport($requests, [['id' => 'genshin']]));
    gd_assert($account->games('mihoyo')[0]['id'] === 'genshin');
    gd_assert($requests[1]['headers']['Authorization'] === 'Bearer site-jwt' && $requests[1]['timeout'] === 2.5);
    $admin = GameDataClient::forAdmin('https://site.example', 'admin-jwt', transport: gd_transport($requests, [['id' => 'genshin']]));
    $admin->games('mihoyo');
    gd_assert($requests[2]['url'] === 'https://site.example/api/admin/mihoyo/games');
    gd_assert($requests[2]['headers']['Authorization'] === 'Bearer admin-jwt');
    gd_assert(!isset($requests[2]['headers']['X-Server-Key']));
    gd_throws(LogicException::class, static fn () => $public->games('mihoyo'));
    gd_throws(LogicException::class, static fn () => $public->assetProviders());
    gd_throws(LogicException::class, static fn () => $account->refreshAssetProvider('arknights'));
    gd_throws(LogicException::class, static fn () => $admin->mihoyoStatus());
    gd_throws(LogicException::class, static fn () => $admin->snapshot('mihoyo', 'genshin', ['accountId' => 7]));
    gd_assert(count($requests) === 3, 'invalid scopes must not make requests');
};
$tests['snapshot routes query encoding and rich JSON preservation'] = static function (): void {
    $requests = [];
    $account = GameDataClient::forAccount('https://site.example', 'site-jwt', transport: gd_transport($requests, gd_snapshot()));
    $snapshot = $account->snapshot('hypergryph', 'arknights', ['roleId' => 'arknights:role:7', 'sections' => ['characters', 'inventory'], 'cursor' => 'a+/=&?', 'locale' => 'zh_CN', 'limit' => 2]);
    gd_assert(str_contains($requests[0]['url'], '/api/account/hypergryph/games/arknights/snapshot?'));
    parse_str(parse_url($requests[0]['url'], PHP_URL_QUERY), $query);
    gd_assert($query === ['limit' => '2', 'roleId' => 'arknights:role:7', 'sections' => 'characters,inventory', 'cursor' => 'a+/=&?', 'locale' => 'zh_CN']);
    gd_assert($snapshot['sections'][0]['fields'][0]['value'] === 0);
    gd_assert($snapshot['sections'][0]['fields'][1]['value'] === false);
    gd_assert(array_key_exists('value', $snapshot['sections'][0]['fields'][2]) && $snapshot['sections'][0]['fields'][2]['value'] === null);
    gd_assert($snapshot['sections'][0]['entities'][0]['policy']['exportable'] === false);
    gd_assert($snapshot['extra']['future'] === 'preserved');
    $mihoyo = GameDataClient::forAccount('https://site.example', 'site-jwt', transport: gd_transport($requests, gd_snapshot('genshin')));
    $mihoyo->snapshot('mihoyo', 'genshin', ['roleId' => 17]);
    gd_assert(str_contains($requests[1]['url'], '/api/account/mihoyo/games/genshin/data?limit=100&roleId=17'));
    $admin = GameDataClient::forAdmin('https://site.example', 'admin-jwt', transport: gd_transport($requests, gd_snapshot()));
    $admin->snapshot('hypergryph', 'arknights', ['accountId' => 91]);
    gd_assert(str_contains($requests[2]['url'], '/api/admin/hypergryph/accounts/91/games/arknights/snapshot'));
    gd_throws(InvalidArgumentException::class, static fn () => $mihoyo->snapshot('mihoyo', 'genshin', ['roleId' => '17']));
    gd_throws(InvalidArgumentException::class, static fn () => $account->snapshot('hypergryph', 'arknights', ['accountId' => 91]));
};
$tests['role discovery selection and status contracts'] = static function (): void {
    $requests = [];
    $view = ['games' => [], 'roles' => [], 'discovery' => ['status' => 'unsupported'], 'selectedRoleId' => 'role:7'];
    $account = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, $view));
    gd_assert($account->hypergryphGames()['discovery']['status'] === 'unsupported');
    $account->selectHypergryphRole('role:7');
    gd_assert($requests[1]['method'] === 'PUT' && $requests[1]['url'] === 'https://site.example/api/account/hypergryph/role');
    gd_assert(json_decode($requests[1]['body'], true) === ['roleId' => 'role:7']);
    $account->mihoyoStatus();
    gd_assert($requests[2]['url'] === 'https://site.example/api/account/mihoyo/status');
    $admin = GameDataClient::forAdmin('https://site.example', 'jwt', transport: gd_transport($requests, $view));
    $admin->hypergryphGames(12);
    gd_assert($requests[3]['url'] === 'https://site.example/api/admin/hypergryph/accounts/12/games');
};
$tests['snapshot cursor terminal cycles and budgets'] = static function (): void {
    $calls = 0;
    $transport = static function () use (&$calls): array {
        $calls++;
        return [200, json_encode(['code' => 0, 'data' => gd_snapshot('arknights', $calls === 1 ? 'next' : '')])];
    };
    $client = GameDataClient::forPublic('https://site.example', transport: $transport);
    gd_assert(count(iterator_to_array($client->snapshotPages('hypergryph', 'arknights'))) === 2 && $calls === 2);
    $requests = [];
    $client = GameDataClient::forPublic('https://site.example', transport: gd_transport($requests, gd_snapshot('arknights', 'same')));
    gd_throws(TransportError::class, static fn () => iterator_to_array($client->snapshotPages('hypergryph', 'arknights')));
    gd_assert(count($requests) === 2);
    $requests = [];
    gd_throws(TransportError::class, static fn () => iterator_to_array($client->snapshotPages('hypergryph', 'arknights', [], 1)));
    gd_assert(count($requests) === 1);
};
$tests['asset defaults page metadata and admin writes preserve false'] = static function (): void {
    $requests = [];
    $catalog = ['provider' => ['id' => 'arknights'], 'cache' => ['disabled' => false], 'status' => 'stale', 'list' => [], 'page' => 1, 'limit' => 50, 'count' => 0];
    $account = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, $catalog));
    gd_assert($account->assetCatalog('arknights')['status'] === 'stale');
    parse_str(parse_url($requests[0]['url'], PHP_URL_QUERY), $query);
    gd_assert($query['locale'] === '' && $query['version'] === '' && $query['provider'] === 'arknights');
    $admin = GameDataClient::forAdmin('https://site.example', 'jwt', transport: gd_transport($requests, ['disabled' => false]));
    $admin->refreshAssetProvider('arknights', 'zh_CN', 'latest');
    gd_assert($requests[1]['method'] === 'POST' && str_ends_with($requests[1]['url'], '/providers/arknights/refresh'));
    $admin->setAssetDisabled('arknights', false);
    gd_assert(json_decode($requests[2]['body'], true) === ['locale' => '', 'version' => '', 'disabled' => false]);
    $bad = $catalog;
    $bad['page'] = 2;
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, $bad));
    gd_throws(TransportError::class, static fn () => $client->assetCatalog('arknights'));
    $bad = $catalog;
    $bad['count'] = 10;
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, $bad));
    gd_throws(TransportError::class, static fn () => iterator_to_array($client->assetCatalogPages('arknights')));
};
$tests['catalog page traversal is bounded and provider metadata must match'] = static function (): void {
    $urls = [];
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: static function ($method, $url) use (&$urls): array {
        $urls[] = $url;
        parse_str(parse_url($url, PHP_URL_QUERY), $query);
        $page = (int) $query['page'];
        return [200, json_encode(['code' => 0, 'data' => ['provider' => ['id' => 'arknights'], 'cache' => (object) [],
            'list' => [['id' => 'entry-' . $page]], 'page' => $page, 'limit' => 1, 'count' => 2]])];
    });
    gd_assert(count(iterator_to_array($client->assetCatalogPages('arknights', ['limit' => 1]))) === 2);
    gd_assert(str_contains($urls[1], 'page=2&limit=1'));
    gd_throws(TransportError::class, static fn () => iterator_to_array($client->assetCatalogPages('arknights', ['limit' => 1], 1)));
    $requests = [];
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, [
        'provider' => ['id' => 'wrong-provider'], 'cache' => (object) [], 'list' => [], 'page' => 1, 'limit' => 50, 'count' => 0,
    ]));
    gd_throws(TransportError::class, static fn () => $client->assetCatalog('arknights'));
};
$tests['archive routes binding IDs revisions export and omitted delete data'] = static function (): void {
    $requests = [];
    $account = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, []));
    $account->archiveSnapshots('mihoyo', 7);
    gd_assert($requests[0]['url'] === 'https://site.example/api/account/mihoyo/accounts/7/archive/snapshots');
    $account->archivePolicies('hypergryph', 8, 'arknights');
    gd_assert(str_ends_with($requests[1]['url'], '/hypergryph/accounts/8/archive/policies?game_id=arknights'));
    $entry = ['provider' => 'mihoyo', 'stale' => true, 'revision' => 3, 'snapshot' => gd_snapshot('genshin')];
    $account = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, $entry));
    gd_assert($account->archiveSnapshot('mihoyo', 7, 'genshin', 'game:role')['stale'] === true);
    parse_str(parse_url($requests[2]['url'], PHP_URL_QUERY), $query);
    gd_assert($query === ['game_id' => 'genshin', 'role_id' => 'game:role']);
    $account->exportArchive('mihoyo', 7);
    gd_assert(str_ends_with($requests[3]['url'], '/mihoyo/accounts/7/archive/export'));
    $admin = GameDataClient::forAdmin('https://site.example', 'jwt', transport: gd_transport($requests, ['revision' => 1]));
    $admin->setArchivePolicy('mihoyo', 7, 'genshin', '/sections/profile/fields/key~1name', 'owner', false, 0);
    gd_assert($requests[4]['method'] === 'PUT' && str_contains($requests[4]['url'], '/api/admin/mihoyo/accounts/7/archive/policies'));
    $body = json_decode($requests[4]['body'], true);
    gd_assert($body['exportable'] === false && $body['revision'] === 0 && $body['path'] === '/sections/profile/fields/key~1name');
    gd_throws(LogicException::class, static fn () => $account->setArchivePolicy('mihoyo', 7, 'genshin', '/sections/x/fields/y', 'owner', false, 0));
    gd_throws(InvalidArgumentException::class, static fn () => $admin->setArchivePolicy('mihoyo', 7, 'genshin', '/sections/x/fields/y~9', 'owner', false, 0));
    $account = GameDataClient::forAccount('https://site.example', 'jwt', transport: static function ($method, $url) use (&$requests): array {
        $requests[] = compact('method', 'url');
        return [200, '{"code":0,"message":"ok"}'];
    });
    $account->clearArchive('mihoyo', 7);
    gd_assert($requests[5]['method'] === 'DELETE' && str_ends_with($requests[5]['url'], '/accounts/7/archive'));
};
$tests['archive descending pagination preserves uint64 IDs'] = static function (): void {
    $requests = [];
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: static function ($method, $url) use (&$requests): array {
        $requests[] = $url;
        return count($requests) === 1
            ? [200, '{"code":0,"data":[{"id":18446744073709551614},{"id":18446744073709551613}]}']
            : [200, '{"code":0,"data":[]}'];
    });
    $pages = iterator_to_array($client->archiveHistoryPages('mihoyo', 7, limit: 2));
    gd_assert(count($pages) === 2 && $pages[0][0]['id'] === '18446744073709551614');
    gd_assert(str_contains($requests[1], 'before=18446744073709551613&limit=2'));
    $requests = [];
    $client = GameDataClient::forAdmin('https://site.example', 'jwt', transport: gd_transport($requests, [['id' => 9], ['id' => 9]]));
    gd_throws(TransportError::class, static fn () => $client->archivePolicyAudit('mihoyo', 7));
    gd_throws(InvalidArgumentException::class, static fn () => $client->archiveHistory('mihoyo', 7, '18446744073709551616'));
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: gd_transport($requests, [['id' => 9]]));
    $error = gd_throws(TransportError::class, static fn () => iterator_to_array($client->archiveHistoryPages('mihoyo', 7, limit: 1, maxPages: 1)));
    gd_assert(str_contains($error->getMessage(), 'budget'));
};
$tests['untrusted errors never expose response bodies credentials or previous exceptions'] = static function (): void {
    $secret = 'credential-value-for-test';
    $client = GameDataClient::forAccount('https://site.example', $secret, transport: static fn () => [403, '{"code":403,"message":"credential-value-for-test"}']);
    $error = gd_throws(ApiError::class, static fn () => $client->games('mihoyo'));
    gd_assert($error->getCode() === 403 && !str_contains($error->getMessage(), $secret));
    $client = GameDataClient::forAccount('https://site.example', $secret, transport: static function () use ($secret): array {
        throw new RuntimeException($secret);
    });
    $error = gd_throws(TransportError::class, static fn () => $client->games('mihoyo'));
    gd_assert($error->getPrevious() === null && !str_contains($error->getMessage(), $secret));
    gd_assert(!str_contains(var_export($client->__debugInfo(), true), $secret));
};
$tests['bounded strict decode rejects malformed envelope redirects ciphertext and nonfinite numbers'] = static function (): void {
    $cases = [
        [200, 'not json'], [200, '[]'], [200, '{"code":"0","data":[]}'], [500, '{"code":0,"data":[]}'],
        [200, '{"code":0}'], [302, '{"code":0,"data":[]}'], [304, ''], [200, str_repeat(' ', 1025)],
        [200, '{"code":0,"data":[{"number":1e999}]}'], [200, '{"code":0,"data":{}}'],
        [200, str_repeat('[', 65) . '0' . str_repeat(']', 65)], [200, '{}', 'bad headers'],
    ];
    foreach ($cases as $reply) {
        $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: static fn () => $reply, maxResponseBytes: 1024);
        gd_throws(TransportError::class, static fn () => $client->games('mihoyo'));
    }
    foreach ([[200, '{"code":0,"data":{"jwe":"opaque"}}'], [200, '{}', ['x-nowork-encryption' => 'jwe']]] as $reply) {
        $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: static fn () => $reply);
        gd_throws(EncryptionError::class, static fn () => $client->games('mihoyo'));
    }
};
$tests['invalid input is rejected before transport'] = static function (): void {
    $requests = [];
    $transport = gd_transport($requests, gd_snapshot());
    foreach (['https://user:secret@site.example', 'https://site.example/?x=1', 'https://site.example/#x', "https://site.example/\r\n"] as $base) {
        gd_throws(InvalidArgumentException::class, static fn () => GameDataClient::forPublic($base, transport: $transport));
    }
    foreach ([0.0, -1.0, INF, NAN, 60.1] as $timeout) {
        gd_throws(InvalidArgumentException::class, static fn () => GameDataClient::forPublic('https://site.example', $timeout, $transport));
    }
    gd_throws(InvalidArgumentException::class, static fn () => GameDataClient::forAccount('https://site.example', "jwt\nsecret", transport: $transport));
    $client = GameDataClient::forAccount('https://site.example', 'jwt', transport: $transport);
    foreach ([['limit' => 101], ['cursor' => str_repeat('x', 2049)], ['sections' => ['a,b']], ['locale' => 'invented'], ['unknown' => true], ['roleId' => 9]] as $options) {
        gd_throws(InvalidArgumentException::class, static fn () => $client->snapshot('hypergryph', 'arknights', $options));
    }
    gd_throws(InvalidArgumentException::class, static fn () => $client->snapshot('hypergryph', '../escape'));
    gd_assert($requests === []);
};
$tests['default cURL captures bounded body refuses redirects and times out'] = static function (): void {
    if (!function_exists('curl_init') || !function_exists('proc_open')) {
        echo "  SKIP loopback transport: ext-curl/proc_open unavailable\n";
        return;
    }
    $socket = @stream_socket_server('tcp://127.0.0.1:0', $errno, $errstr);
    if ($socket === false) {
        echo "  SKIP loopback transport: cannot bind loopback\n";
        return;
    }
    $address = stream_socket_get_name($socket, false);
    fclose($socket);
    $null = PHP_OS_FAMILY === 'Windows' ? 'NUL' : '/dev/null';
    $process = proc_open([PHP_BINARY, '-S', $address, __FILE__], [0 => ['pipe', 'r'], 1 => ['file', $null, 'a'], 2 => ['file', $null, 'a']], $pipes, __DIR__);
    gd_assert(is_resource($process), 'could not start isolated fixture server');
    fclose($pipes[0]);
    try {
        $ready = false;
        for ($i = 0; $i < 100; $i++) {
            $probe = @stream_socket_client('tcp://' . $address, $errno, $errstr, 0.02);
            if ($probe !== false) {
                fclose($probe);
                $ready = true;
                break;
            }
            usleep(20000);
        }
        gd_assert($ready, 'fixture server did not become ready');
        $base = 'http://' . $address;
        gd_assert(GameDataClient::forPublic($base)->games('hypergryph')[0]['id'] === 'arknights');
        $oversize = gd_throws(TransportError::class, static fn () => GameDataClient::forPublic($base . '/large', maxResponseBytes: 1024)->games('hypergryph'));
        gd_assert($oversize->getCode() === 23, 'download must stop inside the cURL write callback');
        $redirect = gd_throws(TransportError::class, static fn () => GameDataClient::forAccount($base . '/redirect', 'jwt')->games('mihoyo'));
        gd_assert(str_contains($redirect->getMessage(), 'redirects'), 'must reject the 302 response without following it');
        $timeout = gd_throws(TransportError::class, static fn () => GameDataClient::forPublic($base . '/slow', timeout: 0.05)->games('hypergryph'));
        gd_assert($timeout->getCode() === 28, 'must enforce the cURL deadline');
    } finally {
        proc_terminate($process);
        proc_close($process);
    }
};

$failed = 0;
foreach ($tests as $name => $test) {
    try {
        $test();
        echo "  ok $name\n";
    } catch (Throwable $e) {
        $failed++;
        echo '  FAIL ' . $name . ': ' . $e::class . ': ' . $e->getMessage() . "\n";
    }
}
echo count($tests) . ' groups, ' . $failed . " failures\n";
exit($failed === 0 ? 0 : 1);
