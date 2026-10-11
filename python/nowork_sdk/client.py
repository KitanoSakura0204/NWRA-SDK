from __future__ import annotations

import json
import math
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Dict, Iterable, List, Mapping, Optional, Tuple

from .delivery import DecryptError, Keys, decrypt_jwe
from .entitlements import Entitlement, EntitlementValues, entitlement_body, entitlement_query
from .errors import ApiError, EncryptionError, TransportError
from .gm import GMPublishResult, _plain
from .serverusers import KickEntry, ServerBan, UserData, UserSchema, target

# Marks a Server API reply whose data is the {"jwe": "..."} envelope (DELIVERY.md section 5).
HEADER_ENCRYPTION = "X-NoWork-Encryption"
# This transport is separate from the game-data transport and its larger archive-export limit.
MAX_RESPONSE_BYTES = 8 * 1024 * 1024
_SEALED_ROUTES = frozenset(("whitelist", "whitelist/check"))


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
    message: str = ""  # extra text for some refusals, e.g. the reason of a per-server ban (reason "banned")


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
    encrypted: bool  # X-NoWork-Encryption is set or data has only the reserved "jwe" field


def _is_jwe_data(data: Any) -> bool:
    return isinstance(data, dict) and len(data) == 1 and "jwe" in data


def _headers_of(res: Any) -> Dict[str, str]:
    items = getattr(getattr(res, "headers", None), "items", None)
    try:
        return {str(k).lower(): str(v) for k, v in items()} if callable(items) else {}
    except (TypeError, ValueError):  # a test double without real headers
        return {}


def _route(path: str) -> str:
    route = path.split("?", 1)[0]
    return route[len("/server/v1/"):] if route.startswith("/server/v1/") else route


def _decryption_keys(keys: Optional[Keys]) -> Tuple[Dict[str, str], ...]:
    if keys is None:
        return ()
    try:
        supplied = [keys] if isinstance(keys, Mapping) else list(keys)
    except TypeError as exc:
        raise ValueError("decryption_keys must be a private JWK or an iterable of private JWKs") from exc
    snapshot = []
    for key in supplied:
        if not isinstance(key, Mapping) or not isinstance(key.get("kty"), str):
            raise ValueError("each decryption key must be a private JWK mapping")
        snapshot.append(dict(key))
    return tuple(snapshot)


def _reject_json_constant(value: str) -> None:
    raise ValueError("non-finite values are not valid JSON")


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


