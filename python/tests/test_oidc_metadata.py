"""Public OIDC metadata stays on the configured issuer and carries no credentials."""

import http.client
import io
import json
import unittest
import urllib.error
import urllib.request
from email.message import Message
from unittest.mock import Mock, patch

from nowork_sdk import NoworkOIDC, TransportError


ISSUER = "https://account.example"
LIMIT = 1024 * 1024
DISCOVERY = {
    "issuer": ISSUER,
    "authorization_endpoint": ISSUER + "/oauth2/authorize",
    "token_endpoint": ISSUER + "/oauth2/token",
    "jwks_uri": "https://untrusted.example/advertised-keys",
    "custom_extension": {"preserved": True},
}


class Reply(io.BytesIO):
    def __init__(self, body, status=200, headers=None):
        super().__init__(body if isinstance(body, bytes) else json.dumps(body).encode())
        self.status = self.code = status
        self.msg = "test response"
        self.headers = Message()
        for name, value in (headers or {}).items():
            self.headers[name] = value
        self.read_sizes = []

    def info(self):
        return self.headers

    def geturl(self):
        return ISSUER

    def read(self, size=-1):
        self.read_sizes.append(size)
        return super().read(size)


class FailingReadReply(Reply):
    def __init__(self, error):
        super().__init__(b"")
        self.error = error

    def read(self, size=-1):
        self.read_sizes.append(size)
        raise self.error


