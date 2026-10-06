"""Minimal client for the NoWork Bot API (/api/bot/v1).

Contract read from the landed backend (Backend/internal/modules/bots: api.go, ws.go, hub.go,
platform.go; commit df7b675 / d52a936), read on 2026-09-29. No public spec exists yet; see
SDK/docs/BOTS_SDK.md for the full field table.
"""
from .client import BotClient
from .errors import BotApiError
from .types import (
    BindResult,
    CommandInfo,
    CommandResult,
    CommandsResult,
    LookupResult,
    Message,
    PluginEvent,
    Segment,
    Target,
    UnbindResult,
    text_message,
)
from .ws import connect_events

__all__ = [
    "BotClient",
    "BotApiError",
    "BindResult",
    "CommandInfo",
    "CommandResult",
    "CommandsResult",
    "LookupResult",
    "Message",
    "PluginEvent",
    "Segment",
    "Target",
    "UnbindResult",
    "text_message",
    "connect_events",
]
