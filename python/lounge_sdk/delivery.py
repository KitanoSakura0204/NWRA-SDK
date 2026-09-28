"""Delivery helpers (SDK/docs/DELIVERY.md): decrypt what the user centre sends when an app
registered an encryption key. RSA-OAEP-256 or ECDH-ES+A256KW (P-256 / X25519), enc A256GCM.

Needs the ``cryptography`` package (``pip install cryptography``); the rest of the SDK does not.
"""

import base64
import json
import time
from typing import Any, Dict, Iterable, List, Mapping, Optional, Tuple, Union

from .errors import LoungeError


class DecryptError(LoungeError):
    """A JWE could not be opened (wrong key, tampered, unsupported) or a check failed."""


Jwk = Mapping[str, str]
Keys = Union[Jwk, Iterable[Jwk]]


def _crypto():
    try:
        from cryptography.hazmat.primitives import hashes, serialization  # noqa: F401
        from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa, x25519
        from cryptography.hazmat.primitives.ciphers.aead import AESGCM
        from cryptography.hazmat.primitives.kdf.concatkdf import ConcatKDFHash
        from cryptography.hazmat.primitives.keywrap import aes_key_unwrap
    except ImportError as exc:  # pragma: no cover - environment dependent
        raise DecryptError("the delivery helpers need the 'cryptography' package: pip install cryptography") from exc
    return hashes, ec, padding, rsa, x25519, AESGCM, ConcatKDFHash, aes_key_unwrap


def _b64u(s: str) -> bytes:
    return base64.urlsafe_b64decode(s + "=" * (-len(s) % 4))


def _int(s: str) -> int:
    return int.from_bytes(_b64u(s), "big")


def _lp(b: bytes) -> bytes:
    return len(b).to_bytes(4, "big") + b


def _content_key(hdr: Dict[str, Any], ek: bytes, jwk: Jwk) -> bytes:
    hashes, ec, padding, rsa, x25519, _, ConcatKDFHash, aes_key_unwrap = _crypto()
    if hdr.get("alg") == "RSA-OAEP-256" and jwk.get("kty") == "RSA":
        n, e, d, p, q = (_int(jwk[k]) for k in ("n", "e", "d", "p", "q"))
        numbers = rsa.RSAPrivateNumbers(p, q, d, rsa.rsa_crt_dmp1(d, p), rsa.rsa_crt_dmq1(d, q), rsa.rsa_crt_iqmp(p, q), rsa.RSAPublicNumbers(e, n))
        oaep = padding.OAEP(mgf=padding.MGF1(hashes.SHA256()), algorithm=hashes.SHA256(), label=None)
        return numbers.private_key().decrypt(ek, oaep)
    epk = hdr.get("epk") or {}
    if hdr.get("alg") != "ECDH-ES+A256KW":
        raise DecryptError()
    if jwk.get("kty") == "EC" and jwk.get("crv") == "P-256" and epk.get("crv") == "P-256":
        curve = ec.SECP256R1()
        priv = ec.EllipticCurvePrivateNumbers(_int(jwk["d"]), ec.EllipticCurvePublicNumbers(_int(jwk["x"]), _int(jwk["y"]), curve)).private_key()
        z = priv.exchange(ec.ECDH(), ec.EllipticCurvePublicNumbers(_int(epk["x"]), _int(epk["y"]), curve).public_key())
    elif jwk.get("kty") == "OKP" and jwk.get("crv") == "X25519" and epk.get("crv") == "X25519":
        priv = x25519.X25519PrivateKey.from_private_bytes(_b64u(jwk["d"]))
        z = priv.exchange(x25519.X25519PublicKey.from_public_bytes(_b64u(epk["x"])))
    else:
        raise DecryptError()
    apu, apv = _b64u(hdr.get("apu", "")), _b64u(hdr.get("apv", ""))
    other = _lp(hdr["alg"].encode()) + _lp(apu) + _lp(apv) + (256).to_bytes(4, "big")
    kek = ConcatKDFHash(algorithm=hashes.SHA256(), length=32, otherinfo=other).derive(z)
    return aes_key_unwrap(kek, ek)


def decrypt_jwe(compact: str, keys: Keys) -> Tuple[bytes, Dict[str, Any]]:
    """Open a compact JWE and return (plaintext, protected header). With several keys (a rotation
    overlap) the header kid picks one; without a kid every key is tried."""
    AESGCM = _crypto()[5]
    parts = compact.split(".")
    if len(parts) != 5:
        raise DecryptError()
    try:
        seg = [_b64u(p) for p in parts]
        hdr = json.loads(seg[0])
    except (ValueError, TypeError) as exc:
        raise DecryptError() from exc
    if hdr.get("enc") != "A256GCM" or "zip" in hdr or "crit" in hdr:
        raise DecryptError()
    for jwk in [keys] if isinstance(keys, Mapping) else list(keys):
        if hdr.get("kid") and jwk.get("kid") and jwk["kid"] != hdr["kid"]:
            continue
        try:
            cek = _content_key(hdr, seg[1], jwk)
            return AESGCM(cek).decrypt(seg[2], seg[3] + seg[4], parts[0].encode()), hdr
        except Exception:  # wrong key or tampered: try the next one
            continue
    raise DecryptError()


