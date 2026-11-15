from __future__ import annotations

import base64
import json
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, Optional

from .entitlements import Entitlement, EntitlementValues, entitlement_body, entitlement_query
from .errors import ApiError, OAuthError, TransportError


_MAX_METADATA_BYTES = 1024 * 1024


class _NoMetadataRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def _reject_json_constant(value: str) -> None:
    raise ValueError("invalid JSON constant: " + value)


class NoworkOIDC:
    """Minimal OIDC relying-party helper (authorization code + PKCE, refresh, userinfo, introspection)."""

    def __init__(self, issuer: str, client_id: str, client_secret: str = "", redirect_uri: str = "", timeout: float = 10.0):
        self.issuer = issuer.rstrip("/")
        self.client_id = client_id
        self.client_secret = client_secret
        self.redirect_uri = redirect_uri
        self.timeout = timeout

    def discover(self) -> dict:
        """Read public discovery from the configured issuer; advertised URLs are not followed."""
        body = self._metadata("/.well-known/openid-configuration")
        for field in ("issuer", "authorization_endpoint", "token_endpoint", "jwks_uri"):
            if not isinstance(body.get(field), str) or not body[field].strip():
                raise TransportError("invalid discovery: missing or invalid " + field)
        if body["issuer"] != self.issuer:
            raise TransportError("discovery issuer mismatch")
        return body

    def jwks(self) -> dict:
        """Read public signing keys from the configured issuer's fixed /jwks.json path."""
        body = self._metadata("/jwks.json")
        if not isinstance(body.get("keys"), list) or any(not isinstance(key, dict) for key in body["keys"]):
            raise TransportError("invalid JWKS: keys must be an array of objects")
        return body

    def _metadata(self, path: str) -> dict:
        req = urllib.request.Request(self.issuer + path, method="GET", headers={"Accept": "application/json"})
        # A fresh opener has no global login/auth/cookie handlers or shared mutable headers.
        # urllib's default proxy handling, including configured proxy auth, stays available.
        opener = urllib.request.build_opener(_NoMetadataRedirect())
        try:
            with opener.open(req, timeout=self.timeout) as res:
                raw, status = res.read(_MAX_METADATA_BYTES + 1), int(res.status)
        except urllib.error.HTTPError as exc:
            try:
                raw, status = exc.read(_MAX_METADATA_BYTES + 1), exc.code
            finally:
                exc.close()
        except (urllib.error.URLError, OSError) as exc:
            raise TransportError(str(exc)) from exc
        if len(raw) > _MAX_METADATA_BYTES:
            raise TransportError("metadata response exceeds 1 MiB")
        if not 200 <= status < 300:
            raise TransportError(f"http {status}")
        try:
            body = json.loads(raw.decode("utf-8"), parse_constant=_reject_json_constant)
        except ValueError as exc:
            raise TransportError(f"invalid json (http {status}): {exc}") from exc
        if not isinstance(body, dict):
            raise TransportError(f"invalid response (http {status}): not an object")
        return body

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

    def _post(self, path: str, form: dict, allow_empty: bool = False) -> dict:
        form = {k: v for k, v in form.items() if v is not None}
        form["client_id"] = self.client_id
        if self.client_secret:
            form["client_secret"] = self.client_secret
        req = urllib.request.Request(self.issuer + path, data=urllib.parse.urlencode(form).encode(), method="POST")
        req.add_header("Content-Type", "application/x-www-form-urlencoded")
        req.add_header("Accept", "application/json")
        return self._send(req, allow_empty)

    def _send(self, req: urllib.request.Request, allow_empty: bool = False) -> dict:
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw, status = res.read(), int(getattr(res, "status", 200) or 200)
        except urllib.error.HTTPError as e:
            try:
                raw, status = e.read(), e.code
            finally:
                e.close()
        except (urllib.error.URLError, OSError) as e:
            raise TransportError(str(e)) from e
        if status >= 400:
            # RFC 6749 error object; anything else is a gateway / transport problem.
            try:
                body = json.loads(raw.decode("utf-8")) if raw else {}
            except ValueError:
                body = {}
            if isinstance(body, dict) and body.get("error"):
                raise OAuthError(str(body["error"]), str(body.get("error_description", "")), status)
            raise TransportError(f"http {status}")
        if not raw:
            # only revocation is allowed to answer with an empty 200 body
            if allow_empty:
                return {}
            raise TransportError(f"empty response (http {status})")
        try:
            body = json.loads(raw.decode("utf-8"))
        except ValueError as e:
            raise TransportError(f"invalid json (http {status}): {e}") from e
        if not isinstance(body, dict):
            raise TransportError(f"invalid response (http {status}): not an object")
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
        self._post("/oauth2/revoke", {"token": token}, allow_empty=True)

    def userinfo(self, access_token: str) -> dict:
        req = urllib.request.Request(self.issuer + "/oauth2/userinfo")
        req.add_header("Authorization", "Bearer " + access_token)
        req.add_header("Accept", "application/json")
        return self._send(req)

    # ---- Custom per-app values (DELIVERY.md section 8, confidential clients only) ----

    def entitlements(self, uid: int = 0, sub: str = "") -> EntitlementValues:
        """Every custom value this OAuth client stored for a user (HTTP Basic credentials)."""
        return EntitlementValues.from_dict(self._delivery("GET", entitlement_query(uid, sub), None))

    def put_entitlement(self, uid: int = 0, sub: str = "", key: str = "", value: Any = None) -> Entitlement:
        """Store one key for a user; the key/value limits are those of the server route."""
        return Entitlement.from_dict(self._delivery("PUT", None, entitlement_body(uid, sub, key, value)))

    def delete_entitlement(self, uid: int = 0, sub: str = "", key: str = "") -> Entitlement:
        """Remove one key (removing a key that was never written is not an error)."""
        q = entitlement_query(uid, sub)
        q["key"] = (key or "").strip()
        if not q["key"]:
            raise ValueError("entitlement key is required")
        return Entitlement.from_dict(self._delivery("DELETE", q, None))

    def _delivery(self, method: str, query: Optional[Dict[str, Any]], body: Any) -> Dict[str, Any]:
        """Talk to /oauth2/delivery/entitlements with HTTP Basic client credentials.

        Uses the same envelope split as the Server API: code != 0 is an ApiError (a 401 invalid
        client credentials stays one), transport trouble a TransportError.
        """
        url = self.issuer + "/oauth2/delivery/entitlements"
        if query:
            url += "?" + urllib.parse.urlencode({k: v for k, v in query.items() if v not in (None, "")})
        try:
            data = json.dumps(body, allow_nan=False).encode() if body is not None else None
        except (TypeError, ValueError) as e:
            raise TransportError(f"cannot encode the request body: {e}") from e
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Accept", "application/json")
        req.add_header("Authorization", "Basic " + base64.b64encode(
            f"{self.client_id}:{self.client_secret}".encode()).decode())
        if data is not None:
            req.add_header("Content-Type", "application/json")
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw, status = res.read(), int(getattr(res, "status", 200) or 200)
        except urllib.error.HTTPError as e:
            try:
                raw, status = e.read(), e.code
            finally:
                e.close()
        except (urllib.error.URLError, OSError) as e:
            raise TransportError(str(e)) from e
        try:
            env = json.loads(raw.decode("utf-8")) if raw else {}
        except ValueError as e:
            raise TransportError(f"invalid json (http {status}): {e}") from e
        if not isinstance(env, dict):
            raise TransportError(f"invalid response (http {status}): not an object")
        code = env.get("code")
        if isinstance(code, bool) or not isinstance(code, int):
            raise TransportError(f"invalid response (http {status}): missing code")
        if code != 0:
            raise ApiError(code, str(env.get("message", "")))
        if status >= 400:
            raise TransportError(f"http {status}")
        data = env.get("data")
        if not isinstance(data, dict):
            raise TransportError(f"invalid response (http {status}): missing data")
        return data

    def logout_url(self, id_token: str = "", post_logout_redirect_uri: str = "", state: str = "") -> str:
        q = {"client_id": self.client_id}
        if id_token:
            q["id_token_hint"] = id_token
        if post_logout_redirect_uri:
            q["post_logout_redirect_uri"] = post_logout_redirect_uri
        if state:
            q["state"] = state
        return f"{self.issuer}/oauth2/logout?{urllib.parse.urlencode(q)}"
