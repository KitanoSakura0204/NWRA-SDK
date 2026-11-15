"""Real signed and nested encrypted JWTs exercise optional nonce binding."""

import base64
import json
import os
import time
import unittest

try:
    from cryptography.hazmat.primitives import hashes
    from cryptography.hazmat.primitives.asymmetric import padding, rsa
    from cryptography.hazmat.primitives.ciphers.aead import AESGCM
    HAS_CRYPTO = True
except ImportError:
    HAS_CRYPTO = False

from nowork_sdk.delivery import DecryptError, decrypt_id_token, verify_id_token


ISSUER, AUDIENCE, NONCE = "https://account.example", "app-client", "session-nonce"
NOW = 1800000000


def b64(data):
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def number(value):
    return b64(value.to_bytes((value.bit_length() + 7) // 8, "big"))


@unittest.skipUnless(HAS_CRYPTO, "optional cryptography package is not installed")
class OIDCNonceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        private = cls.key.private_numbers()
        cls.private_jwk = {"kty": "RSA", "kid": "test-key", "n": number(private.public_numbers.n),
                           "e": number(private.public_numbers.e), "d": number(private.d),
                           "p": number(private.p), "q": number(private.q)}
        cls.jwks = {"keys": [{k: cls.private_jwk[k] for k in ("kty", "kid", "n", "e")}]}

    def token(self, claims=None):
        if claims is None:
            claims = self.claims()
        protected = b64(b'{"alg":"RS256","kid":"test-key"}')
        payload = b64(json.dumps(claims).encode())
        signing_input = (protected + "." + payload).encode()
        signature = self.key.sign(signing_input, padding.PKCS1v15(), hashes.SHA256())
        return signing_input.decode() + "." + b64(signature)

    def claims(self):
        return {"iss": ISSUER, "aud": AUDIENCE, "exp": NOW + 120, "nbf": NOW - 120,
                "nonce": NONCE, "custom": {"unchanged": True}}

    def encrypted(self, token):
        protected = b64(b'{"alg":"RSA-OAEP-256","enc":"A256GCM","kid":"test-key","cty":"JWT"}')
        cek, iv = os.urandom(32), os.urandom(12)
        encrypted_key = self.key.public_key().encrypt(cek, padding.OAEP(
            mgf=padding.MGF1(hashes.SHA256()), algorithm=hashes.SHA256(), label=None))
        ciphertext = AESGCM(cek).encrypt(iv, token.encode(), protected.encode())
        return ".".join((protected, b64(encrypted_key), b64(iv), b64(ciphertext[:-16]), b64(ciphertext[-16:])))

    def test_optional_nonce_is_appended_after_existing_now_argument(self):
        token, claims = self.token(), self.claims()
        self.assertEqual(verify_id_token(token, self.jwks, ISSUER, AUDIENCE, NOW), claims)
        self.assertEqual(verify_id_token(token, self.jwks, ISSUER, AUDIENCE, NOW, NONCE), claims)
        self.assertEqual(verify_id_token(token, json.dumps(self.jwks), ISSUER, AUDIENCE,
                                         now=NOW, expected_nonce=NONCE), claims)

    def test_expected_nonce_must_be_a_nonempty_string_and_match_signed_string_claim(self):
        for expected in ("", "other", False, 17, [], {}):
            with self.subTest(expected=expected), self.assertRaises(DecryptError):
                verify_id_token(self.token(), self.jwks, ISSUER, AUDIENCE, NOW, expected)
        for nonce in (None, "", 17, False, [], {}):
            with self.subTest(nonce=nonce):
                claims = self.claims()
                if nonce is None:
                    claims.pop("nonce")
                else:
                    claims["nonce"] = nonce
                with self.assertRaises(DecryptError):
                    verify_id_token(self.token(claims), self.jwks, ISSUER, AUDIENCE, NOW, NONCE)

    def test_omitted_nonce_preserves_claims_without_a_nonce(self):
        claims = self.claims()
        claims.pop("nonce")
        self.assertEqual(verify_id_token(self.token(claims), self.jwks, ISSUER, AUDIENCE, NOW), claims)
        self.assertEqual(verify_id_token(self.token(claims), self.jwks, ISSUER, AUDIENCE, NOW, None), claims)

    def test_signature_issuer_audience_and_time_validation_precede_nonce(self):
        token = self.token()
        header, payload, signature = token.split(".")
        forged = header + "." + b64(json.dumps(dict(self.claims(), nonce="other")).encode()) + "." + signature
        with self.assertRaisesRegex(DecryptError, "signature"):
            verify_id_token(forged, self.jwks, ISSUER, AUDIENCE, NOW, "")
        for changes, message in (({"iss": "other"}, "issuer"), ({"aud": "other"}, "audience"),
                                 ({"exp": NOW - 61}, "expired"), ({"nbf": NOW + 61}, "not valid yet")):
            with self.subTest(changes=changes), self.assertRaisesRegex(DecryptError, message):
                verify_id_token(self.token(dict(self.claims(), **changes)), self.jwks, ISSUER, AUDIENCE, NOW, "")

    def test_matching_nonce_keeps_existing_audience_array_and_clock_leeway(self):
        claims = dict(self.claims(), aud=["other", AUDIENCE], exp=NOW - 60, nbf=NOW + 60)
        self.assertEqual(verify_id_token(self.token(claims), self.jwks, ISSUER, AUDIENCE, NOW, NONCE), claims)

    def test_encrypted_tokens_bind_nonce_and_keep_legacy_calls(self):
        claims = dict(self.claims(), exp=int(time.time()) + 120, nbf=int(time.time()) - 120)
        compact = self.encrypted(self.token(claims))
        self.assertEqual(decrypt_id_token(compact, self.private_jwk, self.jwks, ISSUER, AUDIENCE), claims)
        self.assertEqual(decrypt_id_token(compact, self.private_jwk, self.jwks, ISSUER, AUDIENCE, NONCE), claims)
        self.assertEqual(decrypt_id_token(compact, self.private_jwk, self.jwks, ISSUER, AUDIENCE,
                                         expected_nonce=NONCE), claims)
        for expected in ("", "other"):
            with self.assertRaises(DecryptError):
                decrypt_id_token(compact, self.private_jwk, self.jwks, ISSUER, AUDIENCE, expected)
        claims.pop("nonce")
        with self.assertRaises(DecryptError):
            decrypt_id_token(self.encrypted(self.token(claims)), self.private_jwk, self.jwks,
                             ISSUER, AUDIENCE, NONCE)

    def test_encrypted_nonce_check_never_skips_inner_signature_verification(self):
        claims = dict(self.claims(), exp=int(time.time()) + 120, nbf=int(time.time()) - 120)
        header, payload, signature = self.token(claims).split(".")
        forged = header + "." + b64(json.dumps(dict(claims, custom="tampered")).encode()) + "." + signature
        with self.assertRaisesRegex(DecryptError, "signature"):
            decrypt_id_token(self.encrypted(forged), self.private_jwk, self.jwks, ISSUER, AUDIENCE, NONCE)


if __name__ == "__main__":
    unittest.main()
