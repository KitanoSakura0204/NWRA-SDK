# 接入协议

所有 SDK 都按本文档实现，字段名以 JSON 为准。基地址 `BASE` 为用户中心后端对外地址（同时也是 OIDC `issuer`），例如 `https://account.example.com`。

## 1. 统一响应

Server API 与 SPA API 使用同一信封：

```json
{ "code": 0, "message": "操作成功", "data": { ... } }
```

`code = 0` 成功；`401` 鉴权失败；`403` 服务器未通过审核（仅 Server API，见 §2.5）；其它非零值为业务错误，`message` 为可读原因。HTTP 状态码通常为 200（鉴权失败为 401，未通过审核为 403）。

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

### 2.4 功能模块与 GM 面板（2026-09-22）

后台可按服务器启用「功能模块」（`docs/MODULES.md`）。插件先查询自己启用了什么：

**GET `BASE/server/v1/modules`** → `{ "server": "s1", "modules": { "server-gm": { "requireLevel": 0, "auditAll": true } } }`

键为模块名，值为该服务器的模块配置。未启用的模块不出现。

`server-gm` 模块的三个接口（模块未启用时上报返回业务错误、拉取返回空数组）：

- **PUT `BASE/server/v1/gm/actions`** 上报插件支持的操作清单（启动时与变更时调用）：

```json
[
  { "key": "kick", "title": "踢出玩家", "description": "立即断开连接",
    "params": [ { "name": "player", "type": "player", "required": true, "label": "玩家" },
                { "name": "reason", "type": "string", "required": false, "label": "原因" } ] },
  { "key": "broadcast", "title": "全服广播", "params": [ { "name": "message", "type": "string", "required": true, "label": "内容" } ] }
]
```

参数定义（2026-09-28 起，向后兼容：只写 `name` / `type` / `required` / `label` 的旧插件照旧有效）：

| 字段 | 说明 |
| --- | --- |
| `type` | `string`（默认）、`text`（多行）、`number`、`boolean`、`select`、`player`（在线玩家选择器，也可手输）；别名 `int` / `integer`（整数）、`bool`、`textarea`；其他值按文本处理 |
| `description` / `placeholder` | 说明与输入提示 |
| `default` | 默认值；不符合定义时忽略 |
| `min` / `max` / `step` / `integer` | 仅 `number` |
| `maxLength` | 文本长度上限（默认 1000，`text` 4000，`player` 64） |
| `options` | `select` 的内联选项 `[{ "value": "set", "label": "设为" }]`（1–200 个，值唯一） |
| `source` | 引用插件上报的具名选项列表：`select` 的值必须在列表里（列表上报前只校验长度），`string` / `player` 用它做输入提示 |

操作还可带 `category`（清单分组）与 `danger`（`normal` / `confirm` / `critical`：后两者需确认、或输入操作标识才会下发；服主只能调得更严）。`key` 不能含冒号或空白（冒号留给后端注册表来源的 `<来源>:<key>`），最长 64 字节。

上报也可用对象形式同时带上具名列表：`{ "actions": [...], "options": { "worlds": [ { "value": "overworld", "label": "主世界" } ] } }`（省略 `actions` 保留已存的操作，省略 `options` 保留已存的列表）。**PUT `BASE/server/v1/gm/options`** 只替换具名列表（最多 20 个，每个 500 项）。回复 `{ "count": 2, "skipped": [ { "key": "bad key", "reason": "invalid key ..." } ] }`：不合规的条目（坏 key、重复、参数定义有误、超过 200 个操作）被跳过并列出，其余照常保存，不会让整次上报失败。下发时后端按定义校验参数（未声明的参数、必填、范围、选项），插件收到的参数一定符合自己上报的定义。

- **GET `BASE/server/v1/gm/commands`** 拉取待执行命令（建议与心跳同频）。返回 `[{ "id": 12, "action": "kick", "params": { "player": "Steve" }, "status": "sent" }]`；拉取即标记为 `sent`，5 分钟内未拉取的命令过期。
- **POST `BASE/server/v1/gm/commands/{id}/result`** 回执：`{ "ok": true, "result": "Steve kicked" }`（`result` ≤ 4000 字符）。
- 命令对象带 `source`：`admin`（后台管理员下发）、`owner`（服主在用户中心控制台下发，见 §2.5）或 `grant`（服主 / 管理员授权的游戏工作人员在用户中心「GM 面板」下发，`operator` 为其登录名；授权可限定操作、最低信任等级与到期时间）。预设（宏）会按顺序下发多条命令。有服主的服务器上，`source=admin` 的命令 `operator` 固定为 `"staff"`，不暴露管理员登录名；站点自有服务器照旧给出登录名。
- 每台服务器同时未完成（待拉取 + 已拉取未回执）的命令最多 50 条，超出时后台与服主控制台的下发会被拒绝，直到插件处理掉积压。

