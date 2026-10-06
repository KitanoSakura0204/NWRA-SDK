"""The webhook endpoint as a plain function, independent of the HTTP server so tests can call it:

- verifies X-NoWork-Signature / X-NoWork-Timestamp over the raw body (Python SDK)
- opens encrypted deliveries (JWE, SDK/docs/DELIVERY.md section 6)
- de-duplicates deliveries by event id (in-memory LRU)

Status codes matter: anything outside 2xx (or a timeout) makes the user center retry with
back-off, so bad signatures get 401 and handler failures get 500.
"""
from __future__ import annotations

import json
import logging
import os
import sys
import threading
from collections import OrderedDict
from typing import Any, Callable, Dict, List, Mapping, Optional, Tuple

# The SDK source in this repo (SDK/python). Copied out of the repo, `pip install` the SDK instead.
_SDK = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "python"))
if os.path.isdir(os.path.join(_SDK, "nowork_sdk")):
    sys.path.insert(0, _SDK)

from nowork_sdk import WebhookError, WebhookEvent, verify_webhook  # noqa: E402
from nowork_sdk.delivery import DecryptError, open_webhook  # noqa: E402

log = logging.getLogger("nowork.webhook")

MAX_BODY = 1 << 20  # 1 MiB, same cap as the SDKs

Response = Tuple[int, Dict[str, Any]]


class SeenSet:
    """Bounded set of recently handled event ids (LRU eviction).

    The user center retries a failed delivery for roughly nine hours (30s, 2m, 10m, 30m, 2h, 6h),
    so a few thousand ids are enough for one endpoint. With several receiver instances behind one
    URL use a shared store keyed by id instead (Redis SET NX EX, a unique DB column).
    """

    def __init__(self, capacity: int = 10000) -> None:
        self._cap = max(1, capacity)
        self._ids: "OrderedDict[str, None]" = OrderedDict()
        self._lock = threading.Lock()

    def __contains__(self, event_id: object) -> bool:
        with self._lock:
            if event_id not in self._ids:
                return False
            self._ids.move_to_end(str(event_id))
            return True

    def add(self, event_id: str) -> None:
        with self._lock:
            self._ids[event_id] = None
            self._ids.move_to_end(event_id)
            while len(self._ids) > self._cap:
                self._ids.popitem(last=False)


def load_keys(paths: str) -> List[Dict[str, str]]:
    """Read private JWKs from a comma-separated list of files. During a key rotation list the new
    and the old key: the JWE header's kid picks the right one (DELIVERY.md section 7)."""
    keys = []
    for p in (s.strip() for s in paths.split(",")):
        if not p:
            continue
        try:
            with open(p, encoding="utf-8") as f:
                jwk = json.load(f)
        except (OSError, ValueError) as exc:
            raise ValueError("%s: %s" % (p, exc)) from exc
        if not isinstance(jwk, dict) or not isinstance(jwk.get("d"), str):
            raise ValueError('%s: not a private JWK (no "d")' % p)
        keys.append(jwk)
    return keys


class _OpenError(Exception):
    """Signed by the user center but not readable here (key rotation or configuration mismatch)."""


def _has_jwe(body: bytes) -> bool:
    """Detects the encrypted shape even when a proxy dropped the X-NoWork-Encryption header."""
    try:
        doc = json.loads(body)
    except ValueError:
        return False
    return isinstance(doc, dict) and isinstance(doc.get("jwe"), str)


class Receiver:
    """handle(method, headers, body) -> (status, JSON body); usable from any HTTP framework."""

    def __init__(self, secret: str, dispatch: Callable[[WebhookEvent], None], tolerance: int = 300,
                 keys: Optional[List[Dict[str, str]]] = None, capacity: int = 10000) -> None:
        self.secret = secret
        self.dispatch = dispatch
        self.tolerance = tolerance  # seconds; 0 turns the replay check off
        self.keys = keys or []  # empty: the endpoint is not encrypted
        self.seen = SeenSet(capacity)

    def _open(self, headers: Mapping[str, str], body: bytes, ev: WebhookEvent) -> WebhookEvent:
        """A plain delivery is returned as verified; an encrypted one is decrypted (open_webhook also
        checks that the inner id, event and timestamp repeat the signed outer ones and that the
        header ctx is "webhook:<endpoint id>")."""
        encrypted = str(headers.get("x-nowork-encryption", "")).upper() == "JWE" or _has_jwe(body)
        if not encrypted:
            # Refuse plaintext from an endpoint that should encrypt rather than silently accept it.
            if self.keys:
                raise _OpenError("decryption keys configured but the delivery is not encrypted")
            return ev
        if not self.keys:
            raise _OpenError("encrypted delivery but no decryption key configured")
        try:
            doc = open_webhook(body, self.keys)
        except (DecryptError, ValueError) as exc:
            raise _OpenError(str(exc) or "cannot decrypt the JWE") from exc
        data = doc.get("data")
        return WebhookEvent(str(doc.get("id", "")), str(doc.get("event", "")),
                            int(doc.get("timestamp", 0) or 0), data if isinstance(data, dict) else {})

    def handle(self, method: str, headers: Mapping[str, str], body: bytes) -> Response:
        if method != "POST":
            return 405, {"error": "method not allowed"}
        lower = {k.lower(): v for k, v in headers.items()}
        delivery = lower.get("x-nowork-delivery", "")

        # Verify against the raw bytes: the signature covers "<timestamp>.<body>" exactly as sent,
        # and an encrypted body is opened only after it verified.
        try:
            ev = self._open(lower, body, verify_webhook(self.secret, lower, body, self.tolerance))
        except _OpenError as exc:
            # 500 keeps the delivery queued: it succeeds once the right key is in place.
            log.warning("delivery=%s cannot open: %s", delivery, exc)
            return 500, {"error": "cannot decrypt"}
        except WebhookError as exc:
            log.warning("delivery=%s rejected: %s", delivery, exc)
            # a malformed body will not get better with retries -> 400; signature / timestamp -> 401
            return (400 if "body" in str(exc) else 401), {"error": str(exc)}
        except (TypeError, ValueError) as exc:  # e.g. a non-numeric "timestamp" in a signed body
            log.warning("delivery=%s rejected: %s", delivery, exc)
            return 400, {"error": "invalid webhook body"}
        if not ev.id:
            return 400, {"error": "missing event id"}

        # Idempotency: the same id may arrive again (retry after a timeout, manual redelivery from
        # the console). Acknowledge without running the handler twice.
        if ev.id in self.seen:
            log.info("delivery=%s event=%s id=%s duplicate, ignored", delivery, ev.event, ev.id)
            return 200, {"status": "duplicate"}
        try:
            self.dispatch(ev)
        except Exception:  # noqa: BLE001 - any handler failure means "retry later"
            log.exception("delivery=%s event=%s id=%s handler failed", delivery, ev.event, ev.id)
            return 500, {"error": "handler failed"}
        self.seen.add(ev.id)
        return 200, {"status": "ok"}
