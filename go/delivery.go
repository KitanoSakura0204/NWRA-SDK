package nowork

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
)

// Delivery helpers (SDK/docs/DELIVERY.md): decrypt the JWE the user centre sends when an app
// registered an encryption key. Algorithms: RSA-OAEP-256 or ECDH-ES+A256KW (P-256 / X25519),
// enc A256GCM. Standard library only.

// ErrDecrypt is returned for any JWE that cannot be opened (wrong key, tampered, unsupported).
var ErrDecrypt = errors.New("nowork: cannot decrypt the JWE")

// DecryptionKey is an app's private key, parsed from its private JWK.
type DecryptionKey struct {
	Kid    string
	rsa    *rsa.PrivateKey
	ec     *ecdh.PrivateKey // P-256
	x25519 *ecdh.PrivateKey
}

// ParseDecryptionKey reads a private JWK: RSA (n, e, d, p, q), EC P-256 (x, y, d) or OKP X25519 (d).
func ParseDecryptionKey(jwk []byte) (*DecryptionKey, error) {
	var j struct{ Kty, Crv, Kid, N, E, D, P, Q, X, Y string }
	if err := json.Unmarshal(jwk, &j); err != nil {
		return nil, err
	}
	k := &DecryptionKey{Kid: j.Kid}
	b := func(s string) []byte { v, _ := base64.RawURLEncoding.DecodeString(s); return v }
	switch {
	case j.Kty == "RSA" && j.D != "" && j.P != "" && j.Q != "":
		n, e := new(big.Int).SetBytes(b(j.N)), new(big.Int).SetBytes(b(j.E))
		priv := &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: n, E: int(e.Int64())}, D: new(big.Int).SetBytes(b(j.D)),
			Primes: []*big.Int{new(big.Int).SetBytes(b(j.P)), new(big.Int).SetBytes(b(j.Q))}}
		if err := priv.Validate(); err != nil {
			return nil, err
		}
		priv.Precompute()
		k.rsa = priv
	case j.Kty == "EC" && j.Crv == "P-256":
		priv, err := ecdh.P256().NewPrivateKey(leftPad(b(j.D), 32))
		if err != nil {
			return nil, err
		}
		k.ec = priv
	case j.Kty == "OKP" && j.Crv == "X25519":
		priv, err := ecdh.X25519().NewPrivateKey(b(j.D))
		if err != nil {
			return nil, err
		}
		k.x25519 = priv
	default:
		return nil, errors.New("nowork: need a private RSA, EC P-256 or X25519 JWK")
	}
	return k, nil
}

func leftPad(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	return append(make([]byte, n-len(b)), b...)
}

