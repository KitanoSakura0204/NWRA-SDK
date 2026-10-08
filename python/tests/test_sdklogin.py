from urllib.parse import parse_qs, urlsplit
from unittest.mock import patch

import pytest

from nowork_sdk import (NoworkClient, OAuthError, build_login_url, create_pkce_pair,
                        create_state, normalize_login_code, parse_login_callback, s256_challenge)


def test_pkce_rfc7636_vector_and_entropy():
    assert s256_challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk") == "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
    first, second = create_pkce_pair(), create_pkce_pair()
    assert len(first.verifier) == 43 and first.challenge == s256_challenge(first.verifier)
    assert first != second and create_state() != create_state()


def test_url_encoding_and_display_code():
    url = build_login_url("https://account.example/", "s/a", "a" * 43,
                          state="x & y", redirect_uri="mygame://cb", device="mobile", prompt="consent")
    assert urlsplit(url).path == "/sdk/login/s%2Fa"
    q = parse_qs(urlsplit(url).query)
    assert q["state"] == ["x & y"] and q["code_challenge_method"] == ["S256"]
    assert q["device"] == ["mobile"] and q["prompt"] == ["consent"]
    assert normalize_login_code(" abcd- 2345 ") == "ABCD2345"


@pytest.mark.parametrize("url,state", [
    ("mygame://cb?code=c", ""), ("mygame://cb?code=c&state=bad", "s"),
    ("mygame://cb?code=c&state=s&state=bad", "s"), ("mygame://cb?code=c&code=d&state=s", "s"),
    ("mygame://cb?code=c&state=s&iss=https://evil.example", "s"),
])
def test_callback_rejects_unbound_ambiguous_or_wrong_issuer(url, state):
    with pytest.raises(ValueError):
        parse_login_callback(url, state, "https://account.example")


def test_callback_preserves_oauth_error_and_code():
    with pytest.raises(OAuthError):
        parse_login_callback("mygame://cb?error=access_denied&state=s", "s")
    assert parse_login_callback("mygame://cb?code=c&state=s", "s") == "c"


def test_gm_exchange_uses_private_json_body():
    client = NoworkClient("https://account.example", "private-key")
    with patch.object(client, "_call", return_value={"subject": "gm-role:7"}) as call:
        assert client.exchange_gm_code("code", "verifier", "https://gm.example/cb")["subject"] == "gm-role:7"
    call.assert_called_once_with("POST", "/server/v1/gm/sso/exchange", body={
        "code": "code", "codeVerifier": "verifier", "redirectUri": "https://gm.example/cb",
    })


def test_catalog_none_preserves_actions_but_empty_list_clears():
    client = NoworkClient("https://account.example", "key")
    with patch.object(client, "_call", return_value={}) as call:
        client.publish_gm_catalog(None, {})
        assert call.call_args.kwargs["body"] == {"options": {}}
        client.publish_gm_catalog([], {})
        assert call.call_args.kwargs["body"] == {"actions": [], "options": {}}
