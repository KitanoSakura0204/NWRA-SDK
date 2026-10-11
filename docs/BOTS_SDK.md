# 机器人框架插件（BOTS SDK）

休息室账号的「机器人 API」(`/api/bot/v1`) 面向开源机器人框架：你的 NoneBot2 / Koishi / Wechaty 机器人安装一个插件，站内账号就能通过聊天完成绑定、查询，并接收站点推送的通知。本文档是协议摘要和各插件的兼容表；网关侧的完整说明（原生平台适配、管理后台配置）见 `docs/BOTS.md`。协议字段读自后端已落地代码 `Backend/internal/modules/bots/{api.go,ws.go,hub.go,platform.go}`（commit df7b675 / d52a936），阅读日期 2026-09-29；目前没有对外发布的协议文档，本文档即第一份。

## 1. 已实现的插件

| 目录 | 包名 | 框架 | 语言 |
| --- | --- | --- | --- |
| `SDK/bots/ts-client` | `@nowork/bot-client` | （共享客户端，非插件） | TypeScript |
| `SDK/bots/python-client` | `nowork-bot-client` | （共享客户端，非插件） | Python |
| `SDK/bots/nonebot2` | `nonebot-plugin-nowork` | NoneBot2 | Python |
| `SDK/bots/koishi` | `koishi-plugin-nowork` | Koishi v4 | TypeScript |
| `SDK/bots/wechaty` | `wechaty-plugin-nowork` | Wechaty（个人微信） | TypeScript |

三个插件给用户的功能一致：`/bind <code>`（绑定）、`/unbind`（解绑）、`/me`（查看绑定状态/等级/实名状态），以及把其余未命中内置指令的聊天转发给站点的指令注册表；同时通过 WebSocket 接收站点推送的通知并发到绑定的聊天。各自的配置项、安装方式见各插件目录下的 README（中文在前，英文在后）。

## 2. 鉴权

每个插件实例对应后台「开放平台 → 机器人」里创建的一个机器人和一个令牌。所有请求带：

```
Authorization: Bot <token>
```

（也接受 `Bearer <token>`，大小写不敏感的前缀。）`/api/bot/v1/events` 的 WebSocket 升级无法带自定义头时改用查询参数 `?token=<token>`。

令牌无效/未启用 → HTTP 401；超出该机器人的速率限制 → HTTP 429；IP 不在该机器人的白名单内 → HTTP 403。

## 3. 信封

与站内其余 API 一致：

```json
{"code": 0, "message": "", "data": {...}}
```

`code` 为 0 表示成功；非 0 时 `data` 省略，`message` 是面向用户的错误信息（按请求的 `Accept-Language` 翻译；不带该头则使用站点机器人模块的默认语言设置）。

## 4. 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/bot/v1/bind` | `{platform, platformUserId, platformName?, code}` → `{userId, uid, nickname, level}`。`code` 是用户在「个人中心 → 机器人」页面获取的绑定码（10 分钟有效，一次性） |
| POST | `/api/bot/v1/unbind` | `{platform, platformUserId}` → `{unbound}` |
| GET | `/api/bot/v1/lookup` | `?platform=&platformUserId=` → `{bound, uid?, nickname?, level?, levelName?, realnameVerified?}`（`realnameVerified` 只在用户开启展示时返回） |
| POST | `/api/bot/v1/command` | `{platform, platformUserId, platformName?, chatType, chatId, text}` → `{handled, reply: Message}`。站点侧的指令注册表（`help`/`level`/`servers`/`ticket` 等，其他模块可追加注册） |
| GET | `/api/bot/v1/commands` | → `{prefixes, commands: [{name, aliases, help, needBind, minLevel}]}`，用于插件自己的帮助/自动补全 |
| GET | `/api/bot/v1/events` | 见下「通知」 |
| POST | `/api/bot/v1/events/ack` | `{ids:[...]}` 或 `{id}` → `{acked}` |

`platform` 的取值与网关的 codec kind 一致（如 `onebot11`、`qq_bot`、`feishu`、`telegram`、`wechat_mp`、`generic`），由插件按所在框架的平台/适配器名映射；没有对应 codec 时用 `generic`。`chatType` 取 `private` / `group` / `channel`。

## 5. 消息格式

```ts
type Segment = { type: string; data?: Record<string, string> }
type Message = Segment[]
```

`type` 目前见到的取值：`text`（`data.text`）、`mention`（`data.userId`）、`mention_all`、`image`（`data.url`）、`link`（`data.url`, `data.label?`）、`reply`、`markdown`。插件把 `Message` 渲染成本框架能发的内容；三个参考实现都把非 `text` 段降级为括号占位符（如 `[image: ...]`），而不是悄悄丢弃。

## 6. 通知（站点推送）

`GET /api/bot/v1/events`：

