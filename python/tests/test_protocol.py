import io
import json
import unittest
import urllib.error
import urllib.parse
from unittest.mock import MagicMock, patch

from lounge_sdk import LoungeClient, LoungeOIDC
from lounge_sdk.errors import ApiError, OAuthError, TransportError


def response(body, status=200):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status = status
    result.read.return_value = json.dumps(body).encode("utf-8")
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


if __name__ == "__main__":
    unittest.main()