class OIDCMetadataTests(unittest.TestCase):
    def setUp(self):
        self.oidc = NoworkOIDC(ISSUER + "///", "client-id", "client-secret", "https://app.example/cb", 2.75)
        self.transport = patch("urllib.request.HTTPSHandler.https_open").start()
        self.addCleanup(patch.stopall)

    def test_discovery_and_jwks_use_fixed_public_gets_and_keep_extensions(self):
        jwks = {"keys": [{"kty": "RSA", "kid": "key-1", "custom": [1, 2]}], "extra": True}
        replies = [Reply(DISCOVERY), Reply(jwks)]
        self.transport.side_effect = replies
        self.assertEqual(self.oidc.discover(), DISCOVERY)
        self.assertEqual(self.oidc.jwks(), jwks)
        requests = [call.args[0] for call in self.transport.call_args_list]
        self.assertEqual([r.full_url for r in requests], [
            ISSUER + "/.well-known/openid-configuration", ISSUER + "/jwks.json"])
        for request, reply in zip(requests, replies):
            self.assertEqual(request.get_method(), "GET")
            self.assertIsNone(request.data)
            self.assertEqual(request.timeout, 2.75)
            self.assertEqual(request.get_header("Accept"), "application/json")
            headers = {name.lower(): value for name, value in request.header_items()}
            for name in ("authorization", "proxy-authorization", "cookie", "x-server-key"):
                self.assertNotIn(name, headers)
            self.assertNotIn("client-secret", repr((request.full_url, headers, request.data)))
            self.assertEqual(reply.read_sizes, [LIMIT + 1])
            self.assertTrue(reply.closed)

    def test_global_opener_credentials_are_isolated_but_environment_proxies_remain_supported(self):
        global_opener = Mock()
        global_opener.addheaders = [("Authorization", "Bearer global-token"), ("X-Server-Key", "server-key")]
        self.transport.return_value = Reply({"keys": []})
        with patch("urllib.request._opener", global_opener), patch("urllib.request.getproxies", return_value={
            "https": "http://proxy-user:proxy-secret@proxy.example:8080"}):
            self.assertEqual(self.oidc.jwks(), {"keys": []})
        global_opener.open.assert_not_called()
        request = self.transport.call_args.args[0]
        self.assertEqual(request.host, "proxy.example:8080")
        self.assertEqual(request._tunnel_host, "account.example")
        self.assertTrue(request.get_header("Proxy-authorization").startswith("Basic "))
        self.assertIsNone(request.get_header("Authorization"))
        self.assertIsNone(request.get_header("X-server-key"))
        self.assertEqual(global_opener.addheaders[0], ("Authorization", "Bearer global-token"))

    def test_invalid_metadata_issuers_fail_before_transport_without_disclosing_configuration(self):
        invalid = (
            ISSUER + "?client_secret=fixture", ISSUER + "?", ISSUER + "#fixture", ISSUER + "#",
            "https://user:fixture@account.example", "https://@account.example",
            "ftp://account.example", "file:///fixture", "account.example", "/tenant",
            "//account.example", "https:///tenant", "https://", "",
            "https://account.example:invalid", "https://account.example:65536", "https://[broken",
            " https://account.example", "https://account.example/\tfixture", "https://account.example\\fixture",
        )
        for configured in invalid:
            for method in ("discover", "jwks"):
                with self.subTest(issuer=configured, method=method):
                    # Capture all schemes, including malformed URLs, without any real network I/O.
                    with patch("urllib.request.OpenerDirector.open", return_value=Reply({
                        **DISCOVERY, "issuer": configured.rstrip("/"), "keys": [],
                    })) as transport:
                        with self.assertRaises(TransportError) as raised:
                            getattr(NoworkOIDC(configured, "client", "client-secret"), method)()
                        transport.assert_not_called()
                    self.assertEqual(str(raised.exception), "invalid metadata issuer URL")

    def test_metadata_preserves_valid_issuer_subpaths_case_and_trailing_slash_contract(self):
        for configured in (ISSUER, ISSUER + "/tenant/v1///", "HTTPS://Account.Example/tenant", "http://localhost:8080/tenant"):
            with self.subTest(issuer=configured):
                expected = configured.rstrip("/")
                document = dict(DISCOVERY, issuer=expected)
                with patch("urllib.request.OpenerDirector.open", side_effect=[Reply(document), Reply({"keys": []})]) as transport:
                    oidc = NoworkOIDC(configured, "client")
                    self.assertEqual(oidc.discover(), document)
                    self.assertEqual(oidc.jwks(), {"keys": []})
                self.assertEqual([call.args[0].full_url for call in transport.call_args_list], [
                    expected + "/.well-known/openid-configuration", expected + "/jwks.json"])

    def test_redirects_are_rejected_without_a_second_request(self):
        for status in (301, 302, 303, 307, 308):
            with self.subTest(status=status):
                reply = Reply(DISCOVERY, status, {"Location": "https://untrusted.example/discovery"})
                self.transport.reset_mock()
                self.transport.return_value = reply
                with self.assertRaises(TransportError):
                    self.oidc.discover()
                self.assertEqual(self.transport.call_count, 1)
                self.assertTrue(reply.closed)

    def test_non_2xx_and_network_errors_are_transport_errors(self):
        for status in (199, 300, 399, 400, 401, 503):
            with self.subTest(status=status):
                reply = Reply({"keys": [], "error": "invalid_client"}, status)
                self.transport.return_value = reply
                with self.assertRaises(TransportError):
                    self.oidc.jwks()
                self.assertTrue(reply.closed)
        for error in (urllib.error.URLError("offline"), TimeoutError("timeout")):
            self.transport.side_effect = error
            with self.assertRaises(TransportError):
                self.oidc.jwks()

    def test_all_2xx_statuses_with_valid_json_are_accepted(self):
        for status in (200, 201, 299):
            self.transport.return_value = Reply({"keys": []}, status)
            self.assertEqual(self.oidc.jwks(), {"keys": []})

    def test_rejects_malformed_json_nonobjects_and_nonstandard_numbers(self):
        for raw in (b"", b"<html>", b"\xff", b"null", b"[]", b"1", b"true",
                    b'{"keys": [], "bad": NaN}', b'{"keys": [], "bad": Infinity}'):
            for method in (self.oidc.discover, self.oidc.jwks):
                with self.subTest(raw=raw, method=method.__name__):
                    self.transport.return_value = Reply(raw)
                    with self.assertRaises(TransportError):
                        method()

    def test_discovery_requires_nonempty_string_fields_and_exact_normalized_issuer(self):
        for field in ("issuer", "authorization_endpoint", "token_endpoint", "jwks_uri"):
            for value in (None, "", "  ", False, 17, []):
                with self.subTest(field=field, value=value):
                    document = dict(DISCOVERY)
                    if value is None:
                        document.pop(field)
                    else:
                        document[field] = value
                    self.transport.return_value = Reply(document)
                    with self.assertRaises(TransportError):
                        self.oidc.discover()
        for issuer in (ISSUER + "/", "https://other.example", "HTTPS://account.example"):
            self.transport.return_value = Reply(dict(DISCOVERY, issuer=issuer))
            with self.assertRaisesRegex(TransportError, "issuer"):
                self.oidc.discover()

    def test_jwks_requires_an_array_of_objects(self):
        for document in ({}, {"keys": None}, {"keys": {}}, {"keys": "RSA"},
                         {"keys": [None]}, {"keys": [[]]}, {"keys": ["RSA"]}, {"keys": [True]}):
            with self.subTest(document=document):
                self.transport.return_value = Reply(document)
                with self.assertRaises(TransportError):
                    self.oidc.jwks()
        self.transport.return_value = Reply({"keys": [{}]})
        self.assertEqual(self.oidc.jwks(), {"keys": [{}]})

    def test_exact_limit_is_accepted_and_oversize_is_bounded_and_closed(self):
        prefix = b'{"keys": []}'
        for extra in (0, 1):
            reply = Reply(prefix + b" " * (LIMIT - len(prefix) + extra))
            self.transport.return_value = reply
            if extra:
                with self.assertRaisesRegex(TransportError, "1 MiB"):
                    self.oidc.jwks()
            else:
                self.assertEqual(self.oidc.jwks(), {"keys": []})
            self.assertEqual(reply.read_sizes, [LIMIT + 1])
            self.assertTrue(reply.closed)

    def test_http_error_body_is_bounded_and_closed(self):
        stream = Reply(b"x" * (LIMIT + 2))
        self.transport.side_effect = urllib.error.HTTPError(ISSUER, 503, "unavailable", {}, stream)
        with self.assertRaises(TransportError):
            self.oidc.jwks()
        self.assertTrue(stream.closed)
        self.assertTrue(all(0 <= size <= LIMIT + 1 for size in stream.read_sizes))

    def test_failed_success_response_read_is_transport_error_and_closes_stream(self):
        for method in (self.oidc.discover, self.oidc.jwks):
            for error in (OSError("response read failed"), http.client.IncompleteRead(b"partial", 20)):
                with self.subTest(method=method.__name__, error=type(error).__name__):
                    reply = FailingReadReply(error)
                    self.transport.return_value = reply
                    with self.assertRaises(TransportError) as caught:
                        method()
                    self.assertIs(caught.exception.__cause__, error)
                    self.assertEqual(reply.read_sizes, [LIMIT + 1])
                    self.assertTrue(reply.closed)

    def test_failed_http_error_body_read_is_transport_error_and_closes_stream(self):
        for method in (self.oidc.discover, self.oidc.jwks):
            for error in (OSError("error response read failed"), http.client.IncompleteRead(b"partial", 20)):
                with self.subTest(method=method.__name__, error=type(error).__name__):
                    reply = FailingReadReply(error)
                    self.transport.side_effect = urllib.error.HTTPError(ISSUER, 503, "unavailable", {}, reply)
                    with self.assertRaises(TransportError) as caught:
                        method()
                    self.assertIs(caught.exception.__cause__, error)
                    self.assertEqual(reply.read_sizes, [LIMIT + 1])
                    self.assertTrue(reply.closed)


if __name__ == "__main__":
    unittest.main()
