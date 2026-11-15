# nowork-sdk-go

```bash
go get github.com/wangework/nowork-sdk-go
```

```go
import nowork "github.com/wangework/nowork-sdk-go"

c := nowork.New("https://account.example.com", nowork.WithServerKey(os.Getenv("NOWORK_SERVER_KEY")))
res, err := c.CheckWhitelist(ctx, player.Name, player.UUID.String())
switch {
case nowork.IsNotAllowed(res, err): // 拒绝，包含无法读取的加密响应；不能按网络异常放行
case err != nil:            // 其他网络/业务错误：按服务器策略处理，建议本地缓存并默认拒绝
default:                    // 放行, res.Player 含 UID / 昵称 / 信任等级
}
```

## 服务器响应解密

在后台为服务器启用 [Delivery 加密](../docs/DELIVERY.md) 后，用现有 `ParseDecryptionKey` 解析私有 JWK，并把密钥传给客户端。`New` 的原有用法不变；未配置密钥时，任何无法读取的加密响应都返回 `*EncryptionError`。

```go
// 在返回 error 的函数中；privateJWK 是安全读取的私有 JWK 文件内容。
key, err := nowork.ParseDecryptionKey(privateJWK)
if err != nil {
    return err
}
c := nowork.New("https://account.example.com",
    nowork.WithServerKey(os.Getenv("NOWORK_SERVER_KEY")),
    nowork.WithDecryptionKeys(key),
    nowork.WithServerCode("survival"),
)
whitelist, err := c.Whitelist(ctx)
if err != nil {
    return err // 尤其不要把 EncryptionError 当成白名单关闭或可放行的网络异常。
}
_ = whitelist
```

`WithServerCode` 使用服务器的代码，核对认证过的 `ctx` 是否精确等于 `server:<代码>:<当前路由>`。它可省略；省略时仍校验 `server` 类型、非空服务器代码及精确路由。它不替代 `WithServerKey`，也不赋予账户 JWT、API Key 或聊天令牌服务器权限。

轮换期间可用 `WithDecryptionKeys(previousKey, currentKey)` 同时配置多把密钥；SDK 保存密钥列表与 `kid` 的快照，并沿用 Delivery 的密钥选择规则。非空密钥列表要求 `whitelist` 和 `whitelist/check` 成功响应必须加密，拒绝明文降级、错误密钥、错误上下文、损坏信封及缺少有效 `mode` / `allowed` 的解密结果。`heartbeat`、GM、模块和其他当前未加密的接口仍接受正常明文；这些接口若返回加密响应，也会完成解密与上下文检查，包括无返回值的写操作。

`Raw` 继续返回未经解密的原始 `Body`、`Data` 和响应头，不执行明文降级检查；需要手动处理时可调用 `OpenServerData(raw.Body, "server:survival:whitelist", key)`。HTTP / 业务错误仍分别返回 `TransportError` / `ApiError`；默认 10 秒超时、自定义 HTTP 客户端和服务器响应 4 MiB 上限保持不变。

Configured keys automatically decrypt typed Server API calls using the existing Delivery helpers. Only the two whitelist routes require encryption when keys are set; plaintext heartbeat and GM replies remain valid. The authenticated route is always checked, and `WithServerCode` adds an exact server binding. Treat `EncryptionError` as a refusal. `Raw` retains the original response for manual processing.

## OIDC 登录与 nonce 校验

OIDC 使用应用的 `clientID` / `clientSecret`，与游戏服务器的 `WithServerKey`、账户 JWT、原生客户端凭据分别配置。发起授权前生成随机 `state` 和 `nonce`，保存在服务端会话中；回调先核对 `state`，再取出该次请求保存的 nonce。

```go
o := nowork.NewOIDC(issuer, clientID, clientSecret, redirectURI)
http.Redirect(w, r, o.AuthorizeURL(state, nonce, "openid profile email game_roles"), 302)
```

在返回 error 的回调函数中，`expectedNonce` 必须来自这次授权请求的服务端会话：

```go
if _, err := o.Discover(ctx); err != nil {
    return err
}
jwks, err := o.JWKS(ctx)
if err != nil {
    return err
}
tok, err := o.Exchange(ctx, r.URL.Query().Get("code"))
if err != nil {
    return err
}
claims, err := nowork.VerifyIDTokenWithNonce(
    tok.IDToken, jwks, o.Issuer, o.ClientID, expectedNonce, time.Now(),
)
if err != nil {
    return err
}
me, err := o.UserInfo(ctx, tok.AccessToken)
if err != nil {
    return err
}
_ = claims
_ = me
```

`Discover(ctx)` 返回 `map[string]any`，保留未知元数据；它要求 `issuer` 精确等于 `NewOIDC` 去掉末尾斜杠后的配置值，并要求 `issuer`、`authorization_endpoint`、`token_endpoint`、`jwks_uri` 都是非空字符串。`JWKS(ctx)` 返回完整 `[]byte`，保留未知 JWK 字段，并要求 `keys` 是对象数组。

两者固定请求配置 issuer 下的 `/.well-known/openid-configuration` 和 `/jwks.json`，使用无凭证 JSON GET，不跟随元数据声明的地址或 HTTP 重定向，SDK 不附带 `client_secret`、Bearer token、服务器密钥或 Cookie；自定义 HTTP 传输也应保持这两条请求无凭证。带用户名密码、查询参数或片段的 issuer 会被拒绝；默认 HTTP 超时仍为 10 秒，调用受 `ctx` 控制。非 2xx、读取失败、格式错误、错误根类型和超过 1 MiB 的响应都返回 `*TransportError`；大小错误可用 `errors.Is(err, nowork.ErrResponseTooLarge)` 识别，恰好 1 MiB 的合法响应可以接受。失败后不要改用未验证的 issuer 或 discovery 文档。

`VerifyIDTokenWithNonce` 先执行现有 RS256 签名、issuer、audience、`exp` / `nbf` 校验（60 秒容差），再要求非空 expected nonce 与签名中的字符串 `nonce` 精确相等；缺失、非字符串、空 expected nonce 或不匹配都失败。已启用 ID token Delivery 加密的应用改用 `DecryptIDTokenWithNonce(tok.IDToken, jwks, o.Issuer, o.ClientID, expectedNonce, key)`，轮换期间可追加多把私有解密密钥；它会解密并校验内部签名和 nonce。原有 `VerifyIDToken` 和 `DecryptIDToken` 的签名与行为保持兼容，仍不绑定 nonce。以上功能只依赖标准库，最低版本仍为 Go 1.21。

示例：`go run ./examples/paper-check`（需要 `NOWORK_SERVER_KEY`）。
