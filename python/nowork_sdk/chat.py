"""Chat REST sessions and messages; server ticket/relay calls live on NoworkClient."""
from __future__ import annotations

from urllib.parse import urlencode

from .account import _JSONClient, _id


class ChatClient(_JSONClient):
    _prefix = "/api/chat"
    _scheme = "Chat"

    def __init__(self, base: str, token: str = "", timeout: float = 10.0, *, site_login=False, opener=None):
        super().__init__(base, token, timeout, opener=opener)
        if site_login:
            self._scheme = "Bearer"

    def redeem_ticket(self, ticket: str, server: str = "") -> dict:
        """Returns a session; create a new ChatClient with its token for subsequent requests."""
        return self._call("POST", "/session", {"ticket": ticket, "server": server})

    def site_session(self) -> dict:
        """Requires a client constructed with site_login=True and a site JWT."""
        return self._call("POST", "/session/site", {})

    def end_session(self) -> None:
        self._call("DELETE", "/session", allow_empty=True)

    def me(self) -> dict:
        return self._call("GET", "/me")

    def channels(self) -> list:
        return self._call("GET", "/channels")

    def history(self, channel_id: int, *, before: int = 0, after: int = 0, limit: int = 50) -> list:
        query = urlencode({"before": before, "after": after, "limit": limit})
        return self._call("GET", f"/channels/{_id(channel_id)}/messages?{query}")

    def send_message(self, channel_id: int, content: str, *, kind: str = "text", extra: str = "",
                     reply_to: int = 0, nonce: str = "") -> dict:
        """Reuse nonce when retrying one message; generate a new nonce for a new message."""
        return self._call("POST", f"/channels/{_id(channel_id)}/messages", {
            "content": content, "kind": kind, "extra": extra, "replyTo": reply_to, "nonce": nonce,
        })

    def mark_read(self, channel_id: int, message_id: int) -> None:
        self._call("POST", f"/channels/{_id(channel_id)}/read", {"id": _id(message_id)}, allow_empty=True)
