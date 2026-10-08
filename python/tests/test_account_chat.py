import json
from unittest.mock import MagicMock, patch

import pytest

from nowork_sdk import AccountClient, ApiError, ChatClient, NoworkClient, TransportError


def response(data, status=200):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status = status
    result.read.return_value = json.dumps(data).encode()
    return result


def test_portal_uses_jwt_and_role_route_even_on_open_servers():
    opener = MagicMock()
    opener.open.return_value = response({"code": 0, "data": {"id": 3, "requiresWhitelist": False}})
    client = AccountClient("https://account.example", "site-jwt", opener=opener)
    assert client.bind_role(3, "Alex")["requiresWhitelist"] is False
    request = opener.open.call_args.args[0]
    assert request.full_url == "https://account.example/api/account/portal/servers/3/roles"
    assert request.get_header("Authorization") == "Bearer site-jwt"
    assert request.get_header("X-server-key") is None
    assert json.loads(request.data)["roleName"] == "Alex"


@pytest.mark.parametrize("body,status,error", [([], 200, TransportError), ({"code": 0}, 200, TransportError),
    ({"code": 0, "data": []}, 503, TransportError), ({"code": 403, "message": "denied"}, 403, ApiError)])
def test_portal_error_contract(body, status, error):
    opener = MagicMock()
    opener.open.return_value = response(body, status)
    with pytest.raises(error): AccountClient("https://account.example", "jwt", opener=opener).servers()


@patch("urllib.request.urlopen")
def test_chat_relay_ack_uses_done_not_bot_ids(send):
    send.return_value = response({"code": 0, "data": {"updated": 1}})
    assert NoworkClient("https://account.example", "server-key").ack_chat_relay([9]) == 1
    request = send.call_args.args[0]
    assert request.full_url.endswith("/server/v1/chat/relay/ack")
    assert json.loads(request.data) == {"done": [9], "failed": []}


def test_chat_session_credentials_nonce_reactions_and_empty_read_response():
    opener = MagicMock()
    opener.open.return_value = response({"code": 0, "data": {"id": 9, "nonce": "retry-1", "ownReactions": ["heart"]}})
    client = ChatClient("https://account.example", "cs_session", opener=opener)
    assert client.send_message(3, "hello", nonce="retry-1")["ownReactions"] == ["heart"]
    request = opener.open.call_args.args[0]
    assert request.get_header("Authorization") == "Chat cs_session"
    assert request.get_header("X-server-key") is None
    assert json.loads(request.data)["nonce"] == "retry-1"
    opener.open.return_value = response({"code": 0, "data": None})
    client.mark_read(3, 9)
    assert opener.open.call_args.args[0].full_url.endswith("/api/chat/channels/3/read")
