# 插件宿主 ABI 与协议（v1）

本文是插件与用户中心之间的全部约定：处理函数收到什么、能调用什么、外部应用与插件页面怎么通信、插件包怎么签名。实现：`Backend/internal/modules/plugins`。

## 1. 处理函数的输入：信封

每次调用（wasm 的输入、外部应用与声明式钩子收到的 POST 正文）都是同一个 JSON：

```json
{
  "id": "5f0c…",               // 本次投递的唯一 id（去重用）
  "type": "event",             // event | filter | job | action | ui
  "point": "user.login",
  "plugin": { "id": "com.example.hello", "version": "1.0.0" },
  "settings": { "greeting": "Hello" },   // 插件当前设置（含 secret 字段）
  "server": { "id": 1, "code": "s1", "name": "…", "gameType": "…", "status": 0, "version": "…", "onlinePlayers": 3, "maxPlayers": 20 },
  "user": { "userId": 7, "uid": 100001, "status": 0, "nickname": "…" },   // 按数据权限过滤
  "data": { … },
  "timestamp": 1790000000
}
```

| type | data | 输出（wasm 用 `output_write`，外部应用用 2xx 响应正文） |
| --- | --- | --- |
| `event` | 事件载荷（按数据权限去掉 ip、email 等字段） | 忽略 |
| `filter` `token.claims` | `{userId, uid, channel, clientId}` | JSON 对象：声明过的字段，或 `{"claims": {...}}` |
| `filter` `server.whitelist_check` | `{name, uuid, userId}` | `{"deny": true, "message": "…"}` 拒绝；其他一律放行 |
| `job` | `{job, every}` | 忽略 |
| `action`（GM） | `{action, params, source, commandId, operator: {userId, uid, superAdmin}}`，带 `server`；`source` 是下发者：`admin`（后台）、`owner`（服主）、`grant`（被授权的账号），只给工作人员用的操作请自己检查 | 10 秒内返回任意 JSON，作为结果存进 GM 面板的命令记录（最多 4000 字节，约定 `{"message": "…"}`）；出错时命令记为失败 |
| `ui` | `{slot, index, action, payload, audience}`；后台页面另有 `operator` | 任意 JSON，原样交给页面 |

`ui` 的 `user`：用户中心页面是当前登录用户；后台页面是宿主页面正在显示的用户（必须在操作者的数据范围内，否则调用被拒绝）。

## 2. WebAssembly

**导出**：`memory`；每个处理函数 `() -> i32`（清单里 `handler` 写的名字）；可选 `_initialize`（反应器模块，例如 Go `-buildmode=c-shared`）。返回 0 = 成功；非 0 = 处理函数自己报告的错误（此时输出作为错误信息，建议 `{"error": "…"}`），只记审计，**不计入**自动停用。

**导入**，模块名 `lounge`（除此之外只允许 `wasi_snapshot_preview1`，其他导入在安装时拒绝）：

| 函数 | 签名 | 说明 |
| --- | --- | --- |
| `input_len` | `() -> i32` | 信封字节数 |
| `input_read` | `(ptr, len: i32) -> i32` | 把信封复制进内存，返回复制的字节数 |
| `output_write` | `(ptr, len: i32) -> i32` | 追加输出，总计 ≤ 256 KiB；超出返回 -4 |
| `log` | `(level, ptr, len: i32) -> i32` | 需要能力 `log`；level 0 debug / 1 info / 2 warn / 3 error |
| `call` | `(name_ptr, name_len, args_ptr, args_len: i32) -> i32` | 调用 §3 的宿主函数，参数是 JSON；返回结果长度，或负的状态码 |
| `result_read` | `(ptr, len: i32) -> i32` | 读取上一次 `call` 的结果（失败时是 `{"error": "…"}`） |

**状态码**：0 成功，-1 能力未授予，-2 参数无效，-3 不存在，-4 超出限额，-5 宿主函数失败，-6 未知函数。

**WASI**：可用，但没有预打开目录（无文件系统）、没有环境变量与参数；stdout 丢弃，stderr 的前 2 KiB 附在错误信息里（Go 的 panic 栈会出现在审计日志）；时钟是真实时间，随机数来自 crypto/rand。

**每次调用都是全新实例**（内存从零开始），状态只能放在 `kv` 里。

| 限制 | 值 |
| --- | --- |
| 内存 | `wasm.memoryMb` 与后台上限 `plugins.wasm_memory_mb`（默认 64）中较小者；声明的内存超过上限的模块在安装时拒绝 |
| 时间 | `wasm.timeoutMs` 与 `plugins.wasm_timeout_ms`（默认 3000）中较小者；过滤另受 `plugins.sync_budget_ms`（默认 300）约束。到时模块被强制关闭，死循环也会停 |
| 燃料 | 每次调用 10000：`input_len` / `input_read` / `result_read` 各 1，`output_write` 1 + 长度/4096，`log` 5，`call` 20，`call("http.fetch")` 500；用完立即中止 |
| 其他 | 日志每次最多 100 行（保存 20 行，每行 1 KiB）；`call` 参数 ≤ 64 KiB；每个插件最多 4 个并发实例 |

