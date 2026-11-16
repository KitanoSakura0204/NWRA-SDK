# nowork/sdk (PHP)

PHP 8.1+，仅依赖 `ext-curl` 与 `ext-json`（PHP 自带扩展，无第三方包），可用于 Laravel / Symfony / ThinkPHP 或原生 PHP 项目。
ID Token 的 RS256 校验与 EC P-256 解密另需 `ext-openssl`；X25519 解密另需 `ext-sodium`。

```bash
composer require nowork/sdk
```

## Server API（游戏服务器）

```php
use NoWork\Sdk\NoworkClient;
use NoWork\Sdk\OnlinePlayer;
use NoWork\Sdk\Whitelist;
use NoWork\Sdk\Exceptions\ApiError;
use NoWork\Sdk\Exceptions\TransportError;

$client = new NoworkClient('https://account.example.com', 'lsk_xxx');   // 第三个参数为超时秒数，默认 10

try {
    $r = $client->checkWhitelist(name: 'Steve', uuid: '069a79f4-...');
    if (!$r->allowed) {
        kick("未绑定账号: {$r->reason}");   // not bound / pending review / banned / account locked
    }
    // 放行: $r->player?->uid / ->nickname / ->securityLevel
} catch (TransportError $e) {
    // 网络异常或响应异常: 不代表玩家被拒绝, 按服务器策略决定放行/拒绝, 建议使用本地缓存
} catch (ApiError $e) {
    // 业务错误: $e->getCode() (401 = 服务器密钥无效), $e->getApiMessage()
}

$list = $client->whitelist();                 // $list->mode === Whitelist::MODE_MANUAL, $list->entries[]->uuid ...
$client->heartbeat(online: 12, max: 50, version: '1.21.1');   // 每 60 秒上报一次
// 可选 TPS 与在线玩家（PROTOCOL 2.3）；非有限值或 <= 0 的 TPS、空列表不会发送
$client->heartbeat(12, 50, '1.21.1', tps: 19.8, players: [new OnlinePlayer('Steve', '069a79f4-...'), 'Alex']);
```

### 加密的 Server API 响应（DELIVERY.md §5）

服务器开启加密后，`whitelist` / `whitelist/check` 的 `data` 变为 `{"jwe": "..."}`（响应头 `X-NoWork-Encryption`）。未配置私钥时 SDK 抛出 `EncryptionError`（fail closed，不是 `TransportError`，不会被“网络异常放行”策略吞掉）。配置私钥后自动解密并核对 `ctx`（`server:<服务器代码>:<路由>`），此时这两个接口若返回明文同样拒绝（防降级）。PHP 仅支持 EC P-256 与 X25519（需 ext-sodium）密钥。

```php
$client = new NoworkClient($base, 'lsk_xxx', decryptionKeys: [file_get_contents('/etc/nowork/server-enc.jwk')]);
$raw = $client->raw('GET', '/server/v1/whitelist');   // 不解码 data：$raw->encrypted / $raw->header('X-NoWork-Encryption') / $raw->body
```

### GM 面板（PROTOCOL 2.4）

```php
use NoWork\Sdk\{GmAction, GmActionParam, GmOption};

$r = $client->publishGmCatalog([
    new GmAction('kick', '踢出玩家', params: [
        new GmActionParam('player', 'player', required: true, label: '玩家'),
        new GmActionParam('world', 'select', source: 'worlds'),
        new GmActionParam('minutes', 'number', min: 0, max: 60, integer: true),
    ], category: 'players', danger: 'confirm'),
], ['worlds' => [new GmOption('world', '主世界'), 'world_nether']]);
// $r->count 已保存数量，$r->skipped 被跳过的条目及原因（不会让整次发布失败）
$client->publishGmOptions(['kits' => ['starter', 'vip']]);   // 只替换命名选项列表
foreach ($client->pollGmCommands() as $cmd) {                  // id / action / params / source / operator
    $client->reportGmResult($cmd['id'], true, 'done');
}
```

