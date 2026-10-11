import json

import httpx
import pytest

from nowork_bot_client import BotApiError, BotClient
from nowork_bot_client import BotTransportError


def make_client(handler):
    transport = httpx.MockTransport(handler)
    http = httpx.AsyncClient(transport=transport)
    return BotClient("https://nowork.example.com", "tok123", client=http)


@pytest.mark.asyncio
async def test_bind_sends_token_header_and_parses_data():
    seen = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen["request"] = request
        seen["body"] = json.loads(request.content)
        return httpx.Response(200, json={"code": 0, "data": {"userId": 1, "uid": 100001, "nickname": "Ada", "level": 2}})

    client = make_client(handler)
    result = await client.bind("qq", "10001", "ABC123", "Ada")
    assert result.nickname == "Ada"
    assert seen["request"].headers["authorization"] == "Bot tok123"
    assert seen["body"] == {"platform": "qq", "platformUserId": "10001", "platformName": "Ada", "code": "ABC123"}
    await client.aclose()


@pytest.mark.asyncio
@pytest.mark.parametrize("body,status", [(None, 200), ([], 200), ({"code": False, "data": {}}, 200),
                                          ({"code": 0}, 200), ({"code": 0, "data": {}}, 503)])
async def test_malformed_and_gateway_responses_are_transport_failures(body, status):
    client = make_client(lambda request: httpx.Response(status, json=body))
    try:
        with pytest.raises(BotTransportError):
            await client.lookup("qq", "1")
    finally:
        await client.aclose()


@pytest.mark.asyncio
async def test_network_timeout_is_classified():
    def handler(request):
        raise httpx.ReadTimeout("slow", request=request)
    client = make_client(handler)
    try:
        with pytest.raises(BotTransportError):
            await client.commands()
    finally:
        await client.aclose()


@pytest.mark.asyncio
async def test_lookup_sends_query_params_and_get():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.method == "GET"
        assert request.url.params["platform"] == "qq"
        assert request.url.params["platformUserId"] == "10001"
        return httpx.Response(200, json={"code": 0, "data": {"bound": True, "nickname": "Ada", "level": 2}})

    client = make_client(handler)
    result = await client.lookup("qq", "10001")
    assert result.bound is True
    await client.aclose()


@pytest.mark.asyncio
async def test_nonzero_code_raises_bot_api_error():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"code": 404, "message": "not bound"})

    client = make_client(handler)
    with pytest.raises(BotApiError) as exc:
        await client.unbind("qq", "x")
    assert exc.value.code == 404
    assert str(exc.value) == "not bound"
    await client.aclose()


@pytest.mark.asyncio
async def test_command_posts_chat_context():
    def handler(request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        assert body["chatId"] == "g1"
        return httpx.Response(200, json={"code": 0, "data": {"handled": True, "reply": [{"type": "text", "data": {"text": "ok"}}]}})

    client = make_client(handler)
    result = await client.command("qq", "1", "group", "g1", "/me")
    assert result.handled is True
    assert result.reply[0]["data"]["text"] == "ok"
    await client.aclose()


@pytest.mark.asyncio
async def test_poll_and_ack_events():
    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.path.endswith("/events"):
            return httpx.Response(
                200,
                json={"code": 0, "data": {"events": [{"type": "message", "id": 7, "botId": 1, "target": {"chatType": "private", "userId": "1"}, "message": [], "time": 0}]}},
            )
        body = json.loads(request.content)
        assert body == {"ids": [7]}
        return httpx.Response(200, json={"code": 0, "data": {"acked": 1}})

    client = make_client(handler)
    events = await client.poll_events()
    assert events[0].id == 7
    acked = await client.ack_events([7])
    assert acked == 1
    await client.aclose()


@pytest.mark.asyncio
@pytest.mark.parametrize("site_id", [None, 0, 42])
async def test_poll_preserves_optional_site_metadata(site_id):
    event = {"type": "message", "id": 7, "botId": 2, "target": {"chatType": "private", "userId": "member"}, "message": [], "time": 0}
    if site_id is not None:
        event["siteId"] = site_id

    def handler(request):
        assert request.headers["authorization"] == "Bot tok123"
        if request.url.path.endswith("/events"):
            return httpx.Response(200, json={"code": 0, "data": {"events": [event]}})
        assert json.loads(request.content) == {"ids": [7]}
        return httpx.Response(200, json={"code": 0, "data": {"acked": 1}})

    client = make_client(handler)
    try:
        events = await client.poll_events()
        assert events[0].site_id == site_id
        assert await client.ack_events([events[0].id]) == 1
    finally:
        await client.aclose()
