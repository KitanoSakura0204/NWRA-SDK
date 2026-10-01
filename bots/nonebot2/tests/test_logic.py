"""Tests the plugin's bind/unbind/me/passthrough behaviour against a fake BotClient (no
network, no running NoneBot app)."""
from __future__ import annotations

from typing import Any, Dict, List, Optional

import pytest
from lounge_bot_client import BindResult, BotApiError, CommandResult, LookupResult, UnbindResult

from nonebot_plugin_lounge import logic


class FakeClient:
    def __init__(self) -> None:
        self.calls: List[Dict[str, Any]] = []
        self.bind_result: Any = BindResult(user_id=1, uid=100001, nickname="Ada", level=2)
        self.unbind_result: Any = UnbindResult(unbound=True)
        self.lookup_result: Any = LookupResult(bound=True, nickname="Ada", level=2, level_name="见习")
        self.command_result: Any = CommandResult(handled=False, reply=[])

    async def bind(self, platform, user_id, code, name=""):
        self.calls.append({"op": "bind", "platform": platform, "user_id": user_id, "code": code, "name": name})
        if isinstance(self.bind_result, Exception):
            raise self.bind_result
        return self.bind_result

    async def unbind(self, platform, user_id):
        self.calls.append({"op": "unbind", "platform": platform, "user_id": user_id})
        if isinstance(self.unbind_result, Exception):
            raise self.unbind_result
        return self.unbind_result

    async def lookup(self, platform, user_id):
        self.calls.append({"op": "lookup", "platform": platform, "user_id": user_id})
        if isinstance(self.lookup_result, Exception):
            raise self.lookup_result
        return self.lookup_result

    async def command(self, **kwargs):
        self.calls.append({"op": "command", **kwargs})
        if isinstance(self.command_result, Exception):
            raise self.command_result
        return self.command_result


@pytest.mark.asyncio
async def test_do_bind_without_code_asks_for_usage():
    client = FakeClient()
    reply = await logic.do_bind(client, "qq", "1", "Ada", "")
    assert "用法" in reply
    assert client.calls == []


@pytest.mark.asyncio
async def test_do_bind_success():
    client = FakeClient()
    reply = await logic.do_bind(client, "qq", "1", "Ada", "ABC123")
    assert "Ada" in reply
    assert client.calls[0] == {"op": "bind", "platform": "qq", "user_id": "1", "code": "ABC123", "name": "Ada"}


@pytest.mark.asyncio
async def test_do_bind_api_error_is_surfaced():
    client = FakeClient()
    client.bind_result = BotApiError(400, "绑定码无效")
    reply = await logic.do_bind(client, "qq", "1", "Ada", "BAD")
    assert "绑定码无效" in reply


@pytest.mark.asyncio
async def test_do_unbind_not_found():
    client = FakeClient()
    client.unbind_result = UnbindResult(unbound=False)
    reply = await logic.do_unbind(client, "qq", "1")
    assert "未找到" in reply


@pytest.mark.asyncio
async def test_do_me_unbound():
    client = FakeClient()
    client.lookup_result = LookupResult(bound=False)
    reply = await logic.do_me(client, "qq", "1")
    assert "尚未绑定" in reply


@pytest.mark.asyncio
async def test_do_me_bound_includes_realname_when_present():
    client = FakeClient()
    client.lookup_result = LookupResult(bound=True, nickname="Ada", level=3, level_name="资深", realname_verified=True)
    reply = await logic.do_me(client, "qq", "1")
    assert "Ada" in reply and "资深" in reply and "已通过" in reply


@pytest.mark.asyncio
async def test_do_passthrough_not_handled_returns_none():
    client = FakeClient()
    reply = await logic.do_passthrough(client, "qq", "1", "Ada", "group", "g1", "随便说点什么")
    assert reply is None


@pytest.mark.asyncio
async def test_do_passthrough_handled_returns_rendered_text():
    client = FakeClient()
    client.command_result = CommandResult(handled=True, reply=[{"type": "text", "data": {"text": "ok"}}])
    reply = await logic.do_passthrough(client, "qq", "1", "Ada", "group", "g1", "/help")
    assert reply == "ok"


@pytest.mark.asyncio
async def test_do_passthrough_api_error_returns_none():
    client = FakeClient()
    client.command_result = BotApiError(500, "boom")
    reply = await logic.do_passthrough(client, "qq", "1", "Ada", "group", "g1", "/help")
    assert reply is None


def test_chat_allowed_empty_list_allows_everything():
    assert logic.chat_allowed([], "anything") is True


def test_chat_allowed_restricts_to_listed_chats():
    assert logic.chat_allowed(["g1"], "g1") is True
    assert logic.chat_allowed(["g1"], "g2") is False