## OIDC（网站 / 启动器登录）

```php
use NoWork\Sdk\NoworkOidc;
use NoWork\Sdk\Delivery;

$oidc = new NoworkOidc($issuer, $clientId, $clientSecret, $redirectUri);   // 公开客户端 $clientSecret 传 '' 并使用 PKCE
$metadata = $oidc->discover();   // GET <issuer>/.well-known/openid-configuration
$jwks = $oidc->jwks();           // GET <issuer>/jwks.json

// 1. 跳转授权页
['verifier' => $verifier, 'challenge' => $challenge] = NoworkOidc::pkce();
$_SESSION['oidc'] = ['state' => $state = bin2hex(random_bytes(16)), 'verifier' => $verifier,
    'nonce' => $nonce = bin2hex(random_bytes(16))];
header('Location: ' . $oidc->authorizeUrl($state, $nonce, 'openid profile email game_roles', $challenge));

// 2. 回调
if (($_GET['state'] ?? '') !== $_SESSION['oidc']['state']) { http_response_code(400); exit; }
$login = $_SESSION['oidc'];
unset($_SESSION['oidc']);
$token = $oidc->exchange($_GET['code'], $login['verifier']);   // access_token / refresh_token / id_token / expires_in
$claims = Delivery::verifyIdToken($token['id_token'], $oidc->jwks(), rtrim($issuer, '/'), $clientId, $login['nonce']);
$me    = $oidc->userInfo($token['access_token']);                          // sub / uid / name / email / security_level / game_roles ...

// 其它
$token = $oidc->refresh($token['refresh_token']);
$info  = $oidc->introspect($token['access_token']);          // ['active' => true, 'sub' => ..., 'scope' => ...]
$oidc->revoke($token['access_token']);
header('Location: ' . $oidc->logoutUrl($token['id_token'] ?? '', 'https://yourapp.com/'));
```

`exchange` / `refresh` / `clientCredentials` / `introspect` / `userInfo` 返回按协议字段名组织的关联数组。

`discover()` / `jwks()` 返回关联数组并保留未知字段，只请求配置 issuer 的上述固定路径，不发送客户端密钥、Bearer Token 或服务器密钥，也不跟随发现文档里的端点。issuer 去除末尾 `/` 后必须与发现文档的 `issuer` 完全一致；缺少非空字符串 `issuer` / `authorization_endpoint` / `token_endpoint` / `jwks_uri`、JWKS 的 `keys` 不是对象数组、非 2xx、错误 JSON、根节点不是对象或响应超过 1 MiB 时抛出 `TransportError`。元数据读取要求无 URL 用户名/密码、查询串或片段的 HTTP(S) issuer。

`Delivery::verifyIdToken($token, $jwks, $issuer, $audience, $expectedNonce)` 先验证 RS256 签名、issuer、audience 与 `exp` / `nbf`（60 秒容差），再将非空的 `$expectedNonce` 与签名内的字符串 `nonce` 比较；缺失、类型错误、不匹配、期望值为空或显式传入 `null` 时抛出 `DecryptError`。原有四参数调用仍可使用，省略末尾参数表示不绑定 nonce。应从本次授权请求的会话取 nonce，避免直接信任回调传值。

加密 ID Token 用同一个 nonce，并继续验证内层签名：

```php
$claims = Delivery::decryptIdToken($token['id_token'], $privateJwks, $oidc->jwks(), rtrim($issuer, '/'), $clientId, $login['nonce']);
```

`$privateJwks` 是一个私有 JWK 关联数组或其列表。PHP 解密支持 `ECDH-ES+A256KW` / `A256GCM` 的 EC P-256 或 X25519 密钥；PHP 8.1–8.4 的 ext-openssl 无法选择 SHA-256 OAEP，因此不支持 `RSA-OAEP-256`，需注册 EC P-256 或 X25519 加密密钥。

