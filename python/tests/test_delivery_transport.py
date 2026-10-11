"""Offline transport regressions; real Backend EC vectors and test-only RSA envelopes."""

import base64
import io
import json
import os
import pathlib
import unittest
import urllib.error
import urllib.parse
from unittest.mock import Mock, patch

from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from nowork_sdk import ApiError, EncryptionError, NoworkClient, TransportError
from nowork_sdk.client import MAX_RESPONSE_BYTES
from nowork_sdk.delivery import DecryptError


VECTORS = json.loads((pathlib.Path(__file__).resolve().parents[2] / "go" / "testdata" / "vectors.json").read_text(encoding="utf-8"))
KEYS = VECTORS["keys"]
SERVER_VECTOR = next(v for v in VECTORS["vectors"] if v["kind"] == "server")
BASE = "https://account.example"


def b64(data):
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def rsa_envelope(data, ctx="server:survival:whitelist", *, kid="vec-rsa", plain=None):
    # The public half of the checked-in Backend TEST key, never a generated production key.
    def number(value):
        return int.from_bytes(base64.urlsafe_b64decode(value + "=" * (-len(value) % 4)), "big")

    key = rsa.RSAPublicNumbers(number(KEYS["rsa"]["e"]), number(KEYS["rsa"]["n"])).public_key()
    header = {"alg": "RSA-OAEP-256", "enc": "A256GCM"}
    if ctx is not None:
        header["ctx"] = ctx
    if kid is not None:
        header["kid"] = kid
    protected = b64(json.dumps(header).encode())
    cek, iv = os.urandom(32), os.urandom(12)
    wrapped = key.encrypt(cek, padding.OAEP(mgf=padding.MGF1(hashes.SHA256()), algorithm=hashes.SHA256(), label=None))
    ciphertext = AESGCM(cek).encrypt(iv, json.dumps(data).encode() if plain is None else plain, protected.encode())
    compact = ".".join((protected, b64(wrapped), b64(iv), b64(ciphertext[:-16]), b64(ciphertext[-16:])))
    return {"code": 0, "message": "ok", "data": {"jwe": compact}}


class Reply(io.BytesIO):
    def __init__(self, body, status=200, headers=None):
        super().__init__(body if isinstance(body, bytes) else json.dumps(body).encode())
        self.status = status
        self.headers = {} if headers is None else headers
        self.read_sizes = []

    def read(self, size=-1):
        self.read_sizes.append(size)
        return super().read(size)


def client_for(reply, *, keys=None, server_code=None, timeout=10.0):
    opener = Mock()
    opener.open.return_value = reply
    return NoworkClient(BASE + "/", "server-key-only", timeout, decryption_keys=keys,
                        server_code=server_code, opener=opener), opener


def wrong_ec_key():
    numbers = ec.derive_private_key(1, ec.SECP256R1()).private_numbers()
    return {"kty": "EC", "crv": "P-256", "kid": KEYS["ec"]["kid"],
            "x": b64(numbers.public_numbers.x.to_bytes(32, "big")),
            "y": b64(numbers.public_numbers.y.to_bytes(32, "big")),
            "d": b64(numbers.private_value.to_bytes(32, "big"))}


