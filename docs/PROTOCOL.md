# 接入协议

所有 SDK 都按本文档实现，字段名以 JSON 为准。基地址 `BASE` 为用户中心后端对外地址（同时也是 OIDC `issuer`），例如 `https://account.example.com`。

## 1. 统一响应

Server API 与 SPA API 使用同一信封：

```json
{ "code": 0, "message": "操作成功", "data": { ... } }
```

`code = 0` 成功；`401` 鉴权失败；其它非零值为业务错误，`message` 为可读原因。HTTP 状态码通常为 200（鉴权失败为 401）。

## 2. Server API（游戏服务器）

鉴权：请求头 `X-Server-Key: lsk_...`（也可用查询参数 `?key=`，不推荐）。

### 2.1 GET `BASE/server/v1/whitelist`

拉取本服务器全部已通过的绑定，用于启动时/定时全量同步。

```json
{
  "server": "survival",
  "mode": 1,
  "count": 2,
  "entries": [
    { "name": "Steve", "uuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5",
      "uid": 100001, "nickname": "旅行者", "securityLevel": 2,
      "since": "2026-09-13 10:00:00" }
  ]
}
```

`mode`：`0` 关闭白名单（任何人可进入）、`1` 人工审核、`2` 满足信任等级自动通过。

### 2.2 GET `BASE/server/v1/whitelist/check?name=&uuid=`

单个玩家进服校验；`name` 与 `uuid` 至少给一个（两者都给时任一匹配即可）。

```json
{ "allowed": true, "bound": true, "reason": "", "player": { ...entry } }
```

`reason` 可能为：`not bound`、`pending review`、`banned`、`account locked`、`whitelist disabled`。
白名单关闭的服务器返回 `allowed: true, bound: false`。

### 2.3 POST `BASE/server/v1/heartbeat`

```json
{ "online": 12, "max": 50, "version": "1.21.1" }
```

后台以此展示在线人数；5 分钟无心跳视为离线。建议每 60 秒上报一次。

可选扩展字段（旧版 SDK 可省略，后台兼容不带这些字段的心跳）：

```json
{
  "online": 12, "max": 50, "version": "1.21.1",
  "tps": 19.8,
  "players": [
    { "name": "Steve", "uuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5" },
    { "name": "Alex" }
  ]
}
```

- `tps`：服务器当前 TPS（浮点数），后台用于绘制性能曲线与统计平均 / 最低 TPS。
- `players`：当前在线玩家列表，`name` 必填、`uuid` 可选；后台据此在「服务器统计」中显示在线玩家并匹配到已绑定账号。列表应与 `online` 一致，仅保留最近一次心跳的内容。

后台每台服务器每分钟最多保留一条心跳采样，保留 30 天，用于 24 小时 / 7 天 / 30 天的在线曲线与在线率统计。

## 3. OIDC（第三方应用登录）

发现文档：`BASE/.well-known/openid-configuration`。端点：

| 端点 | 地址 |
| --- | --- |
| authorize | `BASE/oauth2/authorize` |
| token | `BASE/oauth2/token` |
| userinfo | `BASE/oauth2/userinfo` |
| introspect | `BASE/oauth2/introspect` |
| revoke | `BASE/oauth2/revoke` |
| end session | `BASE/oauth2/logout` |
| JWKS | `BASE/jwks.json` |

支持 `authorization_code`（+PKCE S256）、`refresh_token`、`client_credentials`。客户端鉴权支持 `client_secret_basic` / `client_secret_post` / `none`（公开客户端需 PKCE）。

Scopes：`openid profile email phone security_level realname game_roles offline_access`。

UserInfo 关键字段：

```json
{
  "sub": "uuid", "uid": 100001,
  "name": "旅行者", "preferred_username": "traveler", "picture": "...", "alias": "traveler",
  "email": "a@b.c", "email_verified": true,
  "phone_number": "138****0000", "phone_number_verified": true,
  "security_level": 2, "realname_verified": true,
  "game_roles": [{ "server": "survival", "server_name": "生存服", "name": "Steve", "uuid": "..." }]
}
```

## 4. 错误处理约定

- Server API 网络错误 → 抛出/返回 `TransportError`；`code != 0` → `ApiError{code,message}`。
- OIDC 端点遵循 RFC 6749 错误格式 `{ "error": "...", "error_description": "..." }` → `OAuthError`。
- 所有 SDK 默认超时 10s，可配置；建议服务器插件对白名单校验做本地缓存与降级（网络异常时按 `mode` 决定放行策略）。

## 5. Webhook（事件推送）

后台「开放平台 → Webhook」注册接收地址后，用户中心会以 `POST` + `application/json` 推送事件：

```json
{ "id": "5c5b…", "event": "user.login", "timestamp": 1757800000, "data": { "userId": 1, "uid": 100001, "username": "alice", "ip": "…" } }
```

请求头：

| 头 | 说明 |
| --- | --- |
| `X-Lounge-Event` | 事件名 |
| `X-Lounge-Delivery` | 投递 ID（重投时不变） |
| `X-Lounge-Timestamp` | Unix 秒 |
| `X-Lounge-Signature` | `sha256=` + HMAC-SHA256(secret, `<timestamp>.<body>`) 的十六进制 |

接收方应：校验签名（定时比较）、拒绝时间戳偏差过大的请求（默认 5 分钟；各语言 SDK 的容差参数传 0 / null 表示关闭该检查）、幂等处理同一 `id`，并在 10 秒内返回 2xx；非 2xx 或超时将按 30s / 2m / 10m / 30m / 2h / 6h 退避重试。

事件：`user.registered` `user.login` `user.login_failed` `user.locked` `user.frozen` `user.level_changed` `user.deleted` `realname.reviewed` `game_role.reviewed` `connection.bound` `connection.unbound` `oauth.consent_granted` `oauth.consent_revoked` `server.offline`。

各语言 SDK 提供校验助手：Go `lounge.VerifyWebhook` / `ReadWebhook`，Java `LoungeWebhook.verify`，Python `lounge_sdk.verify_webhook`，TypeScript `verifyWebhook`，C# `LoungeWebhook.Verify`，PHP `Webhook::verify` / `Webhook::verifyRequest`。
