"""Synchronous game records and public reference catalogs (standard library only).

PublicGameClient sends no credentials. AccountGameClient takes a site login JWT;
AdminGameClient takes an administrator JWT. ServerGameClient sends X-Server-Key
only to the existing, explicitly shared Hypergryph role API. Provider cookies and
Skland tokens belong to the site's binding flow, never these clients.

Catalog entities do not imply ownership. Inspect each field's policy, status and
source.auth_scope, and snapshot timestamps. Unsupported/private/challenge sections
are returned as typed data; the SDK never retries or bypasses provider controls.
"""
from __future__ import annotations

import json
import math
import re
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import MISSING, dataclass, field, fields, is_dataclass
from functools import lru_cache
from typing import Any, Dict, Iterator, List, Optional, Sequence, Type, TypeVar, Union, get_args, get_origin, get_type_hints

from .account import _NoRedirect
from .errors import ApiError, EncryptionError, TransportError

MAX_RESPONSE_BYTES = 8 * 1024 * 1024
MAX_ARCHIVE_EXPORT_BYTES = 20 * 1024 * 1024
MAX_TIMEOUT = 60.0
MAX_PAGE_SIZE = 100
MAX_PAGES = 100
_T = TypeVar("_T")
LocalizedText = Dict[str, str]
FieldValue = Union[str, int, float, bool, List[str], None]


class PaginationError(TransportError):
    """The server repeated a cursor or the caller's page budget was exhausted."""


class _DTO:
    @classmethod
    def from_dict(cls: Type[_T], data: dict) -> _T:
        return _decode(cls, data, cls.__name__)


@dataclass
class GamePolicy(_DTO):
    visibility: str = "owner"
    exportable: bool = False


@dataclass
class GameSource(_DTO):
    source_id: str = ""
    kind: str = ""
    name: str = ""
    url: str = ""
    auth_scope: str = ""
    fetched_at: str = ""
    updated_at: Optional[str] = None
    license: str = ""
    attribution: str = ""


@dataclass
class GameIcon(_DTO):
    url: str = ""
    alt: LocalizedText = field(default_factory=dict)
    source_id: str = ""
    width: int = 0
    height: int = 0


@dataclass
class GameCapability(_DTO):
    id: str = ""
    name: LocalizedText = field(default_factory=dict)
    supported: bool = False
    auth_scope: str = ""
    status: str = "unavailable"
    reason: str = ""
    source: Optional[GameSource] = None


@dataclass
class Game(_DTO):
    id: str
    provider: str
    name: LocalizedText = field(default_factory=dict)
    icon: Optional[GameIcon] = None
    regions: List[str] = field(default_factory=list)
    capabilities: List[GameCapability] = field(default_factory=list)


@dataclass
class GameRole(_DTO):
    id: str
    game_id: str
    uid: str = ""
    region: str = ""
    server: str = ""
    nickname: str = ""
    level: Optional[int] = None
    icon: Optional[GameIcon] = None
    policy: GamePolicy = field(default_factory=GamePolicy)


@dataclass
class GameField(_DTO):
    key: str
    name: LocalizedText = field(default_factory=dict)
    kind: str = ""
    value: FieldValue = None
    unit: str = ""
    status: str = "unavailable"
    policy: GamePolicy = field(default_factory=GamePolicy)
    source: Optional[GameSource] = None
    updated_at: Optional[str] = None


@dataclass
class GameEntity(_DTO):
    id: str
    kind: str
    name: LocalizedText = field(default_factory=dict)
    icon: Optional[GameIcon] = None
    fields: List[GameField] = field(default_factory=list)
    policy: GamePolicy = field(default_factory=GamePolicy)
    source: Optional[GameSource] = None


@dataclass
class GameSection(_DTO):
    id: str
    name: LocalizedText = field(default_factory=dict)
    status: str = "unavailable"
    reason: str = ""
    fields: List[GameField] = field(default_factory=list)
    entities: List[GameEntity] = field(default_factory=list)
    policy: GamePolicy = field(default_factory=GamePolicy)
    source: Optional[GameSource] = None
    updated_at: Optional[str] = None
    next_cursor: str = ""


@dataclass
class GameSnapshot(_DTO):
    game_id: str
    sections: List[GameSection]
    fetched_at: str
    role: Optional[GameRole] = None
    expires_at: Optional[str] = None
    next_cursor: str = ""


@dataclass
class GameDiscovery(_DTO):
    status: str = "unavailable"
    updated_at: Optional[str] = field(default=None, metadata={"json": "updatedAt"})
    reason: str = ""


