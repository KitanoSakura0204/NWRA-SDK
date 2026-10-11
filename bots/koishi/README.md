# koishi-plugin-nowork

休息室账号（NoWork Account）的 Koishi v4 插件：`bind` / `unbind` / `me` 指令，以及把其余聊天内容转发给站点的指令注册表（`/api/bot/v1/command`），同时通过 WebSocket 接收站点推送的通知并发送到对应会话。

适配器无关：使用 Koishi `Session` 的通用字段（`userId` / `channelId` / `guildId` / `platform` / `isDirect` / `content`）及统一的 `sendMessage` / `sendPrivateMessage`。多个机器人在线时用 `notificationBotId` 明确选择通知连接。

## 安装

本仓库内尚未发布到 npm。开发期在你的 Koishi 项目里以本地路径依赖安装：

```sh
npm install /path/to/D:/Project/Admin/SDK/bots/koishi
```

然后在 Koishi 控制台或 `koishi.yml` 里启用 `nowork` 插件并填写配置。

## 配置

- `baseUrl`（必填）：休息室账号站点地址。
- `botToken`（必填）：机器人令牌（管理后台「开放平台 -> 机器人」创建并签发）。
- `platform`（可选）：发给站点的 `platform` 字段；留空按 Koishi 平台名自动映射（见 `src/convert.ts` 的 `PLATFORM_ALIASES`）。
- `allowedChats`（可选）：允许使用 `bind` 指令的会话 id 白名单；留空表示不限制。
- `lang`（可选）：发给站点的 `Accept-Language`。

## 指令

- `bind <绑定码>`：绑定码在站点「个人中心 -> 机器人」页面获取，10 分钟内有效。
- `unbind`
- `me`：查看绑定状态、等级、（如用户开启展示）实名认证状态。
- 其余消息通过中间件转发给站点的指令注册表；未命中时调用 `next()`，交给其他插件处理。

## 测试

```sh
npm install
npm test
```

测试针对 `logic.ts`（纯函数，用一个假的 `BotClient` 驱动）和 `convert.ts`，不依赖真实的 Koishi `Context`，不发任何网络请求。

## 已知限制 / 待验证

- 群聊 / 频道 / 私聊的判定（`chatType`）用 `session.isDirect` 和 `session.guildId` 推断；已核对 OneBot 和参考实现（见 `src/index.ts` 的 `unverified:` 注释），其他适配器未实测。
- 通知需要一个明确的在线机器人。多个实例设置 `notificationBotId` 为 selfId；离线、选择有歧义或发送失败时不确认，等待后端重投。

---

# koishi-plugin-nowork (English)

A Koishi v4 plugin for NoWork Account: `bind` / `unbind` / `me` commands, plus a middleware
that bridges any other chat text to the site's command registry
(`/api/bot/v1/command`). It also opens a WebSocket to the site's notification stream and
sends pushed notifications to the right chat.

Adapter-agnostic: only uses `Session`'s generic fields (`userId`, `channelId`, `guildId`,
`platform`, `isDirect`, `content`); notification delivery uses Koishi's own cross-adapter
`bot.sendMessage()`, so no per-platform branching is needed there.

## Install

Not published to npm; install from the local path during development, then enable `nowork`
in your Koishi config with `baseUrl` and `botToken` set.

## Config

See the Chinese section above for the full field list (`baseUrl`, `botToken`, `platform`,
`allowedChats`, `lang`).

## Tests

```sh
npm install
npm test
```

Tests exercise `logic.ts` against a fake `BotClient` and `convert.ts` directly; no real
Koishi `Context`, no network calls.

## Known limitations / to verify

- `chatType` (private/group/channel) is inferred from `isDirect` / `guildId`; checked
  against OneBot semantics (see the `unverified:` comment in `src/index.ts`), not yet
  tested against other adapters.
- Configure `notificationBotId` when multiple bots are online; delivery remains pending if the bot is offline or a send fails.
# 通知选择（2026-10-08）

多个机器人在线时配置 `notificationBotId` 为目标机器人的 selfId；只有一个在线机器人时可省略。私聊使用 `sendPrivateMessage`，群聊使用 `sendMessage`；发送失败保留后端待投递状态，不会 ACK。

When multiple bots are online, set `notificationBotId` to the selected bot's selfId. Failed sends remain pending for retry; private notifications use the adapter's private-message method.
