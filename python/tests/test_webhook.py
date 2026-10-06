import json
import time
import unittest

from nowork_sdk.webhook import WebhookError, verify_webhook, webhook_signature


class WebhookTests(unittest.TestCase):
    def test_verify(self):
        body = json.dumps({"id": "e1", "event": "user.login", "timestamp": 1, "data": {"userId": 7}}).encode()
        ts = int(time.time())
        headers = {"X-NoWork-Timestamp": str(ts), "X-NoWork-Signature": webhook_signature("whsec_test", ts, body)}
        ev = verify_webhook("whsec_test", headers, body)
        self.assertEqual(ev.event, "user.login")
        self.assertEqual(ev.data["userId"], 7)
        with self.assertRaises(WebhookError):
            verify_webhook("wrong", headers, body)
        old = ts - 3600
        stale = {"X-NoWork-Timestamp": str(old), "X-NoWork-Signature": webhook_signature("whsec_test", old, body)}
        with self.assertRaises(WebhookError):
            verify_webhook("whsec_test", stale, body)
        self.assertEqual(verify_webhook("whsec_test", stale, body, tolerance=None).id, "e1")
        self.assertEqual(verify_webhook("whsec_test", stale, body, tolerance=0).id, "e1")


if __name__ == "__main__":
    unittest.main()
