import importlib.util
from pathlib import Path
from unittest.mock import patch
from urllib.parse import parse_qs, urlsplit

import pytest

spec = importlib.util.spec_from_file_location("gm_sso_example", Path(__file__).with_name("server.py"))
server = importlib.util.module_from_spec(spec)
spec.loader.exec_module(server)


def test_pending_session_is_consumed_once_and_expires():
    sessions = server.Sessions(limit=1)
    with patch.object(server.time, "time", return_value=100):
        key = sessions.put({"state": "state", "verifier": "private"}, 10)
        with pytest.raises(ValueError): sessions.put({}, 10)
        assert sessions.get(key, consume=True)["state"] == "state"
        assert sessions.get(key, consume=True) is None
        expired = sessions.put({}, 10)
    with patch.object(server.time, "time", return_value=111):
        assert sessions.get(expired) is None


@pytest.mark.parametrize("public", ["http://gm.example", "https://u:p@gm.example", "https://gm.example?x=1", "https://gm.example/#x"])
def test_configuration_rejects_unsafe_callback_origins(public):
    with pytest.raises(ValueError): server.create_handler("https://account.example", public, 1, "test-key")


def test_local_http_requires_explicit_development_opt_in():
    with pytest.raises(ValueError): server.create_handler("http://localhost:8080", "http://localhost:8088", 1, "test-key")
    assert server.create_handler("http://localhost:8080", "http://localhost:8088", 1, "test-key", allow_loopback_http=True)


def request(handler, path, cookie=""):
    instance = handler.__new__(handler)
    instance.path, instance.headers = path, {"Cookie": cookie}
    captured = {}
    def reply(status, data=None, **kwargs): captured.update(status=status, data=data, **kwargs)
    instance.reply = reply
    instance.do_GET()
    return captured


def test_full_browser_bound_start_callback_and_replay():
    with patch.object(server, "NoworkClient") as factory:
        factory.return_value.exchange_gm_code.return_value = {
            "serverId": 1, "scope": "character:login", "subject": "gm-role:7", "character": {"id": 7},
        }
        handler = server.create_handler("https://account.example", "https://gm.example", 1, "private-key")
        start = request(handler, "/start?gm_request=opaque&gm_server_id=1")
        assert start["status"] == 303
        query = parse_qs(urlsplit(start["location"]).query)
        assert query["code_challenge_method"] == ["S256"]
        assert "code_verifier" not in query and "private-key" not in start["location"]
        pending = "gm_pending=" + start["cookies"][0][1]
        callback = "/callback?code=one-use&state=" + query["state"][0]
        reply = request(handler, callback, pending)
        assert reply["status"] == 303 and reply["location"] == "/me"
        args = factory.return_value.exchange_gm_code.call_args.args
        assert args[0] == "one-use" and args[2] == "https://gm.example/callback"
        assert len(args[1]) == 43
        assert request(handler, callback, pending)["status"] == 403
        assert factory.return_value.exchange_gm_code.call_count == 1


def test_wrong_state_does_not_exchange():
    with patch.object(server, "NoworkClient") as factory:
        handler = server.create_handler("https://account.example", "https://gm.example", 1, "key")
        start = request(handler, "/start?gm_request=opaque&gm_server_id=1")
        cookie = "gm_pending=" + start["cookies"][0][1]
        assert request(handler, "/callback?code=stolen&state=wrong", cookie)["status"] == 403
        factory.return_value.exchange_gm_code.assert_not_called()
