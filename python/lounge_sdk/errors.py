class LoungeError(Exception):
    """Base error."""


class TransportError(LoungeError):
    """Network or decoding failure; the caller decides fail-open vs fail-closed."""


class ApiError(LoungeError):
    """Business error (code != 0)."""

    def __init__(self, code: int, message: str):
        super().__init__(f"api error {code}: {message}")
        self.code = code
        self.message = message


class OAuthError(LoungeError):
    """RFC 6749 error."""

    def __init__(self, error: str, description: str = "", status: int = 0):
        super().__init__(f"oauth error {error}: {description}")
        self.error = error
        self.description = description
        self.status = status
