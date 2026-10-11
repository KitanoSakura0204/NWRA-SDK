"""Verify framework-mode HTTP pushes before delivering or acknowledging them."""
from __future__ import annotations

import hashlib
import hmac
import json
import math
import re
import time
from typing import Mapping

from .types import PluginEvent


class BotPushError(ValueError):
    pass


def verify_push(secret: str, headers: Mapping[str, str], body: bytes, *,
                tolerance: float = 300, now: float | None = None) -> PluginEvent:
    """Check the exact bytes, timestamp, signature and delivery id. Deduplicate the
    returned event.id in durable storage; return HTTP 2xx only after successful delivery.
    A bot push is NOT the site's general WebhookEvent envelope."""
    h = {k.lower(): v for k, v in headers.items()}
    stamp = h.get("x-nowork-timestamp", "")
    signature = h.get("x-nowork-signature", "")
    if not secret or not re.fullmatch(r"[0-9]{1,12}", stamp) or not re.fullmatch(r"sha256=[0-9a-f]{64}", signature):
        raise BotPushError("missing or invalid push authentication")
    current = time.time() if now is None else now
    if not math.isfinite(current) or not math.isfinite(tolerance) or tolerance <= 0 or abs(current - int(stamp)) > tolerance:
        raise BotPushError("push timestamp outside allowed window")
    expected = "sha256=" + hmac.new(secret.encode(), stamp.encode() + b"." + body, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(expected, signature):
        raise BotPushError("push signature mismatch")
    try:
        data = json.loads(body)
    except (ValueError, UnicodeDecodeError) as exc:
        raise BotPushError("invalid push JSON") from exc
    if (not isinstance(data, dict) or data.get("type") != "message" or
            type(data.get("id")) is not int or data["id"] <= 0 or
            h.get("x-nowork-delivery") != str(data["id"]) or h.get("x-nowork-event") != "bot.message" or
            not isinstance(data.get("target"), dict) or not isinstance(data.get("message"), list)):
        raise BotPushError("invalid push event or delivery id")
    return PluginEvent.from_data(data)
