from types import SimpleNamespace

import pytest

from nonebot_plugin_nowork.delivery import deliver, select_bot


class FakeBot:
    def __init__(self, adapter="OneBot V11", bot_id="1"):
        self.self_id = bot_id
        self.adapter = SimpleNamespace(get_name=lambda: adapter)
        self.calls = []

    async def call_api(self, api, **kwargs):
        self.calls.append((api, kwargs))


def test_ambiguous_connected_bots_require_explicit_selection():
    first, second = FakeBot(bot_id="1"), FakeBot(bot_id="2")
    with pytest.raises(RuntimeError): select_bot([first, second])
    assert select_bot([first, second], "2") is second


@pytest.mark.asyncio
@pytest.mark.parametrize("adapter,kind,api,key", [
    ("OneBot V11", "private", "send_private_msg", "user_id"),
    ("OneBot V11", "group", "send_group_msg", "group_id"),
    ("Telegram", "private", "send_message", "chat_id"),
    ("Feishu", "group", "send_message", "receive_id"),
    ("QQ", "group", "post_group_messages", "group_openid"),
])
async def test_adapter_destination_contract(adapter, kind, api, key):
    bot = FakeBot(adapter)
    await deliver(bot, {"chatType": kind, "userId": "123", "chatId": "456"}, "hello")
    assert bot.calls[0][0] == api
    assert str(bot.calls[0][1][key]) == ("123" if kind == "private" else "456")


@pytest.mark.asyncio
async def test_send_failure_propagates_without_ack():
    bot = FakeBot()
    async def fail(*args, **kwargs): raise RuntimeError("offline")
    bot.call_api = fail
    with pytest.raises(RuntimeError): await deliver(bot, {"chatType": "private", "userId": "1"}, "hello")
