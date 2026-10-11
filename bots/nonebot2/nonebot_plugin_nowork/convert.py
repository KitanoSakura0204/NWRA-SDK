"""Converts between a NoneBot Event and the NoWork Bot API's generic shapes.

Deliberately adapter-agnostic: every NoneBot adapter's Event exposes get_user_id(),
get_session_id() and is_tome() the same way (nonebot.adapters.Event base class), which is
enough to fill platform_user_id / chat_id / chat_type without importing any specific
adapter. Group vs private is inferred from the session id shape when the adapter doesn't
say more (OneBot v11 events carry message_type directly, used when present).
"""
from __future__ import annotations

from typing import Optional

from nonebot.adapters import Event

from nowork_bot_client import Message as NoworkMessage
from nowork_bot_client import Segment


def chat_type_of(event: Event) -> str:
    message_type = getattr(event, "message_type", None)
    if message_type == "group":
        return "group"
    if message_type == "private":
        return "private"
    if message_type:
        return str(message_type)
    # Fallback: a session id with more than one part after the user id is a group/channel.
    session_id = event.get_session_id()
    return "group" if session_id.count("_") >= 2 else "private"


def chat_id_of(event: Event) -> str:
    group_id = getattr(event, "group_id", None)
    if group_id is not None:
        return str(group_id)
    channel_id = getattr(event, "channel_id", None)
    if channel_id is not None:
        return str(channel_id)
    return event.get_user_id()


def sender_name_of(event: Event) -> str:
    sender = getattr(event, "sender", None)
    for attr in ("card", "nickname", "name"):
        val = getattr(sender, attr, None) if sender is not None else None
        if val:
            return str(val)
    return ""


def to_nowork_message(text: str) -> NoworkMessage:
    """A plain-text message; segments beyond text are not needed for replies the plugin
    itself sends (bind / unbind / me confirmations)."""
    return [{"type": "text", "data": {"text": text}}]


def nowork_message_to_text(message: Optional[NoworkMessage]) -> str:
    """Renders a NoWork Message (from /command or a pushed notification) down to plain
    text for adapters this plugin sends through generically. Non-text segments degrade to
    a bracketed placeholder rather than being dropped silently."""
    if not message:
        return ""
    parts = []
    for seg in message:
        seg_type = seg.get("type", "text")
        data = seg.get("data") or {}
        if seg_type in ("text", "markdown"):
            parts.append(data.get("text", ""))
        elif seg_type == "mention":
            parts.append(f"@{data.get('user_id', data.get('userId', data.get('name', '')))}")
        elif seg_type == "mention_all":
            parts.append("@all")
        elif seg_type == "link":
            label = data.get("title") or data.get("label") or data.get("url", "")
            parts.append(f"{label} ({data.get('url', '')})" if label != data.get("url") else data.get("url", ""))
        elif seg_type == "image":
            parts.append(f"[image: {data.get('url', '')}]")
        else:
            parts.append(f"[{seg_type}]")
    return "".join(parts)
