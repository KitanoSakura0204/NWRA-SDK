"""PKCE and SDK-login redirects (PROTOCOL §6), using the standard library."""
from __future__ import annotations

import base64
import hashlib
import hmac
import re
import secrets
from dataclasses import dataclass
from urllib.parse import parse_qs, quote, urlencode, urlsplit

from .errors import OAuthError


@dataclass(frozen=True)
class PKCE:
    verifier: str
    challenge: str


def s256_challenge(verifier: str) -> str:
    return base64.urlsafe_b64encode(hashlib.sha256(verifier.encode("ascii")).digest()).decode().rstrip("=")


def create_pkce_pair() -> PKCE:
    verifier = secrets.token_urlsafe(32)
    return PKCE(verifier, s256_challenge(verifier))


def create_state() -> str:
    return secrets.token_urlsafe(16)


def build_login_url(frontend: str, server_code: str, code_challenge: str, *,
                    redirect_uri: str = "", state: str = "", scope: str = "",
                    nonce: str = "", ui_locales: str = "", device: str = "", prompt: str = "") -> str:
    if not server_code or not re.fullmatch(r"[A-Za-z0-9_-]{43}", code_challenge):
        raise ValueError("nowork: server_code and an S256 challenge are required")
    if device not in ("", "pc", "mobile") or prompt not in ("", "consent"):
        raise ValueError("nowork: invalid device or prompt")
    query = {k: v for k, v in dict(redirect_uri=redirect_uri, state=state, scope=scope,
              nonce=nonce, ui_locales=ui_locales, device=device, prompt=prompt).items() if v}
    query.update(code_challenge=code_challenge, code_challenge_method="S256")
    return frontend.rstrip("/") + "/sdk/login/" + quote(server_code, safe="") + "?" + urlencode(query)


def parse_login_callback(callback_url: str, want_state: str, want_issuer: str = "") -> str:
    query = parse_qs(urlsplit(callback_url).query, keep_blank_values=True)
    if any(len(query.get(key, [])) > 1 for key in ("code", "state", "iss", "error", "error_description")):
        raise ValueError("nowork: duplicate callback parameter")
    get = lambda key: query.get(key, [""])[0]
    if not want_state or not hmac.compare_digest(get("state").encode(), want_state.encode()):
        raise ValueError("nowork: state mismatch")
    if want_issuer and get("iss") and get("iss") != want_issuer:
        raise ValueError("nowork: issuer mismatch")
    if get("error"):
        raise OAuthError(get("error"), get("error_description"))
    if not get("code"):
        raise ValueError("nowork: missing code")
    return get("code")


def normalize_login_code(code: str) -> str:
    return re.sub(r"[\s-]+", "", code).upper()
