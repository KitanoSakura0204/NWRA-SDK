# koishi-plugin-nowork

休息室账号（NoWork Account）的 Koishi v4 插件：`bind` / `unbind` / `me` 指令，以及把其余聊天内容转发给站点的指令注册表（`/api/bot/v1/command`），同时通过 WebSocket 接收站点推送的通知并发送到对应会话。

适配器无关：只使用 Koishi `Session` 的通用字段（`userId` / `channelId` / `guildId` / `platform` / `isDirect` / `content`），理论上适用于任意已安装的 Koishi 适配器（OneBot、Discord、Telegram、Lark/飞书……）；发送通知时用 `ctx.bots` 里第一个在线的机器人和 Koishi 统一的 `bot.sendMessage()`，这是 Koishi 自身的跨适配器发送 API，不需要按平台分支。

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
- 通知推送要求至少有一个 `ctx.bots` 中状态为 online 的机器人；多机器人实例时总是用第一个在线的，不区分平台（因为通知的 `Target.chatId` 本身就已经隐含了目标会话所在的平台连接，发送到错的 bot 实例会由适配器自身报错，这里暂不做机器人到会话的归属校验）。

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
- Notification delivery picks the first online bot in `ctx.bots` without checking which
  platform the target chat belongs to; a mismatch surfaces as the adapter's own send error.