@dataclass
class AccountGames(_DTO):
    games: List[Game]
    roles: List[GameRole] = field(default_factory=list)
    discovery: Optional[GameDiscovery] = None
    selected_role_id: str = field(default="", metadata={"json": "selectedRoleId"})


@dataclass
class MihoyoRole(_DTO):
    """A stored role; id is the site database role ID, game_uid is upstream."""
    id: int
    game_biz: str = field(default="", metadata={"json": "gameBiz"})
    game_id: str = field(default="", metadata={"json": "gameId"})
    game_name: str = field(default="", metadata={"json": "gameName"})
    game_uid: str = field(default="", metadata={"json": "gameUid"})
    region: str = ""
    region_name: str = field(default="", metadata={"json": "regionName"})
    nickname: str = ""
    level: int = 0
    is_chosen: bool = field(default=False, metadata={"json": "isChosen"})
    sync_time: Optional[str] = field(default=None, metadata={"json": "syncTime"})


@dataclass
class MihoyoStatus(_DTO):
    enabled: bool
    bound: bool
    roles: List[MihoyoRole]
    account_id: Optional[int] = field(default=None, metadata={"json": "accountId"})
    allow_qr: bool = field(default=False, metadata={"json": "allowQr"})
    allow_cookie: bool = field(default=False, metadata={"json": "allowCookie"})
    record_enabled: bool = field(default=False, metadata={"json": "recordEnabled"})
    last_sync: Optional[str] = field(default=None, metadata={"json": "lastSync"})
    next_sync_at: Optional[str] = field(default=None, metadata={"json": "nextSyncAt"})
    method: str = ""
    state: str = ""
    state_message: str = field(default="", metadata={"json": "stateMessage"})


@dataclass
class HypergryphBinding(_DTO):
    """Site binding identity for archive requests; never an upstream UID."""
    id: int
    status: str = ""
    share: bool = False
    nickname: str = ""
    last_sync: Optional[str] = field(default=None, metadata={"json": "lastSync"})
    selected_role_id: str = field(default="", metadata={"json": "selectedRoleId"})


@dataclass
class HypergryphStatus(_DTO):
    enabled: bool
    account: Optional[HypergryphBinding]
    allow_sms: bool = field(default=False, metadata={"json": "allowSms"})
    allow_token: bool = field(default=False, metadata={"json": "allowToken"})
    server_api: bool = field(default=False, metadata={"json": "serverApi"})


@dataclass
class GameArchiveMetadata(_DTO):
    game_id: str
    role_id: str
    version: int
    revision: int
    status: str
    stale: bool
    bytes: int
    fetched_at: str
    expires_at: str


@dataclass
class GameArchiveEntry(_DTO):
    provider: str
    role_id: str
    version: int
    revision: int
    status: str
    stale: bool
    expires_at: str
    snapshot: GameSnapshot


@dataclass
class GameArchiveHistory(_DTO):
    id: int
    game_id: str
    role_id: str
    revision: int
    status: str
    bytes: int
    created_at: str


@dataclass
class GameArchivePolicy(_DTO):
    id: int
    game_id: str
    path: str
    visibility: str
    exportable: bool
    revision: int
    updated_at: str


@dataclass
class GameArchivePolicyAudit(_DTO):
    """previous/current are JSON policy strings; creation has previous='null'."""
    id: int
    game_id: str
    path: str
    revision: int
    previous: str
    current: str
    created_at: str


@dataclass
class GameArchiveScope(_DTO):
    """Server-derived exported identity; never accepted as a request override."""
    site_id: int
    user_id: int
    account_id: int


@dataclass
class GameArchiveExport(_DTO):
    scope: GameArchiveScope
    provider: str
    exported_at: str
    snapshots: List[GameArchiveEntry]
    history: List[GameArchiveHistory]
    policies: List[GameArchivePolicy]
    policy_audit: List[GameArchivePolicyAudit]


@dataclass
class AssetEndpoint(_DTO):
    locale: str = ""
    version: str = ""
    url: str = ""


@dataclass
class AssetManifest(_DTO):
    id: str
    game_id: str
    name: LocalizedText = field(default_factory=dict)
    source: GameSource = field(default_factory=GameSource)
    endpoints: List[AssetEndpoint] = field(default_factory=list)
    media_hosts: List[str] = field(default_factory=list)
    ttl_seconds: int = 0
    max_bytes: int = 0
    max_entries: int = 0


