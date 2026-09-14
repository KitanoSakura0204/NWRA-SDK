# lounge/sdk (PHP)

PHP 8.1+，仅依赖 `ext-curl` 与 `ext-json`（PHP 自带扩展，无第三方包），可用于 Laravel / Symfony / ThinkPHP 或原生 PHP 项目。

```bash
composer require lounge/sdk
```

## Server API（游戏服务器）

```php
use Lounge\Sdk\LoungeClient;
use Lounge\Sdk\Whitelist;
use Lounge\Sdk\Exceptions\ApiError;
use Lounge\Sdk\Exceptions\TransportError;

$client = new LoungeClient('https://account.example.com', 'lsk_xxx');   // 第三个参数为超时秒数，默认 10

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
```

## OIDC（网站 / 启动器登录）

```php
use Lounge\Sdk\LoungeOidc;

$oidc = new LoungeOidc($issuer, $clientId, $clientSecret, $redirectUri);   // 公开客户端 $clientSecret 传 '' 并使用 PKCE

// 1. 跳转授权页
['verifier' => $verifier, 'challenge' => $challenge] = LoungeOidc::pkce();
$_SESSION['oidc'] = ['state' => $state = bin2hex(random_bytes(16)), 'verifier' => $verifier];
header('Location: ' . $oidc->authorizeUrl($state, $nonce, 'openid profile email game_roles', $challenge));

// 2. 回调
if (($_GET['state'] ?? '') !== $_SESSION['oidc']['state']) { http_response_code(400); exit; }
$token = $oidc->exchange($_GET['code'], $_SESSION['oidc']['verifier']);   // access_token / refresh_token / id_token / expires_in
$me    = $oidc->userInfo($token['access_token']);                          // sub / uid / name / email / security_level / game_roles ...

// 其它
$token = $oidc->refresh($token['refresh_token']);
$info  = $oidc->introspect($token['access_token']);          // ['active' => true, 'sub' => ..., 'scope' => ...]
$oidc->revoke($token['access_token']);
header('Location: ' . $oidc->logoutUrl($token['id_token'] ?? '', 'https://yourapp.com/'));
```

`exchange` / `refresh` / `clientCredentials` / `introspect` / `userInfo` 返回按协议字段名组织的关联数组。

## Webhook 校验

```php
use Lounge\Sdk\Webhook;
use Lounge\Sdk\Exceptions\WebhookError;

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
| `OAuthError` | OIDC 端点返回 RFC 6749 错误对象 | `getError()` / `getDescription()` / `getStatus()` |
| `WebhookError` | 签名、时间戳或请求体校验失败 | 返回 400 |

所有异常继承 `Lounge\Sdk\Exceptions\LoungeException`，可统一捕获。

## 自定义 HTTP 传输

默认使用 cURL。构造函数最后一个参数可注入任意 callable（Guzzle、Symfony HttpClient 或测试桩），约定为：

```php
fn(string $method, string $url, array $headers, ?string $body, float $timeout): array   // 返回 [int $status, string $body]
```

```php
use Lounge\Sdk\CurlTransport;

$transport = new CurlTransport([CURLOPT_PROXY => 'http://127.0.0.1:7890', CURLOPT_CAINFO => '/etc/ssl/certs/ca.pem']);
$client = new LoungeClient($base, $key, timeout: 5.0, transport: $transport);
```

## 测试

```bash
php tests/run.php     # 无需 PHPUnit; 含 php -S 端到端用例, 无法启动内置服务器时自动跳过
```
