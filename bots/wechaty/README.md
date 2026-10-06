# wechaty-plugin-nowork

休息室账号（NoWork Account）的 Wechaty 插件：通过个人微信实现 `/bind` / `/unbind` / `/me`，并把其余聊天内容转发给站点的指令注册表（`/api/bot/v1/command`），同时通过 WebSocket 接收站点推送的通知并发送到对应的群聊或私聊。

与任意 Wechaty puppet 兼容（padlocal、wechat4u 等），因为只使用 `Message` / `Contact` / `Room` 的通用方法。Wechaty 本身没有指令路由器（不像 NoneBot2 / Koishi），本插件自带一个极简的前缀解析（见 `src/logic.ts` 的 `parseCommand`）。

## 安装

本仓库内尚未发布到 npm。开发期在你的 Wechaty 项目里以本地路径依赖安装：

```sh
npm install /path/to/D:/Project/Admin/SDK/bots/wechaty
```

```ts
import { WechatyBuilder } from "wechaty";
import { NoWorkPlugin } from "wechaty-plugin-nowork";

const bot = WechatyBuilder.build({ name: "my-bot" });
bot.use(
  NoWorkPlugin({
    baseUrl: "https://nowork.example.com",
    botToken: "<机器人令牌>",
  }),
);
await bot.start();
```

## 配置

- `baseUrl`（必填）
- `botToken`（必填）
- `platform`（可选，默认 `"wechaty"`）：发给站点的 `platform` 字段。
- `prefixes`（可选，默认 `["/"]`）：本插件自带指令识别的前缀列表。
- `allowedChats`（可选）：允许使用 `/bind` 的会话 id（群聊 room id 或联系人 contact id）白名单。
- `lang`（可选）：发给站点的 `Accept-Language`。

## 指令

- `/bind <绑定码>`
- `/unbind`
- `/me`
- 其余消息转发给站点的指令注册表；未命中时静默忽略。

## 测试

```sh
npm install
npm test
```

测试针对 `logic.ts`（纯函数，用一个假的 `BotClient` 驱动）和 `convert.ts`，不依赖真实的 Wechaty 实例或 puppet，不发任何网络请求。

## 已知限制 / 待验证

- 通知推送用 `bot.Room.load(id)` / `bot.Contact.load(id)` 按 `Target.chatId` 直接加载实体后 `sync()` 再 `say()`；`Contact.load` 的用法已在官方文档的示例中确认，`Room.load` 按 Wechaty 的一致约定推断但未在文档里直接找到签名，标注为 `unverified`，实测时请优先验证群聊通知。
- 没有使用 Wechaty 的 `wechaty-plugin-contrib` 规范（该规范建议单文件 ≤100 行、不依赖重量级包）；本插件依赖 `@nowork/bot-client`，如需严格遵循该规范需要内联客户端代码。

---

# wechaty-plugin-nowork (English)

A Wechaty plugin for NoWork Account: `/bind` / `/unbind` / `/me` through personal WeChat,
plus a bridge that forwards other chat text to the NoWork Bot API's command registry. Also
opens a WebSocket to the site's notification stream and delivers pushed notifications to the
right room or contact.

Works with any Wechaty puppet since it only uses the generic `Message` / `Contact` / `Room`
surface. Wechaty itself has no command router (unlike NoneBot2 / Koishi), so this plugin
ships a minimal prefix parser (`parseCommand` in `src/logic.ts`).

## Install

Not published to npm; see the Chinese section above for a full usage example with
`bot.use(NoWorkPlugin({ ... }))`.

## Config

`baseUrl`, `botToken` (required); `platform` (default `"wechaty"`), `prefixes` (default
`["/"]`), `allowedChats`, `lang` (optional).

## Tests

```sh
npm install
npm test
```

Tests exercise `logic.ts` against a fake `BotClient` and `convert.ts` directly; no real
Wechaty instance or puppet, no network calls.

## Known limitations / to verify

- Notification delivery uses `bot.Room.load(id)` / `bot.Contact.load(id)` on
  `Target.chatId`, then `sync()` and `say()`. `Contact.load` matches the official docs'
  example; `Room.load` is inferred from Wechaty's own naming convention and marked
  `unverified` in the code - verify group notifications first against a real platform.
- Does not follow the `wechaty-plugin-contrib` convention (single file, <=100 SLOC, no
  heavy dependencies); this plugin depends on `@nowork/bot-client`, so adopting that
  convention would mean inlining the client.
