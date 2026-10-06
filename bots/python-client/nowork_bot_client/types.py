"""Wire types for the Lounge Bot API. Mirrors
Backend/internal/modules/bots/platform.go (Segment, Message, Target) and the handlers in
api.go / ws.go / hub.go.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, TypedDict


class Segment(TypedDict, total=False):
    """One piece of a message (text / mention / mention_all / image / link / reply / markdown)."""

    type: str
    data: Dict[str, str]


Message = List[Segment]


def text_message(text: str) -> Message:
    return [{"type": "text", "data": {"text": text}}]


class Target(TypedDict, total=False):
    chatType: str
    chatId: str
    userId: str
    reply: Dict[str, str]


@dataclass
class BindResult:
    user_id: int
    uid: int
    nickname: str
    level: int

    @classmethod
    def from_data(cls, data: Dict[str, Any]) -> "BindResult":
        return cls(user_id=data.get("userId", 0), uid=data.get("uid", 0), nickname=data.get("nickname", ""), level=data.get("level", 0))


@dataclass
class UnbindResult:
    unbound: bool

    @classmethod
    def from_data(cls, data: Dict[str, Any]) -> "UnbindResult":
        return cls(unbound=bool(data.get("unbound", False)))


@dataclass
class LookupResult:
    bound: bool
    uid: Optional[int] = None
    nickname: Optional[str] = None
    level: Optional[int] = None
    level_name: Optional[str] = None
    realname_verified: Optional[bool] = None

    @classmethod
    def from_data(cls, data: Dict[str, Any]) -> "LookupResult":
        return cls(
            bound=bool(data.get("bound", False)),
            uid=data.get("uid"),
            nickname=data.get("nickname"),
            level=data.get("level"),
            level_name=data.get("levelName"),
            realname_verified=data.get("realnameVerified"),
        )


@dataclass
class CommandResult:
    handled: bool
    reply: Message = field(default_factory=list)

    @classmethod
    def from_data(cls, data: Dict[str, Any]) -> "CommandResult":
        return cls(handled=bool(data.get("handled", False)), reply=data.get("reply") or [])


@dataclass
class CommandInfo:
    name: str
    aliases: List[str]
    help: str
    need_bind: bool
    min_level: int


@dataclass
class CommandsResult:
    prefixes: List[str]
    commands: List[CommandInfo]

    @classmethod
    def from_data(cls, data: Dict[str, Any]) -> "CommandsResult":
        cmds = [
            CommandInfo(
                name=c.get("name", ""),
                aliases=c.get("aliases") or [],
                help=c.get("help", ""),
                need_bind=bool(c.get("needBind", False)),
                min_level=c.get("minLevel", 0),
            )
            for c in data.get("commands") or []
        ]
        return cls(prefixes=data.get("prefixes") or [], commands=cmds)


@dataclass
class PluginEvent:
    """One message offered to the plugin, over the WebSocket or the long-poll fallback."""

    id: int
    bot_id: int
    target: Target
    message: Message
    time: int
    kind: Optional[str] = None

    @classmethod
    def from_data(cls, data: Dict[str, Any]) -> "PluginEvent":
        return cls(
            id=data.get("id", 0),
            bot_id=data.get("botId", 0),
            kind=data.get("kind"),
            target=data.get("target") or {},
            message=data.get("message") or [],
            time=data.get("time", 0),
        )
