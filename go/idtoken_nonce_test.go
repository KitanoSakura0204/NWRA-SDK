package nowork

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"
)

func oidcNonceSigningKey(t *testing.T) (*rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	jwks, err := json.Marshal(map[string]any{"keys": []any{map[string]any{
		"kty": "RSA", "kid": "nonce-signing", "n": encode(key.N.Bytes()), "e": encode(big.NewInt(int64(key.E)).Bytes()),
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return key, jwks
}

func oidcNonceSignedToken(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode([]byte(`{"alg":"RS256","kid":"nonce-signing"}`)) + "." + encode(raw)
	sum := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return unsigned + "." + encode(sig)
}

func oidcNonceClaims(now time.Time) map[string]any {
	return map[string]any{"iss": "https://account.example", "aud": []any{"other", "client"}, "sub": "subject", "exp": now.Unix() + 3600, "nbf": now.Unix() - 60, "iat": now.Unix(), "nonce": "request-nonce"}
}

func TestVerifyIDTokenWithNonce(t *testing.T) {
	key, jwks := oidcNonceSigningKey(t)
	now := time.Unix(1800000000, 0)
	for _, tc := range []struct {
		name     string
		nonce    any
		missing  bool
		expected string
		valid    bool
	}{
		{"match", "request-nonce", false, "request-nonce", true},
		{"mismatch", "other", false, "request-nonce", false},
		{"missing", nil, true, "request-nonce", false},
		{"null", nil, false, "request-nonce", false},
		{"number", 123, false, "request-nonce", false},
		{"array", []any{"request-nonce"}, false, "request-nonce", false},
		{"empty expected", "request-nonce", false, "", false},
		{"both empty", "", false, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := oidcNonceClaims(now)
			claims["nonce"] = tc.nonce
			if tc.missing {
				delete(claims, "nonce")
			}
			token := oidcNonceSignedToken(t, key, claims)
			got, err := VerifyIDTokenWithNonce(token, jwks, "https://account.example", "client", tc.expected, now)
			if (err == nil) != tc.valid || (!tc.valid && got != nil) {
				t.Fatalf("claims=%v err=%v", got, err)
			}
			if tc.valid && (got["iat"] != float64(now.Unix()) || got["nonce"] != tc.nonce) {
				t.Fatal("signed claims changed")
			}
			if _, err := VerifyIDToken(token, jwks, "https://account.example", "client", now); err != nil {
				t.Fatalf("legacy verification changed: %v", err)
			}
		})
	}
}

func TestVerifyIDTokenWithNonceValidatesTokenFirst(t *testing.T) {
	key, jwks := oidcNonceSigningKey(t)
	now := time.Unix(1800000000, 0)
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"issuer", "iss", "https://other.example"}, {"audience", "aud", "other"},
		{"expired", "exp", now.Unix() - 120}, {"future", "nbf", now.Unix() + 120},
		{"signature", "nonce", "request-nonce"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := oidcNonceClaims(now)
			claims[tc.field] = tc.value
			token := oidcNonceSignedToken(t, key, claims)
			if tc.name == "signature" {
				parts := strings.Split(token, ".")
				sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
				sig[0] ^= 1
				parts[2] = base64.RawURLEncoding.EncodeToString(sig)
				token = strings.Join(parts, ".")
			}
			_, want := VerifyIDToken(token, jwks, "https://account.example", "client", now)
			got, err := VerifyIDTokenWithNonce(token, jwks, "https://account.example", "client", "wrong-nonce", now)
			if want == nil || err == nil || want.Error() != err.Error() || got != nil {
				t.Fatalf("token validation skipped: want=%v got=%v claims=%v", want, err, got)
			}
		})
	}
}

func oidcNonceEncryptToken(t *testing.T, inner string, key *rsa.PrivateKey) string {
	t.Helper()
	cek := make([]byte, 32)
	if _, err := rand.Read(cek); err != nil {
		t.Fatal(err)
	}
	ek, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &key.PublicKey, cek, nil)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	header := encode([]byte(`{"alg":"RSA-OAEP-256","enc":"A256GCM","cty":"JWT","kid":"nonce-encryption"}`))
	sealed := gcm.Seal(nil, iv, []byte(inner), []byte(header))
	return strings.Join([]string{header, encode(ek), encode(iv), encode(sealed[:len(sealed)-gcm.Overhead()]), encode(sealed[len(sealed)-gcm.Overhead():])}, ".")
}

func TestDecryptIDTokenWithNonce(t *testing.T) {
	signing, jwks := oidcNonceSigningKey(t)
	encryption, _ := oidcNonceSigningKey(t)
	key := &DecryptionKey{Kid: "nonce-encryption", rsa: encryption}
	now := time.Now()
	for _, tc := range []struct {
		name, field string
		value       any
		expected    string
		valid       bool
	}{
		{"match", "nonce", "request-nonce", "request-nonce", true},
		{"mismatch", "nonce", "other", "request-nonce", false},
		{"missing", "nonce", nil, "request-nonce", false},
		{"non-string", "nonce", 123, "request-nonce", false},
		{"empty expected", "nonce", "request-nonce", "", false},
		{"issuer", "iss", "https://other.example", "request-nonce", false},
		{"audience", "aud", "other", "request-nonce", false},
		{"expired", "exp", now.Unix() - 120, "request-nonce", false},
		{"signature", "nonce", "request-nonce", "request-nonce", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := oidcNonceClaims(now)
			claims[tc.field] = tc.value
			if tc.name == "missing" {
				delete(claims, "nonce")
			}
			inner := oidcNonceSignedToken(t, signing, claims)
			if tc.name == "signature" {
				parts := strings.Split(inner, ".")
				parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 256))
				inner = strings.Join(parts, ".")
			}
			token := oidcNonceEncryptToken(t, inner, encryption)
			got, err := DecryptIDTokenWithNonce(token, jwks, "https://account.example", "client", tc.expected, nil, key)
			if (err == nil) != tc.valid || (!tc.valid && got != nil) {
				t.Fatalf("claims=%v err=%v", got, err)
			}
			if tc.name == "missing" {
				if _, err := DecryptIDToken(token, jwks, "https://account.example", "client", key); err != nil {
					t.Fatalf("legacy decrypt changed: %v", err)
				}
			}
		})
	}
}