@dataclass
class AssetCache(_DTO):
    provider: str = ""
    locale: str = ""
    version: str = ""
    disabled: bool = False
    source_url: str = ""
    content_hash: str = ""
    entry_count: int = 0
    fetched_at: Optional[str] = None
    checked_at: Optional[str] = None
    expires_at: Optional[str] = None
    attempted_at: Optional[str] = None
    last_error_code: str = ""


@dataclass
class AssetProvider(_DTO):
    manifest: AssetManifest
    caches: List[AssetCache] = field(default_factory=list)


@dataclass
class AssetCatalogPage(_DTO):
    provider: AssetManifest
    cache: AssetCache
    status: str
    list: List[GameEntity]
    count: int
    page: int
    limit: int

    @property
    def has_more(self) -> bool:
        return self.page * self.limit < self.count


@dataclass
class SharedHypergryphRole(_DTO):
    """Legacy server-shared Arknights projection; summary is the legacy JSON schema."""
    uid: str
    nickname: str = ""
    channel_name: str = field(default="", metadata={"json": "channelName"})
    is_official: bool = field(default=False, metadata={"json": "isOfficial"})
    is_default: bool = field(default=False, metadata={"json": "isDefault"})
    level: int = 0
    ap_now: int = field(default=0, metadata={"json": "apNow"})
    summary: Optional[Dict[str, Any]] = None
    last_sync: Optional[str] = field(default=None, metadata={"json": "lastSync"})


@dataclass
class SharedHypergryphRoles(_DTO):
    uid: int
    status: str
    roles: List[SharedHypergryphRole]
    last_sync: Optional[str] = field(default=None, metadata={"json": "lastSync"})
    server_time: int = field(default=0, metadata={"json": "serverTime"})


@lru_cache(maxsize=64)
def _hints(cls):
    return get_type_hints(cls)


def _decode(expected, value, path):
    """Validate containers/scalars without coercing IDs, booleans or empty data."""
    origin, args = get_origin(expected), get_args(expected)
    if expected is Any:
        return value
    if origin is Union:
        if value is None and type(None) in args:
            return None
        for candidate in args:
            try:
                return _decode(candidate, value, path)
            except TransportError:
                pass
    elif origin is list:
        # Go's nil slices serialize as null and represent an empty collection.
        if value is None:
            return []
        if isinstance(value, list):
            return [_decode(args[0], item, path + "[]") for item in value]
    elif origin is dict:
        if isinstance(value, dict):
            return {_decode(args[0], key, path): _decode(args[1], item, path) for key, item in value.items()}
    elif is_dataclass(expected):
        if isinstance(value, dict):
            parsed = {}
            for f in fields(expected):
                key = f.metadata.get("json", f.name)
                if key in value:
                    parsed[f.name] = _decode(_hints(expected)[f.name], value[key], path + "." + key)
                elif f.default is MISSING and f.default_factory is MISSING:
                    raise TransportError("missing game response field: " + path + "." + key)
            return expected(**parsed)
    elif expected is type(None) and value is None:
        return None
    elif expected is float and type(value) in (int, float) and math.isfinite(value):
        return value
    elif expected in (str, int, bool) and type(value) is expected:
        return value
    raise TransportError("invalid game response field: " + path)


def _positive(value: int, name: str, maximum: int) -> int:
    if type(value) is not int or not 1 <= value <= maximum:
        raise ValueError(f"{name} must be an integer from 1 to {maximum}")
    return value


def _uint64(value: int, name: str) -> int:
    if type(value) is not int or not 0 <= value <= 2**64 - 1:
        raise ValueError(name + " must be an unsigned 64-bit integer")
    return value


def _archive_game_id(game_id: str) -> str:
    _segment(game_id, "game_id")
    return _text(game_id, "game_id", 64)


def _policy_path(path: str) -> str:
    _text(path, "policy path", 320)
    parts = path.split("/")
    shape = (len(parts) == 5 and parts[3] == "fields") or (len(parts) == 7 and parts[3] == "entities" and parts[5] == "fields")
    if not path.startswith("/sections/") or not shape or any(not part or re.search(r"~(?![01])", part) for part in parts[1:]):
        raise ValueError("policy path must select a section or entity field using JSON-pointer escaping")
    return path


