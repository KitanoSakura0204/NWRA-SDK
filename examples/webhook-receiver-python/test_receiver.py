"""Receiver tests: python -m unittest -v (from this directory). They call Receiver.handle directly,
no listener and no network."""
import json
import logging
import os
import tempfile
import time
import unittest

from receiver import Receiver, SeenSet, load_keys  # first: it puts the repo's SDK on sys.path
from events import who
from nowork_sdk import webhook_signature

logging.getLogger("nowork").addHandler(logging.NullHandler())  # keep the expected rejections quiet

SECRET = "whsec_test"
VECTORS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..", "go", "testdata", "vectors.json")


def signed(secret, ts, body):
    return {"Content-Type": "application/json", "X-NoWork-Timestamp": str(ts),
            "X-NoWork-Signature": webhook_signature(secret, ts, body)}


def event(event_id, name, ts, data=None):
    return json.dumps({"id": event_id, "event": name, "timestamp": ts, "data": data or {}}).encode()


class ReceiverTest(unittest.TestCase):
    def test_plain_delivery_then_duplicate(self):
        got = []
        rc = Receiver(SECRET, got.append)
        ts = int(time.time())
        body = event("evt-1", "user.level_changed", ts, {"userId": 1, "uid": 100001, "from": 1, "to": 2})
        self.assertEqual(rc.handle("POST", signed(SECRET, ts, body), body), (200, {"status": "ok"}))
        self.assertEqual(rc.handle("POST", signed(SECRET, ts, body), body), (200, {"status": "duplicate"}))
        self.assertEqual(len(got), 1)
        self.assertEqual(got[0].data["to"], 2)

    def test_rejections(self):
        rc = Receiver(SECRET, lambda ev: None)
        ts = int(time.time())
        body = event("evt-2", "ping", ts)
        no_id = json.dumps({"event": "ping", "timestamp": ts, "data": {}}).encode()
        bad_ts = json.dumps({"id": "evt-x", "event": "ping", "timestamp": "soon", "data": {}}).encode()
        cases = [
            ("wrong secret", signed("wrong", ts, body), body, 401),
            ("tampered", signed(SECRET, ts, body), body + b" ", 401),
            ("stale", signed(SECRET, ts - 3600, body), body, 401),
            ("unsigned", {}, body, 401),
            ("not json", signed(SECRET, ts, b"not json"), b"not json", 400),
            ("missing id", signed(SECRET, ts, no_id), no_id, 400),
            ("non-numeric timestamp", signed(SECRET, ts, bad_ts), bad_ts, 400),
        ]
        for name, headers, payload, want in cases:
            with self.subTest(name):
                self.assertEqual(rc.handle("POST", headers, payload)[0], want)
        self.assertEqual(rc.handle("GET", {}, b"")[0], 405)

    def test_failed_handler_is_retried(self):
        calls = []

        def flaky(ev):
            calls.append(ev.id)
            if len(calls) == 1:
                raise RuntimeError("database down")

        rc = Receiver(SECRET, flaky)
        ts = int(time.time())
        body = event("evt-3", "ping", ts)
        with self.assertLogs("nowork.webhook", "ERROR"):
            self.assertEqual(rc.handle("POST", signed(SECRET, ts, body), body)[0], 500)
        self.assertEqual(rc.handle("POST", signed(SECRET, ts, body), body), (200, {"status": "ok"}))
        self.assertEqual(calls, ["evt-3", "evt-3"])


def _has_cryptography():
    try:
        import cryptography  # noqa: F401
    except ImportError:
        return False
    return True


# The shared SDK test vectors hold a signed, encrypted webhook (SDK/go/testdata/vectors.json).
@unittest.skipUnless(os.path.exists(VECTORS), "SDK test vectors not found (example copied out of the repo)")
class EncryptedTest(unittest.TestCase):
    def setUp(self):
        with open(VECTORS, encoding="utf-8") as f:
            self.vs = json.load(f)
        self.v = next(x for x in self.vs["vectors"] if x.get("kind") == "webhook" and not x.get("fail"))
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.key_file = os.path.join(tmp.name, "key.jwk")
        with open(self.key_file, "w", encoding="utf-8") as f:
            json.dump(self.vs["keys"][self.v["key"]], f)
        self.keys = load_keys(" %s ," % self.key_file)

    @unittest.skipUnless(_has_cryptography(), "pip install cryptography to run the decryption case")
    def test_encrypted_delivery(self):
        got = []
        # The vector's timestamp is fixed, so the replay window is off (tolerance 0).
        rc = Receiver(self.v["secret"], got.append, tolerance=0, keys=self.keys)
        self.assertEqual(rc.handle("POST", self.v["headers"], self.v["body"].encode()), (200, {"status": "ok"}))
        self.assertEqual(got[0].id, "evt-vector")
        self.assertEqual(got[0].data["user"]["nickname"], "旅行者")

    def test_fails_closed(self):
        with self.assertLogs("nowork.webhook", "WARNING"):
            # signed but not readable here: 500 keeps the delivery queued
            no_key = Receiver(self.v["secret"], lambda ev: None, tolerance=0)
            self.assertEqual(no_key.handle("POST", self.v["headers"], self.v["body"].encode())[0], 500)
            # keys configured but the delivery is plain: refused
            ts = int(time.time())
            plain = event("evt-4", "ping", ts)
            with_key = Receiver(self.v["secret"], lambda ev: None, keys=self.keys)
            self.assertEqual(with_key.handle("POST", signed(self.v["secret"], ts, plain), plain)[0], 500)

    def test_public_jwk_is_refused(self):
        self.assertEqual(len(self.keys), 1)
        pub = {k: val for k, val in self.vs["keys"][self.v["key"]].items() if k != "d"}
        with open(self.key_file, "w", encoding="utf-8") as f:
            json.dump(pub, f)
        with self.assertRaisesRegex(ValueError, "not a private JWK"):
            load_keys(self.key_file)


class HelpersTest(unittest.TestCase):
    def test_who_prefers_pairwise_sub(self):
        self.assertEqual(who(100001, None), "uid=100001")
        self.assertEqual(who(None, {"sub": "2f1c8e0a"}), "sub=2f1c8e0a")

    def test_seen_set_evicts_least_recent(self):
        s = SeenSet(2)
        s.add("a")
        s.add("b")
        self.assertIn("a", s)  # touching a leaves b as the oldest
        s.add("c")
        self.assertNotIn("b", s)
        self.assertIn("a", s)
        self.assertIn("c", s)


if __name__ == "__main__":
    unittest.main()