wazero 没有逐指令计量，所以 CPU 由墙钟时限约束，燃料计量的是与宿主的交互。陷阱（trap）、超时、超出限额、传输错误都算失败；连续失败 `plugins.max_failures`（默认 5）次插件自动停用，超级管理员收到通知。

## 3. 宿主函数

wasm 用 `call(name, args)`，外部应用用 `POST /api/plugin/v1/call/<name>`（§4），同一张表、同样的能力检查。参数严格解码：多余字段返回 -2。

| 函数 | 能力 | 参数 → 结果 |
| --- | --- | --- |
| `log` | `log` | `{level, message}` → `{}` |
| `kv.get` | `kv` | `{key}` → `{found, value}` |
| `kv.set` | `kv` | `{key, value}` → `{}`；key 1–128 字节，value 任意 JSON ≤ 64 KiB，每插件最多 10000 个键 |
| `kv.delete` | `kv` | `{key}` → `{deleted}` |
| `kv.list` | `kv` | `{prefix?, after?, limit? ≤ 100}` → `{keys}`（按字典序，用 `after` 翻页） |
| `user.get` | `user.read` | `{userId}` 或 `{uid}` → 用户（按数据权限过滤） |
| `fields.get` | `fields` | `{userId \| uid}` → `{userId, values}` |
| `fields.set` | `fields` | `{userId \| uid, values: {key: 值 \| null}}` → `{}`；按清单的类型与长度校验，null 删除 |
| `notify.send` | `notify` | `{userId \| uid, title ≤ 64, body? ≤ 500, link? "/home/…"}` → `{}`；每插件每秒 2 条（突发 100）；显示为「插件消息：来自插件 X」 |
| `http.fetch` | `http` | `{method?, url, headers?, body? ≤ 256 KiB}` → `{status, headers: {"content-type"}, body}`；见下 |
| `server.get` | `server.read` | `{id \| code}` → 服务器（server 范围插件只能看到它开启的服务器） |
| `servers.list` | `server.read` | `{}` → `{servers}`（已审核，最多 200 台） |

`http.fetch`：只允许 https（调试模式下也允许 http）；主机必须在 `http.allowHosts`；连接前按解析出的 IP 再做一次 SSRF 检查（内网、回环、元数据地址一律拒绝）；最多 3 次跳转且必须留在允许的主机内；10 秒超时；响应 ≤ 1 MiB（`body` 是文本，二进制会被替换为 U+FFFD）；不能设置 Host、Content-Length、Connection 等逐跳头；User-Agent 固定为 `LoungePlugin/1 (<插件 id>)`。

## 4. 外部应用与声明式钩子

**投递**：external 插件的所有调用都 POST 到 `external.endpoint`，声明式插件的事件 POST 到各自的 `url`。签名方式与核心 Webhook 相同：

```
Content-Type: application/json
X-Lounge-Plugin: com.example.app        X-Lounge-Event: user.login
X-Lounge-Delivery: <信封 id>             X-Lounge-Timestamp: <unix 秒>
X-Lounge-Signature: sha256=<hex HMAC-SHA256(签名密钥, "<timestamp>.<正文>")>
```

接收方必须校验签名、拒绝超过 5 分钟的时间戳、按 `X-Lounge-Delivery` 去重。签名密钥（`lps_…`）安装时只显示一次，可在插件详情里重新生成。

**响应**：10 秒内返回 2xx（过滤在同步预算内）；正文 ≤ 256 KiB，就是处理函数的输出。非 2xx、超时、连接失败都算失败。业务错误请返回 200 与 `{"error": "…"}`。事件只投递一次、不重试；每个实例 4 个工作协程、队列 1024，队列满时丢弃并记审计（`dropped`）。慢任务请先回 204 再在后台处理。

**回调接口**：`/api/plugin/v1`，请求头 `X-Plugin-Token: lpt_…`（安装时只显示一次）。

| 请求 | 说明 |
| --- | --- |
| `GET /me` | `{id, version, scope, permissions, capabilities, settings, servers}` |
| `POST /call/<函数>` | 正文是参数，返回 `{"code": 0, "message": "ok", "data": 结果}` |

失败时 HTTP 403（令牌无效、插件停用、能力未授予）/ 400 / 404 / 429 / 502，正文 `{"code": <状态码>, "message": "…"}`。每个插件每秒 10 次（突发 20），每次请求写审计。示例：`examples/external-app`。

## 5. 插件页面与桥接

页面地址 `/api/plugin-ui/<票据>/<入口>`（票据签名、一小时有效，页面里的相对地址照常可用；插件更新或停用后页面立即失效）。每个响应都带：

