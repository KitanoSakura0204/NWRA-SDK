"""WebSocket notification stream for GET /api/bot/v1/events.

Protocol (Backend/internal/modules/bots/ws.go, hub.go): the server sends
{"type":"hello","botId","heartbeat"} then {"type":"message","id",...} per queued
notification; the client answers {"type":"ack","ids":[id]}, and a {"type":"ping"} is
answered with {"type":"pong"}. Clients also send JSON pings at hello.heartbeat seconds;
control ping frames do not refresh the server's application-message read deadline.
"""
from __future__ import annotations

import asyncio
import json
import logging
import math
from typing import Awaitable, Callable, List, Optional, Sequence
from urllib.parse import urlencode, urlsplit, urlunsplit

from .types import PluginEvent

log = logging.getLogger("nowork_bot_client.ws")

DEFAULT_BACKOFF = [1.0, 2.0, 5.0, 10.0, 30.0]
_DEFAULT_HEARTBEAT_SECONDS = 60.0
_MIN_HEARTBEAT_SECONDS = 5.0

OnEvent = Callable[[PluginEvent], Awaitable[None]]


def _heartbeat_seconds(value: object) -> float:
    if type(value) not in (int, float) or value <= 0:
        return _DEFAULT_HEARTBEAT_SECONDS
    if isinstance(value, float) and not math.isfinite(value):
        return _DEFAULT_HEARTBEAT_SECONDS
    return float(min(_DEFAULT_HEARTBEAT_SECONDS, max(_MIN_HEARTBEAT_SECONDS, value)))


async def _serve_connection(ws: object, on_event: OnEvent) -> None:
    interval = _DEFAULT_HEARTBEAT_SECONDS
    changed = asyncio.Event()

    def on_hello(value: object) -> None:
        nonlocal interval
        interval = _heartbeat_seconds(value)
        changed.set()

    async def receive() -> None:
        while True:
            raw = await ws.recv()  # type: ignore[attr-defined]
            await _handle_frame(ws, raw, on_event, on_hello=on_hello)

    async def heartbeat() -> None:
        while True:
            changed.clear()
            try:
                await asyncio.wait_for(changed.wait(), timeout=interval)
            except asyncio.TimeoutError:
                await ws.send(json.dumps({"type": "ping"}))  # type: ignore[attr-defined]

    tasks = [asyncio.create_task(receive()), asyncio.create_task(heartbeat())]
    try:
        done, _ = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
        for task in done:
            await task
    finally:
        for task in tasks:
            task.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)


def _ws_url(base_url: str, token: str) -> str:
    parts = urlsplit(base_url.rstrip("/") + "/api/bot/v1/events")
    scheme = "wss" if parts.scheme == "https" else "ws"
    return urlunsplit((scheme, parts.netloc, parts.path, urlencode({"token": token}), ""))


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
    if not backoff or any(delay <= 0 for delay in backoff):
        raise ValueError("backoff must contain positive delays")
    url = _ws_url(base_url, token)
    attempt = 0

    async def run_connection() -> None:
        nonlocal attempt
        connection = connector(url)
        # websockets.connect is itself the async context manager; test connectors
        # may instead be coroutines returning one. Do not await away its cleanup.
        context = connection if hasattr(connection, "__aenter__") else await connection
        async with context as ws:  # type: ignore[misc]
            attempt = 0
            await _serve_connection(ws, on_event)

    while not stop.is_set():
        stream = asyncio.create_task(run_connection())
        stopping = asyncio.create_task(stop.wait())
        try:
            done, _ = await asyncio.wait([stream, stopping], return_when=asyncio.FIRST_COMPLETED)
            if stopping in done:
                return
            await stream
        except asyncio.CancelledError:
            raise
        except Exception as exc:  # noqa: BLE001 - any transport error triggers a reconnect
            log.info("bot events stream dropped: %s", exc)
        finally:
            stream.cancel()
            stopping.cancel()
            await asyncio.gather(stream, stopping, return_exceptions=True)
        if stop.is_set():
            return
        delay = backoff[min(attempt, len(backoff) - 1)]
        attempt += 1
        try:
            await asyncio.wait_for(stop.wait(), timeout=delay)
        except asyncio.TimeoutError:
            pass


async def _handle_frame(ws: object, raw: str, on_event: OnEvent, *,
                        on_hello: Optional[Callable[[object], None]] = None) -> None:
    try:
        frame = json.loads(raw)
    except (ValueError, TypeError):
        return
    if not isinstance(frame, dict):
        return
    kind = frame.get("type")
    if kind == "hello":
        if on_hello is not None:
            on_hello(frame.get("heartbeat"))
        return
    if kind == "ping":
        await ws.send(json.dumps({"type": "pong"}))  # type: ignore[attr-defined]
        return
    if kind == "message":
        if type(frame.get("id")) is not int or frame["id"] <= 0:
            return
        ev = PluginEvent.from_data(frame)
        await on_event(ev)
        await ws.send(json.dumps({"type": "ack", "ids": [ev.id]}))  # type: ignore[attr-defined]
