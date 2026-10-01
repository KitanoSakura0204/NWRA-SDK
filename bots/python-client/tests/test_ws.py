import asyncio
import json

import pytest

from lounge_bot_client.ws import connect_events


class FakeConnection:
    """Stands in for a `websockets` connection (async context manager with recv/send)."""

    def __init__(self, frames):
        self._frames = list(frames)
        self.sent = []

    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        return False

    async def recv(self):
        if not self._frames:
            raise ConnectionResetError("no more frames")
        return self._frames.pop(0)

    async def send(self, data):
        self.sent.append(data)


@pytest.mark.asyncio
async def test_acks_a_delivered_message_and_answers_ping():
    frames = [
        json.dumps({"type": "hello", "botId": 1, "heartbeat": 60}),
        json.dumps({"type": "message", "id": 42, "botId": 1, "target": {"chatType": "private", "userId": "1"}, "message": [], "time": 0}),
        json.dumps({"type": "ping"}),
    ]
    conn = FakeConnection(frames)
    received = []

    async def connector(url):
        assert "token=tok" in url
        return conn

    stop = asyncio.Event()

    async def on_event(ev):
        received.append(ev)

    async def runner():
        await connect_events("https://lounge.example.com", "tok", on_event, connector=connector, stop=stop)

    task = asyncio.create_task(runner())
    for _ in range(20):
        if len(conn.sent) >= 2:
            break
        await asyncio.sleep(0.01)
    stop.set()
    task.cancel()
    try:
        await task
    except asyncio.CancelledError:
        pass

    assert len(received) == 1
    assert received[0].id == 42
    assert json.loads(conn.sent[0]) == {"type": "ack", "ids": [42]}
    assert json.loads(conn.sent[1]) == {"type": "pong"}


@pytest.mark.asyncio
async def test_reconnects_with_backoff_after_drop():
    attempts = []

    class DropConnection(FakeConnection):
        async def recv(self):
            raise ConnectionResetError("dropped")

    async def connector(url):
        attempts.append(url)
        return DropConnection([])

    stop = asyncio.Event()

    async def on_event(_ev):
        pass

    async def runner():
        await connect_events("https://lounge.example.com", "tok", on_event, connector=connector, backoff=[0.01, 0.01], stop=stop)

    task = asyncio.create_task(runner())
    for _ in range(50):
        if len(attempts) >= 2:
            break
        await asyncio.sleep(0.01)
    stop.set()
    task.cancel()
    try:
        await task
    except asyncio.CancelledError:
        pass

    assert len(attempts) >= 2
