# 角色 GM 免密登录（2026-10-08）

此功能把用户已批准的游戏角色登录到独立 GM 站点。它与 SDK OAuth 登录、GM 指令下发是三个独立接口；成功只证明角色身份，GM 自己决定该角色可以执行哪些操作。

## 配置与浏览器流程

1. 服务器启用独立 GM 登录模块，配置 GM 启动地址和回调地址。两者必须同源 HTTPS，不能带查询、片段或账号密码；后端调试模式才允许 loopback HTTP。
2. 用户持站内登录 JWT 调用 `POST /api/account/gm/servers/{serverId}/launch`，JSON `{ "roleId": 7 }`，得到 `{ "startUrl": "…", "expiresIn": 180 }`。`startUrl` 带一次性 `gm_request` 和 `gm_server_id`，应立即在浏览器打开，不能缓存到日志或分享。
3. GM `/start` 为浏览器创建短期服务端会话，生成新的 PKCE verifier 和随机 state。verifier 为 43–128 个 RFC 7636 字符；state 为 16–256 个 `[A-Za-z0-9._~-]` 字符。会话 Cookie 使用 HttpOnly、Secure、SameSite=Lax。
4. 浏览器跳转到中心 `/home/gm/authorize?request=<gm_request>&state=…&code_challenge=…&code_challenge_method=S256`。中心核对当前登录账号和角色后跳回配置的精确回调地址，携带 `code` 和 `state`。
5. GM 必须先用浏览器会话核对 state，再在服务端兑换 code；兑换成功后轮换本地会话，跳到没有 code 的页面。不能用浏览器收到的角色参数直接登录。

## 服务端兑换

`POST /server/v1/gm/sso/exchange`，请求头 `X-Server-Key`：

```json
{"code":"one-use-code","codeVerifier":"the-verifier-kept-in-the-GM-session","redirectUri":"https://gm.example/callback"}
```

成功信封的 `data`：

```json
{"serverId":2,"serverCode":"survival","subject":"gm-role:7","character":{"id":7,"roleName":"Alex","roleUuid":"…"},"authenticatedAt":1791417600,"scope":"character:login"}
```

授权码 90 秒有效、单次原子消费，绑定服务器、角色、PKCE 和回调地址。过期、重放、错误密钥、PKCE/回调不匹配、账号冻结、角色失效或被封禁、服务器/模块/服主/密钥/配置变更等拒绝统一返回 403。插件不能把 403 当作网络失败放行；重新从用户中心发起登录。服务端错误继续遵守各 SDK 的 ApiError / TransportError / EncryptionError 分类。

| 语言 | 兑换方法 |
| --- | --- |
| Go | `Client.ExchangeGMCode(ctx, code, verifier, redirectURI)` |
| TypeScript | `NoworkClient.exchangeGMCode(code, verifier, redirectUri)` |
| Python | `NoworkClient.exchange_gm_code(code, verifier, redirect_uri)` |
| Java | `NoworkClient.exchangeGMCode(code, verifier, redirectUri)` |
| C# | `NoworkClient.ExchangeGMCodeAsync(code, verifier, redirectUri, ct)` |
| PHP | `NoworkClient::exchangeGmCode(code, verifier, redirectUri)` |

Go/C#/TS 提供结果类型；Python/Java/PHP 保留 JSON 对象字段。六种语言沿用现有 Server API 的可配置默认 10 秒超时。服务器密钥和 verifier 不应进入浏览器 URL、前端包或访问日志。

可运行 RP 示例：[examples/gm-sso-python](../examples/gm-sso-python/README.md)。用户门户的发起方法见 [ACCOUNT_API.md](ACCOUNT_API.md)。

## English

Character SSO uses a short-lived launch request, a browser-bound PKCE/state flow, and a one-use backend code exchange. All six Server SDKs expose the exchange. The result grants `character:login` identity only, not GM administration. Validate state before exchange, bind the returned server ID and subject to your own authorization rules, rotate the local session, and keep server keys/verifiers out of browser URLs and logs. Invalid or stale authorization returns 403; restart the flow instead of retrying consumed codes.
