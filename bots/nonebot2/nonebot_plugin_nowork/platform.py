"""Maps a NoneBot2 adapter to the platform name the NoWork Bot API expects.

The gateway's codecs (Backend/internal/modules/bots/platform/**) register kinds such as
"onebot11", "feishu", "telegram", "discord", "wecom", "dingtalk", "wechat_mp", "qq_bot",
"generic". Config `nowork_platform` overrides this when a bot bridges a kind the gateway
does not know the adapter name for.
"""
from __future__ import annotations

_ADAPTER_TO_PLATFORM = {
    "onebot v11": "onebot11",
    "onebot v12": "onebot12",
    "telegram": "telegram",
    "feishu": "feishu",
    "discord": "discord",
    "dodo": "dodo",
    "kaiheila": "kaiheila",
    "ding": "dingtalk",
    "qq": "qq_bot",
    "wxmp": "wechat_mp",
    "satori": "satori",
}


def platform_of(adapter_name: str, override: str | None) -> str:
    if override:
        return override
    return _ADAPTER_TO_PLATFORM.get(adapter_name.strip().lower(), "generic")
