from __future__ import annotations

import json
import math
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, Mapping, Optional

from .errors import ApiError, EncryptionError, TransportError
from .gm import GMPublishResult, _plain

# Marks a Server API reply whose data is the {"jwe": "..."} envelope (DELIVERY.md section 5).
HEADER_ENCRYPTION = "X-Lounge-Encryption"


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


@dataclass
class OnlinePlayer:
    """One entry of the heartbeat player list (name required, uuid optional)."""

    name: str
    uuid: str = ""


@dataclass
class RawResponse:
    """A Server API reply as received, data left undecoded (pass envelope to open_server_data)."""

    status: int
    headers: Dict[str, str]
    message: str
    data: Any
    envelope: Dict[str, Any]  # the whole parsed body {code, message, data}
    body: bytes
    encrypted: bool  # X-Lounge-Encryption is set or data is exactly {"jwe": "<string>"}


def _is_jwe_data(data: Any) -> bool:
    return isinstance(data, dict) and len(data) == 1 and isinstance(data.get("jwe"), str)


def _headers_of(res: Any) -> Dict[str, str]:
    items = getattr(getattr(res, "headers", None), "items", None)
    try:
        return {str(k).lower(): str(v) for k, v in items()} if callable(items) else {}
    except (TypeError, ValueError):  # a test double without real headers
        return {}


def _player(p: Any) -> Dict[str, str]:
    if isinstance(p, OnlinePlayer):
        name, uuid = p.name, p.uuid
    elif isinstance(p, Mapping):
        name, uuid = str(p.get("name", "") or ""), str(p.get("uuid", "") or "")
    else:
        name, uuid = str(p), ""
    return {"name": name, "uuid": uuid} if uuid else {"name": name}


def _option_lists(options: Mapping[str, list]) -> Dict[str, list]:
    return {str(k): [_plain(o) for o in v] for k, v in options.items()}


class LoungeClient:
    """Server API client authenticated with an X-Server-Key."""

    def __init__(self, base: str, server_key: str, timeout: float = 10.0):
        self.base = base.rstrip("/")
        self.server_key = server_key
        self.timeout = timeout

    def raw(self, method: str, path: str, query: Optional[dict] = None, body: Any = None) -> RawResponse:
        """Call a Server API route (e.g. "/server/v1/whitelist") and return the reply without decoding
        data. Transport failures and business errors (code != 0) still raise."""
        return self._send(method, path, query, body)

    def _call(self, method: str, path: str, query: Optional[dict] = None, body: Any = None, allow_list: bool = False) -> Any:
        """Decoded data; an encrypted reply raises EncryptionError instead (fail closed)."""
        res = self._send(method, path, query, body)
        if res.encrypted:
            route = path[len("/server/v1/"):] if path.startswith("/server/v1/") else path
            raise EncryptionError(route, "no decryption key configured")
        data = res.data
        if not isinstance(data, dict) and not (allow_list and isinstance(data, list)):
            raise TransportError(f"invalid response (http {res.status}): missing data")
        return data

    def _send(self, method: str, path: str, query: Optional[dict] = None, body: Any = None) -> RawResponse:
        url = self.base + path
        if query:
            url += "?" + urllib.parse.urlencode({k: v for k, v in query.items() if v})
        try:  # NaN / Infinity are not JSON: fail here instead of sending a body the server rejects
            data = json.dumps(body, allow_nan=False).encode() if body is not None else None
        except (TypeError, ValueError) as e:
            raise TransportError(f"cannot encode the request body: {e}") from e
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Accept", "application/json")
        req.add_header("X-Server-Key", self.server_key)
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw, status, headers = res.read(), int(getattr(res, "status", 200) or 200), _headers_of(res)
        except urllib.error.HTTPError as e:
            try:
                raw, status, headers = e.read(), e.code, _headers_of(e)
            finally:
                e.close()
        except (urllib.error.URLError, OSError) as e:
            raise TransportError(str(e)) from e
        try:
            env = json.loads(raw.decode("utf-8"))
        except ValueError as e:
            raise TransportError(f"invalid json (http {status}): {e}") from e
        if not isinstance(env, dict):
            raise TransportError(f"invalid response (http {status}): not an object")
        code = env.get("code")
        if isinstance(code, bool) or not isinstance(code, int):
            raise TransportError(f"invalid response (http {status}): missing code")
        if code != 0:
            raise ApiError(code, str(env.get("message", "")))
        if status >= 400:
            raise TransportError(f"http {status}")
        data = env.get("data")
        encrypted = bool(headers.get(HEADER_ENCRYPTION.lower())) or _is_jwe_data(data)
        return RawResponse(status, headers, str(env.get("message", "") or ""), data, env, raw, encrypted)

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

    def heartbeat(self, online: int, max_players: int = 0, version: str = "", tps: Optional[float] = None,
                  players: Optional[Iterable[Any]] = None) -> None:
        """Report live status (about every 60s). tps and players are optional (PROTOCOL 2.3): a tps that
        is not a finite number > 0 is left out; players are OnlinePlayer, {"name", "uuid"} dicts or names."""
        body: Dict[str, Any] = {"online": online, "max": max_players, "version": version}
        if isinstance(tps, (int, float)) and not isinstance(tps, bool) and math.isfinite(tps) and tps > 0:
            body["tps"] = tps
        listed = [_player(p) for p in (players or [])]
        if listed:
            body["players"] = listed
        self._call("POST", "/server/v1/heartbeat", body=body)

    # ---- Feature modules / GM channel (PROTOCOL 2.4) ----

    def modules(self) -> dict:
        """Modules enabled for this server, keyed by name, with their per-server config."""
        return dict(self._call("GET", "/server/v1/modules").get("modules") or {})

    def publish_gm_actions(self, actions: list) -> int:
        """Replace the GM action catalogue (GMAction or dicts: key/title/description/params); returns the count stored."""
        return int(self._call("PUT", "/server/v1/gm/actions", body=[_plain(a) for a in actions]).get("count", 0))

    def publish_gm_catalog(self, actions: list, options: Optional[Mapping[str, list]] = None) -> GMPublishResult:
        """Replace the actions and the named option lists (select / suggestion sources such as worlds or
        kits) in one call; options=None keeps the stored lists. Entries the server cannot accept are
        skipped and listed in the result, never a failure for the whole call."""
        body: Dict[str, Any] = {"actions": [_plain(a) for a in actions]}
        if options is not None:
            body["options"] = _option_lists(options)
        return GMPublishResult.from_dict(self._call("PUT", "/server/v1/gm/actions", body=body))

    def publish_gm_options(self, options: Mapping[str, list]) -> GMPublishResult:
        """Replace only the named option lists (the published actions stay), e.g. when a world loads."""
        return GMPublishResult.from_dict(self._call("PUT", "/server/v1/gm/options", body=_option_lists(options)))

    def poll_gm_commands(self) -> list:
        """Queued GM commands (each becomes "sent"); poll alongside the heartbeat."""
        return list(self._call("GET", "/server/v1/gm/commands", allow_list=True) or [])

    def report_gm_result(self, command_id: int, ok: bool, result: str = "") -> None:
        """Report how a command went; result is free text (<= 4000 chars)."""
        self._call("POST", f"/server/v1/gm/commands/{int(command_id)}/result", body={"ok": bool(ok), "result": result})
