"""Errors raised by the Bot API client."""


class BotApiError(Exception):
    """A non-zero `code` from the Bot API envelope, or a transport failure."""

    def __init__(self, code: int, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
