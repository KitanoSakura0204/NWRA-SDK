"""Player data and per-server bans (module "server-users", SDK/docs/SERVER_USERS.md)."""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from .errors import ApiError

# Business codes of the server-users routes (ApiError.code); other refusals use code 1.
CODE_NOT_ON_ROSTER = 404
CODE_VERSION_CONFLICT = 409  # the data changed since the version you read: read again, then write
CODE_ADMIN_BAN = 403  # a ban an administrator placed: only an administrator can change it


def is_version_conflict(e: BaseException) -> bool:
    """Whether a data write was refused because of a stale version."""
    return isinstance(e, ApiError) and e.code == CODE_VERSION_CONFLICT


@dataclass
class UserField:
    """One entry of the data schema the server owner defined. type: text | number | bool | select | json."""

    key: str
    label: str = ""
    type: str = "text"
    description: str = ""
    visible: bool = False  # shown read-only to the player in the user centre
    default: Any = None
    max_length: int = 0  # characters (text) or bytes (json)
    min: Optional[float] = None
    max: Optional[float] = None
    integer: bool = False
    options: List[Dict[str, str]] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "UserField":
        return cls(
            str(d.get("key", "")),
            str(d.get("label", "") or ""),
            str(d.get("type", "text") or "text"),
            str(d.get("description", "") or ""),
            bool(d.get("visible")),
            d.get("default"),
            int(d.get("maxLength", 0) or 0),
            d.get("min"),
            d.get("max"),
            bool(d.get("integer")),
            [dict(o) for o in (d.get("options") or []) if isinstance(o, dict)],
        )


@dataclass
class UserSchema:
    fields: List[UserField] = field(default_factory=list)
    max_user_bytes: int = 0  # cap on one account's stored values (JSON bytes)

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "UserSchema":
        return cls([UserField.from_dict(f) for f in (d.get("fields") or []) if isinstance(f, dict)], int(d.get("maxUserBytes", 0) or 0))


@dataclass
class UserData:
    """One account's data: every schema field with its stored value, else the default, else None.
    version is the optimistic lock (0 = nothing stored yet)."""

    uid: int = 0
    data: Dict[str, Any] = field(default_factory=dict)
    version: int = 0
    updated_at: Optional[str] = None  # "2006-01-02 15:04:05" server time; None before the first write
    updated_by: str = ""
    size: int = 0

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "UserData":
        return cls(int(d.get("uid", 0) or 0), dict(d.get("data") or {}), int(d.get("version", 0) or 0),
                   d.get("updatedAt"), str(d.get("updatedBy", "") or ""), int(d.get("size", 0) or 0))


@dataclass
class ServerBan:
    """An active ban on this server with the account's bindings there. expire_time None = permanent."""

    uid: int = 0
    names: List[str] = field(default_factory=list)
    uuids: List[str] = field(default_factory=list)
    reason: str = ""
    expire_time: Optional[str] = None
    create_time: str = ""
    source: str = ""  # admin | owner | plugin

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "ServerBan":
        return cls(int(d.get("uid", 0) or 0), [str(n) for n in d.get("names") or []], [str(u) for u in d.get("uuids") or []],
                   str(d.get("reason", "") or ""), d.get("expireTime"), str(d.get("createTime", "") or ""), str(d.get("source", "") or ""))


@dataclass
class KickEntry:
    """An online player (from the last heartbeat) whose account is banned here."""

    name: str = ""
    uuid: str = ""
    uid: int = 0
    reason: str = ""
    expire_time: Optional[str] = None

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "KickEntry":
        return cls(str(d.get("name", "") or ""), str(d.get("uuid", "") or ""), int(d.get("uid", 0) or 0),
                   str(d.get("reason", "") or ""), d.get("expireTime"))


def target(uid: int = 0, name: str = "", uuid: str = "") -> Dict[str, Any]:
    """The account of a ban call: uid, or the name / uuid of one of its bindings on this server."""
    out: Dict[str, Any] = {}
    if uid:
        out["uid"] = int(uid)
    if name:
        out["name"] = name
    if uuid:
        out["uuid"] = uuid
    return out
