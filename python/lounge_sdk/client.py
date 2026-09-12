from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Optional

from .errors import ApiError, TransportError


@dataclass
class WhitelistEntry:
    name: str = ""
    uuid: str = ""
    uid: int = 0
    nickname: str = ""
    security_level: int = 0
    since: str = ""

    @classmethod
    def from_dict(cls, d: dict) -> "WhitelistEntry":
        return cls(
            d.get("name", ""),
            d.get("uuid", "") or "",
            int(d.get("uid", 0)),
            d.get("nickname", ""),
            int(d.get("securityLevel", 0)),
            d.get("since", ""),
        )


@dataclass
class Whitelist:
    server: str = ""
    mode: int = 0
    count: int = 0
    entries: list = field(default_factory=list)


@dataclass
class CheckResult:
    allowed: bool = False
    bound: bool = False
    reason: str = ""
    player: Optional[WhitelistEntry] = None


class LoungeClient:
    """Server API client authenticated with an X-Server-Key."""

    def __init__(self, base: str, server_key: str, timeout: float = 10.0):
        self.base = base.rstrip("/")
        self.server_key = server_key
        self.timeout = timeout

    def _call(self, method: str, path: str, query: Optional[dict] = None, body: Any = None) -> dict:
        url = self.base + path
        if query:
            url += "?" + urllib.parse.urlencode({k: v for k, v in query.items() if v})
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Accept", "application/json")
        req.add_header("X-Server-Key", self.server_key)
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw = res.read()
        except urllib.error.HTTPError as e:
            raw = e.read()
        except (urllib.error.URLError, OSError) as e:
            raise TransportError(str(e)) from e
        try:
            env = json.loads(raw.decode("utf-8"))
        except ValueError as e:
            raise TransportError(f"invalid json: {e}") from e
        if env.get("code", 1) != 0:
            raise ApiError(int(env.get("code", 1)), str(env.get("message", "")))
        return env.get("data") or {}

    def whitelist(self) -> Whitelist:
        d = self._call("GET", "/server/v1/whitelist")
        return Whitelist(
            d.get("server", ""),
            int(d.get("mode", 0)),
            int(d.get("count", 0)),
            [WhitelistEntry.from_dict(e) for e in d.get("entries", [])],
        )

    def check_whitelist(self, name: str = "", uuid: str = "") -> CheckResult:
        d = self._call("GET", "/server/v1/whitelist/check", {"name": name, "uuid": uuid})
        p = d.get("player")
        return CheckResult(
            bool(d.get("allowed")),
            bool(d.get("bound")),
            d.get("reason", ""),
            WhitelistEntry.from_dict(p) if p else None,
        )

    def heartbeat(self, online: int, max_players: int = 0, version: str = "") -> None:
        self._call("POST", "/server/v1/heartbeat", body={"online": online, "max": max_players, "version": version})
