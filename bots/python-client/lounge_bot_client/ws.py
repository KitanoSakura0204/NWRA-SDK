"""WebSocket notification stream for GET /api/bot/v1/events.

Protocol (Backend/internal/modules/bots/ws.go, hub.go): the server sends
{"type":"hello","botId","heartbeat"} then {"type":"message","id",...} per queued
notification; the client answers {"type":"ack","ids":[id]}, and a {"type":"ping"} is
answered with {"type":"pong"}.
"""
from __future__ import annotations

import asyncio
import json
import logging
from typing import Awaitable, Callable, List, Optional, Sequence
from urllib.parse import urlsplit, urlunsplit

from .types import PluginEvent

log = logging.getLogger("lounge_bot_client.ws")

DEFAULT_BACKOFF = [1.0, 2.0, 5.0, 10.0, 30.0]

OnEvent = Callable[[PluginEvent], Awaitable[None]]


def _ws_url(base_url: str, token: str) -> str:
    parts = urlsplit(base_url.rstrip("/") + "/api/bot/v1/events")
    scheme = "wss" if parts.scheme == "https" else "ws"
    return urlunsplit((scheme, parts.netloc, parts.path, f"token={token}", ""))


async def connect_events(
    base_url: str,
    token: str,
    on_event: OnEvent,
    *,
    connector: Optional[Callable[[str], Awaitable[object]]] = None,
    backoff: Sequence[float] = DEFAULT_BACKOFF,
    stop: Optional[asyncio.Event] = None,
) -> None:
    """Runs until `stop` is set (or forever). `connector` defaults to `websockets.connect`;
    tests inject a fake that yields a fake connection (async context manager with
    `send`/`recv`, as `websockets` connections provide).
    """
    if connector is None:
        import websockets

        connector = websockets.connect
    stop = stop or asyncio.Event()
    url = _ws_url(base_url, token)
    attempt = 0
    while not stop.is_set():
        try:
            async with await connector(url) as ws:  # type: ignore[misc]
                attempt = 0
                while not stop.is_set():
                    raw = await ws.recv()
                    await _handle_frame(ws, raw, on_event)
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001 - any transport error triggers a reconnect
            log.info("bot events stream dropped: %s", exc)
        if stop.is_set():
            return
        delay = backoff[min(attempt, len(backoff) - 1)]
        attempt += 1
        await asyncio.sleep(delay)


async def _handle_frame(ws: object, raw: str, on_event: OnEvent) -> None:
    try:
        frame = json.loads(raw)
    except (ValueError, TypeError):
        return
    kind = frame.get("type")
    if kind == "hello":
        return
    if kind == "ping":
        await ws.send(json.dumps({"type": "pong"}))  # type: ignore[attr-defined]
        return
    if kind == "message":
        ev = PluginEvent.from_data(frame)
        await on_event(ev)
        await ws.send(json.dumps({"type": "ack", "ids": [ev.id]}))  # type: ignore[attr-defined]
