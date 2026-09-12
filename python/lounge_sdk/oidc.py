from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request

from .errors import OAuthError, TransportError


class LoungeOIDC:
    """Minimal OIDC relying-party helper (authorization code + PKCE, refresh, userinfo, introspection)."""

    def __init__(self, issuer: str, client_id: str, client_secret: str = "", redirect_uri: str = "", timeout: float = 10.0):
        self.issuer = issuer.rstrip("/")
        self.client_id = client_id
        self.client_secret = client_secret
        self.redirect_uri = redirect_uri
        self.timeout = timeout

    def authorize_url(self, state: str, nonce: str = "", scope: str = "openid profile email", code_challenge: str = "") -> str:
        q = {
            "response_type": "code",
            "client_id": self.client_id,
            "redirect_uri": self.redirect_uri,
            "scope": scope,
            "state": state,
        }
        if nonce:
            q["nonce"] = nonce
        if code_challenge:
            q["code_challenge"] = code_challenge
            q["code_challenge_method"] = "S256"
        return f"{self.issuer}/oauth2/authorize?{urllib.parse.urlencode(q)}"

    def _post(self, path: str, form: dict) -> dict:
        form = {k: v for k, v in form.items() if v is not None}
        form["client_id"] = self.client_id
        if self.client_secret:
            form["client_secret"] = self.client_secret
        req = urllib.request.Request(self.issuer + path, data=urllib.parse.urlencode(form).encode(), method="POST")
        req.add_header("Content-Type", "application/x-www-form-urlencoded")
        req.add_header("Accept", "application/json")
        return self._send(req)

    def _send(self, req: urllib.request.Request) -> dict:
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw, status = res.read(), res.status
        except urllib.error.HTTPError as e:
            raw, status = e.read(), e.code
        except (urllib.error.URLError, OSError) as e:
            raise TransportError(str(e)) from e
        body = json.loads(raw.decode("utf-8")) if raw else {}
        if status >= 400:
            raise OAuthError(body.get("error", "server_error"), body.get("error_description", ""), status)
        return body

    def exchange(self, code: str, code_verifier: str = "") -> dict:
        return self._post(
            "/oauth2/token",
            {
                "grant_type": "authorization_code",
                "code": code,
                "redirect_uri": self.redirect_uri,
                "code_verifier": code_verifier or None,
            },
        )

    def refresh(self, refresh_token: str) -> dict:
        return self._post("/oauth2/token", {"grant_type": "refresh_token", "refresh_token": refresh_token})

    def client_credentials(self, scope: str = "") -> dict:
        return self._post("/oauth2/token", {"grant_type": "client_credentials", "scope": scope})

    def introspect(self, token: str) -> dict:
        return self._post("/oauth2/introspect", {"token": token})

    def revoke(self, token: str) -> None:
        self._post("/oauth2/revoke", {"token": token})

    def userinfo(self, access_token: str) -> dict:
        req = urllib.request.Request(self.issuer + "/oauth2/userinfo")
        req.add_header("Authorization", "Bearer " + access_token)
        req.add_header("Accept", "application/json")
        return self._send(req)

    def logout_url(self, id_token: str = "", post_logout_redirect_uri: str = "", state: str = "") -> str:
        q = {"client_id": self.client_id}
        if id_token:
            q["id_token_hint"] = id_token
        if post_logout_redirect_uri:
            q["post_logout_redirect_uri"] = post_logout_redirect_uri
        if state:
            q["state"] = state
        return f"{self.issuer}/oauth2/logout?{urllib.parse.urlencode(q)}"