后台侧：管理员在「游戏服务 → GM 面板」选择服务器、填参数、点执行（权限 `gm:execute`，模块配置 `requireLevel` 限制操作者的信任等级），每次执行写入操作日志。SDK：Go `Modules` / `PublishGMActions` / `PublishGMCatalog`（对象形式，含具名列表，返回跳过的条目）/ `PublishGMOptions` / `PollGMCommands` / `ReportGMResult`（其他语言的 publish 方法仍上报数组形式），TypeScript `modules` / `publishGMActions` / `pollGMCommands` / `reportGMResult`，Python `modules` / `publish_gm_actions` / `poll_gm_commands` / `report_gm_result`，Java `modules` / `publishGMActions` / `pollGMCommands` / `reportGMResult`，C# `ModulesAsync` / `PublishGMActionsAsync` / `PollGMCommandsAsync` / `ReportGMResultAsync`，PHP `modules` / `publishGmActions` / `pollGmCommands` / `reportGmResult`（PHP 本机无解释器，仅静态核对）。

### 2.5 服务器审核与服主（2026-09-28）

任何账号都可以在用户中心「我的服务器」申请接入自己的服务器，管理员审核通过后才能使用 Server API。

- 密钥有效但服务器处于审核中、已驳回或已停用时，所有 `/server/v1/*` 返回 **HTTP 403** `{ "code": 403, "message": "server not approved" }`。插件应把它与 `401 invalid server key` 区分：停止上报、降低重试频率；审核通过或恢复后原密钥继续可用，无需更换。
- 自助接入的服务器，密钥由服主在控制台「接入」页生成，明文只显示一次（后端只存哈希）。后台直接创建的服务器与本功能上线前的旧服务器视为已通过，行为不变。
- 未通过审核的服务器不出现在玩家的服务器列表里，玩家也不能绑定它。

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

## 6. SDK 登录页（游戏客户端登录，2026-09-28）

游戏客户端（桌面 / 移动 / 内嵌 WebView）打开用户中心的品牌化登录页让玩家登录，拿到授权码后按 §3 换取令牌。后台「游戏服务 → SDK 登录页」按服务器配置接入应用（OAuth 客户端）、回调地址、回传方式、登录方式与外观；服务器需启用 `sdk-login` 模块且已通过审核。

> 登录页分 PC 与手机两种版式，客户端可在 URL 上加 `device=pc|mobile` 指定（缺省按请求头判断）；每种版式可由管理员换成自定义模板，模板开发见 [LOGIN_TEMPLATES.md](LOGIN_TEMPLATES.md)。

### 6.1 打开登录页

`FRONTEND/sdk/login/{serverCode}?redirect_uri=&state=&code_challenge=&code_challenge_method=S256`（`FRONTEND` 为用户中心网页地址；后端托管前端时即 `BASE`）

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `redirect_uri` | 是（code-display 可省） | 与该服务器配置的回调地址**逐字符一致**，且同时登记在接入应用上；省略时（仅 code-display）取配置的第一个 |
| `state` | 是（code-display 可省） | ≤ 512 个可见 ASCII 字符，原样带回；客户端必须校验 |
| `code_challenge` | 是 | `BASE64URL(SHA256(code_verifier))`，43 字符 |
| `code_challenge_method` | 是 | 只接受 `S256`（不接受 `plain`，缺省也不接受） |
| `scope` | 否 | 不填用服务器配置的范围；只能是其子集 |
| `nonce` | 否 | 写入 ID Token |
| `ui_locales` | 否 | `zh-CN` / `zh-TW` / `en` / `ja`，只影响本次页面语言 |
| `prompt` | 否 | `consent`：非可信应用强制再次确认 |

参数有误但 `redirect_uri` 可信时，页面显示错误，玩家点「返回游戏」后按回传方式交回 `error=invalid_request`（或 `invalid_scope`）与 `state`。`redirect_uri` 不可信、服务器未启用模块 / 未通过审核、应用停用时只显示错误页，**不做任何跳转**。玩家取消或拒绝授权时交回 `error=access_denied`。

### 6.2 回传方式