SERVER_CALLS = [
    ("whitelist", lambda c: c.whitelist().mode, {"mode": 1}, 1),
    ("whitelist/check", lambda c: c.check_whitelist("Player & One").allowed, {"allowed": True}, True),
    ("heartbeat", lambda c: c.heartbeat(2), {"status": "ok"}, None),
    ("modules", lambda c: c.modules(), {"modules": {"gm": True}}, {"gm": True}),
    ("gm/sso/exchange", lambda c: c.exchange_gm_code("code", "verifier", "https://gm.example/cb"), {"serverCode": "survival"}, {"serverCode": "survival"}),
    ("gm/actions", lambda c: c.publish_gm_actions([]), {"count": 2}, 2),
    ("gm/actions", lambda c: c.publish_gm_catalog([], {}).count, {"count": 3}, 3),
    ("gm/options", lambda c: c.publish_gm_options({}).count, {"count": 4}, 4),
    ("gm/commands", lambda c: c.poll_gm_commands(), [{"id": 7}], [{"id": 7}]),
    ("gm/commands/7/result", lambda c: c.report_gm_result(7, True), {}, None),
    ("chat/tickets", lambda c: c.issue_chat_ticket(uid=7), {"ticket": "one-use"}, {"ticket": "one-use"}),
    ("chat/relay", lambda c: c.poll_chat_relay(), {"items": [{"id": 7}]}, [{"id": 7}]),
    ("chat/relay/ack", lambda c: c.ack_chat_relay([7]), {"updated": 1}, 1),
    ("chat/messages", lambda c: c.post_chat_message("Steve", "Hello"), {"id": 7}, {"id": 7}),
    ("users/schema", lambda c: c.user_schema().max_user_bytes, {"maxUserBytes": 1024}, 1024),
    ("users/7/data", lambda c: c.user_data(7).version, {"uid": 7, "version": 2}, 2),
    ("users/7/data", lambda c: c.put_user_data(7, {}, 2).version, {"uid": 7, "version": 3}, 3),
    ("users/7/data", lambda c: c.patch_user_data(7, {}, 3).version, {"uid": 7, "version": 4}, 4),
    ("bans", lambda c: c.bans()[0].reason, {"bans": [{"uid": 7, "reason": "test"}]}, "test"),
    ("bans/kick", lambda c: c.kick_list()[0].name, {"players": [{"name": "Steve"}]}, "Steve"),
    ("bans", lambda c: c.ban(uid=7)[0], {"uid": 7}, 7),
    ("bans/lift", lambda c: c.unban(uid=7), {"uid": 7}, 7),
]