// DecryptJWE opens a compact JWE. With several keys (a rotation overlap) the header kid picks
// one; without a kid every key is tried. The protected header is returned for ctx / iat checks.
func DecryptJWE(compact string, keys ...*DecryptionKey) ([]byte, map[string]any, error) {
	parts := strings.Split(compact, ".")
	if len(parts) != 5 {
		return nil, nil, ErrDecrypt
	}
	seg := make([][]byte, 5)
	for i, p := range parts {
		v, err := base64.RawURLEncoding.DecodeString(p)
		if err != nil {
			return nil, nil, ErrDecrypt
		}
		seg[i] = v
	}
	var hdr map[string]any
	if json.Unmarshal(seg[0], &hdr) != nil || hdr["enc"] != "A256GCM" || hdr["zip"] != nil || hdr["crit"] != nil {
		return nil, nil, ErrDecrypt
	}
	kid, _ := hdr["kid"].(string)
	for _, k := range keys {
		if k == nil || (kid != "" && k.Kid != "" && k.Kid != kid) {
			continue
		}
		cek, err := k.unwrap(hdr, seg[1])
		if err != nil {
			continue
		}
		block, err := aes.NewCipher(cek)
		if err != nil {
			continue
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil || len(seg[2]) != gcm.NonceSize() {
			continue
		}
		plain, err := gcm.Open(nil, seg[2], append(append([]byte{}, seg[3]...), seg[4]...), []byte(parts[0]))
		if err == nil {
			return plain, hdr, nil
		}
	}
	return nil, nil, ErrDecrypt
}

// unwrap recovers the content key for this key's algorithm.
func (k *DecryptionKey) unwrap(hdr map[string]any, ek []byte) ([]byte, error) {
	switch hdr["alg"] {
	case "RSA-OAEP-256":
		if k.rsa == nil {
			return nil, ErrDecrypt
		}
		return rsa.DecryptOAEP(sha256.New(), nil, k.rsa, ek, nil)
	case "ECDH-ES+A256KW":
		epk, _ := hdr["epk"].(map[string]any)
		str := func(name string) []byte {
			s, _ := epk[name].(string)
			v, _ := base64.RawURLEncoding.DecodeString(s)
			return v
		}
		var z []byte
		var err error
		switch {
		case k.ec != nil && epk["kty"] == "EC" && epk["crv"] == "P-256":
			var pub *ecdh.PublicKey
			if pub, err = ecdh.P256().NewPublicKey(append(append([]byte{4}, leftPad(str("x"), 32)...), leftPad(str("y"), 32)...)); err == nil {
				z, err = k.ec.ECDH(pub)
			}
		case k.x25519 != nil && epk["kty"] == "OKP" && epk["crv"] == "X25519":
			var pub *ecdh.PublicKey
			if pub, err = ecdh.X25519().NewPublicKey(str("x")); err == nil {
				z, err = k.x25519.ECDH(pub)
			}
		default:
			return nil, ErrDecrypt
		}
		if err != nil {
			return nil, ErrDecrypt
		}
		apu, _ := base64.RawURLEncoding.DecodeString(stringOf(hdr["apu"]))
		apv, _ := base64.RawURLEncoding.DecodeString(stringOf(hdr["apv"]))
		return aesKeyUnwrap(concatKDF256(z, "ECDH-ES+A256KW", apu, apv), ek)
	}
	return nil, ErrDecrypt
}

func stringOf(v any) string { s, _ := v.(string); return s }

// concatKDF256 is the JWA §4.6.2 Concat KDF for a 256-bit key: one SHA-256 round.
func concatKDF256(z []byte, alg string, apu, apv []byte) []byte {
	h := sha256.New()
	lp := func(b []byte) { _ = binary.Write(h, binary.BigEndian, uint32(len(b))); h.Write(b) }
	h.Write([]byte{0, 0, 0, 1})
	h.Write(z)
	lp([]byte(alg))
	lp(apu)
	lp(apv)
	_ = binary.Write(h, binary.BigEndian, uint32(256))
	return h.Sum(nil)
}

// aesKeyUnwrap is RFC 3394 key unwrap.
func aesKeyUnwrap(kek, wrapped []byte) ([]byte, error) {
	if len(wrapped)%8 != 0 || len(wrapped) < 24 {
		return nil, ErrDecrypt
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	n := len(wrapped)/8 - 1
	a, r := append([]byte{}, wrapped[:8]...), append([]byte{}, wrapped[8:]...)
	buf := make([]byte, 16)
	for j := 5; j >= 0; j-- {
		for i := n; i >= 1; i-- {
			binary.BigEndian.PutUint64(buf[:8], binary.BigEndian.Uint64(a)^uint64(n*j+i))
			copy(buf[8:], r[(i-1)*8:i*8])
			block.Decrypt(buf, buf)
			copy(a, buf[:8])
			copy(r[(i-1)*8:i*8], buf[8:])
		}
	}
	if subtle.ConstantTimeCompare(a, []byte{0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6, 0xA6}) != 1 {
		return nil, ErrDecrypt
	}
	return r, nil
}

// VerifyIDToken checks an RS256 ID token against the provider JWKS (GET BASE/jwks.json): the
// signature, iss, aud (when set) and exp / nbf with 60 s leeway. It returns the claims.
func VerifyIDToken(token string, jwks []byte, issuer, audience string, now time.Time) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("nowork: not a signed JWT")
	}
	raw := make([][]byte, 3)
	for i, p := range parts {
		v, err := base64.RawURLEncoding.DecodeString(p)
		if err != nil {
			return nil, errors.New("nowork: malformed JWT")
		}
		raw[i] = v
	}
	var hdr struct{ Alg, Kid string }
	if json.Unmarshal(raw[0], &hdr) != nil || hdr.Alg != "RS256" {
		return nil, errors.New("nowork: ID token must be RS256")
	}
	var set struct {
		Keys []struct{ Kty, Kid, N, E string } `json:"keys"`
	}
	if err := json.Unmarshal(jwks, &set); err != nil {
		return nil, err
	}
	var pub *rsa.PublicKey
	for _, k := range set.Keys {
		if k.Kty == "RSA" && (hdr.Kid == "" || k.Kid == hdr.Kid) {
			n, _ := base64.RawURLEncoding.DecodeString(k.N)
			e, _ := base64.RawURLEncoding.DecodeString(k.E)
			pub = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
			break
		}
	}
	if pub == nil {
		return nil, errors.New("nowork: signing key not in the JWKS")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], raw[2]); err != nil {
		return nil, errors.New("nowork: invalid ID token signature")
	}
	var claims map[string]any
	if err := json.Unmarshal(raw[1], &claims); err != nil {
		return nil, err
	}
	if issuer != "" && claims["iss"] != issuer {
		return nil, errors.New("nowork: ID token issuer mismatch")
	}
	if audience != "" && !hasAudience(claims["aud"], audience) {
		return nil, errors.New("nowork: ID token audience mismatch")
	}
	leeway := int64(60)
	if exp, ok := claims["exp"].(float64); !ok || int64(exp)+leeway < now.Unix() {
		return nil, errors.New("nowork: ID token expired")
	}
	if nbf, ok := claims["nbf"].(float64); ok && int64(nbf)-leeway > now.Unix() {
		return nil, errors.New("nowork: ID token not valid yet")
	}
	return claims, nil
}