```
Content-Security-Policy: sandbox allow-scripts; default-src 'none'; script-src 'self' <站点>; style-src 'self' <站点> 'unsafe-inline';
  img-src 'self' <站点> data: blob:; font-src 'self' <站点> data:; media-src 'self' <站点>; connect-src 'none'; frame-src 'none';
  worker-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self' <站点>
```

因此页面是不透明源：**不能写内联脚本**（脚本放包里的 .js 文件），不能 fetch / XHR / WebSocket，没有 Cookie 与 localStorage，不能提交表单或弹窗，只能嵌在本站。宿主 iframe 另有 `sandbox="allow-scripts"`、`referrerpolicy="no-referrer"`、`allow=""`。

**协议**（`examples/hello/ui/bridge.js` 可直接复制）：页面 → 宿主 `{lounge: 1, id, method, params}`；宿主 → 页面 `{lounge: 1, id, ok, result | error}`；宿主推送 `{lounge: 1, event: "context", data}`（主题或语言变化）。宿主只接受自己 iframe 发出的消息。

| method | params → result |
| --- | --- |
| `ready` / `context` | → `{slot, plugin, index, locale, theme, context}`（`context` 是宿主传入的，如后台用户详情的 `{userId}`） |
| `resize` | `{height}` → 调整 iframe 高度（60–2000 px） |
| `invoke` | `{action, payload}` → 插槽 `handler` 的输出；action 匹配 `^[A-Za-z0-9_.:-]{1,64}$`，payload ≤ 64 KiB，每用户每秒 5 次 |
| `navigate` | `{path}` → 站内跳转，用户中心插槽只能 `/home/…`，后台插槽只能 `/admin/…` |
| `toast` | `{type: info\|success\|warning\|error, message ≤ 200}` → 提示（前缀插件名） |
| `open` | `{url}` → https 链接，用户确认后在新标签页打开 |

## 6. 插件包与签名

zip 根目录：`plugin.json`、可选 `signature.json` 与其他文件。

```json
{ "algorithm": "ed25519", "keyId": "<SHA-256(公钥) 前 16 个十六进制字符>", "signature": "<64 字节签名的标准 base64>" }
```

- **摘要** = SHA-256（十六进制）覆盖按路径排序的每一行 `"<路径>\x00<该文件 SHA-256 十六进制>\n"`，包含除 `signature.json` 外的所有文件（含 `plugin.json`）。
- **签名内容** = `"lounge-plugin-v1\n<id>\n<version>\n<摘要>\n"`（id、version 取自 plugin.json，去掉首尾空白）。
- **受信任的发布者**：后台设置里的 32 字节 ed25519 公钥（标准 base64），只有超级管理员能改。
- **安装策略**：受信任签名 → 任何有 `plugin:manage` 的管理员可以安装；未签名或签名者未受信任 → 拒绝，除非超级管理员开启了「允许未签名插件」**且**安装者是超级管理员；在受信任公钥下验证失败 → 永远拒绝（包被改过）。

## 7. 插件市场索引

管理员配置的 https 地址（默认关闭），格式：

```json
{
  "name": "Example plugins",
  "plugins": [{
    "id": "com.example.hello", "name": "Hello", "author": "…", "description": "…", "kind": "wasm", "homepage": "https://…",
    "versions": [{ "version": "1.0.0", "url": "com.example.hello-1.0.0.zip", "sha256": "<zip 的 SHA-256>", "keyId": "…", "size": 960957 }]
  }]
}
```

`url` 可以相对索引地址。索引 ≤ 2 MiB、最多 500 个插件，经 SSRF 防护获取，缓存 10 分钟并由定时维护刷新（用于“有新版本”提示）。索引只是指针：下载的包必须与 `sha256` 一致、通过 §6 的签名校验，且 id / version 与索引一致。用 `loungeplugin index -dir dist` 生成。

### 7.1 索引 1.1

`"format": "1.1"` 的索引在 1.0 的基础上增加（都可选，1.0 读取方忽略它们）：

- 顶层：`url`（市场站点）、`updatedAt`、`publisher`（`keyId`、`name`、`url`、`publicKey`，仅供展示，信任仍以后台设置为准）、`revokedKeys`（`[{keyId, reason, revokedAt}]`：宿主拒绝安装这些密钥签名的版本）。
- 每个插件：`scope`、`locales`、`icon`、`screenshots`（`[{url, caption}]`）、`categories`、`keywords`、`repository`、`support`、`featured`、`downloads`、`updatedAt`；`icon` 与截图 `url` 可相对索引地址。
- 每个版本：`publishedAt`、`changelog`、`permissions`、`capabilities`、`engines`、`yanked` + `yankedReason`（显示但不能安装）。

定义见 [schema/index.schema.json](schema/index.schema.json)。`loungeplugin index` 写 1.1 索引（`-publisher-key`、`-revoked`、`-yanked`，并把图标、截图复制到 `assets/<id>/<version>/`）；站点自带的插件市场模块在 `GET /api/market/index.json` 提供同样格式。
