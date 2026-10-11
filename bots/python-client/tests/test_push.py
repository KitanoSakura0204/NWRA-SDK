import hashlib
import hmac
import json

import pytest

from nowork_bot_client import BotPushError, verify_push


def fixture(site_id=None):
    event = {"type": "message", "id": 7, "botId": 2, "target": {"chatType": "private", "userId": "x"}, "message": [], "time": 1000}
    if site_id is not None:
        event["siteId"] = site_id
    body = json.dumps(event).encode()
    headers = {"X-Nowork-Timestamp": "1000", "X-Nowork-Delivery": "7", "X-Nowork-Event": "bot.message",
               "X-Nowork-Signature": "sha256=" + hmac.new(b"secret", b"1000." + body, hashlib.sha256).hexdigest()}
    return body, headers


def test_verified_push_and_case_insensitive_headers():
    body, headers = fixture()
    assert verify_push("secret", {k.upper(): v for k, v in headers.items()}, body, now=1001).id == 7


@pytest.mark.parametrize("change", ["body", "key", "stale", "delivery", "event", "unsigned"])
def test_rejects_forged_stale_or_mismatched_push(change):
    body, headers = fixture()
    if change == "body": body += b" "
    if change == "delivery": headers["X-Nowork-Delivery"] = "8"
    if change == "event": headers["X-Nowork-Event"] = "user.login"
    if change == "unsigned": del headers["X-Nowork-Signature"]
    with pytest.raises(BotPushError):
        verify_push("wrong" if change == "key" else "secret", headers, body, now=2000 if change == "stale" else 1001)


@pytest.mark.parametrize("site_id", [None, 0, 42])
def test_optional_site_metadata_is_preserved_and_authenticated(site_id):
    body, headers = fixture(site_id)
    assert verify_push("secret", headers, body, now=1001).site_id == site_id
    altered = json.loads(body)
    altered["siteId"] = (site_id or 0) + 1
    with pytest.raises(BotPushError):
        verify_push("secret", headers, json.dumps(altered).encode(), now=1001)
