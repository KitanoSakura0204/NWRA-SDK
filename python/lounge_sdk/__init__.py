"""Lounge Account SDK: game-server whitelist API and OIDC helpers (standard library only)."""
from .client import CheckResult, LoungeClient, Whitelist, WhitelistEntry
from .errors import ApiError, LoungeError, OAuthError, TransportError
from .oidc import LoungeOIDC
from .webhook import WebhookError, WebhookEvent, verify_webhook, webhook_signature

__all__ = [
    "LoungeClient",
    "Whitelist",
    "WhitelistEntry",
    "CheckResult",
    "LoungeError",
    "ApiError",
    "TransportError",
    "OAuthError",
    "LoungeOIDC",
    "WebhookEvent",
    "WebhookError",
    "verify_webhook",
    "webhook_signature",
]
__version__ = "1.1.0"