def archive_field_path(section_id: str, field_key: str, *, entity_id: str = "") -> str:
    """Build the backend's field selector, escaping '~' and '/' in upstream IDs."""
    def escape(value):
        _text(value, "field selector component", 320)
        if not value:
            raise ValueError("section_id and field_key must not be empty")
        return value.replace("~", "~0").replace("/", "~1")
    path = "/sections/" + escape(section_id)
    if entity_id:
        path += "/entities/" + escape(entity_id)
    elif not isinstance(entity_id, str):
        raise ValueError("entity_id must be a string")
    return _policy_path(path + "/fields/" + escape(field_key))


def _segment(value: str, name: str = "identifier") -> str:
    if not isinstance(value, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}", value):
        raise ValueError(name + " has an invalid format")
    return urllib.parse.quote(value, safe="")


def _provider(value: str) -> str:
    if value not in ("mihoyo", "hypergryph"):
        raise ValueError("provider must be mihoyo or hypergryph")
    return value


def _locale(value: str) -> str:
    if value not in ("en", "ja", "zh_CN", "zh_TW"):
        raise ValueError("locale must be en, ja, zh_CN or zh_TW")
    return value


def _text(value: str, name: str, max_bytes: int) -> str:
    if not isinstance(value, str) or len(value.encode("utf-8")) > max_bytes or any(ord(c) < 32 or ord(c) == 127 for c in value):
        raise ValueError(name + " is invalid or exceeds its byte limit")
    return value


def _reject_json_constant(value):
    raise ValueError("non-finite JSON number")


def _asset_variant(locale: str, version: str) -> Dict[str, str]:
    _text(locale, "locale", 16)
    _text(version, "version", 64)
    if locale:
        _locale(locale)
    if version:
        _segment(version, "version")
    return {"locale": locale, "version": version}


def _check_snapshot(snapshot: GameSnapshot, game_id: str) -> GameSnapshot:
    if snapshot.game_id != game_id:
        raise TransportError("snapshot game_id does not match the requested game")
    try:
        _text(snapshot.next_cursor, "response cursor", 2048)
        for section in snapshot.sections:
            _text(section.next_cursor, "section cursor", 2048)
    except ValueError as error:
        raise TransportError("invalid game response cursor") from error
    return snapshot


def _snapshot_query(provider, role_id, sections, cursor, limit, locale):
    query = {"limit": _positive(limit, "limit", MAX_PAGE_SIZE), "locale": _locale(locale)}
    if role_id is not None:
        if provider == "mihoyo":
            query["roleId"] = _positive(role_id, "role_id (stored database ID)", 2**63 - 1)
        else:
            if not isinstance(role_id, str) or not role_id:
                raise ValueError("role_id must be a nonempty bound-role ID string")
            query["roleId"] = _text(role_id, "role_id", 128)
    if sections is not None:
        if not isinstance(sections, (list, tuple)) or len(sections) > 32:
            raise ValueError("sections must contain at most 32 section IDs")
        for section in sections:
            _segment(section, "section")
        if sections:
            query["sections"] = ",".join(sections)
    if cursor:
        query["cursor"] = _text(cursor, "cursor", 2048)
    elif not isinstance(cursor, str):
        raise ValueError("cursor must be a string")
    return query


class _Transport:
    def __init__(self, base: str, timeout: float = 10.0, *, opener=None, header: str = "", credential: str = ""):
        if type(timeout) not in (int, float) or not math.isfinite(timeout) or not 0 < timeout <= MAX_TIMEOUT:
            raise ValueError("timeout must be finite and between 0 and 60 seconds")
        if not isinstance(base, str) or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in base):
            raise ValueError("base must be an HTTP(S) site URL")
        try:
            url = urllib.parse.urlsplit(base)
            valid = url.scheme in ("http", "https") and url.hostname and url.username is None and url.password is None and not (url.query or url.fragment)
            url.port  # Validate malformed ports before any request.
        except ValueError:
            valid = False
        if not valid:
            raise ValueError("base must be an HTTP(S) site URL without credentials, query or fragment")
        if header and (not isinstance(credential, str) or not credential or any(c.isspace() or ord(c) < 32 or ord(c) == 127 for c in credential)):
            raise ValueError("a nonempty credential without whitespace is required")
        self.base, self.timeout = base.rstrip("/"), timeout
        self._header, self._credential = header, credential
        self._opener = opener if opener is not None else urllib.request.build_opener(_NoRedirect())

    def _call(self, method, path, query=None, body=None, *, allow_empty=False,
              response_limit=MAX_RESPONSE_BYTES):
        address = self.base + path
        if query:
            address += "?" + urllib.parse.urlencode(query)
        headers = {"Accept": "application/json"}
        if self._header:
            headers[self._header] = ("Bearer " if self._header == "Authorization" else "") + self._credential
        try:
            data = json.dumps(body, allow_nan=False).encode("utf-8") if body is not None else None
            if data is not None:
                headers["Content-Type"] = "application/json"
            request = urllib.request.Request(address, data=data, headers=headers, method=method)
            try:
                with self._opener.open(request, timeout=self.timeout) as response:
                    raw, status, response_headers = response.read(response_limit + 1), response.status, response.headers
            except urllib.error.HTTPError as error:
                try:
                    raw, status, response_headers = error.read(response_limit + 1), error.code, error.headers
                finally:
                    error.close()
            if len(raw) > response_limit:
                raise TransportError(f"game response exceeds {response_limit // (1024 * 1024)} MiB")
            envelope = json.loads(raw, parse_constant=_reject_json_constant)
        except (OSError, ValueError, TypeError, RecursionError) as error:
            raise TransportError("game API transport or JSON decoding failed") from error
        if not isinstance(envelope, dict) or type(envelope.get("code")) is not int:
            raise TransportError("invalid game API envelope")
        if envelope["code"] != 0:
            raise ApiError(envelope["code"], str(envelope.get("message", "")))
        if not 200 <= status < 300 or "data" not in envelope or (envelope["data"] is None and not allow_empty):
            raise TransportError(f"invalid game API response (HTTP {status})")
        result = envelope["data"]
        if (response_headers and response_headers.get("X-NoWork-Encryption")) or (isinstance(result, dict) and "jwe" in result):
            raise EncryptionError(path, "decrypt the server response before reading game data")
        return result


