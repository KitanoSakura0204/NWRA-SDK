import asyncio
import json

import pytest

from nowork_bot_client.ws import connect_events
from nowork_bot_client.ws import _handle_frame, _ws_url
from nowork_bot_client import ws as stream_module


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
async def test_failed_delivery_remains_unacknowledged():
    conn = FakeConnection([])
    async def fail(_event):
        raise RuntimeError("adapter offline")
    with pytest.raises(RuntimeError):
        await _handle_frame(conn, json.dumps({"type": "message", "id": 8}), fail)
    assert conn.sent == []


def test_ws_token_is_url_encoded():
    from urllib.parse import parse_qs, urlsplit
    url = _ws_url("https://account.example/prefix", "x&other=y")
    assert urlsplit(url).path == "/prefix/api/bot/v1/events"
    assert parse_qs(urlsplit(url).query) == {"token": ["x&other=y"]}


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
        await connect_events("https://nowork.example.com", "tok", on_event, connector=connector, stop=stop)

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
        await connect_events("https://nowork.example.com", "tok", on_event, connector=connector, backoff=[0.01, 0.01], stop=stop)

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


@pytest.mark.asyncio
@pytest.mark.parametrize("site_id", [None, 0, 42])
async def test_websocket_preserves_optional_site_metadata(site_id):
    frame = {"type": "message", "id": 7, "botId": 2, "target": {"chatType": "private", "userId": "member"}, "message": [], "time": 0}
    if site_id is not None:
        frame["siteId"] = site_id
    connection = FakeConnection([])
    received = []

    async def on_event(event):
        received.append(event)

    await _handle_frame(connection, json.dumps(frame), on_event)
    assert received[0].site_id == site_id
    assert json.loads(connection.sent[0]) == {"type": "ack", "ids": [7]}


def test_old_positional_plugin_event_constructor_is_unchanged():
    from nowork_bot_client import PluginEvent
    event = PluginEvent(7, 2, {"chatType": "private"}, [], 0, "notice")
    assert event.kind == "notice"
    assert event.site_id is None


@pytest.mark.parametrize("value,expected", [
    (1, 5), (20, 20), (10**500, 60), (0, 60), (-1, 60),
    (None, 60), (True, 60), ("1", 60), ([], 60),
    (float("nan"), 60), (float("inf"), 60),
])
def test_heartbeat_interval_is_bounded_and_invalid_values_use_default(value, expected):
    assert stream_module._heartbeat_seconds(value) == expected


class IdleConnection:
    """A connection that stays open until a queued frame, failure or cancellation."""

    def __init__(self, *, fail_ping=False):
        self.frames = asyncio.Queue()
        self.frames.put_nowait(json.dumps({"type": "hello", "heartbeat": 0.01}))
        self.outgoing = asyncio.Queue()
        self.sent = []
        self.closed = asyncio.Event()
        self.recv_cancelled = False
        self.fail_ping = fail_ping
        self.ping_failures = 0

    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        self.closed.set()
        return False

    async def recv(self):
        try:
            frame = await self.frames.get()
        except asyncio.CancelledError:
            self.recv_cancelled = True
            raise
        if isinstance(frame, Exception):
            raise frame
        return frame

    async def send(self, raw):
        assert not self.closed.is_set(), "old heartbeat wrote after connection cleanup"
        frame = json.loads(raw)
        if self.fail_ping and frame["type"] == "ping":
            self.ping_failures += 1
            raise ConnectionResetError("heartbeat transport failed")
        self.sent.append(frame)
        self.outgoing.put_nowait(frame)


async def next_frame(connection, kind):
    async def find():
        while True:
            frame = await connection.outgoing.get()
            if frame.get("type") == kind:
                return frame
    return await asyncio.wait_for(find(), timeout=1)


async def ignore_event(_event):
    pass


@pytest.mark.asyncio
async def test_idle_connection_sends_negotiated_json_ping_and_stop_cleans_blocked_receive(monkeypatch):
    # Scale the lower time bound only; the separate table tests the production bounds.
    monkeypatch.setattr(stream_module, "_MIN_HEARTBEAT_SECONDS", 0.01)
    connection = IdleConnection()
    attempts = []

    async def connector(url):
        attempts.append(url)
        return connection

    stop = asyncio.Event()
    task = asyncio.create_task(connect_events("https://account.example", "t", ignore_event, connector=connector, stop=stop))
    try:
        assert await next_frame(connection, "ping") == {"type": "ping"}
        stop.set()
        await asyncio.wait_for(task, timeout=1)
        assert connection.closed.is_set()
        assert connection.recv_cancelled
        sent = len(connection.sent)
        await asyncio.sleep(0.03)
        assert len(connection.sent) == sent
        assert len(attempts) == 1
    finally:
        stop.set()
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)


