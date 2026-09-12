"""Lounge Account SDK: game-server whitelist API and OIDC helpers (standard library only)."""
from .client import CheckResult, LoungeClient, Whitelist, WhitelistEntry
from .errors import ApiError, LoungeError, OAuthError, TransportError
from .oidc import LoungeOIDC

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
]
__version__ = "1.0.0"