// VerifyIDTokenWithNonce performs VerifyIDToken's signature, issuer, audience and time
// checks first, then requires a nonempty expected nonce equal to the signed string claim.
func VerifyIDTokenWithNonce(token string, jwks []byte, issuer, audience, nonce string, now time.Time) (map[string]any, error) {
	claims, err := VerifyIDToken(token, jwks, issuer, audience, now)
	if err != nil {
		return nil, err
	}
	if nonce == "" {
		return nil, errors.New("nowork: expected ID token nonce must not be empty")
	}
	signedNonce, ok := claims["nonce"].(string)
	if !ok || signedNonce != nonce {
		return nil, errors.New("nowork: ID token nonce mismatch")
	}
	return claims, nil
}

func hasAudience(aud any, want string) bool {
	switch v := aud.(type) {
	case string:
		return v == want
	case []any:
		for _, a := range v {
			if a == want {
				return true
			}
		}
	}
	return false
}

// DecryptIDToken opens a signed-then-encrypted ID token (the client registered
// id_token_encrypted_response_alg) and verifies the inner token.
func DecryptIDToken(token string, jwks []byte, issuer, audience string, keys ...*DecryptionKey) (map[string]any, error) {
	inner, _, err := DecryptJWE(token, keys...)
	if err != nil {
		return nil, err
	}
	return VerifyIDToken(string(inner), jwks, issuer, audience, time.Now())
}

// DecryptIDTokenWithNonce opens a signed-then-encrypted ID token and verifies the inner
// signature, issuer, audience, time and nonce using VerifyIDTokenWithNonce.
func DecryptIDTokenWithNonce(token string, jwks []byte, issuer, audience, nonce string, keys ...*DecryptionKey) (map[string]any, error) {
	inner, _, err := DecryptJWE(token, keys...)
	if err != nil {
		return nil, err
	}
	return VerifyIDTokenWithNonce(string(inner), jwks, issuer, audience, nonce, time.Now())
}

// OpenServerData opens the plugin-API envelope: data {"jwe": "..."} of a response that carried
// X-NoWork-Encryption: JWE (the whole response body is accepted too). ctx, when set, must equal
// the header's ctx ("server:<code>:<route>"). It fails on a plain body: a server that turned
// encryption on must not silently accept plaintext.
func OpenServerData(data []byte, ctx string, keys ...*DecryptionKey) ([]byte, error) {
	var env struct {
		JWE  string `json:"jwe"`
		Data *struct {
			JWE string `json:"jwe"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if env.JWE == "" && env.Data != nil {
		env.JWE = env.Data.JWE
	}
	if env.JWE == "" {
		return nil, errors.New("nowork: response is not encrypted")
	}
	plain, hdr, err := DecryptJWE(env.JWE, keys...)
	if err != nil {
		return nil, err
	}
	if ctx != "" && hdr["ctx"] != ctx {
		return nil, errors.New("nowork: envelope ctx mismatch")
	}
	return plain, nil
}

// OpenWebhook opens an encrypted webhook body ({"id","event","timestamp","jwe"}). Verify the
// signature over the raw body with VerifyWebhook first. The inner document must repeat the
// outer id, event and timestamp, and the header ctx must be "webhook:<endpoint id>".
func OpenWebhook(body []byte, keys ...*DecryptionKey) (*WebhookEvent, error) {
	var outer struct {
		ID        string `json:"id"`
		Event     string `json:"event"`
		Timestamp int64  `json:"timestamp"`
		JWE       string `json:"jwe"`
	}
	if err := json.Unmarshal(body, &outer); err != nil {
		return nil, err
	}
	if outer.JWE == "" {
		return nil, errors.New("nowork: webhook is not encrypted")
	}
	plain, hdr, err := DecryptJWE(outer.JWE, keys...)
	if err != nil {
		return nil, err
	}
	if ctx, _ := hdr["ctx"].(string); !strings.HasPrefix(ctx, "webhook:") {
		return nil, errors.New("nowork: envelope ctx mismatch")
	}
	var ev WebhookEvent
	if err := json.Unmarshal(plain, &ev); err != nil {
		return nil, err
	}
	if ev.ID != outer.ID || ev.Event != outer.Event || ev.Timestamp != outer.Timestamp {
		return nil, errors.New("nowork: webhook envelope does not match its content")
	}
	return &ev, nil
}
