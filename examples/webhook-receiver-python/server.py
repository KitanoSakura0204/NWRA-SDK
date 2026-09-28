"""Lounge Account webhook receiver on the standard library's http.server (no framework).

The endpoint itself is receiver.py, the business logic events.py. Configuration comes from the
environment; see README.md.
"""
from __future__ import annotations

import json
import logging
import os
import signal
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from events import dispatch
from receiver import MAX_BODY, Receiver, load_keys

log = logging.getLogger("lounge.webhook")


def _env_int(name: str, default: int) -> int:
    try:
        return int(os.environ.get(name, "") or default)
    except ValueError:
        return default


def make_handler(receiver: Receiver, path: str):
    class Handler(BaseHTTPRequestHandler):
        server_version = "lounge-webhook-receiver"
        timeout = 10  # seconds per socket read: a slow client cannot hold a thread forever

        def _reply(self, status: int, body: dict) -> None:
            data = json.dumps(body).encode("utf-8")
            self.send_response(status)
            if status == 405:
                self.send_header("Allow", "POST")
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def _serve(self) -> None:
            route = self.path.split("?", 1)[0]
            if route == "/healthz":
                return self._reply(200, {"ok": True})
            if route != path:
                return self._reply(404, {"error": "not found"})
            if self.command != "POST":
                return self._reply(*receiver.handle(self.command, dict(self.headers.items()), b""))
            try:
                length = int(self.headers.get("Content-Length", ""))
            except ValueError:
                return self._reply(411, {"error": "length required"})
            if length < 0 or length > MAX_BODY:
                return self._reply(413, {"error": "payload too large"})
            try:
                body = self.rfile.read(length)
            except OSError as exc:  # timeout or reset: nobody is left to answer
                log.warning("reading the body failed: %s", exc)
                return
            self._reply(*receiver.handle("POST", dict(self.headers.items()), body))

        do_GET = do_POST = do_PUT = do_DELETE = do_PATCH = _serve

        def log_message(self, fmt: str, *args) -> None:  # route access logs through logging
            log.debug("%s %s", self.address_string(), fmt % args)

    return Handler


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    secret = os.environ.get("LOUNGE_WEBHOOK_SECRET", "")
    if not secret:
        sys.exit("set LOUNGE_WEBHOOK_SECRET (admin console: 开放平台 -> Webhook -> 签名密钥)")
    try:
        keys = load_keys(os.environ.get("LOUNGE_DECRYPTION_KEYS", ""))
    except ValueError as exc:
        sys.exit("LOUNGE_DECRYPTION_KEYS: %s" % exc)
    if keys:
        try:  # the delivery helpers need it: fail at start, not on the first encrypted delivery
            import cryptography  # noqa: F401
        except ImportError:
            sys.exit("LOUNGE_DECRYPTION_KEYS is set: pip install cryptography")
    tolerance = _env_int("WEBHOOK_TOLERANCE", 300)
    receiver = Receiver(secret, dispatch, tolerance=tolerance, keys=keys, capacity=_env_int("SEEN_CAPACITY", 10000))
    host, port = os.environ.get("HOST", "0.0.0.0"), _env_int("PORT", 8092)
    path = os.environ.get("WEBHOOK_PATH", "/lounge/webhook")

    server = ThreadingHTTPServer((host, port), make_handler(receiver, path))
    server.daemon_threads = True
    # SIGTERM (systemd, docker stop): shutdown() must run outside the serve_forever thread.
    signal.signal(signal.SIGTERM, lambda *_: threading.Thread(target=server.shutdown, daemon=True).start())
    log.info("listening on http://%s:%d%s (timestamp tolerance %ds, %d decryption key(s))",
             host, port, path, tolerance, len(keys))
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