@pytest.mark.asyncio
async def test_reconnect_cleans_old_heartbeat_before_starting_the_next_connection(monkeypatch):
    monkeypatch.setattr(stream_module, "_MIN_HEARTBEAT_SECONDS", 0.01)
    first, second = IdleConnection(), IdleConnection()
    attempts = []

    async def connector(url):
        attempts.append(url)
        return first if len(attempts) == 1 else second

    stop = asyncio.Event()
    task = asyncio.create_task(connect_events("https://account.example", "t", ignore_event,
                                             connector=connector, backoff=[0.01], stop=stop))
    try:
        await next_frame(first, "ping")
        first.frames.put_nowait(ConnectionResetError("dropped"))
        await asyncio.wait_for(first.closed.wait(), timeout=1)
        sent = len(first.sent)
        await next_frame(second, "ping")
        await next_frame(second, "ping")
        assert len(first.sent) == sent
        assert len(attempts) == 2
    finally:
        stop.set()
        await asyncio.wait_for(task, timeout=1)
    assert second.closed.is_set()
    assert second.recv_cancelled


@pytest.mark.asyncio
async def test_failed_heartbeat_closes_before_reconnect_and_does_not_acknowledge_any_event(monkeypatch):
    monkeypatch.setattr(stream_module, "_MIN_HEARTBEAT_SECONDS", 0.01)
    first, second = IdleConnection(fail_ping=True), IdleConnection()
    attempts = []

    async def connector(url):
        attempts.append(url)
        return first if len(attempts) == 1 else second

    stop = asyncio.Event()
    task = asyncio.create_task(connect_events("https://account.example", "t", ignore_event,
                                             connector=connector, backoff=[0.01], stop=stop))
    try:
        await next_frame(second, "ping")
        assert first.closed.is_set()
        assert first.recv_cancelled
        assert first.ping_failures == 1
        assert first.sent == []
        assert len(attempts) == 2
    finally:
        stop.set()
        await asyncio.wait_for(task, timeout=1)


@pytest.mark.asyncio
@pytest.mark.parametrize("mode", ["stop", "cancel"])
async def test_stop_and_cancellation_clean_up_a_pending_connector(mode):
    entered, cleaned, stop = asyncio.Event(), asyncio.Event(), asyncio.Event()

    async def connector(_url):
        entered.set()
        try:
            await asyncio.Event().wait()
        finally:
            cleaned.set()

    task = asyncio.create_task(connect_events("https://account.example", "t", ignore_event, connector=connector, stop=stop))
    try:
        await asyncio.wait_for(entered.wait(), timeout=1)
        if mode == "stop":
            stop.set()
            await asyncio.wait_for(task, timeout=1)
        else:
            task.cancel()
            with pytest.raises(asyncio.CancelledError):
                await asyncio.wait_for(task, timeout=1)
        assert cleaned.is_set()
    finally:
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)


@pytest.mark.asyncio
async def test_heartbeat_continues_during_delivery_but_stop_never_acknowledges_that_pending_event(monkeypatch):
    monkeypatch.setattr(stream_module, "_MIN_HEARTBEAT_SECONDS", 0.01)
    connection = IdleConnection()
    connection.frames.put_nowait(json.dumps({"type": "message", "id": 42}))
    started, cancelled, stop = asyncio.Event(), asyncio.Event(), asyncio.Event()

    async def connector(_url):
        return connection

    async def on_event(_event):
        started.set()
        try:
            await asyncio.Event().wait()
        finally:
            cancelled.set()

    task = asyncio.create_task(connect_events("https://account.example", "t", on_event, connector=connector, stop=stop))
    try:
        await asyncio.wait_for(started.wait(), timeout=1)
        await next_frame(connection, "ping")
        stop.set()
        await asyncio.wait_for(task, timeout=1)
        assert cancelled.is_set()
        assert connection.closed.is_set()
        assert not any(frame["type"] == "ack" for frame in connection.sent)
    finally:
        task.cancel()
        await asyncio.gather(task, return_exceptions=True)