## Webhook 校验

```php
use NoWork\Sdk\Webhook;
use NoWork\Sdk\Exceptions\WebhookError;

try {
    // 原生 PHP: 直接读取 $_SERVER 与 php://input, 默认时间戳容差 300 秒
    $event = Webhook::verifyRequest('whsec_xxx');
    // 框架: Webhook::verify($secret, $request->headers->all(), $request->getContent());
} catch (WebhookError $e) {
    http_response_code(400);
    exit;
}

// 重试会带相同的 $event->id, 请幂等处理
match ($event->event) {
    'user.login'   => handleLogin($event->data['uid'], $event->data['ip']),
    'user.deleted' => handleDeleted($event->data['userId']),
    default        => null,
};
http_response_code(204);
```

`Webhook::signature($secret, $timestamp, $body)` 可用于自测或转发签名。

## 错误处理约定

| 异常 | 场景 | 处理 |
| --- | --- | --- |
| `TransportError` | 网络/超时/非 JSON 响应；信封缺 `code`、缺 `data` 或 `data` 为 null；HTTP ≥ 400 却 `code = 0` | 不代表玩家被拒绝，按白名单 `mode` 降级；`getCode()` 为 cURL errno（28 超时、7 连接失败） |
| `ApiError` | `code != 0` | `getCode()` 业务码（401 = 密钥无效），`getApiMessage()` 可读原因 |
| `EncryptionError` | 加密响应无法打开（未配置私钥、解密失败、`ctx` 不符），或配置私钥后白名单接口返回明文 | 按拒绝处理（fail closed）；`getRoute()` / `getReason()` |
| `OAuthError` | OIDC 端点返回 RFC 6749 错误对象 | `getError()` / `getDescription()` / `getStatus()` |
| `DecryptError` | ID Token 签名、issuer、audience、时间或 nonce 不符，或 JWE 解密失败 | 拒绝该次登录 |
| `WebhookError` | 签名、时间戳或请求体校验失败 | 返回 400 |

所有异常继承 `NoWork\Sdk\Exceptions\NoWorkException`，可统一捕获。

## 自定义 HTTP 传输

默认使用 cURL。构造函数最后一个参数可注入任意 callable（Guzzle、Symfony HttpClient 或测试桩），约定为：

```php
fn(string $method, string $url, array $headers, ?string $body, float $timeout): array   // 返回 [int $status, string $body]
```

```php
use NoWork\Sdk\CurlTransport;

$transport = new CurlTransport([CURLOPT_PROXY => 'http://127.0.0.1:7890', CURLOPT_CAINFO => '/etc/ssl/certs/ca.pem']);
$client = new NoworkClient($base, $key, timeout: 5.0, transport: $transport);
```

元数据读取在默认传输或直接注入的 `CurlTransport` 中使用读取回调，将下载限制在 1 MiB，固定 URL 派生的请求路径，禁止重定向和 cURL 源站认证，并隔离显式 Cookie、Cookie 文件/列表/jar 与共享 Cookie 状态。它使用局部配置副本，不读写调用方 Cookie jar、不修改复用传输的普通请求配置，并保留调用方超时、代理、TLS 配置及已有写回调的更严格限制。其它自定义 callable 仍只接收五个参数；SDK 会在收到完整响应后检查 1 MiB 限制，适配器必须自行限制实际下载、禁止元数据重定向并保持请求不带凭据。若把 `CurlTransport` 包装成闭包或 `[$transport, '__invoke']`，它也属于这一类自定义 callable。

## 测试

```bash
php tests/run.php     # 无需 PHPUnit; 含 php -S 端到端用例, 无法启动内置服务器时自动跳过
php tests/game_data.php
php tests/oidc_metadata_nonce.php   # 含本地 HTTP 边界、RS256 与 X25519 加密 ID Token 测试；需 curl/openssl/sodium
```
