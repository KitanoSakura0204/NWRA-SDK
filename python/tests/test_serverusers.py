import json
import unittest
import urllib.parse
from unittest.mock import MagicMock, patch

from lounge_sdk import LoungeClient, is_version_conflict
from lounge_sdk.errors import ApiError


def response(body, status=200):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status = status
    result.read.return_value = json.dumps(body).encode("utf-8")
    return result


STORED = {"uid": 100001, "data": {"coins": 12, "rank": None}, "version": 2, "updatedAt": "2026-09-29 10:00:00", "updatedBy": "plugin", "size": 11}
BAN = {"uid": 100001, "names": ["Steve"], "uuids": [], "reason": "x-ray", "expireTime": None, "createTime": "2026-09-29 10:00:00", "source": "plugin"}


def sent(send, i):
    req = send.call_args_list[i].args[0]
    url = urllib.parse.urlsplit(req.full_url)
    body = json.loads(req.data.decode("utf-8")) if req.data else None
    return req.get_method(), url.path, urllib.parse.parse_qs(url.query), body


class ServerUsersTests(unittest.TestCase):
    @patch("urllib.request.urlopen")
    def test_user_data(self, send):
        send.side_effect = [
            response({"code": 0, "data": {"fields": [{"key": "coins", "label": "Coins", "type": "number", "visible": True, "integer": True, "min": 0}], "maxUserBytes": 8192}}),
            response({"code": 0, "data": STORED}),
            response({"code": 0, "data": STORED}),
            response({"code": 0, "data": STORED}),
            response({"code": 409, "message": "stale"}),
        ]
        c = LoungeClient("https://account.example", "k")
        schema = c.user_schema()
        self.assertEqual(schema.max_user_bytes, 8192)
        self.assertTrue(schema.fields[0].integer)
        self.assertEqual(schema.fields[0].min, 0)
        d = c.user_data(100001)
        self.assertEqual((d.version, d.data, d.updated_at), (2, {"coins": 12, "rank": None}, "2026-09-29 10:00:00"))
        c.put_user_data(100001, {"coins": 1}, version=2)
        c.patch_user_data(100001, {"rank": None})
        self.assertEqual(sent(send, 2), ("PUT", "/server/v1/users/100001/data", {}, {"data": {"coins": 1}, "version": 2}))
        self.assertEqual(sent(send, 3), ("PATCH", "/server/v1/users/100001/data", {}, {"data": {"rank": None}}))
        with self.assertRaises(ApiError) as cm:
            c.put_user_data(100002, {"coins": 1}, version=1)
        self.assertTrue(is_version_conflict(cm.exception))
        with self.assertRaises(ValueError):
            c.user_data(0)
        with self.assertRaises(ValueError):
            c.user_data(True)

    @patch("urllib.request.urlopen")
    def test_bans(self, send):
        send.side_effect = [
            response({"code": 0, "data": {"count": 1, "bans": [BAN]}}),
            response({"code": 0, "data": {"count": 1, "players": [{"name": "Steve", "uid": 100001, "reason": "x-ray", "expireTime": None}]}}),
            response({"code": 0, "data": {"uid": 100001, "ban": BAN}}),
            response({"code": 0, "data": {"uid": 100001, "lifted": True}}),
        ]
        c = LoungeClient("https://account.example", "k")
        bans = c.bans(name="Steve")
        self.assertEqual((bans[0].uid, bans[0].names, bans[0].expire_time, bans[0].source), (100001, ["Steve"], None, "plugin"))
        self.assertEqual(sent(send, 0)[2], {"name": ["Steve"]})
        kick = c.kick_list()
        self.assertEqual((kick[0].name, kick[0].uid, kick[0].uuid), ("Steve", 100001, ""))
        uid, ban = c.ban(name="Steve", reason="x-ray", duration_minutes=1440)
        self.assertEqual((uid, ban.reason), (100001, "x-ray"))
        self.assertEqual(sent(send, 2), ("POST", "/server/v1/bans", {}, {"name": "Steve", "reason": "x-ray", "durationMinutes": 1440}))
        self.assertEqual(c.unban(uid=100001, reason="appeal"), 100001)
        self.assertEqual(sent(send, 3), ("POST", "/server/v1/bans/lift", {}, {"uid": 100001, "reason": "appeal"}))

    @patch("urllib.request.urlopen")
    def test_join_check_carries_the_ban_message(self, send):
        send.return_value = response({"code": 0, "data": {"allowed": False, "bound": True, "reason": "banned", "message": "x-ray", "player": None}})
        r = LoungeClient("https://account.example", "k").check_whitelist("Steve")
        self.assertEqual((r.allowed, r.reason, r.message), (False, "banned", "x-ray"))


if __name__ == "__main__":
    unittest.main()