class _Snapshots(_Transport):
    def _snapshot(self, prefix, provider, game_id, role_id, sections, cursor, limit, locale):
        provider = _provider(provider)
        query = _snapshot_query(provider, role_id, sections, cursor, limit, locale)
        ending = "data" if provider == "mihoyo" else "snapshot"
        result = self._call("GET", f"{prefix}/{provider}/games/{_segment(game_id, 'game_id')}/{ending}", query)
        return _check_snapshot(GameSnapshot.from_dict(result), game_id)

    def snapshot_pages(self, provider: str, game_id: str, *, role_id: Optional[Union[int, str]] = None,
                       sections: Optional[Sequence[str]] = None,
                       cursor: str = "", limit: int = 20, locale: str = "en", max_pages: int = 10) -> Iterator[GameSnapshot]:
        """Yield explicit pages; fail on cycles/budget exhaustion instead of truncating.

        Section-local cursors are exposed on GameSection for explicit requests;
        only the snapshot's top-level next_cursor drives this iterator.
        """
        _positive(max_pages, "max_pages", MAX_PAGES)
        _text(cursor, "cursor", 2048)
        seen = {cursor}
        for _ in range(max_pages):
            page = self.snapshot(provider, game_id, role_id=role_id, sections=sections, cursor=cursor, limit=limit, locale=locale)
            yield page
            cursor = page.next_cursor
            if not cursor:
                return
            if cursor in seen:
                raise PaginationError("game snapshot cursor repeated")
            seen.add(cursor)
        raise PaginationError("game snapshot page budget exhausted; resume with the last next_cursor")


class PublicGameClient(_Snapshots):
    """Credential-free Hypergryph capabilities and public sections.

    miHoYo currently exposes these operations only through its account API.
    A supplied custom opener must also refuse redirects and add no credentials.
    """
    def __init__(self, base: str, timeout: float = 10.0, *, opener=None):
        super().__init__(base, timeout, opener=opener)

    def games(self, provider: str = "hypergryph") -> List[Game]:
        self._public_provider(provider)
        return _games_wrapper(self._call("GET", "/api/hypergryph/games"))

    def snapshot(self, provider: str, game_id: str, *, role_id: Optional[str] = None,
                 sections: Optional[Sequence[str]] = None,
                 cursor: str = "", limit: int = 20, locale: str = "en") -> GameSnapshot:
        self._public_provider(provider)
        if role_id is not None:
            raise ValueError("public snapshots cannot select account roles")
        return self._snapshot("/api", provider, game_id, None, sections, cursor, limit, locale)

    @staticmethod
    def _public_provider(provider):
        _provider(provider)
        if provider != "hypergryph":
            raise ValueError("miHoYo game data requires AccountGameClient")


def _games_wrapper(value) -> List[Game]:
    if not isinstance(value, dict) or "games" not in value:
        raise TransportError("invalid games response: missing games")
    return _decode(List[Game], value["games"], "games")


