"""Plugin config, read from the bot's .env (NoneBot2's Driver config), prefixed `nowork_`.

See https://nonebot.dev (Driver.config / get_plugin_config), read 2026-09-29.
"""
from __future__ import annotations

from typing import List, Optional

from pydantic import BaseModel, Field


class Config(BaseModel):
    """nowork_base_url / nowork_bot_token / ... in the bot's .env."""

    nowork_base_url: str = Field(..., description="NoWork site base URL, e.g. https://nowork.example.com")
    nowork_bot_token: str = Field(..., description="This bot's token (issued in the admin: 开放平台 -> 机器人)")
    nowork_command_prefix: str = Field(default="/", min_length=1, description="Prefix this plugin's own commands answer to")
    nowork_platform: Optional[str] = Field(
        default=None,
        description="Platform name sent to the Bot API; defaults to the adapter name lower-cased (onebot11 -> qq, feishu, telegram, ...)",
    )
    nowork_allowed_chats: List[str] = Field(
        default_factory=list,
        description="Chat ids allowed to use /bind (empty = every chat the bot is in)",
    )
    nowork_lang: Optional[str] = Field(default=None, description="Accept-Language sent to the Bot API (zh-CN, en, ja, zh-TW)")
    nowork_notification_bot_id: Optional[str] = Field(default=None, description="Self ID of the bot to deliver notifications; required when multiple bots connect")
