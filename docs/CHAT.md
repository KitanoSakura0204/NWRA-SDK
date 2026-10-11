# 聊天 SDK 接入（2026-10-08）

聊天需要全局模块开启、服务器审核通过并启用服务器聊天配置。所有 Server API 由六种语言的现有 Server 客户端调用，发送 `X-Server-Key`，沿用 10 秒默认超时和统一错误信封。

## 游戏服务器 API

| 方法 | 请求 | 成功 data |
| --- | --- | --- |
| POST `/server/v1/chat/tickets` | `{accessToken?,uid?,playerName?,playerUuid?,channels?}` | `{ticket,expiresAt,channels,embedPath,wsPath}` |
| GET `/server/v1/chat/relay` | 无 | `{items:[{id,kind,channelId,payload,attempts}]}` |
| POST `/server/v1/chat/relay/ack` | `{done:[id],failed:[{id,error}]}` | `{updated}` |
| POST `/server/v1/chat/messages` | `{player,text,playerUuid?,uid?,kind?}` | MessageView |

`accessToken` 与 `uid` 必须二选一。accessToken 是游戏通过 SDK OAuth 登录取得、属于此服务器接入应用的令牌；UID 签发需配置允许，且玩家在该服有已批准的角色。票据单次使用，失效后重新向服务器申请；不要记录 ticket 或包含 ticket 的 embedPath。

中继 `payload` 是 JSON 对象，保留消息或撤回等事件的字段。每次最多拉取 100 项，未确认消息约 20 秒后再投递、最多约 10 分钟。接入端应按 `id` 去重，发送到游戏成功后才把 ID 放入 `done`。`failed` 会永久结束该条投递，仅用于确定无法处理的消息；临时网络或游戏离线应保留待重投。这里的 ACK 使用 **done/failed**，与 Bot API 的 `ids` 不同。

`kind` 为 `chat`（默认）或 `system`；消息仍接受站点过滤、封禁和角色校验，客户端不能自行伪造玩家身份绕过校验。

| 语言 | 签发 / 拉取 / 确认 / 游戏消息 |
| --- | --- |
| Go | `IssueChatTicket` / `PollChatRelay` / `AckChatRelay` / `PostChatMessage` |
| TypeScript | `issueChatTicket` / `pollChatRelay` / `ackChatRelay` / `postChatMessage` |
| Python | `issue_chat_ticket` / `poll_chat_relay` / `ack_chat_relay` / `post_chat_message` |
| Java | `issueChatTicket` / `pollChatRelay` / `ackChatRelay` / `postChatMessage` |
| C# | `IssueChatTicketAsync` / `PollChatRelayAsync` / `AckChatRelayAsync` / `PostChatMessageAsync` |
| PHP | `issueChatTicket` / `pollChatRelay` / `ackChatRelay` / `postChatMessage` |

可运行控制台中继：在 `SDK/go` 设置 `NOWORK_BASE_URL` 与 `NOWORK_SERVER_KEY`，执行 `go run ./examples/chat-relay`；`-player Alex -post "hello"` 向站点发送一条游戏消息。例子在 stdout 写入成功后确认，实际游戏适配器应替换输出函数并持久化去重 ID。

## 浏览器 / 原生聊天连接

可直接打开签发结果的 `embedPath`，或 `POST /api/chat/session`，JSON `{ticket,server}` 兑换 `{token,expiresAt,via,channels,serverId,player}`。站内登录用户可用其 JWT 调用 `POST /api/chat/session/site`。聊天 session token 与站内 JWT、服务器密钥不同，不可互换。

Python / TypeScript / Go 另提供聊天 REST 客户端，默认发送 `Authorization: Chat <token>`：

- Python `ChatClient(base, token)`；票据兑换可先用不带 token 的实例，站内 JWT 用 `site_login=True`。
- TypeScript `new ChatClient(base, { token })`；站内 JWT 用 `siteLogin: true`。
- Go `NewChat(base, token, nil)`；站内 JWT 用 `NewSiteChat(base, jwt, nil)`。

三个客户端均提供票据兑换、站内 session 创建、结束 session、本人信息、频道列表、历史分页、消息发送、已读标记。方法分别使用 Python snake_case、TypeScript camelCase、Go PascalCase（`redeem_ticket` / `redeemTicket` / `RedeemTicket` 等）。`history` 支持 before/after/limit；`send_message` 的 nonce 应在重试同一消息时保持一致，新消息使用新的 nonce，后端按用户/站点/频道幂等处理。返回消息保留 `ownReactions`，不应从所有人的反应总数推断自己的反应。

```ts
const publicChat = new ChatClient(base);
const session = await publicChat.redeemTicket(ticket, serverCode);
const chat = new ChatClient(base, { token: session.token });
const messages = await chat.history(channelId, { after: lastSeenId, limit: 50 });
const sent = await chat.sendMessage(channelId, 'hello', { nonce: crypto.randomUUID() });
await chat.markRead(channelId, sent.id);
```

WebSocket 地址 `/api/chat/ws`，鉴权放在首帧（10 秒内），**不放查询或 Cookie**：

```json
{"op":"auth","token":"cs_..."}
```

也可首帧用 `{op:"auth",ticket:"ct_...",server:"survival"}` 或 `{op:"auth",bearer:"<site JWT>"}`。收到 `ready` 后订阅 `{op:"sub",channels:[1],since:{"1":120}}`，消费 `resume` 与 `event`。重连使用 session token，不能复用已兑换的 ticket。每 25–30 秒发送 `{op:"ping"}`；收到 `resume.more=true` 时通过 `/api/chat/channels/{id}/messages?after=…&limit=…` 补齐历史。消息发送用 `{op:"send",ref:"client-id",channelId:1,content:"hello"}`，等待带同一 ref 的 `ack` 或 `error`，不能把写入 socket 当成发送成功。

本轮 SDK 提供六种语言 Server API 和三种语言基本聊天 REST 客户端；未引入新的跨语言 WebSocket 依赖，浏览器/原生客户端使用自己的 WebSocket 实现。图片上传、表情管理、私聊创建和管理接口由现有 Web 聊天界面提供，不属于本次基本聊天 SDK 包装范围。

## English

All six Server SDKs now support one-use chat tickets, polling relay items, ACKing delivered items and posting in-game messages. ACK with `done`, reserve `failed` for permanent failures, and retain transient failures for retry. Browser/native sockets authenticate in the first frame, resume from the last seen message ID, and await the server's ACK before confirming a send. The console Go adapter is runnable without third-party dependencies.
