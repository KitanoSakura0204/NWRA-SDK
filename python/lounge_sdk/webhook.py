from __future__ import annotations

import hashlib
import hmac
import json
import time
from dataclasses import dataclass
from typing import Any, Mapping, Optional

from .errors import LoungeError


class WebhookError(LoungeError):
    """Signature or timestamp verification failed."""


@dataclass
class WebhookEvent:
    id: str
    event: str
    timestamp: int
    data: dict


def webhook_signature(secret: str, ts: int, body: bytes) -> str:
    """sha256=HMAC_SHA256(secret, "<ts>.<body>")"""
    mac = hmac.new(secret.encode("utf-8"), str(ts).encode("ascii") + b"." + body, hashlib.sha256)
    return "sha256=" + mac.hexdigest()


def verify_webhook(secret: str, headers: Mapping[str, str], body: bytes, tolerance: Optional[int] = 300) -> WebhookEvent:
    """Verify X-Lounge-Signature / X-Lounge-Timestamp and parse the event.

    headers may be any case-insensitive mapping (Flask/Django request.headers work). tolerance is the
    replay window in seconds (None disables).
    """
    lookup = {k.lower(): v for k, v in headers.items()}
    try:
        ts = int(str(lookup.get("x-lounge-timestamp", "")).strip())
    except ValueError as e:
        raise WebhookError("missing or invalid X-Lounge-Timestamp") from e
    if tolerance is not None and abs(time.time() - ts) > tolerance:
        raise WebhookError("webhook timestamp outside tolerance")
    expected = webhook_signature(secret, ts, body)
    given = str(lookup.get("x-lounge-signature", "")).strip()
    if not hmac.compare_digest(expected, given):
        raise WebhookError("invalid webhook signature")
    try:
        payload: Any = json.loads(body.decode("utf-8"))
    except ValueError as e:
        raise WebhookError("invalid webhook body") from e
    if not isinstance(payload, dict):
        raise WebhookError("invalid webhook body")
    data = payload.get("data")
    return WebhookEvent(str(payload.get("id", "")), str(payload.get("event", "")), int(payload.get("timestamp", 0) or 0), data if isinstance(data, dict) else {})
