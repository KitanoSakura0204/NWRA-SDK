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

OIDC：

```go
o := nowork.NewOIDC(issuer, clientID, clientSecret, redirectURI)
http.Redirect(w, r, o.AuthorizeURL(state, nonce, "openid profile email game_roles"), 302)
// 回调
tok, _ := o.Exchange(ctx, r.URL.Query().Get("code"))
me, _  := o.UserInfo(ctx, tok.AccessToken)
```

示例：`go run ./examples/paper-check`（需要 `NOWORK_SERVER_KEY`）。
