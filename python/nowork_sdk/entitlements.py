"""Custom per-app values (module "delivery", SDK/docs/DELIVERY.md section 8).

A game server reads and writes them through :class:`~nowork_sdk.NoworkClient`
(X-Server-Key) on ``/server/v1/delivery/entitlements``; a confidential OAuth client through
:class:`~nowork_sdk.NoworkOIDC` (HTTP Basic) on ``/oauth2/delivery/entitlements``. The caller
must be bound to a delivery profile, and the user is named by ``uid`` or by the ``sub`` that
caller sees. A pairwise profile accepts ``sub`` only: a ``uid`` would link its pseudonym back
to the account.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Optional


@dataclass
class EntitlementValues:
    """GET reply: the sub this caller sees plus every key stored for the user (raw JSON values)."""

    sub: str
    entitlements: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "EntitlementValues":
        stored = d.get("entitlements")
        return cls(str(d.get("sub", "") or ""), dict(stored) if isinstance(stored, dict) else {})


@dataclass
class Entitlement:
    """PUT / DELETE reply for a single key."""

    sub: str
    key: str = ""
    value: Any = None

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "Entitlement":
        return cls(str(d.get("sub", "") or ""), str(d.get("key", "") or ""), d.get("value"))


def entitlement_query(uid: int = 0, sub: str = "") -> Dict[str, Any]:
    """Name the user by ``sub`` first (the only form a pairwise profile accepts), then by ``uid``.

    An empty sub and a non-positive uid are refused before any request is sent.
    """
    sub = (sub or "").strip()
    if sub:
        return {"sub": sub}
    if isinstance(uid, bool) or not isinstance(uid, int) or uid <= 0:
        raise ValueError("provide a uid or a sub")
    return {"uid": uid}


def entitlement_body(uid: int, sub: str, key: str, value: Any) -> Dict[str, Any]:
    """The PUT body. ``value`` may be any JSON value, including ``None``."""
    key = (key or "").strip()
    if not key:
        raise ValueError("entitlement key is required")
    body: Dict[str, Any] = {"key": key, "value": value}
    sub = (sub or "").strip()
    if sub:
        body["sub"] = sub
        return body
    if isinstance(uid, bool) or not isinstance(uid, int) or uid <= 0:
        raise ValueError("provide a uid or a sub")
    body["uid"] = uid
    return body


__all__ = ["Entitlement", "EntitlementValues", "entitlement_body", "entitlement_query"]
