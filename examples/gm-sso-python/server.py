"""Runnable GM relying party: browser-bound PKCE start/callback and a local session.
Run behind an HTTPS reverse proxy; see README. Standard library + nowork_sdk only.
"""
from __future__ import annotations

import hmac
import json
import os
import secrets
import threading
import time
from http.cookies import SimpleCookie
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlencode, urlsplit

from nowork_sdk import NoWorkError, NoworkClient, create_pkce_pair, create_state, parse_login_callback


class Sessions:
    def __init__(self, limit=1024):
        self.limit = limit
        self.items = {}
        self.lock = threading.Lock()

    def put(self, value, ttl):
        with self.lock:
            now = time.time()
            self.items = {key: item for key, item in self.items.items() if item[0] > now}
            if len(self.items) >= self.limit:
                raise ValueError("session capacity reached")
            key = secrets.token_urlsafe(32)
            self.items[key] = (now + ttl, value)
            return key

    def get(self, key, consume=False):
        with self.lock:
            entry = self.items.pop(key, None) if consume else self.items.get(key)
            return entry[1] if entry and entry[0] > time.time() else None


def create_handler(center: str, public: str, server_id: int, server_key: str, *, allow_loopback_http=False):
    for base in (center, public):
        url = urlsplit(base)
        local_http = allow_loopback_http and url.scheme == "http" and url.hostname in ("127.0.0.1", "localhost", "::1")
        if (url.scheme != "https" and not local_http) or not url.hostname or url.username or url.password or url.query or url.fragment:
            raise ValueError("configured URLs require HTTPS without credentials, queries or fragments")
    if server_id <= 0 or not server_key:
        raise ValueError("server ID and key are required")
    center, public = center.rstrip("/"), public.rstrip("/")
    if urlsplit(public).path:
        raise ValueError("this example expects a GM origin without a path prefix")
    callback = public + "/callback"
    secure = urlsplit(public).scheme == "https"
    pending, active = Sessions(), Sessions()
    client = NoworkClient(center, server_key)

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, fmt, *args):
            # Authorization codes and gm_request must not enter access logs.
            return

        def cookie(self, name):
            try:
                cookies = SimpleCookie(self.headers.get("Cookie", ""))
                return cookies[name].value if name in cookies else ""
            except Exception:
                return ""

        def reply(self, status, data=None, *, location=None, cookies=()):
            body = json.dumps(data or {}, ensure_ascii=False).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Referrer-Policy", "no-referrer")
            self.send_header("X-Content-Type-Options", "nosniff")
            if location: self.send_header("Location", location)
            for name, value, ttl in cookies:
                suffix = "; Secure" if secure else ""
                self.send_header("Set-Cookie", f"{name}={value}; Path=/; Max-Age={ttl}; HttpOnly; SameSite=Lax{suffix}")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def do_GET(self):
            url = urlsplit(self.path)
            query = parse_qs(url.query, keep_blank_values=True)
            if url.path == "/start":
                request = query.get("gm_request", [""])
                selected_server = query.get("gm_server_id", [""])
                if (len(request) != 1 or not request[0] or len(request[0]) > 512 or
                        selected_server != [str(server_id)]):
                    self.reply(400, {"error": "invalid launch request"})
                    return
                pkce, state = create_pkce_pair(), create_state()
                try:
                    pending_id = pending.put({"verifier": pkce.verifier, "state": state}, 180)
                except ValueError:
                    self.reply(503, {"error": "try again later"})
                    return
                location = center + "/home/gm/authorize?" + urlencode({
                    "request": request[0], "state": state, "code_challenge": pkce.challenge,
                    "code_challenge_method": "S256",
                })
                self.reply(303, location=location, cookies=(("gm_pending", pending_id, 180),))
                return
            if url.path == "/callback":
                attempt = pending.get(self.cookie("gm_pending"), consume=True)
                if attempt is None:
                    self.reply(403, {"error": "login expired; launch again from your account"})
                    return
                try:
                    code = parse_login_callback(public + self.path, attempt["state"])
                    session = client.exchange_gm_code(code, attempt["verifier"], callback)
                    character = session.get("character", {})
                    if (not isinstance(character, dict) or session.get("serverId") != server_id or session.get("scope") != "character:login" or
                            type(character.get("id")) is not int or character["id"] <= 0 or
                            not hmac.compare_digest(str(session.get("subject", "")), f"gm-role:{character['id']}")):
                        raise ValueError("unexpected character identity")
                    session_id = active.put(session, 900)
                except (NoWorkError, ValueError, TypeError, KeyError):
                    self.reply(403, {"error": "login failed; launch again from your account"}, cookies=(("gm_pending", "", 0),))
                    return
                self.reply(303, location="/me", cookies=(("gm_pending", "", 0), ("gm_session", session_id, 900)))
                return
            if url.path == "/me":
                session = active.get(self.cookie("gm_session"))
                self.reply(200 if session else 401, session or {"error": "launch this GM from your account"})
                return
            self.reply(404, {"error": "not found"})

    return Handler


if __name__ == "__main__":
    handler = create_handler(os.environ["NOWORK_BASE_URL"], os.environ["GM_PUBLIC_URL"],
                             int(os.environ["NOWORK_SERVER_ID"]), os.environ["NOWORK_SERVER_KEY"],
                             allow_loopback_http=os.getenv("GM_ALLOW_LOOPBACK_HTTP") == "1")
    ThreadingHTTPServer(("127.0.0.1", int(os.getenv("PORT", "8088"))), handler).serve_forever()
