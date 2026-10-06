"""nonebot-plugin-nowork: binds a chat account to a NoWork Account user, and lets any chat
reach the NoWork Bot API's command registry. Adapter-agnostic: uses nonebot.adapters.Event's
generic surface, so it works on OneBot v11 / v12, Feishu, Telegram and any adapter that
reaches NoneBot's generic Message handling.

Protocol: see SDK/docs/BOTS_SDK.md and nowork_bot_client (read from the landed backend,
Backend/internal/modules/bots, commit df7b675 / d52a936, 2026-09-29).
"""
from __future__ import annotations

import asyncio
import contextlib
from typing import Optional

from nonebot import get_driver, get_plugin_config, logger, on_command, on_message
from nonebot.adapters import Bot, Event
from nonebot.plugin import PluginMetadata
from nonebot.rule import to_me

from nowork_bot_client import BotClient, PluginEvent, connect_events

from . import logic
from .config import Config
from .convert import chat_id_of, chat_type_of, nowork_message_to_text, sender_name_of
from .platform import platform_of

__plugin_meta__ = PluginMetadata(
    name="休息室账号",
    description="NoWork Account 绑定 / 解绑 / 查询，并把聊天指令转发给站点的指令注册表",
    usage="/bind <code>  /unbind  /me  其余未命中内置指令的消息转发给站点处理",
    type="application",
    homepage="https://github.com/wangework/nowork",
    config=Config,
    supported_adapters=None,  # generic: no adapter-specific import, works on every adapter
)

plugin_config: Config = get_plugin_config(Config)
driver = get_driver()

_client: Optional[BotClient] = None
_events_stop: Optional[asyncio.Event] = None
_events_task: Optional[asyncio.Task] = None


def client() -> BotClient:
    global _client
    if _client is None:
        _client = BotClient(plugin_config.nowork_base_url, plugin_config.nowork_bot_token, lang=plugin_config.nowork_lang)
    return _client


def _platform(bot: Bot) -> str:
    return platform_of(bot.adapter.get_name(), plugin_config.nowork_platform)


def _chat_allowed(chat_id: str) -> bool:
    return logic.chat_allowed(plugin_config.nowork_allowed_chats, chat_id)


bind_cmd = on_command("bind", aliases={"绑定"}, priority=5, block=True)
unbind_cmd = on_command("unbind", aliases={"解绑"}, priority=5, block=True)
me_cmd = on_command("me", aliases={"我的"}, priority=5, block=True)
# Lowest priority, not blocking: anything another plugin didn't already consume goes here.
passthrough = on_message(rule=to_me(), priority=900, block=False)


@bind_cmd.handle()
async def _bind(bot: Bot, event: Event) -> None:
    arg = event.get_plaintext().split(maxsplit=1)
    code = arg[1].strip() if len(arg) > 1 else ""
    chat_id = chat_id_of(event)
    if code and not _chat_allowed(chat_id):
        await bind_cmd.finish("此会话不允许使用绑定指令")
    reply = await logic.do_bind(client(), _platform(bot), event.get_user_id(), sender_name_of(event), code)
    await bind_cmd.finish(reply)


@unbind_cmd.handle()
async def _unbind(bot: Bot, event: Event) -> None:
    reply = await logic.do_unbind(client(), _platform(bot), event.get_user_id())
    await unbind_cmd.finish(reply)


@me_cmd.handle()
async def _me(bot: Bot, event: Event) -> None:
    reply = await logic.do_me(client(), _platform(bot), event.get_user_id())
    await me_cmd.finish(reply)


@passthrough.handle()
async def _passthrough(bot: Bot, event: Event) -> None:
    text = event.get_plaintext().strip()
    if not text:
        return
    chat_id = chat_id_of(event)
    if not _chat_allowed(chat_id):
        return
    reply = await logic.do_passthrough(
        client(), _platform(bot), event.get_user_id(), sender_name_of(event), chat_type_of(event), chat_id, text
    )
    if reply:
        await passthrough.finish(reply)


@driver.on_startup
async def _start_events() -> None:
    global _events_stop, _events_task
    _events_stop = asyncio.Event()

    async def on_event(ev: PluginEvent) -> None:
        text = nowork_message_to_text(ev.message)
        if not text:
            return
        for bot in get_driver().bots.values():
            try:
                target_user = ev.target.get("userId")
                target_chat = ev.target.get("chatId") or target_user
                if target_chat is None:
                    continue
                await bot.send_msg(user_id=target_user, group_id=ev.target.get("chatId"), message=text)  # type: ignore[call-arg]
                return
            except Exception as exc:  # noqa: BLE001 - adapters raise different types; try the next bot
                logger.debug(f"nowork: notify send attempt failed on {bot.self_id}: {exc}")
        logger.warning("nowork: no connected bot could deliver a notification")

    async def run() -> None:
        await connect_events(plugin_config.nowork_base_url, plugin_config.nowork_bot_token, on_event, stop=_events_stop)

    _events_task = asyncio.create_task(run())


@driver.on_shutdown
async def _stop_events() -> None:
    if _events_stop is not None:
        _events_stop.set()
    if _events_task is not None:
        _events_task.cancel()
        with contextlib.suppress(asyncio.CancelledError):
            await _events_task
    if _client is not None:
        await _client.aclose()
