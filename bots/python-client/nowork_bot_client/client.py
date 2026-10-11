"""Async HTTP client for the NoWork Bot API (/api/bot/v1)."""
from __future__ import annotations

import asyncio
import math
from typing import Any, Dict, List, Optional

import httpx

from .errors import BotApiError, BotTransportError
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
        raise BotTransportError(f"bot api: non-JSON response (HTTP {resp.status_code})") from exc
    if not isinstance(env, dict) or type(env.get("code")) is not int:
        raise BotTransportError(f"bot api: invalid envelope (HTTP {resp.status_code})")
    code = env["code"]
    if code != 0:
        raise BotApiError(code, env.get("message") or f"bot api error {code}")
    if not resp.is_success or not isinstance(env.get("data"), dict):
        raise BotTransportError(f"bot api: invalid data or HTTP failure ({resp.status_code})")
    return env["data"]


class BotClient:
    """Async client with a configurable total request deadline (10 seconds by default).

    Contract read from the landed backend (Backend/internal/modules/bots), 2026-09-29; see
    SDK/docs/BOTS_SDK.md for the full field table.
    """

    def __init__(self, base_url: str, token: str, *, lang: Optional[str] = None,
                 client: Optional[httpx.AsyncClient] = None, timeout: float = 10.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.token = token
        self._lang = lang
        if not math.isfinite(timeout) or timeout <= 0:
            raise ValueError("timeout must be positive")
        self.timeout = timeout
        self._client = client or httpx.AsyncClient(timeout=timeout, follow_redirects=False)

    async def _request(self, method: str, url: str, **kwargs: Any) -> httpx.Response:
        try:
            return await asyncio.wait_for(
                self._client.request(method, url, timeout=self.timeout, follow_redirects=False, **kwargs),
                timeout=self.timeout,
            )
        except (httpx.HTTPError, asyncio.TimeoutError, OSError) as exc:
            raise BotTransportError("bot api: request failed or timed out") from exc

    def _headers(self) -> Dict[str, str]:
        h = {"Authorization": f"Bot {self.token}"}
        if self._lang:
            h["Accept-Language"] = self._lang
        return h

    async def aclose(self) -> None:
        await self._client.aclose()

    async def bind(self, platform: str, platform_user_id: str, code: str, platform_name: str = "") -> BindResult:
        resp = await self._request("POST",
            f"{self.base_url}/api/bot/v1/bind",
            json={"platform": platform, "platformUserId": platform_user_id, "platformName": platform_name, "code": code},
            headers=self._headers(),
        )
        return BindResult.from_data(_unwrap(resp))

    async def unbind(self, platform: str, platform_user_id: str) -> UnbindResult:
        resp = await self._request("POST",
            f"{self.base_url}/api/bot/v1/unbind",
            json={"platform": platform, "platformUserId": platform_user_id},
            headers=self._headers(),
        )
        return UnbindResult.from_data(_unwrap(resp))

    async def lookup(self, platform: str, platform_user_id: str) -> LookupResult:
        resp = await self._request("GET",
            f"{self.base_url}/api/bot/v1/lookup",
            params={"platform": platform, "platformUserId": platform_user_id},
            headers=self._headers(),
        )
        return LookupResult.from_data(_unwrap(resp))

    async def command(self, platform: str, platform_user_id: str, chat_type: str, chat_id: str, text: str, platform_name: str = "") -> CommandResult:
        resp = await self._request("POST",
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
        resp = await self._request("GET", f"{self.base_url}/api/bot/v1/commands", headers=self._headers())
        return CommandsResult.from_data(_unwrap(resp))

    async def poll_events(self) -> List[PluginEvent]:
        resp = await self._request("GET", f"{self.base_url}/api/bot/v1/events", headers=self._headers())
        data = _unwrap(resp) or {}
        return [PluginEvent.from_data(e) for e in data.get("events") or []]

    async def ack_events(self, ids: List[int]) -> int:
        resp = await self._request("POST", f"{self.base_url}/api/bot/v1/events/ack", json={"ids": ids}, headers=self._headers())
        data = _unwrap(resp) or {}
        return int(data.get("acked", 0))