class _ArchiveClient(_Transport):
    """Archive operations shared only by the account/admin JWT clients."""
    _prefix = "/api/account"

    def _archive_path(self, provider: str, account_id: int) -> str:
        return f"{self._prefix}/{_provider(provider)}/accounts/{_positive(account_id, 'account_id', 2**63 - 1)}/archive"

    def archive_snapshots(self, provider: str, account_id: int) -> List[GameArchiveMetadata]:
        """List at most 64 saved snapshots for one owned/authorized binding."""
        result = self._call("GET", self._archive_path(provider, account_id) + "/snapshots")
        return self._archive_list(GameArchiveMetadata, result, 64)

    def archived_snapshot(self, provider: str, account_id: int, game_id: str,
                          role_id: str = "") -> GameArchiveEntry:
        path = self._archive_path(provider, account_id)
        query = {"game_id": _archive_game_id(game_id)}
        if role_id:
            _segment(role_id, "archive role_id")
            query["role_id"] = role_id
        elif not isinstance(role_id, str):
            raise ValueError("archive role_id must be a string")
        entry = GameArchiveEntry.from_dict(self._call("GET", path + "/snapshots", query))
        if entry.provider != provider:
            raise TransportError("archive entry provider does not match the request")
        _check_snapshot(entry.snapshot, game_id)
        # A blank role identity is a legitimate server-enforced redaction.
        return entry

    @staticmethod
    def _archive_list(model: Type[_T], data, maximum: int) -> List[_T]:
        if not isinstance(data, list) or len(data) > maximum:
            raise TransportError("invalid or oversized archive list")
        return [_decode(model, item, model.__name__) for item in data]

    def _archive_page(self, provider, account_id, route, model, before, limit):
        query = {"before": _uint64(before, "before"), "limit": _positive(limit, "limit", MAX_PAGE_SIZE)}
        data = self._call("GET", self._archive_path(provider, account_id) + route, query)
        rows = self._archive_list(model, data, limit)
        previous = before if before else 2**64
        for row in rows:
            if not 0 < row.id < previous:
                raise PaginationError("archive page IDs must descend below the before cursor")
            previous = row.id
        return rows

    def archive_history(self, provider: str, account_id: int, *, before: int = 0,
                        limit: int = 50) -> List[GameArchiveHistory]:
        """Read backwards by ID; use the last returned ID as the next before."""
        return self._archive_page(provider, account_id, "/history", GameArchiveHistory, before, limit)

    def archive_policies(self, provider: str, account_id: int, game_id: str) -> List[GameArchivePolicy]:
        data = self._call("GET", self._archive_path(provider, account_id) + "/policies", {"game_id": _archive_game_id(game_id)})
        return self._archive_list(GameArchivePolicy, data, 256)

    def archive_policy_audit(self, provider: str, account_id: int, *, before: int = 0,
                             limit: int = 50) -> List[GameArchivePolicyAudit]:
        return self._archive_page(provider, account_id, "/policy-audit", GameArchivePolicyAudit, before, limit)

    def _archive_pages(self, reader, provider, account_id, before, limit, max_pages):
        _positive(max_pages, "max_pages", MAX_PAGES)
        for _ in range(max_pages):
            rows = reader(provider, account_id, before=before, limit=limit)
            if not rows:
                return
            yield rows
            if len(rows) < limit:
                return
            before = rows[-1].id
        raise PaginationError("archive page budget exhausted; resume using the last returned ID")

    def archive_history_pages(self, provider: str, account_id: int, *, before: int = 0,
                              limit: int = 50, max_pages: int = 10) -> Iterator[List[GameArchiveHistory]]:
        return self._archive_pages(self.archive_history, provider, account_id, before, limit, max_pages)

    def archive_policy_audit_pages(self, provider: str, account_id: int, *, before: int = 0,
                                   limit: int = 50, max_pages: int = 10) -> Iterator[List[GameArchivePolicyAudit]]:
        return self._archive_pages(self.archive_policy_audit, provider, account_id, before, limit, max_pages)

    def export_archive(self, provider: str, account_id: int) -> GameArchiveExport:
        """Request the server-redacted export; only this request permits 20 MiB.

        The backend permits 16 MiB of account snapshot data plus export metadata.
        Serializing an owner snapshot locally is not a substitute for this API.
        """
        data = self._call("GET", self._archive_path(provider, account_id) + "/export",
                          response_limit=MAX_ARCHIVE_EXPORT_BYTES)
        result = GameArchiveExport.from_dict(data)
        if result.provider != provider or result.scope.account_id != account_id:
            raise TransportError("archive export scope does not match the requested binding")
        if len(result.snapshots) > 64 or len(result.history) > 200 or len(result.policies) > 256 or len(result.policy_audit) > 200:
            raise TransportError("archive export exceeds collection limits")
        for entry in result.snapshots:
            if entry.provider != provider:
                raise TransportError("archive export contains a different provider")
        return result

    def clear_archive(self, provider: str, account_id: int) -> None:
        """Clear saved snapshots/history; retain policies, audit, and binding.

        This destructive request is made once and is never retried by the SDK.
        """
        self._call("DELETE", self._archive_path(provider, account_id), allow_empty=True)


