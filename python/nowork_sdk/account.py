"""User portal API. Authentication is the site's login JWT, never X-Server-Key."""
from __future__ import annotations

import json
import math
import urllib.error
import urllib.request
from typing import Any

from .errors import ApiError, TransportError


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


class _JSONClient:
    _prefix = "/api/account"
    _scheme = "Bearer"

    def __init__(self, base: str, token: str, timeout: float = 10.0, *, opener=None):
        if not math.isfinite(timeout) or timeout <= 0:
            raise ValueError("timeout must be positive")
        self.base, self.token, self.timeout = base.rstrip("/"), token, timeout
        self._opener = opener or urllib.request.build_opener(_NoRedirect())

    def _call(self, method: str, path: str, body: Any = None, *, allow_empty: bool = False) -> Any:
        try:
            encoded = json.dumps(body, allow_nan=False).encode() if body is not None else None
            request = urllib.request.Request(self.base + self._prefix + path, data=encoded, method=method,
                headers={**({"Authorization": self._scheme + " " + self.token} if self.token else {}), "Accept": "application/json",
                         **({"Content-Type": "application/json"} if encoded is not None else {})})
            try:
                with self._opener.open(request, timeout=self.timeout) as response:
                    raw, status = response.read(), response.status
            except urllib.error.HTTPError as error:
                try: raw, status = error.read(), error.code
                finally: error.close()
            env = json.loads(raw)
        except (OSError, ValueError, TypeError) as error:
            raise TransportError("API request failed") from error
        if not isinstance(env, dict) or type(env.get("code")) is not int:
            raise TransportError("invalid API envelope")
        if env["code"] != 0:
            raise ApiError(env["code"], str(env.get("message", "")))
        if status >= 300 or (env.get("data") is None and not allow_empty):
            raise TransportError(f"invalid API response ({status})")
        return env.get("data")


class AccountClient(_JSONClient):
    """Player portal authenticated with a site login JWT."""

    def servers(self) -> list:
        return self._call("GET", "/portal/servers")

    def whitelist_status(self, server_id: int) -> dict:
        return self._call("GET", f"/portal/servers/{_id(server_id)}/whitelist-status")

    def whitelist_applications(self) -> list:
        return self._call("GET", "/whitelist/applications")

    def apply_whitelist(self, server_id: int, role_name: str, role_uuid: str = "", remark: str = "") -> dict:
        return self._call("POST", f"/portal/servers/{_id(server_id)}/whitelist-apply",
                          {"roleName": role_name, "roleUuid": role_uuid, "remark": remark})

    def bind_role(self, server_id: int, role_name: str, role_uuid: str = "", remark: str = "") -> dict:
        return self._call("POST", f"/portal/servers/{_id(server_id)}/roles",
                          {"roleName": role_name, "roleUuid": role_uuid, "remark": remark})

    def launch_gm(self, server_id: int, role_id: int) -> dict:
        return self._call("POST", f"/gm/servers/{_id(server_id)}/launch", {"roleId": _id(role_id)})


def _id(value: int) -> int:
    if type(value) is not int or value <= 0:
        raise ValueError("id must be a positive integer")
    return value
