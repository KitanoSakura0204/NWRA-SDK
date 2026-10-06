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

- **带 WebSocket 升级**：服务端先发 `{"type":"hello","botId","heartbeat"}`，随后每条待投递通知发 `{"type":"message","id","botId","kind?","target","message","time"}`；客户端用 `{"type":"ack","ids":[id]}`（也接受 `{"id":n}`）确认，`{"type":"ping"}` 要回 `{"type":"pong"}`。未确认的通知按退避重试，最长 24 小时后放弃。
- **不带升级（轮询）**：返回 `{"events": [...]}`（当前到期的，最多 50 条），处理后调用 `POST /api/bot/v1/events/ack`。

`target: {chatType, chatId?, userId?}` 指出该发到哪个会话；三个插件都在收到后立即发送并确认，不做排队。

**没有面向插件的 HMAC 签名 HTTP 推送**——那是网关对原生平台（Feishu/DingTalk/WeCom 等 webhook）入站请求的鉴权方式，与本 SDK 的插件鉴权（Bot 令牌 + WebSocket/轮询）是两条不同的路径，不要混用。

## 7. 兼容表

| 框架 | 适配器/环境 | 测试方式 | 实测状态 |
| --- | --- | --- | --- |
| NoneBot2 | OneBot v11 (go-cqhttp/NapCat/LLOneBot/Lagrange.OneBot) | pytest + 假 BotClient | 未对真实平台实测（见下「实测清单」） |
| NoneBot2 | QQ 官方机器人、Feishu、Telegram 等 | 同上（`convert.py` 通用适配） | 未实测；通知推送假定 OneBot 风格 `send_msg`，其他适配器需要补充分支（见插件 README「已知限制」） |
| Koishi v4 | 任意已安装的 Koishi 适配器 | node:test + 假 BotClient | 未实测；`chatType` 推断已核对 OneBot，其他适配器未核对 |
| Wechaty | 任意 puppet（padlocal、wechat4u 等） | node:test + 假 BotClient | 未实测；通知投递的 `Room.load(id)` 路径标记 `unverified`（见插件 README） |

## 8. 实测清单（需要用户对真实平台验证）

1. NoneBot2 + 真实 OneBot v11 实现：`/bind` 绑定码流程、`/unbind`、`/me`、未命中指令的穿透、以及站点推送通知能否正常收到并在群/私聊里显示。
2. NoneBot2 + 非 OneBot 适配器（Feishu/Telegram/QQ 官方机器人任一）：通知推送目前假定 `bot.send_msg(user_id=,group_id=,message=)`，在这些适配器上会调用失败；需要确认失败后的回退（遍历其他已连接的 bot）是否足够，或需要按适配器分支实现发送。
3. Koishi + 至少两种不同协议族的适配器（如 OneBot 和 Discord 或 Telegram）：`chatType` 判定（私聊/群聊/频道）、`bind`/`unbind`/`me` 指令、中间件穿透、通知投递（`ctx.bots` 选第一个在线实例）。
4. Wechaty + 真实 puppet：群聊通知投递的 `Room.load(id)` 路径（标记为 unverified，最值得优先验证）、联系人通知投递、指令前缀解析在实际微信消息格式下的表现（@提及、引用消息等 Wechaty 的 `message.text()` 返回值是否符合预期）。
5. 三个插件通用：绑定码过期/重复使用的报错文案是否清晰；机器人令牌撤销后插件的报错与重连行为；WebSocket 断线重连（拔网线/重启网关）后退避是否生效、通知是否补发且不重复。
