"""The actual bind/unbind/me/passthrough behaviour, kept free of NoneBot's Matcher so it is
testable with a fake BotClient and no running NoneBot app."""
from __future__ import annotations

from typing import List, Optional

from lounge_bot_client import BotApiError, BotClient

from .convert import lounge_message_to_text


async def do_bind(client: BotClient, platform: str, user_id: str, name: str, code: str) -> str:
    if not code:
        return "用法: /bind <绑定码>（在「个人中心 -> 机器人」页面获取）"
    try:
        result = await client.bind(platform, user_id, code, name)
    except BotApiError as exc:
        return f"绑定失败：{exc.message}"
    return f"绑定成功，欢迎 {result.nickname}（等级 {result.level}）"


async def do_unbind(client: BotClient, platform: str, user_id: str) -> str:
    try:
        result = await client.unbind(platform, user_id)
    except BotApiError as exc:
        return f"解绑失败：{exc.message}"
    return "已解绑" if result.unbound else "未找到绑定记录"


async def do_me(client: BotClient, platform: str, user_id: str) -> str:
    try:
        info = await client.lookup(platform, user_id)
    except BotApiError as exc:
        return f"查询失败：{exc.message}"
    if not info.bound:
        return "尚未绑定，请使用 /bind <绑定码>"
    lines = [f"昵称：{info.nickname}", f"等级：{info.level_name or info.level}"]
    if info.realname_verified is not None:
        lines.append("实名认证：已通过" if info.realname_verified else "实名认证：未通过")
    return "\n".join(lines)


async def do_passthrough(
    client: BotClient, platform: str, user_id: str, name: str, chat_type: str, chat_id: str, text: str
) -> Optional[str]:
    """Returns the reply text, or None when the server did not handle the text (another
    plugin, or ordinary chat, should take it)."""
    try:
        result = await client.command(
            platform=platform, platform_user_id=user_id, platform_name=name, chat_type=chat_type, chat_id=chat_id, text=text
        )
    except BotApiError:
        return None
    if not result.handled:
        return None
    return lounge_message_to_text(result.reply) or None


def chat_allowed(allowed_chats: List[str], chat_id: str) -> bool:
    return not allowed_chats or chat_id in allowed_chats
