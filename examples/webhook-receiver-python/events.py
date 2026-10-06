"""Business logic: the events this receiver understands. Field names are exactly what the user
center emits (docs/PROTOCOL.md section 5); anything else stays in ev.data."""
from __future__ import annotations

import json
import logging
import time
from typing import TYPE_CHECKING, Any, Mapping, Optional

if TYPE_CHECKING:
    from nowork_sdk import WebhookEvent

log = logging.getLogger("nowork.events")

# Review results carried by game_role.reviewed -> data.status.
ROLE_PENDING, ROLE_APPROVED, ROLE_REJECTED, ROLE_BANNED = 0, 1, 2, 3


def who(uid: Any, user: Optional[Mapping[str, Any]]) -> str:
    """Names the account in logs, whichever identifier the endpoint receives. An endpoint bound to
    a delivery profile with subject "pairwise" gets no userId / uid / username / ip / externalId /
    externalName / email / phone; data.user.sub identifies the account instead (DELIVERY.md
    section 2)."""
    sub = user.get("sub") if isinstance(user, Mapping) else None
    return "sub=%s" % sub if sub else "uid=%s" % (uid or 0)


def _iso(ts: int) -> str:
    try:
        return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(ts))
    except (OverflowError, OSError, ValueError):
        return str(ts)


def dispatch(ev: "WebhookEvent") -> None:
    """Keep this short (the sender waits at most 10 s); queue anything heavier. Raise to answer 500
    and have the delivery retried."""
    at, d = _iso(ev.timestamp), ev.data
    if ev.event == "game_role.reviewed":
        role = 'role="%s" uuid=%s user=%s server=%s reviewer=%s' % (
            d.get("roleName"), d.get("roleUuid"), d.get("userId"), d.get("serverId"), d.get("reviewerId"))
        status = d.get("status")
        if status == ROLE_APPROVED:
            log.info("[%s] role APPROVED: %s", at, role)
            # e.g. add the player to the server whitelist / grant a rank
        elif status == ROLE_REJECTED:
            log.info("[%s] role REJECTED: %s", at, role)
        elif status == ROLE_BANNED:
            log.info("[%s] role BANNED: %s", at, role)
            # e.g. kick the player if online
        elif status == ROLE_PENDING:
            log.info("[%s] role back to PENDING: %s", at, role)
        else:
            log.info("[%s] role status %s (unknown): %s", at, status, role)
    elif ev.event == "user.deleted":
        log.info('[%s] user DELETED: userId=%s uid=%s username="%s" reason="%s"',
                 at, d.get("userId"), d.get("uid"), d.get("username"), d.get("reason"))
        # e.g. purge local data tied to this account
    elif ev.event == "server.offline":
        log.info('[%s] server OFFLINE: code=%s name="%s" id=%s lastHeartbeat=%s',
                 at, d.get("code"), d.get("name"), d.get("serverId"), d.get("lastHeartbeat"))
        # e.g. page the on-call channel
    elif ev.event == "user.level_changed":
        log.info("[%s] trust level %s -> %s: %s", at, d.get("from"), d.get("to"), who(d.get("uid"), d.get("user")))
        # e.g. re-check a rank that requires a minimum trust level
    elif ev.event in ("connection.bound", "connection.unbound"):
        log.info('[%s] %s provider=%s external="%s": %s', at, ev.event, d.get("provider"),
                 d.get("externalId", ""), who(d.get("uid"), d.get("user")))
        # e.g. grant or revoke a Discord role (see ../discord-role-sync)
    elif ev.event == "ping":
        # sent by the "测试" (Test) button in the admin console
        log.info("[%s] ping from console: %s", at, json.dumps(d, ensure_ascii=False))
    else:
        log.info("[%s] event %s ignored (id=%s)", at, ev.event, ev.id)
