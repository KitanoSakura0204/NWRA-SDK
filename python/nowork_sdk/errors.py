class NoWorkError(Exception):
    """Base error."""


class TransportError(NoWorkError):
    """Network or decoding failure; the caller decides fail-open vs fail-closed."""


class EncryptionError(NoWorkError):
    """An encrypted Server API reply could not be used (no keys, decryption/context/data failure),
    or a whitelist reply was plaintext while decryption keys were configured.

    Not a TransportError on purpose: the data was never read, so never fail open on it (an
    unreadable whitelist is not a disabled whitelist).
    """

    def __init__(self, route: str, reason: str):
        super().__init__(f"encrypted response on {route}: {reason}")
        self.route = route
        self.reason = reason


class ApiError(NoWorkError):
    """Business error (code != 0)."""

    def __init__(self, code: int, message: str):
        super().__init__(f"api error {code}: {message}")
        self.code = code
        self.message = message


class OAuthError(NoWorkError):
    """RFC 6749 error."""

    def __init__(self, error: str, description: str = "", status: int = 0):
        super().__init__(f"oauth error {error}: {description}")
        self.error = error
        self.description = description
        self.status = status
