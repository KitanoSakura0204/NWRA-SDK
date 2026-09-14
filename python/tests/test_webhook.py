import json
import time
import unittest

from lounge_sdk.webhook import WebhookError, verify_webhook, webhook_signature


class WebhookTests(unittest.TestCase):
    def test_verify(self):
        body = json.dumps({"id": "e1", "event": "user.login", "timestamp": 1, "data": {"userId": 7}}).encode()
        ts = int(time.time())
        headers = {"X-Lounge-Timestamp": str(ts), "X-Lounge-Signature": webhook_signature("whsec_test", ts, body)}
        ev = verify_webhook("whsec_test", headers, body)
        self.assertEqual(ev.event, "user.login")
        self.assertEqual(ev.data["userId"], 7)
        with self.assertRaises(WebhookError):
            verify_webhook("wrong", headers, body)
        old = ts - 3600
        stale = {"X-Lounge-Timestamp": str(old), "X-Lounge-Signature": webhook_signature("whsec_test", old, body)}
        with self.assertRaises(WebhookError):
            verify_webhook("whsec_test", stale, body)
        self.assertEqual(verify_webhook("whsec_test", stale, body, tolerance=None).id, "e1")


if __name__ == "__main__":
    unittest.main()
