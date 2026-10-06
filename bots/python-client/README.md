# nowork-bot-client

休息室账号「机器人 API」(`/api/bot/v1`) 的最小 Python 客户端，供 `nonebot-plugin-nowork` 使用，避免重复实现鉴权、信封解析和事件重连逻辑。

## 安装

本包未发布到 PyPI；`../nonebot2` 以本地路径依赖的方式引用它（见其 `pyproject.toml`）。

## 用法

```python
from nowork_bot_client import BotClient, connect_events

client = BotClient("https://nowork.example.com", "<bot token>")

# 绑定
bound = await client.bind("qq", "10001", "ABC123", "昵称")

# 查询
info = await client.lookup("qq", "10001")

# 命令穿透
result = await client.command("qq", "10001", "group", "g1", "/me")

# 接收站点推送的通知（WebSocket，自动重连 + 应答 ack）
async def on_event(ev):
    print("notify", ev.target, ev.message)

await connect_events("https://nowork.example.com", "<bot token>", on_event)
```

没有 WebSocket 的环境可改用轮询：`await client.poll_events()` 取回当前到期的通知，处理后调用 `await client.ack_events([id, ...])`。

## 协议来源

字段名和返回结构来自后端已落地代码 `Backend/internal/modules/bots/{api.go,ws.go,hub.go,platform.go}`（commit df7b675 / d52a936），阅读日期 2026-09-29；完整字段表见 `SDK/docs/BOTS_SDK.md`。

## 测试

```sh
pip install -e ".[test]"
pytest
```

测试使用 `httpx.MockTransport` 和一个假的 WebSocket 连接对象，不发任何真实网络请求。

---

# nowork-bot-client (English)

Minimal Python client for the NoWork Bot API (`/api/bot/v1`), used by
`nonebot-plugin-nowork` to avoid duplicating auth, envelope parsing and the WebSocket
reconnect logic.

## Install

Not published to PyPI; `../nonebot2` depends on it via a local path (see its
`pyproject.toml`).

## Usage

See the Chinese section above for a full example; the API is `BotClient` (bind / unbind /
lookup / command / commands / poll_events / ack_events) and `connect_events` for the
WebSocket notification stream with automatic reconnect and per-message ack.

## Where the protocol comes from

Field names and shapes were read from the landed backend code
(`Backend/internal/modules/bots/{api.go,ws.go,hub.go,platform.go}`, commit df7b675 /
d52a936), read on 2026-09-29. The full field table is in `SDK/docs/BOTS_SDK.md`.

## Tests

```sh
pip install -e ".[test]"
pytest
```

Uses `httpx.MockTransport` and a fake WebSocket connection object; no real network calls.
