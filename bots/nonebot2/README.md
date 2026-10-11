# nonebot-plugin-nowork

休息室账号（NoWork Account）的 NoneBot2 插件：绑定 / 解绑 / 查询个人信息，并把其余未命中内置指令的聊天内容转发给站点的指令注册表（`/api/bot/v1/command`），同时通过 WebSocket 接收站点推送的通知（如安全提醒、工单回复）并转发到已绑定的聊天。

适配器无关：只使用 NoneBot2 `Event` 的通用方法（`get_user_id` / `get_plaintext` / `get_session_id`），理论上适用于 OneBot v11 / v12、Feishu、Telegram 等任意 NoneBot2 适配器；已验证见下方「实测清单」。

## 安装

本仓库内尚未发布到 PyPI。开发期使用你的 NoneBot2 项目虚拟环境，从 `SDK/bots/nonebot2` 目录同时安装本插件和相邻的共享客户端：

```sh
python -m pip install -e "../python-client" -e "."
```

然后在 `pyproject.toml` 或 `bot.py` 里加载 `nonebot_plugin_nowork`。

## 配置（.env）

```ini
NOWORK_BASE_URL=https://nowork.example.com
NOWORK_BOT_TOKEN=<机器人令牌，在管理后台「开放平台 -> 机器人」创建并签发>
NOWORK_COMMAND_PREFIX=/
NOWORK_PLATFORM=            # 留空按适配器自动判断；桥接型机器人可强制指定，如 feishu
NOWORK_ALLOWED_CHATS=       # 逗号分隔的会话 id 白名单；留空表示不限制
NOWORK_LANG=                # 发给站点的 Accept-Language（zh-CN / en / ja / zh-TW）
```

## 指令

- `/bind <绑定码>`（别名 `/绑定`）：绑定码在站点「个人中心 -> 机器人」页面获取，10 分钟内有效。
- `/unbind`（别名 `/解绑`）
- `/me`（别名 `/我的`）：查看绑定状态、等级、（如用户开启展示）实名认证状态。
- 其余 @机器人 的消息会转发给站点的指令注册表（`help` / `level` / `servers` / `ticket` 等，由站点侧注册，详见 `SDK/docs/BOTS_SDK.md`）；未命中时静默忽略，交给其他插件处理。

## 通知推送

启动时建立到 `/api/bot/v1/events` 的 WebSocket 连接（断线自动重连，退避重试），按 `Target.chatType` / `userId` / `chatId` 调用对应适配器发送 API；多机器人用 `NOWORK_NOTIFICATION_BOT_ID` 选择连接。只有发送成功才确认，失败保留待重投。

## 测试

```sh
python -m pip install -e "../python-client" -e ".[test]"
python -m pytest
```

测试针对 `logic.py`（纯函数，用一个假的 `BotClient` 驱动）和 `convert.py` / `platform.py`（消息渲染、适配器名到平台名的映射），不依赖真实的 NoneBot driver，不发任何网络请求。

## 已知限制

- 通知已按 OneBot v11/v12、Telegram、Feishu、QQ 分别调用发送 API。多个机器人连接时设置 `NOWORK_NOTIFICATION_BOT_ID` 指定 self ID；未指定且存在歧义时保持未确认，避免误发到其他平台。其他适配器需在 `delivery.py` 增加发送分支；QQ 主动消息取决于平台权限和配额。
- `NOWORK_COMMAND_PREFIX` 控制本插件 bind/unbind/me 指令前缀，不修改 NoneBot 全局配置。发送失败不会确认通知，等待后端重投。
- 群聊中默认仅在 @机器人 时触发指令转发（`passthrough` 用了 `to_me()` 规则），避免误吞群内其他对话；如需对所有消息生效可在站点侧调整或改规则。

---

# nonebot-plugin-nowork (English)

A NoneBot2 plugin for NoWork Account: bind / unbind / check your profile, and bridge any
chat text that isn't a built-in command to the site's command registry
(`/api/bot/v1/command`). It also opens a WebSocket to the site's notification stream and
forwards pushed notifications (security alerts, ticket replies, ...) to the bound chat.

Adapter-agnostic: only uses `Event`'s generic surface (`get_user_id`, `get_plaintext`,
`get_session_id`), so it should work on any NoneBot2 adapter (OneBot v11/v12, Feishu,
Telegram, ...); see "实测清单 / things to verify" below for what still needs testing against
a real platform.

## Install

Not published to PyPI yet. In your NoneBot2 project's virtual environment, run from
`SDK/bots/nonebot2` to install this plugin and its sibling shared client together:

```sh
python -m pip install -e "../python-client" -e "."
```

Then load `nonebot_plugin_nowork` from your `pyproject.toml` or `bot.py`.

## Config (.env)

See the Chinese section above for the full list of `NOWORK_*` keys.

## Commands

`/bind <code>`, `/unbind`, `/me`, and anything else said to the bot is passed through to the
site's command registry; unhandled text is silently ignored so other plugins can take it.

## Notifications

On startup it connects to `/api/bot/v1/events` with reconnect backoff and acknowledges only successful deliveries. OneBot v11/v12, Telegram, Feishu and QQ have explicit send branches. Set `NOWORK_NOTIFICATION_BOT_ID` when several bots connect; unsupported adapters or failed sends remain unacknowledged for retry. Real provider permissions and quotas still apply.

## Tests

```sh
python -m pip install -e "../python-client" -e ".[test]"
python -m pytest
```

Tests exercise `logic.py` against a fake `BotClient` and `convert.py` / `platform.py`
directly; no real NoneBot driver, no network calls.
