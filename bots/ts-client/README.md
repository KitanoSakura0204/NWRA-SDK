# @nowork/bot-client

休息室账号「机器人 API」(`/api/bot/v1`) 的最小 TypeScript 客户端，供 Koishi、Wechaty 插件共用，避免重复实现鉴权、信封解析和事件重连逻辑。不直接给用户使用；如果你在写自己的机器人框架插件，可以参照本包或直接实现协议（见 `SDK/docs/BOTS_SDK.md`）。

## 安装

本包未发布到 npm，仓库内以工作区依赖的方式被 `../koishi` 与 `../wechaty` 引用（见各自 package.json 的 `file:` 依赖）。

## 用法

```ts
import { BotClient, connectEvents, textMessage } from "@nowork/bot-client";

const client = new BotClient({ baseUrl: "https://nowork.example.com", token: "<bot token>" });

// 绑定
const bound = await client.bind("qq", "10001", "ABC123", "昵称");

// 查询
const info = await client.lookup("qq", "10001");

// 命令穿透（未命中内置命令时，交给站点的命令注册表）
const { handled, reply } = await client.command({
  platform: "qq",
  platformUserId: "10001",
  chatType: "group",
  chatId: "g1",
  text: "/me",
});

// 接收站点推送的通知（WebSocket，自动重连 + 应答 ack）
const stop = connectEvents({
  baseUrl: "https://nowork.example.com",
  token: "<bot token>",
  onEvent: (ev) => {
    console.log("notify", ev.target, ev.message);
  },
});
// stop() 关闭连接
```

没有 WebSocket 的环境可改用轮询：`client.pollEvents()` 取回当前到期的通知，处理后调用 `client.ackEvents([id, ...])`。

## 协议来源

字段名和返回结构来自后端已落地代码 `Backend/internal/modules/bots/{api.go,ws.go,hub.go,platform.go}`（commit df7b675 / d52a936），阅读日期 2026-09-29；目前没有对外发布的协议文档。完整字段表见 `SDK/docs/BOTS_SDK.md`。

## 测试

```sh
npm install
npm test
```

测试使用内置的 `fetch` stub 和一个假的 `WebSocket` 实现，不发任何真实网络请求。

---

# @nowork/bot-client (English)

Minimal TypeScript client for the NoWork Bot API (`/api/bot/v1`), shared by the Koishi and
Wechaty plugins in this directory so they don't duplicate auth, envelope parsing and the
WebSocket reconnect logic. Not meant for end users directly; if you're writing a plugin for
another framework, use this package or implement the protocol yourself (see
`SDK/docs/BOTS_SDK.md`).

## Install

Not published to npm; `../koishi` and `../wechaty` depend on it via a `file:` path in their
own `package.json`.

## Usage

See the Chinese section above for a full example; the API is `BotClient` (bind / unbind /
lookup / command / commands / pollEvents / ackEvents) and `connectEvents` for the WebSocket
notification stream with automatic reconnect and per-message ack.

## Where the protocol comes from

Field names and shapes were read from the landed backend code
(`Backend/internal/modules/bots/{api.go,ws.go,hub.go,platform.go}`, commit df7b675 /
d52a936), read on 2026-09-29. No public protocol document exists yet; the full field table
is in `SDK/docs/BOTS_SDK.md`.

## Tests

```sh
npm install
npm test
```

Uses an injected `fetch` stub and a fake `WebSocket`; no real network calls.
