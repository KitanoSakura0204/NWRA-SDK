"""Synchronous and async HTTP client for the NoWork Bot API (/api/bot/v1)."""
from __future__ import annotations

from typing import Any, Dict, List, Optional

import httpx

from .errors import BotApiError
from .types import (
    BindResult,
    CommandResult,
    CommandsResult,
    LookupResult,
    Message,
    PluginEvent,
    UnbindResult,
)


def _unwrap(resp: httpx.Response) -> Any:
    try:
        env = resp.json()
    except ValueError as exc:
        raise BotApiError(resp.status_code, f"bot api: non-JSON response (HTTP {resp.status_code})") from exc
    code = env.get("code", -1)
    if code != 0:
        raise BotApiError(code, env.get("message") or f"bot api error {code}")
    return env.get("data")


class BotClient:
    """Async client. Use `BotClient.sync(...)` for a blocking variant (same methods, no await).

    Contract read from the landed backend (Backend/internal/modules/bots), 2026-09-29; see
    SDK/docs/BOTS_SDK.md for the full field table.
    """

    def __init__(self, base_url: str, token: str, *, lang: Optional[str] = None, client: Optional[httpx.AsyncClient] = None) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token
        self._lang = lang
        self._client = client or httpx.AsyncClient()

    def _headers(self) -> Dict[str, str]:
        h = {"Authorization": f"Bot {self.token}"}
        if self._lang:
            h["Accept-Language"] = self._lang
        return h

    async def aclose(self) -> None:
        await self._client.aclose()

    async def bind(self, platform: str, platform_user_id: str, code: str, platform_name: str = "") -> BindResult:
        resp = await self._client.post(
            f"{self.base_url}/api/bot/v1/bind",
            json={"platform": platform, "platformUserId": platform_user_id, "platformName": platform_name, "code": code},
            headers=self._headers(),
        )
        return BindResult.from_data(_unwrap(resp))

    async def unbind(self, platform: str, platform_user_id: str) -> UnbindResult:
        resp = await self._client.post(
            f"{self.base_url}/api/bot/v1/unbind",
            json={"platform": platform, "platformUserId": platform_user_id},
            headers=self._headers(),
        )
        return UnbindResult.from_data(_unwrap(resp))

    async def lookup(self, platform: str, platform_user_id: str) -> LookupResult:
        resp = await self._client.get(
            f"{self.base_url}/api/bot/v1/lookup",
            params={"platform": platform, "platformUserId": platform_user_id},
            headers=self._headers(),
        )
        return LookupResult.from_data(_unwrap(resp))

    async def command(self, platform: str, platform_user_id: str, chat_type: str, chat_id: str, text: str, platform_name: str = "") -> CommandResult:
        resp = await self._client.post(
            f"{self.base_url}/api/bot/v1/command",
            json={
                "platform": platform,
                "platformUserId": platform_user_id,
                "platformName": platform_name,
                "chatType": chat_type,
                "chatId": chat_id,
                "text": text,
            },
            headers=self._headers(),
        )
        return CommandResult.from_data(_unwrap(resp))

    async def commands(self) -> CommandsResult:
        resp = await self._client.get(f"{self.base_url}/api/bot/v1/commands", headers=self._headers())
        return CommandsResult.from_data(_unwrap(resp))

    async def poll_events(self) -> List[PluginEvent]:
        resp = await self._client.get(f"{self.base_url}/api/bot/v1/events", headers=self._headers())
        data = _unwrap(resp) or {}
        return [PluginEvent.from_data(e) for e in data.get("events") or []]

    async def ack_events(self, ids: List[int]) -> int:
        resp = await self._client.post(f"{self.base_url}/api/bot/v1/events/ack", json={"ids": ids}, headers=self._headers())
        data = _unwrap(resp) or {}
        return int(data.get("acked", 0))