- **带 WebSocket 升级**：服务端先发 `{"type":"hello","botId","heartbeat"}`，随后每条待投递通知发 `{"type":"message","id","botId","kind?","target","message","time"}`；客户端用 `{"type":"ack","ids":[id]}`（也接受 `{"id":n}`）确认。客户端按 `heartbeat` 指定的秒数主动发送 JSON `{"type":"ping"}`，服务端回复 `{"type":"pong"}`；如收到对端 ping，客户端也回复 pong。两个共享客户端将间隔限制在 5–60 秒，缺省或无效值回退到 60 秒，并在关闭、停止或重连时清理旧连接的心跳；WebSocket 控制帧 ping 不能替代此应用层心跳。未确认的通知按退避重试，最长 24 小时后放弃。
- **不带升级（轮询）**：返回 `{"events": [...]}`（当前到期的，最多 50 条），处理后调用 `POST /api/bot/v1/events/ack`。

`target: {chatType, chatId?, userId?}` 指出目的会话。三个插件只在发送成功后确认；机器人离线、目标缺失、适配器发送失败时保留待投递，服务端会重试。多机器人实例必须明确选择通知机器人，避免同一个 ID 被误发到另一个平台。

**签名 HTTP 推送已支持**：framework 模式配置 `pushUrl` 与 `pushSecret`，无 WebSocket 连接时后端向插件 POST 一个与 WebSocket 相同的 `PluginEvent`。请求头为 `X-Nowork-Event: bot.message`、`X-Nowork-Delivery: <event.id>`、`X-Nowork-Timestamp: <Unix秒>`、`X-Nowork-Signature: sha256=<hex HMAC-SHA256(pushSecret, timestamp + "." + 原始body)>`。

Python `verify_push(secret, headers, bodyBytes)` 与 TypeScript `verifyPush(secret, headers, rawBytes)` 校验原始字节、签名、默认 300 秒时间窗、事件名和投递 ID 后返回事件；不能先解析并重新序列化 JSON。接收方以事件 ID 幂等处理，投递成功才返回 2xx；失败返回非 2xx 触发重试。这是 framework Bot 推送协议，与普通站点 Webhook 信封和原生平台回调不同。签名助手不自动启动 HTTP 监听，宿主框架自行挂载路由。

```python
from nowork_bot_client import verify_push
event = verify_push(push_secret, request_headers, raw_body_bytes)
# Deliver through the selected adapter, persist event.id, then return HTTP 204.
```

Python 与 TypeScript Bot HTTP 客户端默认总超时 10 秒（Python `timeout`，TypeScript `timeoutMs`），覆盖响应体读取。非零服务器 code 为 `BotApiError`；网络/超时/坏信封/异常 HTTP 为 `BotTransportError`，该类型继承 `BotApiError`（本地 code=-1）以兼容现有适配器错误处理。

## 7. 兼容表

| 框架 | 适配器/环境 | 测试方式 | 实测状态 |
| --- | --- | --- | --- |
| NoneBot2 | OneBot v11 (go-cqhttp/NapCat/LLOneBot/Lagrange.OneBot) | pytest + 假 BotClient | 未对真实平台实测（见下「实测清单」） |
| NoneBot2 | OneBot v12、QQ 官方机器人、Feishu、Telegram | `delivery.py` 适配器发送分支 + 假 Bot 测试 | 已实现各自发送 API；未对真实平台实测，QQ 主动消息仍受平台配额/权限约束 |
| Koishi v4 | 任意已安装的 Koishi 适配器 | node:test + 假 BotClient | 未实测；`chatType` 推断已核对 OneBot，其他适配器未核对 |
| Wechaty | 任意 puppet（padlocal、wechat4u 等） | node:test + 假 BotClient / 通知 API | 使用 1.20.2 公开的 Room/Contact `find({id})`、`say()` 和 `emitError()`；真实 puppet 投递及权限未实测 |

## 8. 实测清单（需要用户对真实平台验证）

1. NoneBot2 + 真实 OneBot v11 实现：`/bind` 绑定码流程、`/unbind`、`/me`、未命中指令的穿透、以及站点推送通知能否正常收到并在群/私聊里显示。
2. NoneBot2 + 非 OneBot 适配器（Feishu/Telegram/QQ 官方机器人任一）：核对配置的通知 bot ID、目标 ID 类型和平台发送权限；发送失败必须保持未确认。多机器人设置 `nowork_notification_bot_id`。
3. Koishi + 至少两种不同协议族的适配器：核对私聊 `sendPrivateMessage` 和群聊 `sendMessage`、通知 `notificationBotId`、指令和未命中透传。
4. Wechaty + 真实 puppet：通过公开的 Room/Contact `find({id})` 按不透明 ID 投递到群聊和联系人；目标不存在或发送失败时保持未确认。验证指令前缀在实际微信消息格式下的表现（@提及、引用消息等 `message.text()` 返回值是否符合预期）。
5. 三个插件通用：绑定码过期/重复使用的报错文案是否清晰；机器人令牌撤销后插件的报错与重连行为；WebSocket 断线重连（拔网线/重启网关）后退避是否生效、通知是否补发且不重复。

## 9. English update (2026-10-08)

Framework bots support signed HTTP pushes while no WebSocket is connected. Verify the exact raw body with `verify_push` / `verifyPush`, persist the delivery ID, and return 2xx only after successful delivery. HTTP clients now enforce a configurable 10-second total deadline and distinguish `BotTransportError` from server business errors. Failed adapter delivery is not acknowledged. NoneBot provides explicit OneBot v11/v12, Telegram, Feishu and QQ send branches; select a notification bot explicitly when several connect. Real-platform verification remains required.
