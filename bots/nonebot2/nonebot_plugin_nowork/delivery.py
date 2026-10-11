"""Adapter-specific notification sends. Failures propagate so no ACK is sent."""
from __future__ import annotations

import json
from typing import Any, Iterable, Mapping


def select_bot(bots: Iterable[Any], bot_id: str | None = None) -> Any:
    candidates = [bot for bot in bots if not bot_id or str(bot.self_id) == bot_id]
    if len(candidates) != 1:
        raise RuntimeError("nowork: select one connected notification bot using nowork_notification_bot_id")
    return candidates[0]


async def deliver(bot: Any, target: Mapping[str, str], text: str) -> None:
    kind = target.get("chatType", "private")
    private = kind == "private"
    destination = (target.get("userId") or target.get("chatId")) if private else target.get("chatId")
    if not destination:
        raise ValueError("nowork: notification destination is missing")
    adapter = bot.adapter.get_name().strip().lower()
    if adapter == "onebot v11":
        await bot.call_api("send_private_msg" if private else "send_group_msg",
                           **({"user_id": int(destination)} if private else {"group_id": int(destination)}), message=text)
    elif adapter == "onebot v12":
        await bot.call_api("send_message", detail_type=kind,
                           **{("user_id" if private else "channel_id" if kind == "channel" else "group_id"): destination},
                           message=[{"type": "text", "data": {"text": text}}])
    elif adapter == "telegram":
        await bot.call_api("send_message", chat_id=destination, text=text)
    elif adapter == "feishu":
        await bot.call_api("send_message", receive_id_type="open_id" if private else "chat_id",
                           receive_id=destination, msg_type="text", content=json.dumps({"text": text}, ensure_ascii=False))
    elif adapter == "qq":
        api, field = ("post_c2c_messages", "openid") if private else (
            ("post_messages", "channel_id") if kind == "channel" else ("post_group_messages", "group_openid"))
        kwargs = {field: destination, "content": text}
        if kind != "channel":
            kwargs["msg_type"] = 0
        await bot.call_api(api, **kwargs)
    else:
        raise RuntimeError("nowork: this adapter needs a notification delivery implementation")