class EncryptedTransportTests(unittest.TestCase):
    def test_backend_vector_is_opened_even_when_a_proxy_drops_the_header(self):
        for headers in ({}, {"x-nOwOrK-eNcRyPtIoN": "JWE"}):
            with self.subTest(headers=headers):
                reply = Reply(json.loads(SERVER_VECTOR["response"]), headers=headers)
                client, _ = client_for(reply, keys=KEYS["ec"], server_code="survival")
                result = client.whitelist()
                self.assertEqual((result.server, result.mode, result.count), ("survival", 1, 1))
                self.assertEqual((result.entries[0].name, result.entries[0].nickname), ("Steve", "旅行者"))
                self.assertTrue(reply.closed)

    def test_every_server_method_opens_an_encrypted_reply_including_void_and_list_results(self):
        for route, call, data, expected in SERVER_CALLS:
            with self.subTest(route=route, data=data):
                client, _ = client_for(Reply(rsa_envelope(data, "server:survival:" + route)),
                                       keys=KEYS["rsa"], server_code="survival")
                self.assertEqual(call(client), expected)

    def test_every_server_method_refuses_encrypted_data_without_keys(self):
        for route, call, _, _ in SERVER_CALLS:
            with self.subTest(route=route):
                client, _ = client_for(Reply(json.loads(SERVER_VECTOR["response"])))
                with self.assertRaises(EncryptionError) as caught:
                    call(client)
                self.assertEqual(caught.exception.route, route)
                self.assertEqual(caught.exception.reason, "no decryption key configured")
                self.assertNotIsInstance(caught.exception, TransportError)

    def test_only_the_two_backend_sealed_routes_refuse_plaintext_with_keys(self):
        for route, call, data, expected in SERVER_CALLS:
            with self.subTest(route=route, data=data):
                client, _ = client_for(Reply({"code": 0, "data": data}), keys=KEYS["rsa"], server_code="survival")
                if route in ("whitelist", "whitelist/check"):
                    with self.assertRaises(EncryptionError):
                        call(client)
                else:
                    with patch("nowork_sdk.delivery._crypto", side_effect=AssertionError("plaintext needs no crypto")):
                        self.assertEqual(call(client), expected)

    def test_unconfigured_keys_keep_plain_whitelist_support(self):
        for keys in (None, [], ()):
            with self.subTest(keys=keys):
                client, _ = client_for(Reply({"code": 0, "data": {"mode": 0}}), keys=keys)
                self.assertEqual(client.whitelist().mode, 0)

    def test_rotation_generator_and_caller_mutation_do_not_lose_keys_between_requests(self):
        supplied = [dict(KEYS["ec"]), dict(KEYS["rsa"])]
        client, opener = client_for(None, keys=(key for key in supplied), server_code="survival")
        opener.open.side_effect = [
            Reply(json.loads(SERVER_VECTOR["response"])),
            Reply(rsa_envelope({"mode": 2})),
            Reply(rsa_envelope({"mode": 0}, kid=None)),
        ]
        for key in supplied:
            key.clear()
        self.assertEqual([client.whitelist().mode for _ in range(3)], [1, 2, 0])

    def test_matching_kid_rotation_tries_the_next_key_after_a_wrong_private_key(self):
        client, _ = client_for(Reply(json.loads(SERVER_VECTOR["response"])),
                               keys=[wrong_ec_key(), KEYS["ec"]], server_code="survival")
        self.assertEqual(client.whitelist().mode, 1)

    def test_server_key_query_and_positional_timeout_are_preserved_without_sending_private_keys(self):
        client, opener = client_for(Reply(rsa_envelope({"allowed": True}, "server:survival:whitelist/check")),
                                    keys=KEYS["rsa"], server_code="survival", timeout=2.75)
        self.assertTrue(client.check_whitelist("Player & One").allowed)
        request = opener.open.call_args.args[0]
        self.assertEqual(opener.open.call_args.kwargs, {"timeout": 2.75})
        self.assertEqual(request.get_header("X-server-key"), "server-key-only")
        self.assertIsNone(request.get_header("Authorization"))
        self.assertIsNone(request.data)
        self.assertEqual(urllib.parse.parse_qs(urllib.parse.urlsplit(request.full_url).query), {"name": ["Player & One"]})
        sent = repr((request.full_url, request.header_items(), request.data))
        for field in ("d", "p", "q"):
            self.assertNotIn(KEYS["rsa"][field], sent)

    def test_backend_ciphertext_cannot_be_replayed_on_another_route_or_expected_server(self):
        for server, call in (("other", lambda c: c.whitelist()), ("survival", lambda c: c.check_whitelist("Steve"))):
            with self.subTest(server=server):
                client, _ = client_for(Reply(json.loads(SERVER_VECTOR["response"])), keys=KEYS["ec"], server_code=server)
                with self.assertRaisesRegex(EncryptionError, "ctx mismatch"):
                    call(client)

    def test_unpinned_server_still_requires_authenticated_kind_nonempty_server_and_exact_route(self):
        contexts = [None, 17, {}, "", "server::whitelist", "webhook:survival:whitelist",
                    "server:survival:whitelist/check", "server:survival:extra:whitelist"]
        for ctx in contexts:
            with self.subTest(ctx=ctx):
                client, _ = client_for(Reply(rsa_envelope({"mode": 0}, ctx)), keys=KEYS["rsa"])
                with self.assertRaisesRegex(EncryptionError, "ctx mismatch"):
                    client.whitelist()
        client, _ = client_for(Reply(rsa_envelope({"mode": 1}, "server:other:whitelist")), keys=KEYS["rsa"])
        self.assertEqual(client.whitelist().mode, 1)

    def test_query_is_not_part_of_the_authenticated_route(self):
        client, _ = client_for(Reply(json.loads(SERVER_VECTOR["response"])), keys=KEYS["ec"], server_code="survival")
        self.assertEqual(client._call("GET", "/server/v1/whitelist?format=full")["mode"], 1)

    def test_raw_retains_original_envelope_and_never_decrypts_or_applies_downgrade_policy(self):
        for body in (SERVER_VECTOR["response"].encode(), b'{"code":0,"data":{"mode":0}}'):
            with self.subTest(encrypted=b'"jwe"' in body):
                client, _ = client_for(Reply(body), keys=KEYS["ec"], server_code="wrong-on-purpose")
                with patch("nowork_sdk.client.decrypt_jwe", side_effect=AssertionError("raw must not decrypt")):
                    result = client.raw("GET", "/server/v1/whitelist")
                self.assertEqual(result.body, body)
                self.assertEqual(result.envelope, json.loads(body))
                self.assertEqual(result.data, json.loads(body)["data"])
                self.assertEqual(result.encrypted, b'"jwe"' in body)

    def test_wrong_key_unknown_kid_and_corrupt_ciphertext_are_encryption_errors(self):
        compact = SERVER_VECTOR["compact"]
        parts = compact.split(".")
        parts[3] = ("A" if parts[3][0] != "A" else "B") + parts[3][1:]
        cases = [
            (compact, KEYS["rsa"]),
            (compact, wrong_ec_key()),
            (".".join(parts), KEYS["ec"]),
            (rsa_envelope({"mode": 0}, kid="unknown")["data"]["jwe"], KEYS["rsa"]),
            ("not-a-jwe", KEYS["rsa"]),
            (b64(b"null") + ".AA.AA.AA.AA", KEYS["rsa"]),
            (b64(b"[]") + ".AA.AA.AA.AA", KEYS["rsa"]),
        ]
        for token, keys in cases:
            with self.subTest(token=token[:24], kty=keys["kty"]):
                client, _ = client_for(Reply({"code": 0, "data": {"jwe": token}}), keys=keys)
                with self.assertRaises(EncryptionError) as caught:
                    client.whitelist()
                self.assertNotIsInstance(caught.exception, TransportError)

    def test_malformed_jwe_singleton_never_becomes_a_disabled_whitelist(self):
        for value in (None, 17, {}, [], False, ""):
            for keys in (None, KEYS["ec"]):
                with self.subTest(value=value, configured=keys is not None):
                    client, _ = client_for(Reply({"code": 0, "data": {"jwe": value}}), keys=keys)
                    with self.assertRaises(EncryptionError):
                        client.whitelist()

    def test_marked_replies_need_a_canonical_jwe_envelope(self):
        for data in (None, [], {}, {"mode": 0}, {"jwe": SERVER_VECTOR["compact"], "mode": 0}):
            with self.subTest(data_type=type(data).__name__):
                client, _ = client_for(Reply({"code": 0, "data": data}, headers={"X-NoWork-Encryption": "JWE"}), keys=KEYS["ec"])
                with self.assertRaises(EncryptionError):
                    client.whitelist()
        client, _ = client_for(Reply({"code": 0, "data": {"jwe": "ordinary-field", "mode": 1}}))
        self.assertEqual(client.whitelist().mode, 1)

    def test_invalid_decrypted_json_and_top_level_shapes_are_encryption_errors(self):
        for plain in (b"not json", b"null", b"[]", b"true", b"17", b'{"mode":NaN}', b'{"mode":Infinity}'):
            with self.subTest(plain=plain):
                client, _ = client_for(Reply(rsa_envelope({}, plain=plain)), keys=KEYS["rsa"], server_code="survival")
                with self.assertRaises(EncryptionError):
                    client.whitelist()

    def test_encrypted_whitelist_requires_an_explicit_valid_mode(self):
        for data in ({}, {"mode": None}, {"mode": False}, {"mode": "0"}, {"mode": 0.0}, {"mode": -1}, {"mode": 3}):
            with self.subTest(data=data):
                client, _ = client_for(Reply(rsa_envelope(data)), keys=KEYS["rsa"])
                with self.assertRaises(EncryptionError):
                    client.whitelist()
        client, _ = client_for(Reply(rsa_envelope({"mode": 0})), keys=KEYS["rsa"])
        self.assertEqual(client.whitelist().mode, 0)
        legacy, _ = client_for(Reply({"code": 0, "data": {}}))
        self.assertEqual(legacy.whitelist().mode, 0)

    def test_encrypted_join_check_requires_an_explicit_boolean_allowed(self):
        for data in ({}, {"allowed": None}, {"allowed": "false"}, {"allowed": "true"}, {"allowed": 0}, {"allowed": 1}):
            with self.subTest(data=data):
                client, _ = client_for(Reply(rsa_envelope(data, "server:survival:whitelist/check")), keys=KEYS["rsa"])
                with self.assertRaises(EncryptionError):
                    client.check_whitelist("Steve")
        for allowed in (False, True):
            client, _ = client_for(Reply(rsa_envelope({"allowed": allowed}, "server:survival:whitelist/check")), keys=KEYS["rsa"])
            self.assertEqual(client.check_whitelist("Steve").allowed, allowed)

    def test_bad_configuration_fails_locally_without_loading_crypto(self):
        with patch("nowork_sdk.delivery._crypto", side_effect=AssertionError("configuration must not load crypto")):
            for keys in ({}, "a JSON string is not a mapping", [None], ["key"], 17):
                with self.subTest(keys=keys), self.assertRaises(ValueError):
                    NoworkClient(BASE, "server-key", decryption_keys=keys)
            for code in ("", "has:colon", "has space", "line\nbreak", False):
                with self.subTest(code=code), self.assertRaises(ValueError):
                    NoworkClient(BASE, "server-key", server_code=code)

    def test_business_http_and_network_errors_keep_their_original_categories(self):
        for status, code, category in ((200, 403, ApiError), (403, 403, ApiError), (503, 0, TransportError)):
            with self.subTest(status=status, code=code):
                body = {"code": code, "message": "server refused", "data": {"jwe": "unreadable"}}
                client, _ = client_for(Reply(body, status, {"X-NoWork-Encryption": "JWE"}), keys=KEYS["rsa"])
                with patch("nowork_sdk.client.decrypt_jwe", side_effect=AssertionError("errors precede crypto")):
                    with self.assertRaises(category) as caught:
                        client.whitelist()
                if category is ApiError:
                    self.assertEqual((caught.exception.code, caught.exception.message), (403, "server refused"))
        for error in (urllib.error.URLError("offline"), TimeoutError("timed out")):
            client, opener = client_for(None, keys=KEYS["rsa"])
            opener.open.side_effect = error
            with self.assertRaises(TransportError):
                client.whitelist()

    def test_http_error_stream_is_closed_and_api_error_precedes_crypto(self):
        stream = io.BytesIO(b'{"code":403,"message":"invalid key","data":{"jwe":"opaque"}}')
        error = urllib.error.HTTPError(BASE, 403, "Forbidden", {"X-NoWork-Encryption": "JWE"}, stream)
        client, opener = client_for(None, keys=KEYS["rsa"])
        opener.open.side_effect = error
        with self.assertRaises(ApiError) as caught:
            client.whitelist()
        self.assertEqual(caught.exception.code, 403)
        self.assertTrue(stream.closed)

    def test_wire_limit_accepts_exact_limit_and_rejects_oversize_before_decrypting(self):
        prefix = b'{"code":0,"data":{"jwe":"opaque"}}'
        for extra in (0, 1):
            with self.subTest(extra=extra):
                body = prefix + b" " * (MAX_RESPONSE_BYTES - len(prefix) + extra)
                reply = Reply(body, headers={"X-NoWork-Encryption": "JWE"})
                client, _ = client_for(reply, keys=KEYS["rsa"])
                with patch("nowork_sdk.client.decrypt_jwe", side_effect=AssertionError("bounded transport precedes crypto")):
                    if extra:
                        with self.assertRaisesRegex(TransportError, "response exceeds 8 MiB"):
                            client.whitelist()
                    else:
                        self.assertEqual(len(client.raw("GET", "/server/v1/whitelist").body), MAX_RESPONSE_BYTES)
                self.assertEqual(reply.read_sizes, [MAX_RESPONSE_BYTES + 1])
                self.assertTrue(reply.closed)

    def test_http_error_bodies_are_also_bounded_and_closed(self):
        stream = Reply(b"x" * (MAX_RESPONSE_BYTES + 2))
        error = urllib.error.HTTPError(BASE, 503, "Unavailable", {"X-NoWork-Encryption": "JWE"}, stream)
        client, opener = client_for(None, keys=KEYS["rsa"])
        opener.open.side_effect = error
        with self.assertRaisesRegex(TransportError, "response exceeds 8 MiB"):
            client.whitelist()
        self.assertEqual(stream.read_sizes, [MAX_RESPONSE_BYTES + 1])
        self.assertTrue(stream.closed)

    def test_missing_optional_crypto_dependency_is_an_encryption_error(self):
        client, _ = client_for(Reply(json.loads(SERVER_VECTOR["response"])), keys=KEYS["ec"])
        with patch("nowork_sdk.delivery._crypto", side_effect=DecryptError("install cryptography")):
            with self.assertRaises(EncryptionError) as caught:
                client.whitelist()
        self.assertIsInstance(caught.exception.__cause__, DecryptError)


if __name__ == "__main__":
    unittest.main()
