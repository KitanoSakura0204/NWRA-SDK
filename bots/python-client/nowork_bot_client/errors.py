"""Errors raised by the Bot API client."""


class BotApiError(Exception):
    """A non-zero `code` from the Bot API envelope, or a transport failure."""

    def __init__(self, code: int, message: str) -> None:
        super().__init__(message)
        self.code = code
        self.message = message


class BotTransportError(BotApiError):
    """Network, timeout, malformed envelope or HTTP failure (local code -1).
    Remains catchable by existing adapter BotApiError handlers."""

    def __init__(self, message: str) -> None:
        super().__init__(-1, message)
