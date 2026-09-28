import io
import json
import unittest
import urllib.error
import urllib.parse
from unittest.mock import MagicMock, patch

from lounge_sdk import GMAction, GMActionParam, GMOption, LoungeClient, LoungeOIDC, OnlinePlayer
from lounge_sdk.errors import ApiError, EncryptionError, OAuthError, TransportError


def response(body, status=200, headers=None):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status = status
    result.read.return_value = json.dumps(body).encode("utf-8")
    if headers is not None:
        result.headers = headers
    return result


class ProtocolTests(unittest.TestCase):
    @patch("urllib.request.urlopen")
    def test_join_check(self, send):
        send.return_value = response({"code": 0, "data": {"allowed": True, "bound": True, "reason": "", "player": {"name": "Player & One", "uid": 100001, "securityLevel": 2}}})
        result = LoungeClient("https://account.example/", "test-key").check_whitelist("Player & One")
        request = send.call_args.args[0]
        self.assertEqual(request.get_header("X-server-key"), "test-key")
        self.assertEqual(urllib.parse.parse_qs(urllib.parse.urlsplit(request.full_url).query)["name"], ["Player & One"])
        self.assertTrue(result.allowed)
        self.assertEqual(result.player.security_level, 2)
        self.assertEqual(result.player.uuid, "")

    @patch("urllib.request.urlopen")
    def test_reject_invalid_api_envelopes(self, send):
        client = LoungeClient("https://account.example", "test-key")
        for body in [{}, None, [], {"code": 0}, {"code": True}, {"code": 0, "data": None}]:
            with self.subTest(body=body):
                send.return_value = response(body)
                with self.assertRaises(TransportError):
                    client.whitelist()

    @patch("urllib.request.urlopen")
    def test_http_errors(self, send):
        client = LoungeClient("https://account.example", "test-key")
        send.side_effect = urllib.error.HTTPError("https://account.example", 401, "Unauthorized", {}, io.BytesIO(b'{"code":401,"message":"invalid server key"}'))
        with self.assertRaises(ApiError):
            client.whitelist()
        send.side_effect = urllib.error.HTTPError("https://account.example", 503, "Unavailable", {}, io.BytesIO(b'{"code":0,"data":{}}'))
        with self.assertRaises(TransportError):
            client.whitelist()

    @patch("urllib.request.urlopen")
    def test_oidc_exchange_and_pkce(self, send):
        send.return_value = response({"access_token": "access", "token_type": "Bearer", "expires_in": 3600})
        oidc = LoungeOIDC("https://account.example", "client", "secret", "https://app.example/callback")
        params = urllib.parse.parse_qs(urllib.parse.urlsplit(oidc.authorize_url("state & +", code_challenge="challenge")).query)
        self.assertEqual(params["state"], ["state & +"])
        self.assertEqual(params["code_challenge_method"], ["S256"])
        self.assertEqual(oidc.exchange("code", "verifier")["access_token"], "access")
        body = urllib.parse.parse_qs(send.call_args.args[0].data.decode())
        self.assertEqual(body["client_id"], ["client"])
        self.assertEqual(body["code_verifier"], ["verifier"])

    @patch("urllib.request.urlopen")
    def test_oidc_empty_revoke_and_invalid_json(self, send):
        result = response({})
        result.read.return_value = b""
        send.return_value = result
        oidc = LoungeOIDC("https://account.example", "client")
        oidc.revoke("access")
        with self.assertRaises(TransportError):
            oidc.exchange("code")
        result.read.return_value = b"<html>unavailable</html>"
        with self.assertRaises(TransportError):
            oidc.userinfo("access")

    @patch("urllib.request.urlopen")
    def test_oidc_protocol_error(self, send):
        send.side_effect = urllib.error.HTTPError("https://account.example", 400, "Bad Request", {}, io.BytesIO(b'{"error":"invalid_grant","error_description":"expired"}'))
        with self.assertRaises(OAuthError):
            LoungeOIDC("https://account.example", "client").exchange("code")

    @patch("urllib.request.urlopen")
    def test_encrypted_reply_fails_closed(self, send):
        sealed = {"code": 0, "message": "ok", "data": {"jwe": "aGVhZA.a2V5.aXY.Y2lwaGVy.dGFn"}}
        client = LoungeClient("https://account.example", "test-key")
        for headers in ({"X-Lounge-Encryption": "JWE"}, {}):  # a proxy may drop the header: the body shape still counts
            with self.subTest(headers=headers):
                send.return_value = response(sealed, headers=headers)
                with self.assertRaises(EncryptionError) as caught:
                    client.whitelist()
                self.assertEqual(caught.exception.route, "whitelist")
                with self.assertRaises(EncryptionError) as caught:
                    client.check_whitelist("Steve")
                self.assertEqual(caught.exception.route, "whitelist/check")
                with self.assertRaises(EncryptionError):
                    client.poll_gm_commands()
                raw = client.raw("GET", "/server/v1/whitelist")
                self.assertTrue(raw.encrypted)
                self.assertEqual(raw.envelope, sealed)
        send.return_value = response({"code": 0, "data": {"jwe": "x", "mode": 1, "count": 0, "entries": []}})
        self.assertEqual(client.whitelist().mode, 1)

    @patch("urllib.request.urlopen")
    def test_heartbeat_optional_fields(self, send):
        send.return_value = response({"code": 0, "data": {"status": "ok"}})
        client = LoungeClient("https://account.example", "test-key")
        client.heartbeat(online=2, max_players=20, version="1.21.1")
        client.heartbeat(2, 20, "1.21.1", tps=19.8, players=[OnlinePlayer("Steve", "069a79f4-44e9-4726-a5be-fca90e38aaf5"), {"name": "Alex"}, "Herobrine"])
        client.heartbeat(0, tps=float("nan"), players=[])
        client.heartbeat(0, tps=True)
        bodies = [json.loads(c.args[0].data) for c in send.call_args_list]
        self.assertEqual(bodies, [
            {"online": 2, "max": 20, "version": "1.21.1"},
            {"online": 2, "max": 20, "version": "1.21.1", "tps": 19.8, "players": [
                {"name": "Steve", "uuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5"}, {"name": "Alex"}, {"name": "Herobrine"}]},
            {"online": 0, "max": 0, "version": ""},
            {"online": 0, "max": 0, "version": ""},
        ])

    @patch("urllib.request.urlopen")
    def test_gm_catalog_publish(self, send):
        send.return_value = response({"code": 0, "data": {"count": 1, "skipped": [{"key": "bad key", "reason": "invalid key"}]}})
        client = LoungeClient("https://account.example", "test-key")
        kick = GMAction("kick", "Kick", category="players", danger="confirm", params=[
            GMActionParam("player", "player", required=True),
            GMActionParam("world", "select", source="worlds", default="overworld"),
            GMActionParam("amount", "number", min=0, max=64, integer=True),
        ])
        result = client.publish_gm_catalog([kick], {"worlds": [GMOption("overworld", "Overworld"), {"value": "nether"}]})
        self.assertEqual((result.count, result.skipped), (1, [{"key": "bad key", "reason": "invalid key"}]))
        request = send.call_args.args[0]
        self.assertEqual((request.get_method(), urllib.parse.urlsplit(request.full_url).path), ("PUT", "/server/v1/gm/actions"))
        self.assertEqual(json.loads(request.data), {
            "actions": [{"key": "kick", "title": "Kick", "category": "players", "danger": "confirm", "params": [
                {"name": "player", "type": "player", "required": True, "label": ""},
                {"name": "world", "type": "select", "required": False, "label": "", "source": "worlds", "default": "overworld"},
                {"name": "amount", "type": "number", "required": False, "label": "", "integer": True, "min": 0, "max": 64},
            ]}],
            "options": {"worlds": [{"value": "overworld", "label": "Overworld"}, {"value": "nether"}]},
        })
        send.return_value = response({"code": 0, "data": {"count": 1}})
        self.assertEqual(client.publish_gm_options({"worlds": [GMOption("overworld")]}).skipped, [])
        request = send.call_args.args[0]
        self.assertEqual(urllib.parse.urlsplit(request.full_url).path, "/server/v1/gm/options")
        self.assertEqual(json.loads(request.data), {"worlds": [{"value": "overworld"}]})
        self.assertEqual(client.publish_gm_actions([{"key": "a", "title": "A"}]), 1)


if __name__ == "__main__":
    unittest.main()