- **redirect**：页面跳转到 `redirect_uri?code=…&state=…&iss=…`（失败时为 `error` / `error_description` / `state` / `iss`）。适用于自定义协议（`mygame://auth`、`com.example.game:/cb`）、本机回环（`http://127.0.0.1:47823/cb`，端口须与登记一致）或 https。浏览器可能拦下自定义协议，页面 2.5 秒后给出手动返回的链接。
- **postMessage**：页面向 `window.opener`（弹窗）或 `window.parent`（内嵌）发送 `{ type: "lounge:sdk-login", code, state, iss, server }`（失败时带 `error`、`error_description`），`targetOrigin` 恒为回调地址的源，从不使用 `*`。接收方必须校验 `event.origin`（登录页的源）、`type` 与 `state`。该模式的回调地址只能是 https 或本机 http。后端托管前端时，该服务器登录页的响应头为 `Content-Security-Policy: frame-ancestors <回调地址的源…>`（其他模式与站内其他页面仍为 `'none'` + `X-Frame-Options: DENY`）；前端由独立 Web 服务器托管时，需为 `/sdk/login/{serverCode}` 自行下发同样的头。内嵌时页面不提供第三方登录（需要整页跳转）；通行密钥需要宿主 iframe 带 `allow="publickey-credentials-get"`。弹窗失去 opener 时页面提供「返回 {host}」按钮，跳转到 redirect 形式的地址。
- **code-display**：页面显示 8 位登录码（字母表 `ABCDEFGHJKMNPQRSTVWXYZ23456789`，分两组显示），玩家在客户端输入。登录码就是授权码：单次有效、与普通授权码同一有效期（默认 300 秒）、绑定 PKCE，没有 `code_verifier` 猜中也无用。客户端去掉空格与横线并转大写后作为 `code` 使用（SDK `NormalizeLoginCode` / `normalizeLoginCode`）。适用于主机、电视等无法接收回调的客户端。

### 6.3 换取令牌

与 §3 相同：`POST BASE/oauth2/token`，`grant_type=authorization_code`、`code`、`code_verifier`、`client_id`（机密客户端另带 `client_secret` 或 Basic 认证）；授权时带了 `redirect_uri` 则必须一致，code-display 省略时换取也可省略。授权码重放会吊销该次登录签发的全部令牌。

### 6.4 授权确认

只有在「接入应用」里勾选「跳过授权页（可信应用）」的应用直接签发授权码；其他应用首次登录显示授权确认，之后在权限范围不扩大、未带 `prompt=consent` 时不再询问。玩家没有任何操作时页面不会签发授权码：已登录的玩家看到的是「以 X 继续」。

### 6.5 安全要点

- 每次登录生成新的 `code_verifier` 与 `state`；收到结果时校验 `state`，有 `iss` 时校验 `iss`（RFC 9207）。
- 回调地址只允许 https、本机 http 与私有协议；`javascript:` `data:` `file:` `blob:` `vbscript:` `intent:` 等协议、`#` 片段、内嵌账号密码一律拒绝。保存与每次读取配置时都会校验，不合格即停止服务（fail closed）。
- 不要让玩家把登录码告诉他人（页面有提示）；登录码有效期短且单次有效。

### 6.6 页面自用接口

- `GET BASE/api/sdk-login/{serverCode}?…`（查询参数同 6.1）：品牌、`client.clientId` 与名称、回传方式、可用登录方式与身份源代码、权限说明、`requestError` / `cancel` 交回结果、当前登录用户摘要；不含任何密钥。
- `POST BASE/api/sdk-login/{serverCode}/authorize`（需登录）：请求体为 6.1 的参数加 `decision`（空 / `approve` / `deny`），返回 `{ status: "consent_required", scopes }` 或 `{ status: "done", handoff }`。
- 后台：`GET /api/admin/sdk-login/servers`、`GET` / `PUT /api/admin/sdk-login/servers/{id}`（权限 `sdklogin:view` / `sdklogin:edit`）。

SDK：Go `NewPKCE` / `S256Challenge` / `NewState` / `BuildLoginURL` / `ParseLoginCallback` / `NormalizeLoginCode`；TypeScript `createPkcePair` / `s256Challenge` / `createState` / `buildLoginUrl` / `parseLoginCallback` / `readLoginMessage` / `normalizeLoginCode`（另有 `SDK_LOGIN_MESSAGE`、`SdkLoginError`）。

## 7. 数据投递与加密（2026-09-28）

按应用 / 服务器 / Webhook 定制下发的属性、成对匿名标识（pairwise sub）与 JWE 加密（ID Token、userinfo、Server API 信封、Webhook 信封、密钥轮换、自定义属性接口），以及退出登录后的跳转规则，见 [DELIVERY.md](DELIVERY.md)。