def verify_id_token(token: str, jwks: Union[str, Mapping[str, Any]], issuer: Optional[str] = None,
                    audience: Optional[str] = None, now: Optional[int] = None) -> Dict[str, Any]:
    """Verify an RS256 ID token against the provider JWKS (GET BASE/jwks.json): signature, iss,
    aud and exp / nbf with 60 s leeway. Returns the claims."""
    hashes, _, padding, rsa = _crypto()[:4]
    parts = token.split(".")
    if len(parts) != 3:
        raise DecryptError("not a signed JWT")
    hdr = json.loads(_b64u(parts[0]))
    if hdr.get("alg") != "RS256":
        raise DecryptError("ID token must be RS256")
    keyset = json.loads(jwks) if isinstance(jwks, str) else jwks
    jwk = next((k for k in keyset.get("keys", []) if k.get("kty") == "RSA" and (not hdr.get("kid") or k.get("kid") == hdr["kid"])), None)
    if jwk is None:
        raise DecryptError("signing key not in the JWKS")
    pub = rsa.RSAPublicNumbers(_int(jwk["e"]), _int(jwk["n"])).public_key()
    try:
        pub.verify(_b64u(parts[2]), (parts[0] + "." + parts[1]).encode(), padding.PKCS1v15(), hashes.SHA256())
    except Exception as exc:
        raise DecryptError("invalid ID token signature") from exc
    claims = json.loads(_b64u(parts[1]))
    now = int(time.time()) if now is None else now
    if issuer and claims.get("iss") != issuer:
        raise DecryptError("ID token issuer mismatch")
    aud = claims.get("aud")
    if audience and aud != audience and not (isinstance(aud, list) and audience in aud):
        raise DecryptError("ID token audience mismatch")
    if not isinstance(claims.get("exp"), (int, float)) or claims["exp"] + 60 < now:
        raise DecryptError("ID token expired")
    if isinstance(claims.get("nbf"), (int, float)) and claims["nbf"] - 60 > now:
        raise DecryptError("ID token not valid yet")
    return claims


def decrypt_id_token(token: str, keys: Keys, jwks: Union[str, Mapping[str, Any]], issuer: Optional[str] = None,
                     audience: Optional[str] = None) -> Dict[str, Any]:
    """Open a signed-then-encrypted ID token (id_token_encrypted_response_alg) and verify it."""
    inner, _ = decrypt_jwe(token, keys)
    return verify_id_token(inner.decode(), jwks, issuer, audience)


def open_server_data(response: Union[str, bytes, Mapping[str, Any]], keys: Keys, ctx: Optional[str] = None) -> Any:
    """Open the plugin-API envelope: data {"jwe"} of a response with X-Lounge-Encryption: JWE (the
    whole body is accepted too). ctx, when set, must equal the header ctx ("server:<code>:<route>").
    A plain body is refused: a server that turned encryption on must not accept plaintext."""
    obj = json.loads(response) if isinstance(response, (str, bytes)) else response
    jwe = obj.get("jwe") or (obj.get("data") or {}).get("jwe") if isinstance(obj, Mapping) else None
    if not isinstance(jwe, str):
        raise DecryptError("response is not encrypted")
    plain, hdr = decrypt_jwe(jwe, keys)
    if ctx and hdr.get("ctx") != ctx:
        raise DecryptError("envelope ctx mismatch")
    return json.loads(plain)


def open_webhook(body: Union[str, bytes], keys: Keys) -> Dict[str, Any]:
    """Open an encrypted webhook body ({"id","event","timestamp","jwe"}); verify the signature over
    the raw body with verify_webhook first. Returns the inner document (id, event, timestamp, data)."""
    outer = json.loads(body)
    if not isinstance(outer.get("jwe"), str):
        raise DecryptError("webhook is not encrypted")
    plain, hdr = decrypt_jwe(outer["jwe"], keys)
    if not str(hdr.get("ctx", "")).startswith("webhook:"):
        raise DecryptError("envelope ctx mismatch")
    doc = json.loads(plain)
    if any(doc.get(k) != outer.get(k) for k in ("id", "event", "timestamp")):
        raise DecryptError("webhook envelope does not match its content")
    return doc


__all__: List[str] = ["DecryptError", "decrypt_jwe", "verify_id_token", "decrypt_id_token", "open_server_data", "open_webhook"]
