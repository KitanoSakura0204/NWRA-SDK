<?php

declare(strict_types=1);

/*
 * Router script for the end-to-end section of tests/run.php, started as `php -S 127.0.0.1:<port> tests/server.php`.
 * It fakes the user-center endpoints (docs/PROTOCOL.md) and misbehaves on demand, keyed by X-Server-Key:
 *   test-key  -> normal answers            wrong-key -> 401 {"code":401,...}
 *   fail-503  -> 503 with a success envelope   html  -> 502 HTML gateway page      slow -> sleeps 2s
 */

$path = (string) parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);
$method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
$header = static fn (string $name): string => (string) ($_SERVER['HTTP_' . strtoupper(str_replace('-', '_', $name))] ?? '');
$contentType = (string) ($_SERVER['CONTENT_TYPE'] ?? $_SERVER['HTTP_CONTENT_TYPE'] ?? '');

$reply = static function (int $status, mixed $body, string $type = 'application/json'): never {
    http_response_code($status);
    header('Content-Type: ' . $type);
    echo is_string($body) ? $body : json_encode($body, JSON_UNESCAPED_UNICODE);
    exit;
};

if (str_starts_with($path, '/server/v1/')) {
    $key = $header('X-Server-Key');
    if ($key === 'slow') {
        sleep(2);
    }
    if ($key === 'fail-503') {
        $reply(503, ['code' => 0, 'data' => ['allowed' => true]]);
    }
    if ($key === 'html') {
        $reply(502, '<html>upstream unavailable</html>', 'text/html');
    }
    if ($key !== 'test-key' && $key !== 'slow') {
        $reply(401, ['code' => 401, 'message' => 'invalid server key']);
    }
    if ($header('Accept') !== 'application/json') {
        $reply(200, ['code' => 2, 'message' => 'missing Accept header']);
    }
    switch ($path) {
        case '/server/v1/whitelist':
            $reply(200, ['code' => 0, 'message' => '操作成功', 'data' => [
                'server' => 'survival', 'mode' => 1, 'count' => 1,
                'entries' => [['name' => 'Steve', 'uid' => 100001, 'nickname' => 'Player', 'securityLevel' => 2, 'since' => '2026-09-13 10:00:00']],
            ]]);
        case '/server/v1/whitelist/check':
            $ok = ($_GET['name'] ?? '') === 'Player & One' && !isset($_GET['uuid']);
            $reply(200, ['code' => 0, 'data' => ['allowed' => $ok, 'bound' => false, 'reason' => $ok ? 'whitelist disabled' : 'bad query: ' . http_build_query($_GET)]]);
        case '/server/v1/heartbeat':
            $in = json_decode((string) file_get_contents('php://input'), true);
            if ($method !== 'POST' || $contentType !== 'application/json' || $in !== ['online' => 2, 'max' => 20, 'version' => '']) {
                $reply(200, ['code' => 3, 'message' => sprintf('invalid heartbeat request: %s %s %s', $method, $contentType, json_encode($in))]);
            }
            $reply(200, ['code' => 0, 'data' => ['server' => 'survival', 'status' => 'ok']]);
    }
    $reply(404, ['code' => 404, 'message' => 'not found']);
}

switch ($path) {
    case '/oauth2/token':
        if ($method !== 'POST' || ($_POST['client_id'] ?? '') !== 'client' || ($_POST['client_secret'] ?? '') !== 'secret') {
            $reply(401, ['error' => 'invalid_client', 'error_description' => 'client authentication failed']);
        }
        switch ($_POST['grant_type'] ?? '') {
            case 'authorization_code':
                if (($_POST['code'] ?? '') === 'expired') {
                    $reply(400, ['error' => 'invalid_grant', 'error_description' => 'expired code']);
                }
                if (($_POST['code_verifier'] ?? '') !== 'verifier' || ($_POST['redirect_uri'] ?? '') !== 'https://app.example/callback') {
                    $reply(400, ['error' => 'invalid_grant', 'error_description' => 'pkce or redirect_uri mismatch']);
                }
                $reply(200, ['access_token' => 'access', 'token_type' => 'Bearer', 'expires_in' => 3600, 'refresh_token' => 'refresh', 'scope' => 'openid']);
            case 'refresh_token':
                $reply(200, ['access_token' => 'access2', 'token_type' => 'Bearer', 'expires_in' => 3600, 'refresh_token' => 'r2', 'scope' => 'openid']);
            case 'client_credentials':
                $reply(200, ['access_token' => 'machine', 'token_type' => 'Bearer', 'expires_in' => 3600, 'scope' => (string) ($_POST['scope'] ?? '')]);
        }
        $reply(400, ['error' => 'unsupported_grant_type']);
    case '/oauth2/introspect':
        $reply(200, ['active' => ($_POST['token'] ?? '') === 'access', 'sub' => 'subject', 'scope' => 'openid security_level']);
    case '/oauth2/revoke':
        http_response_code(200); // empty body on purpose
        exit;
    case '/oauth2/userinfo':
        if ($header('Authorization') !== 'Bearer access') {
            $reply(401, ['error' => 'invalid_token']);
        }
        $reply(200, ['sub' => 'subject', 'uid' => 100001, 'security_level' => 2, 'game_roles' => []]);
}

$reply(404, ['error' => 'not_found', 'error_description' => $path]);