class _AssetClient(_ArchiveClient):
    _prefix = "/api/account"

    def __init__(self, base: str, token: str, timeout: float = 10.0, *, opener=None):
        super().__init__(base, timeout, opener=opener, header="Authorization", credential=token)

    def asset_providers(self) -> List[AssetProvider]:
        """Read site-specific cache metadata for public catalogs; requires a JWT."""
        return _decode(List[AssetProvider], self._call("GET", self._prefix + "/game-assets/providers"), "providers")

    def asset_catalog(self, provider: str, *, locale: str = "", version: str = "",
                      search: str = "", kind: str = "", page: int = 1, limit: int = 50) -> AssetCatalogPage:
        """Empty locale/version selects the provider manifest's first endpoint."""
        _segment(provider, "asset provider")
        variant = _asset_variant(locale, version)
        if kind not in ("", "character", "item", "weapon", "equipment", "quest", "achievement", "other"):
            raise ValueError("unknown catalog entity kind")
        query = {"provider": provider, **variant,
                 "q": _text(search, "search", 128), "kind": kind,
                 "page": _positive(page, "page", 20000), "limit": _positive(limit, "limit", MAX_PAGE_SIZE)}
        result = AssetCatalogPage.from_dict(self._call("GET", self._prefix + "/game-assets/catalog", query))
        if result.page != page or result.limit != limit or result.count < 0 or len(result.list) > limit or result.provider.id != provider:
            raise TransportError("catalog response does not match the requested page")
        return result

    def asset_catalog_pages(self, provider: str, *, locale: str = "", version: str = "",
                            search: str = "", kind: str = "", page: int = 1, limit: int = 50,
                            max_pages: int = 10) -> Iterator[AssetCatalogPage]:
        _positive(max_pages, "max_pages", MAX_PAGES)
        for _ in range(max_pages):
            result = self.asset_catalog(provider, locale=locale, version=version, search=search, kind=kind, page=page, limit=limit)
            yield result
            if not result.has_more:
                return
            if not result.list:
                raise PaginationError("catalog returned an empty page before its advertised total")
            page += 1
        raise PaginationError("asset catalog page budget exhausted; resume with the next page number")


class AccountGameClient(_AssetClient, _Snapshots):
    """Own game data, using the site's login JWT and backend-managed bindings."""
    def games(self, provider: str) -> List[Game]:
        provider = _provider(provider)
        result = self._call("GET", f"{self._prefix}/{provider}/games")
        return _decode(List[Game], result, "games") if provider == "mihoyo" else _games_wrapper(result)

    def hypergryph_games(self) -> AccountGames:
        return AccountGames.from_dict(self._call("GET", self._prefix + "/hypergryph/games"))

    def hypergryph_status(self) -> HypergryphStatus:
        """Read the binding's account.id for archive calls, including when disabled."""
        return HypergryphStatus.from_dict(self._call("GET", self._prefix + "/hypergryph"))

    def mihoyo_status(self) -> MihoyoStatus:
        return MihoyoStatus.from_dict(self._call("GET", self._prefix + "/mihoyo/status"))

    def snapshot(self, provider: str, game_id: str, *, role_id: Optional[Union[int, str]] = None,
                 sections: Optional[Sequence[str]] = None,
                 cursor: str = "", limit: int = 20, locale: str = "en") -> GameSnapshot:
        """role_id is a stored numeric miHoYo ID or a Hypergryph BoundRole.id.

        Omit role_id for public/account-only sections. The backend decides which
        owned/default role, if any, applies and enforces binding/site isolation.
        """
        return self._snapshot(self._prefix, provider, game_id, role_id, sections, cursor, limit, locale)

    def select_hypergryph_role(self, role_id: str) -> AccountGames:
        if not isinstance(role_id, str) or not role_id:
            raise ValueError("role_id must be a nonempty bound-role ID string")
        body = {"roleId": _text(role_id, "role_id", 128)}
        return AccountGames.from_dict(self._call("PUT", self._prefix + "/hypergryph/role", body=body))


