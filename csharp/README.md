# NoWork.Sdk (C# / .NET 6+)

零第三方依赖，适用于 Unity 服务端 / 独立 .NET 游戏服务器 / ASP.NET 应用。

```csharp
var client = new NoworkClient("https://account.example.com", Environment.GetEnvironmentVariable("NOWORK_SERVER_KEY")!);
var r = await client.CheckWhitelistAsync(player.Name, player.Uuid);
if (!r.Allowed) Kick(player, $"未绑定账号: {r.Reason}");
await client.HeartbeatAsync(online: 12, max: 50, version: "1.0");

var oidc = new NoworkOidc(issuer, clientId, clientSecret, redirectUri);
var url = oidc.AuthorizeUrl(state, nonce, "openid profile email game_roles");
var token = await oidc.ExchangeAsync(code);
var me = await oidc.UserInfoAsync(token.GetProperty("access_token").GetString()!);
```

OIDC 登录应为每次请求生成并保存独立的 `state`、`nonce` 和 PKCE verifier；回调先校验 `state`，再用保存的 verifier 换取 token。发现文档与签名公钥可通过固定的 issuer 路径读取：

```csharp
using NoWork.Sdk;

var oidc = new NoworkOidc(issuer, clientId, clientSecret, redirectUri);
var discovery = await oidc.DiscoverAsync(ct); // JsonElement，包含未知扩展字段
var jwks = await oidc.JwksAsync(ct);          // 原始 JWKS JSON
var tokens = await oidc.ExchangeAsync(code, savedCodeVerifier, ct);
var claims = NoWorkDelivery.VerifyIdToken(
    tokens.GetProperty("id_token").GetString()!, jwks,
    issuer.TrimEnd('/'), clientId, savedNonce);
```

`DiscoverAsync(CancellationToken)` 只读取 `/.well-known/openid-configuration`；`JwksAsync(CancellationToken)` 只读取 `/jwks.json`。两者是无需认证的 JSON GET，不会使用发现文档中提供的其他地址，也不会发送 client secret、Bearer token 或服务器密钥。发现文档必须包含非空字符串 `issuer`、`authorization_endpoint`、`token_endpoint`、`jwks_uri`，且 `issuer` 必须与配置值去除末尾 `/` 后完全一致；JWKS 的 `keys` 必须是对象数组。非 2xx、无效 JSON、错误根类型和超限响应均抛出 `TransportException`，不返回未验证的文档。

元数据读取限制为 1 MiB（字节数，包含流式响应），同时遵守注入 `HttpClient.MaxResponseContentBufferSize` 中更小的限制。超时沿用 `HttpClient.Timeout` 并覆盖响应体读取；默认 10 秒，显式 `Timeout.InfiniteTimeSpan` 可禁用超时。调用方取消抛出 `OperationCanceledException`，超时或网络失败抛出 `TransportException`。

注入的共享 `HttpClient` 仅可包含 `Accept`、`Accept-Encoding`、`Accept-Language`、`User-Agent` 默认请求头；存在其他默认头时，元数据方法在发请求前抛出 `InvalidOperationException`，不会修改共享客户端。请为这些公开 GET 提供无凭据客户端，并确保自定义 handler 不自动添加 cookie/凭据或跟随重定向；请求期间不要并发修改默认头。SDK 默认 handler 已禁用 cookie 与重定向。

带 `expectedNonce` 的 `VerifyIdToken` 重载先验证内层 RS256 签名、issuer、audience 与原有时间条件（60 秒容差），再要求 nonce 为相等的已签名字符串；显式空值、缺失、非字符串或不匹配均失败。原四参数调用继续保留原行为。

加密 ID token 使用单独命名的助手，避免把旧调用中的私钥误当作 nonce：

```csharp
var claims = NoWorkDelivery.DecryptIdTokenWithNonce(
    encryptedIdToken, jwks, issuer.TrimEnd('/'), clientId, savedNonce,
    privateJwk, previousPrivateJwk);

// 旧签名仍然有效，每个尾部字符串仍然是私钥：
var legacyClaims = NoWorkDelivery.DecryptIdToken(
    encryptedIdToken, jwks, issuer.TrimEnd('/'), clientId, privateJwk);
```

解密后仍须通过内层签名及所有 ID token 校验。支持现有 RSA-OAEP-256 与 P-256 ECDH-ES+A256KW / A256GCM 解密；.NET 类库不支持 X25519，应注册 RSA 或 P-256 密钥。OIDC client 凭据用于用户登录，与 `NoworkClient` 游戏服务器密钥、站点账号 JWT 和原生厂商凭据分别配置。

本地验证（无第三方测试依赖，目标运行时仍为 .NET 6）：

```powershell
dotnet build csharp/Nowork.Sdk
dotnet run --project csharp/Nowork.Sdk.Oidc.Tests
dotnet run --project csharp/Nowork.Sdk.GameData.Tests
```
