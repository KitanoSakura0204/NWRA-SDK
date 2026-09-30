"""Lounge Account SDK: game-server whitelist API and OIDC helpers (standard library only)."""
from .client import HEADER_ENCRYPTION, CheckResult, LoungeClient, OnlinePlayer, RawResponse, Whitelist, WhitelistEntry
from .errors import ApiError, EncryptionError, LoungeError, OAuthError, TransportError
from .gm import GMAction, GMActionParam, GMOption, GMPublishResult
from .oidc import LoungeOIDC
from .serverusers import CODE_ADMIN_BAN, CODE_NOT_ON_ROSTER, CODE_VERSION_CONFLICT, KickEntry, ServerBan, UserData, UserField, UserSchema, is_version_conflict
from .webhook import WebhookError, WebhookEvent, verify_webhook, webhook_signature

__all__ = [
    "LoungeClient",
    "Whitelist",
    "WhitelistEntry",
    "CheckResult",
    "OnlinePlayer",
    "RawResponse",
    "HEADER_ENCRYPTION",
    "GMAction",
    "GMActionParam",
    "GMOption",
    "GMPublishResult",
    "LoungeError",
    "ApiError",
    "EncryptionError",
    "TransportError",
    "OAuthError",
    "LoungeOIDC",
    "UserSchema",
    "UserField",
    "UserData",
    "ServerBan",
    "KickEntry",
    "is_version_conflict",
    "CODE_NOT_ON_ROSTER",
    "CODE_VERSION_CONFLICT",
    "CODE_ADMIN_BAN",
    "WebhookEvent",
    "WebhookError",
    "verify_webhook",
    "webhook_signature",
]
__version__ = "1.1.0"
