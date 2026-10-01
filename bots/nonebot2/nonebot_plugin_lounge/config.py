"""Plugin config, read from the bot's .env (NoneBot2's Driver config), prefixed `lounge_`.

See https://nonebot.dev (Driver.config / get_plugin_config), read 2026-09-29.
"""
from __future__ import annotations

from typing import List, Optional

from pydantic import BaseModel, Field


class Config(BaseModel):
    """lounge_base_url / lounge_bot_token / ... in the bot's .env."""

    lounge_base_url: str = Field(..., description="Lounge site base URL, e.g. https://lounge.example.com")
    lounge_bot_token: str = Field(..., description="This bot's token (issued in the admin: 开放平台 -> 机器人)")
    lounge_command_prefix: str = Field(default="/", description="Prefix this plugin's own commands answer to")
    lounge_platform: Optional[str] = Field(
        default=None,
        description="Platform name sent to the Bot API; defaults to the adapter name lower-cased (onebot11 -> qq, feishu, telegram, ...)",
    )
    lounge_allowed_chats: List[str] = Field(
        default_factory=list,
        description="Chat ids allowed to use /bind (empty = every chat the bot is in)",
    )
    lounge_lang: Optional[str] = Field(default=None, description="Accept-Language sent to the Bot API (zh-CN, en, ja, zh-TW)")