class NoworkClient:
    """Server API client authenticated with an X-Server-Key."""

    def __init__(self, base: str, server_key: str, timeout: float = 10.0, *,
                 decryption_keys: Optional[Keys] = None, server_code: Optional[str] = None, opener=None):
        """Private JWK mappings enable automatic delivery decryption (requires cryptography).

        Keys are copied once, including an iterable used for rotation. Configuring keys requires
        encrypted whitelist replies; other routes may still reply in plaintext. server_code pins
        the authenticated context to that server; without it the context's kind/route are checked.
        opener, when supplied, must provide open(request, timeout=...) like urllib's OpenerDirector.
        """
        self.base = base.rstrip("/")
        self.server_key = server_key
        self.timeout = timeout
        self._decryption_keys = _decryption_keys(decryption_keys)
        if server_code is not None and (not isinstance(server_code, str) or not server_code
                                       or ":" in server_code or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in server_code)):
            raise ValueError("server_code must be a nonempty server code without colons or whitespace")
        self._server_code = server_code
        self._opener = opener

    def raw(self, method: str, path: str, query: Optional[dict] = None, body: Any = None) -> RawResponse:
        """Call a Server API route (e.g. "/server/v1/whitelist") and return the reply without decoding
        data. Transport failures and business errors (code != 0) still raise."""
        return self._send(method, path, query, body)

    def _call(self, method: str, path: str, query: Optional[dict] = None, body: Any = None, allow_list: bool = False) -> Any:
        """Decoded data; unusable encrypted replies and whitelist downgrades fail closed."""
        res = self._send(method, path, query, body)
        route = _route(path)
        data = res.data
        if res.encrypted:
            if not self._decryption_keys:
                raise EncryptionError(route, "no decryption key configured")
            data = self._open(data, route)
            if route == "whitelist" and (not isinstance(data, dict) or type(data.get("mode")) is not int
                                         or data["mode"] not in (0, 1, 2)):
                raise EncryptionError(route, "decrypted whitelist must contain an integer mode from 0 to 2")
            if route == "whitelist/check" and (not isinstance(data, dict) or type(data.get("allowed")) is not bool):
                raise EncryptionError(route, "decrypted join check must contain a boolean allowed")
        elif self._decryption_keys and route in _SEALED_ROUTES:
            raise EncryptionError(route, "plain reply while decryption keys are configured")
        if not isinstance(data, dict) and not (allow_list and isinstance(data, list)):
            if res.encrypted:
                raise EncryptionError(route, "decrypted data has an invalid shape")
            raise TransportError(f"invalid response (http {res.status}): missing data")
        return data

    def _open(self, data: Any, route: str) -> Any:
        jwe = data.get("jwe") if _is_jwe_data(data) else None
        if not isinstance(jwe, str) or not jwe:
            raise EncryptionError(route, "encrypted reply without a JWE")
        try:
            plain, header = decrypt_jwe(jwe, self._decryption_keys)
        except (DecryptError, ValueError, TypeError, AttributeError, KeyError, IndexError, OverflowError, RecursionError) as exc:
            raise EncryptionError(route, "the JWE did not open") from exc
        ctx = header.get("ctx") if isinstance(header, dict) else None
        parts = ctx.split(":", 2) if isinstance(ctx, str) else []
        if (len(parts) != 3 or parts[0] != "server" or not parts[1] or parts[2] != route
                or (self._server_code is not None and parts[1] != self._server_code)):
            raise EncryptionError(route, "envelope ctx mismatch")
        try:
            return json.loads(plain, parse_constant=_reject_json_constant)
        except (ValueError, TypeError, RecursionError) as exc:
            raise EncryptionError(route, "decrypted data is not JSON") from exc

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
            send = self._opener.open if self._opener is not None else urllib.request.urlopen
            with send(req, timeout=self.timeout) as res:
                raw, status, headers = res.read(MAX_RESPONSE_BYTES + 1), int(getattr(res, "status", 200) or 200), _headers_of(res)
        except urllib.error.HTTPError as e:
            try:
                raw, status, headers = e.read(MAX_RESPONSE_BYTES + 1), e.code, _headers_of(e)
            finally:
                e.close()
        except (urllib.error.URLError, OSError) as e:
            raise TransportError(str(e)) from e
        if len(raw) > MAX_RESPONSE_BYTES:
            raise TransportError(f"response exceeds {MAX_RESPONSE_BYTES // (1024 * 1024)} MiB")
        try:
            env = json.loads(raw.decode("utf-8"), parse_constant=_reject_json_constant)
        except (ValueError, RecursionError) as e:
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
            str(d.get("message", "") or ""),
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

    def exchange_gm_code(self, code: str, code_verifier: str, redirect_uri: str) -> dict:
        """Backend-only character SSO exchange, after checking callback state in the GM
        browser session. Returns serverId/serverCode/subject/character/authenticatedAt/scope.
        Expired, consumed or invalid codes raise ApiError 403; never retry a consumed code."""
        return self._call("POST", "/server/v1/gm/sso/exchange", body={
            "code": code, "codeVerifier": code_verifier, "redirectUri": redirect_uri,
        })

    def modules(self) -> dict:
        """Modules enabled for this server, keyed by name, with their per-server config."""
        return dict(self._call("GET", "/server/v1/modules").get("modules") or {})

    def publish_gm_actions(self, actions: list) -> int:
        """Replace the GM action catalogue (GMAction or dicts: key/title/description/params); returns the count stored."""
        return int(self._call("PUT", "/server/v1/gm/actions", body=[_plain(a) for a in actions]).get("count", 0))

    def publish_gm_catalog(self, actions: Optional[list], options: Optional[Mapping[str, list]] = None) -> GMPublishResult:
        """Replace the actions and the named option lists (select / suggestion sources such as worlds or
        kits) in one call; options=None keeps the stored lists. Entries the server cannot accept are
        skipped and listed in the result, never a failure for the whole call."""
        body: Dict[str, Any] = {}
        if actions is not None:
            body["actions"] = [_plain(a) for a in actions]
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

    # ---- Player data and per-server bans (SERVER_USERS.md) ----

    def issue_chat_ticket(self, *, access_token: str = "", uid: int = 0, player_name: str = "",
                          player_uuid: str = "", channels: Optional[List[int]] = None) -> dict:
        """One-use chat login. Supply exactly one of access_token or uid (UID mode requires server permission)."""
        body = {"accessToken": access_token, "uid": uid, "playerName": player_name,
                "playerUuid": player_uuid, "channels": channels or []}
        return self._call("POST", "/server/v1/chat/tickets", body=body)

    def poll_chat_relay(self) -> list:
        return self._call("GET", "/server/v1/chat/relay").get("items") or []

    def ack_chat_relay(self, done: List[int], failed: Optional[List[dict]] = None) -> int:
        """Failed entries are permanent {id,error} failures. Leave transient failures pending."""
        return int(self._call("POST", "/server/v1/chat/relay/ack", body={"done": done, "failed": failed or []}).get("updated", 0))

    def post_chat_message(self, player: str, text: str, *, player_uuid: str = "", uid: int = 0, kind: str = "chat") -> dict:
        return self._call("POST", "/server/v1/chat/messages", body={
            "player": player, "text": text, "playerUuid": player_uuid, "uid": uid, "kind": kind,
        })

    def user_schema(self) -> UserSchema:
        """The data schema the server owner defined (needs the module on for this server)."""
        return UserSchema.from_dict(self._call("GET", "/server/v1/users/schema"))

    def user_data(self, uid: int) -> UserData:
        """One account's data (uid from the join check or the whitelist)."""
        return UserData.from_dict(self._call("GET", _user_path(uid)))

    def put_user_data(self, uid: int, data: Mapping[str, Any], version: Optional[int] = None) -> UserData:
        """Replace the account's stored values (fields left out are unset). Pass the version you last
        read to refuse a concurrent change (ApiError 409, see is_version_conflict)."""
        return UserData.from_dict(self._call("PUT", _user_path(uid), body=_write_body(data, version)))

    def patch_user_data(self, uid: int, data: Mapping[str, Any], version: Optional[int] = None) -> UserData:
        """Merge values into the stored ones; a None value deletes that field."""
        return UserData.from_dict(self._call("PATCH", _user_path(uid), body=_write_body(data, version)))

    def bans(self, uid: int = 0, name: str = "", uuid: str = "") -> List[ServerBan]:
        """Active bans on this server: all (at most 5000, newest first) or those of the matching
        accounts. Works with the module off too."""
        q = {k: str(v) for k, v in target(uid, name, uuid).items()}
        d = self._call("GET", "/server/v1/bans", q or None)
        return [ServerBan.from_dict(b) for b in (d.get("bans") or []) if isinstance(b, dict)]

    def kick_list(self) -> List[KickEntry]:
        """Online players (as of the last heartbeat) whose account is banned here: disconnect them."""
        d = self._call("GET", "/server/v1/bans/kick")
        return [KickEntry.from_dict(k) for k in (d.get("players") or []) if isinstance(k, dict)]

    def ban(self, uid: int = 0, name: str = "", uuid: str = "", reason: str = "", expire_time: str = "",
            duration_minutes: int = 0) -> Tuple[int, Optional[ServerBan]]:
        """Ban an account on this server (module on, pluginBans allowed). expire_time or duration_minutes;
        neither = permanent. A ban an administrator placed cannot be replaced (ApiError 403)."""
        body = target(uid, name, uuid)
        if reason:
            body["reason"] = reason
        if expire_time:
            body["expireTime"] = expire_time
        if duration_minutes:
            body["durationMinutes"] = int(duration_minutes)
        d = self._call("POST", "/server/v1/bans", body=body)
        b = d.get("ban")
        return int(d.get("uid", 0) or 0), ServerBan.from_dict(b) if isinstance(b, dict) else None

    def unban(self, uid: int = 0, name: str = "", uuid: str = "", reason: str = "") -> int:
        """Lift the account's active ban on this server (not one an administrator placed); returns the uid."""
        body = target(uid, name, uuid)
        if reason:
            body["reason"] = reason
        return int(self._call("POST", "/server/v1/bans/lift", body=body).get("uid", 0) or 0)

    # ---- Custom per-app values (module "delivery", DELIVERY.md section 8) ----

    def entitlements(self, uid: int = 0, sub: str = "") -> EntitlementValues:
        """Every custom value this server stored for a user. Name the user by uid, or by the sub
        this server sees (required by a pairwise delivery profile)."""
        return EntitlementValues.from_dict(self._call("GET", "/server/v1/delivery/entitlements", entitlement_query(uid, sub)))

    def put_entitlement(self, uid: int = 0, sub: str = "", key: str = "", value: Any = None) -> Entitlement:
        """Store one key for a user and return what the backend stored. The key must start with a
        letter and hold only letters, digits and ". _ -" (at most 64 bytes); the value may be any
        JSON up to 4096 bytes, and a profile holds at most 50 keys per user."""
        return Entitlement.from_dict(self._call("PUT", "/server/v1/delivery/entitlements", body=entitlement_body(uid, sub, key, value)))

    def delete_entitlement(self, uid: int = 0, sub: str = "", key: str = "") -> Entitlement:
        """Remove one key. Removing a key that was never written is not an error."""
        q = entitlement_query(uid, sub)
        q["key"] = (key or "").strip()
        if not q["key"]:
            raise ValueError("entitlement key is required")
        return Entitlement.from_dict(self._call("DELETE", "/server/v1/delivery/entitlements", q))


def _user_path(uid: int) -> str:
    if isinstance(uid, bool) or not isinstance(uid, int) or uid <= 0:
        raise ValueError(f"nowork: uid must be a positive integer, got {uid!r}")
    return f"/server/v1/users/{uid}/data"


def _write_body(data: Mapping[str, Any], version: Optional[int]) -> Dict[str, Any]:
    body: Dict[str, Any] = {"data": dict(data or {})}  # the server wants a data object, even an empty one
    if version is not None:
        body["version"] = int(version)
    return body
