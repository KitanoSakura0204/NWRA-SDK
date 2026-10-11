import base64
import json
import unittest
import urllib.parse
from unittest.mock import MagicMock, patch

from nowork_sdk import Entitlement, EntitlementValues, NoworkClient, NoworkOIDC
from nowork_sdk.errors import ApiError


def response(body, status=200):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status = status
    result.read.return_value = json.dumps(body).encode("utf-8")
    return result


def sent(send, i):
    req = send.call_args_list[i].args[0]
    url = urllib.parse.urlsplit(req.full_url)
    body = json.loads(req.data.decode("utf-8")) if req.data else None
    return req.get_method(), url.path, urllib.parse.parse_qs(url.query), body


class EntitlementTests(unittest.TestCase):
    @patch("urllib.request.urlopen")
    def test_server_routes(self, send):
        send.side_effect = [
            response({"code": 0, "data": {"sub": "pair-1", "entitlements": {"vip": {"tier": 3}, "beta": True}}}),
            response({"code": 0, "data": {"sub": "pair-1", "entitlements": {"vip": {"tier": 3}, "beta": True}}}),
            response({"code": 0, "data": {"sub": "pair-1", "key": "vip", "value": {"tier": 4}}}),
            response({"code": 0, "data": {"sub": "pair-1", "key": "vip"}}),
        ]
        c = NoworkClient("https://account.example", "k")

        all_values = c.entitlements(uid=100001)
        self.assertIsInstance(all_values, EntitlementValues)
        self.assertEqual((all_values.sub, all_values.entitlements), ("pair-1", {"vip": {"tier": 3}, "beta": True}))
        self.assertEqual(sent(send, 0), ("GET", "/server/v1/delivery/entitlements", {"uid": ["100001"]}, None))
        self.assertEqual(send.call_args_list[0].args[0].get_header("X-server-key"), "k")

        # A sub wins over a uid: it is the only form a pairwise profile accepts.
        c.entitlements(uid=100001, sub=" pair-1 ")
        self.assertEqual(sent(send, 1)[1:], ("/server/v1/delivery/entitlements", {"sub": ["pair-1"]}, None))

        put = c.put_entitlement(sub="pair-1", key="vip", value={"tier": 4})
        self.assertEqual(sent(send, 2)[1:], ("/server/v1/delivery/entitlements", {}, {"key": "vip", "value": {"tier": 4}, "sub": "pair-1"}))
        self.assertIsInstance(put, Entitlement)
        self.assertEqual((put.sub, put.key, put.value), ("pair-1", "vip", {"tier": 4}))

        removed = c.delete_entitlement(sub="pair-1", key="vip")
        self.assertEqual((removed.sub, removed.key), ("pair-1", "vip"))
        self.assertEqual(sent(send, 3), ("DELETE", "/server/v1/delivery/entitlements", {"sub": ["pair-1"], "key": ["vip"]}, None))

    @patch("urllib.request.urlopen")
    def test_incomplete_targets_never_reach_the_network(self, send):
        c = NoworkClient("https://account.example", "k")
        for call in (
            lambda: c.entitlements(),
            lambda: c.put_entitlement(uid=7, key="  ", value=1),
            lambda: c.delete_entitlement(uid=7, key=""),
            lambda: c.entitlements(uid=0, sub="   "),
        ):
            with self.assertRaises(ValueError):
                call()
        send.assert_not_called()

    @patch("urllib.request.urlopen")
    def test_business_error_is_api_error(self, send):
        send.side_effect = [response({"code": 1, "message": "成对标识的投递配置只能用 sub 指定用户"})]
        with self.assertRaises(ApiError) as ctx:
            NoworkClient("https://account.example", "k").entitlements(uid=100001)
        self.assertEqual(ctx.exception.code, 1)


class OAuthEntitlementTests(unittest.TestCase):
    @patch("urllib.request.urlopen")
    def test_oauth_routes_use_basic_auth(self, send):
        send.side_effect = [
            response({"code": 0, "data": {"sub": "pair-1", "entitlements": {"vip": 1}}}),
            response({"code": 0, "data": {"sub": "pair-1", "key": "vip", "value": 1}}),
            response({"code": 401, "message": "invalid client credentials"}, status=401),
        ]
        oidc = NoworkOIDC("https://account.example", "lc_app", "secret", "https://app.example/cb")

        self.assertEqual(oidc.entitlements(sub="pair-1").entitlements, {"vip": 1})
        self.assertEqual(oidc.put_entitlement(sub="pair-1", key="vip", value=1).key, "vip")
        method, path, query, body = sent(send, 1)
        self.assertEqual((method, path, body), ("PUT", "/oauth2/delivery/entitlements", {"key": "vip", "value": 1, "sub": "pair-1"}))
        self.assertEqual(send.call_args_list[0].args[0].get_header("Authorization"),
                         "Basic " + base64.b64encode(b"lc_app:secret").decode())

        with self.assertRaises(ApiError) as ctx:
            oidc.delete_entitlement(sub="pair-1", key="vip")
        self.assertEqual(ctx.exception.code, 401)
        self.assertEqual(sent(send, 2)[2], {"sub": ["pair-1"], "key": ["vip"]})

    @patch("urllib.request.urlopen")
    def test_oauth_incomplete_target_is_refused(self, send):
        with self.assertRaises(ValueError):
            NoworkOIDC("https://account.example", "lc_app", "secret").entitlements()
        send.assert_not_called()


if __name__ == "__main__":
    unittest.main()