class AdminGameClient(_AssetClient):
    """Administrator JWT; server enforces provider view and gameassets permissions."""
    _prefix = "/api/admin"

    def set_archive_policy(self, provider: str, account_id: int, game_id: str, path: str, *,
                           visibility: str, exportable: bool, revision: int) -> GameArchivePolicy:
        """Create with revision=0; update using this policy row's last revision.

        ApiError.code == 409 means the policy changed; read it again before
        deciding whether to write. No automatic retry can overwrite that change.
        """
        if visibility not in ("public", "shared", "owner", "secret"):
            raise ValueError("visibility must be public, shared, owner or secret")
        if type(exportable) is not bool:
            raise ValueError("exportable must be a boolean")
        body = {"game_id": _archive_game_id(game_id), "path": _policy_path(path), "visibility": visibility,
                "exportable": exportable, "revision": _uint64(revision, "revision")}
        result = GameArchivePolicy.from_dict(self._call("PUT", self._archive_path(provider, account_id) + "/policies", body=body))
        if result.game_id != game_id or result.path != path:
            raise TransportError("archive policy does not match the requested field")
        return result

    def hypergryph_games(self, account_id: Optional[int] = None) -> AccountGames:
        path = self._prefix + "/hypergryph"
        if account_id is not None:
            path += "/accounts/" + str(_positive(account_id, "account_id", 2**63 - 1))
        result = self._call("GET", path + "/games")
        return AccountGames.from_dict(result)

    def hypergryph_snapshot(self, account_id: int, game_id: str, *, role_id: Optional[str] = None,
                            sections: Optional[Sequence[str]] = None, cursor: str = "", limit: int = 20,
                            locale: str = "en") -> GameSnapshot:
        query = _snapshot_query("hypergryph", role_id, sections, cursor, limit, locale)
        path = self._prefix + "/hypergryph/accounts/" + str(_positive(account_id, "account_id", 2**63 - 1))
        result = GameSnapshot.from_dict(self._call("GET", path + "/games/" + _segment(game_id, "game_id") + "/snapshot", query))
        return _check_snapshot(result, game_id)

    def refresh_asset_provider(self, provider: str, *, locale: str = "", version: str = "") -> AssetCache:
        variant = _asset_variant(locale, version)
        path = self._prefix + "/game-assets/providers/" + _segment(provider, "asset provider") + "/refresh"
        return AssetCache.from_dict(self._call("POST", path, body=variant))

    def set_asset_disabled(self, provider: str, disabled: bool, *, locale: str = "", version: str = "") -> AssetCache:
        if type(disabled) is not bool:
            raise ValueError("disabled must be a boolean")
        variant = _asset_variant(locale, version)
        path = self._prefix + "/game-assets/providers/" + _segment(provider, "asset provider") + "/state"
        return AssetCache.from_dict(self._call("PUT", path, body={**variant, "disabled": disabled}))


class ServerGameClient(_Transport):
    """Existing server API: only the player's explicitly shared Arknights data."""
    def __init__(self, base: str, server_key: str, timeout: float = 10.0, *, opener=None):
        super().__init__(base, timeout, opener=opener, header="X-Server-Key", credential=server_key)

    def hypergryph_roles(self, uid: int) -> SharedHypergryphRoles:
        """uid is the site UID of a player with an approved role on this server."""
        result = self._call("GET", "/server/v1/hypergryph/roles", {"uid": _positive(uid, "site uid", 2**63 - 1)})
        return SharedHypergryphRoles.from_dict(result)


__all__ = [
    "PublicGameClient", "AccountGameClient", "AdminGameClient", "ServerGameClient",
    "Game", "GameCapability", "GameRole", "GameSnapshot", "GameSection", "GameEntity",
    "GameField", "GameSource", "GameIcon", "GamePolicy", "GameDiscovery", "AccountGames",
    "MihoyoRole", "MihoyoStatus", "AssetEndpoint", "AssetManifest", "AssetCache", "AssetProvider",
    "AssetCatalogPage", "SharedHypergryphRole", "SharedHypergryphRoles", "PaginationError",
    "HypergryphBinding", "HypergryphStatus", "GameArchiveMetadata", "GameArchiveEntry", "GameArchiveHistory",
    "GameArchivePolicy", "GameArchivePolicyAudit", "GameArchiveScope", "GameArchiveExport", "archive_field_path",
]
