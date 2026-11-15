package cyou.nowork.sdk;

import java.math.BigInteger;
import java.nio.charset.StandardCharsets;
import java.security.AlgorithmParameters;
import java.security.Key;
import java.security.KeyFactory;
import java.security.MessageDigest;
import java.security.PrivateKey;
import java.security.PublicKey;
import java.security.Signature;
import java.security.spec.ECGenParameterSpec;
import java.security.spec.ECParameterSpec;
import java.security.spec.ECPoint;
import java.security.spec.ECPrivateKeySpec;
import java.security.spec.ECPublicKeySpec;
import java.security.spec.MGF1ParameterSpec;
import java.security.spec.NamedParameterSpec;
import java.security.spec.RSAPrivateCrtKeySpec;
import java.security.spec.RSAPublicKeySpec;
import java.security.spec.XECPrivateKeySpec;
import java.security.spec.XECPublicKeySpec;
import java.util.Base64;
import java.util.List;
import java.util.Map;
import javax.crypto.Cipher;
import javax.crypto.KeyAgreement;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.OAEPParameterSpec;
import javax.crypto.spec.PSource;
import javax.crypto.spec.SecretKeySpec;

/**
 * Delivery helpers (SDK/docs/DELIVERY.md): decrypt what the user centre sends when an app registered an
 * encryption key. RSA-OAEP-256 or ECDH-ES+A256KW (P-256 / X25519), enc A256GCM. JCA only (Java 17+).
 * Keys are private JWKs as JSON text.
 */
public final class NoWorkDelivery {
    private NoWorkDelivery() {
    }

    /** A JWE could not be opened, or a check on its content failed. */
    public static final class DecryptException extends RuntimeException {
        private static final long serialVersionUID = 1L;

        public DecryptException(String message) {
            super(message);
        }
    }

    /** A decrypted JWE: plaintext and protected header. */
    public static final class Decrypted {
        public final byte[] bytes;
        public final Map<String, Object> header;

        Decrypted(byte[] bytes, Map<String, Object> header) {
            this.bytes = bytes;
            this.header = header;
        }

        public String text() {
            return new String(bytes, StandardCharsets.UTF_8);
        }
    }

    private static byte[] b64(String s) {
        return Base64.getUrlDecoder().decode(s);
    }

    private static BigInteger num(Object s) {
        return new BigInteger(1, b64(Json.str(s)));
    }

    @SuppressWarnings("unchecked")
    private static Map<String, Object> obj(Object o) {
        if (!(o instanceof Map)) {
            throw new DecryptException("expected a JSON object");
        }
        return (Map<String, Object>) o;
    }

    /** An app's private JWK (RSA, EC P-256 or OKP X25519). */
    public static final class PrivateJwk {
        final Map<String, Object> m;

        private PrivateJwk(Map<String, Object> m) {
            this.m = m;
        }

        /** Parses the JWK JSON text. */
        public static PrivateJwk parse(String json) {
            return new PrivateJwk(obj(Json.parse(json)));
        }

        static PrivateJwk of(Object parsed) {
            return new PrivateJwk(obj(parsed));
        }
    }

    /** Opens a compact JWE; the header kid picks among several keys, without a kid every key is tried. */
    public static Decrypted decrypt(String compact, PrivateJwk... keys) {
        String[] parts = compact.split("\\.", -1);
        if (parts.length != 5) {
            throw new DecryptException("cannot decrypt the JWE");
        }
        Map<String, Object> hdr;
        try {
            hdr = obj(Json.parse(new String(b64(parts[0]), StandardCharsets.UTF_8)));
        } catch (RuntimeException e) {
            throw new DecryptException("cannot decrypt the JWE");
        }
        if (!"A256GCM".equals(hdr.get("enc")) || hdr.containsKey("zip") || hdr.containsKey("crit")) {
            throw new DecryptException("cannot decrypt the JWE");
        }
        for (PrivateJwk key : keys) {
            try {
                Map<String, Object> jwk = key.m;
                if (hdr.get("kid") != null && jwk.get("kid") != null && !hdr.get("kid").equals(jwk.get("kid"))) {
                    continue;
                }
                byte[] cek = contentKey(hdr, b64(parts[1]), jwk);
                Cipher gcm = Cipher.getInstance("AES/GCM/NoPadding");
                gcm.init(Cipher.DECRYPT_MODE, new SecretKeySpec(cek, "AES"), new GCMParameterSpec(128, b64(parts[2])));
                gcm.updateAAD(parts[0].getBytes(StandardCharsets.US_ASCII));
                byte[] ct = b64(parts[3]);
                byte[] tag = b64(parts[4]);
                byte[] joined = new byte[ct.length + tag.length];
                System.arraycopy(ct, 0, joined, 0, ct.length);
                System.arraycopy(tag, 0, joined, ct.length, tag.length);
                return new Decrypted(gcm.doFinal(joined), hdr);
            } catch (Exception e) {
                // wrong key or tampered: try the next key
            }
        }
        throw new DecryptException("cannot decrypt the JWE");
    }

    private static byte[] contentKey(Map<String, Object> hdr, byte[] ek, Map<String, Object> jwk) throws Exception {
        if ("RSA-OAEP-256".equals(hdr.get("alg")) && "RSA".equals(jwk.get("kty"))) {
            PrivateKey priv = KeyFactory.getInstance("RSA").generatePrivate(new RSAPrivateCrtKeySpec(num(jwk.get("n")), num(jwk.get("e")),
                    num(jwk.get("d")), num(jwk.get("p")), num(jwk.get("q")), num(jwk.get("dp")), num(jwk.get("dq")), num(jwk.get("qi"))));
            Cipher oaep = Cipher.getInstance("RSA/ECB/OAEPPadding");
            // MGF1 must use SHA-256 too (the JCA default for OAEPPadding is SHA-1).
            oaep.init(Cipher.DECRYPT_MODE, priv, new OAEPParameterSpec("SHA-256", "MGF1", MGF1ParameterSpec.SHA256, PSource.PSpecified.DEFAULT));
            return oaep.doFinal(ek);
        }
        if (!"ECDH-ES+A256KW".equals(hdr.get("alg"))) {
            throw new DecryptException("unsupported alg");
        }
        Map<String, Object> epk = obj(hdr.get("epk"));
        byte[] z;
        if ("EC".equals(jwk.get("kty")) && "P-256".equals(jwk.get("crv")) && "P-256".equals(epk.get("crv"))) {
            AlgorithmParameters params = AlgorithmParameters.getInstance("EC");
            params.init(new ECGenParameterSpec("secp256r1"));
            ECParameterSpec curve = params.getParameterSpec(ECParameterSpec.class);
            KeyFactory kf = KeyFactory.getInstance("EC");
            PrivateKey priv = kf.generatePrivate(new ECPrivateKeySpec(num(jwk.get("d")), curve));
            PublicKey pub = kf.generatePublic(new ECPublicKeySpec(new ECPoint(num(epk.get("x")), num(epk.get("y"))), curve));
            z = agree("ECDH", priv, pub);
        } else if ("OKP".equals(jwk.get("kty")) && "X25519".equals(jwk.get("crv")) && "X25519".equals(epk.get("crv"))) {
            KeyFactory kf = KeyFactory.getInstance("XDH");
            PrivateKey priv = kf.generatePrivate(new XECPrivateKeySpec(NamedParameterSpec.X25519, b64(Json.str(jwk.get("d")))));
            byte[] u = b64(Json.str(epk.get("x"))); // RFC 7748: little-endian, top bit masked
            byte[] be = new byte[u.length];
            for (int i = 0; i < u.length; i++) {
                be[i] = u[u.length - 1 - i];
            }
            be[0] &= 0x7f;
            z = agree("XDH", priv, kf.generatePublic(new XECPublicKeySpec(NamedParameterSpec.X25519, new BigInteger(1, be))));
        } else {
            throw new DecryptException("key does not match the JWE");
        }
        byte[] apu = hdr.get("apu") == null ? new byte[0] : b64(Json.str(hdr.get("apu")));
        byte[] apv = hdr.get("apv") == null ? new byte[0] : b64(Json.str(hdr.get("apv")));
        Cipher kw = Cipher.getInstance("AESWrap");
        kw.init(Cipher.UNWRAP_MODE, new SecretKeySpec(concatKdf(z, "ECDH-ES+A256KW", apu, apv), "AES"));
        return kw.unwrap(ek, "AES", Cipher.SECRET_KEY).getEncoded();
    }

    private static byte[] agree(String alg, Key priv, Key pub) throws Exception {
        KeyAgreement ka = KeyAgreement.getInstance(alg);
        ka.init(priv);
        ka.doPhase(pub, true);
        return ka.generateSecret();
    }

    /** JWA 4.6.2 Concat KDF for a 256-bit key (one SHA-256 round). */
    private static byte[] concatKdf(byte[] z, String alg, byte[] apu, byte[] apv) throws Exception {
        MessageDigest h = MessageDigest.getInstance("SHA-256");
        byte[] a = alg.getBytes(StandardCharsets.US_ASCII);
        h.update(new byte[] {0, 0, 0, 1});
        h.update(z);
        for (byte[] part : new byte[][] {a, apu, apv}) {
            h.update(new byte[] {(byte) (part.length >>> 24), (byte) (part.length >>> 16), (byte) (part.length >>> 8), (byte) part.length});
            h.update(part);
        }
        h.update(new byte[] {0, 0, 1, 0}); // 256 bits
        return h.digest();
    }

    /** Verifies an RS256 ID token against the provider JWKS JSON: signature, iss, aud, exp / nbf (60 s leeway). */
    @SuppressWarnings("unchecked")
    public static Map<String, Object> verifyIdToken(String token, String jwks, String issuer, String audience) {
        String[] parts = token.split("\\.", -1);
        if (parts.length != 3) {
            throw new DecryptException("not a signed JWT");
        }
        Map<String, Object> hdr = obj(Json.parse(new String(b64(parts[0]), StandardCharsets.UTF_8)));
        if (!"RS256".equals(hdr.get("alg"))) {
            throw new DecryptException("ID token must be RS256");
        }
        Map<String, Object> jwk = null;
        for (Object k : (List<Object>) obj(Json.parse(jwks)).get("keys")) {
            Map<String, Object> m = obj(k);
            if ("RSA".equals(m.get("kty")) && (hdr.get("kid") == null || hdr.get("kid").equals(m.get("kid")))) {
                jwk = m;
                break;
            }
        }
        if (jwk == null) {
            throw new DecryptException("signing key not in the JWKS");
        }
        try {
            Signature sig = Signature.getInstance("SHA256withRSA");
            sig.initVerify(KeyFactory.getInstance("RSA").generatePublic(new RSAPublicKeySpec(num(jwk.get("n")), num(jwk.get("e")))));
            sig.update((parts[0] + "." + parts[1]).getBytes(StandardCharsets.US_ASCII));
            if (!sig.verify(b64(parts[2]))) {
                throw new DecryptException("invalid ID token signature");
            }
        } catch (DecryptException e) {
            throw e;
        } catch (Exception e) {
            throw new DecryptException("invalid ID token signature");
        }
        Map<String, Object> claims = obj(Json.parse(new String(b64(parts[1]), StandardCharsets.UTF_8)));
        long now = System.currentTimeMillis() / 1000;
        Object aud = claims.get("aud");
        if (issuer != null && !issuer.equals(claims.get("iss"))) {
            throw new DecryptException("ID token issuer mismatch");
        }
        if (audience != null && !audience.equals(aud) && !(aud instanceof List && ((List<Object>) aud).contains(audience))) {
            throw new DecryptException("ID token audience mismatch");
        }
        if (claims.get("exp") == null || Json.num(claims.get("exp")).longValue() + 60 < now) {
            throw new DecryptException("ID token expired");
        }
        if (claims.get("nbf") != null && Json.num(claims.get("nbf")).longValue() - 60 > now) {
            throw new DecryptException("ID token not valid yet");
        }
        return claims;
    }

    /**
     * Verifies the signature, issuer, audience and time claims, then binds the signed nonce to the
     * authorization request. The explicitly supplied expected nonce must be non-null and nonempty.
     */
    public static Map<String, Object> verifyIdToken(String token, String jwks, String issuer, String audience, String expectedNonce) {
        Map<String, Object> claims = verifyIdToken(token, jwks, issuer, audience);
        if (expectedNonce == null || expectedNonce.isEmpty()) {
            throw new DecryptException("expected ID token nonce must be nonempty");
        }
        if (!(claims.get("nonce") instanceof String nonce) || !expectedNonce.equals(nonce)) {
            throw new DecryptException("ID token nonce mismatch");
        }
        return claims;
    }

    /** Opens a signed-then-encrypted ID token and verifies the inner token. */
    public static Map<String, Object> decryptIdToken(String token, String jwks, String issuer, String audience, PrivateJwk... keys) {
        return verifyIdToken(decrypt(token, keys).text(), jwks, issuer, audience);
    }

    /** Opens a signed-then-encrypted ID token and verifies its inner signature, claims and nonce. */
    public static Map<String, Object> decryptIdToken(String token, String jwks, String issuer, String audience, String expectedNonce, PrivateJwk... keys) {
        return verifyIdToken(decrypt(token, keys).text(), jwks, issuer, audience, expectedNonce);
    }

    /**
     * Opens the plugin-API envelope (data {"jwe"}, or the whole response body) and returns the plaintext
     * JSON. ctx, when not null, must equal the header ctx. A plain body is refused (fail closed).
     */
    public static String openServerData(String response, String ctx, PrivateJwk... keys) {
        Map<String, Object> o = obj(Json.parse(response));
        Object jwe = o.get("jwe");
        if (jwe == null && o.get("data") instanceof Map) {
            jwe = obj(o.get("data")).get("jwe");
        }
        if (!(jwe instanceof String)) {
            throw new DecryptException("response is not encrypted");
        }
        Decrypted d = decrypt((String) jwe, keys);
        if (ctx != null && !ctx.equals(d.header.get("ctx"))) {
            throw new DecryptException("envelope ctx mismatch");
        }
        return d.text();
    }

    /**
     * Opens an encrypted webhook body ({"id","event","timestamp","jwe"}); verify the signature with
     * NoWorkWebhook first. Returns the inner document JSON (it must repeat id, event and timestamp).
     */
    public static String openWebhook(String body, PrivateJwk... keys) {
        Map<String, Object> outer = obj(Json.parse(body));
        if (!(outer.get("jwe") instanceof String)) {
            throw new DecryptException("webhook is not encrypted");
        }
        Decrypted d = decrypt((String) outer.get("jwe"), keys);
        Object ctx = d.header.get("ctx");
        if (!(ctx instanceof String) || !((String) ctx).startsWith("webhook:")) {
            throw new DecryptException("envelope ctx mismatch");
        }
        Map<String, Object> doc = obj(Json.parse(d.text()));
        if (!String.valueOf(doc.get("id")).equals(String.valueOf(outer.get("id")))
                || !String.valueOf(doc.get("event")).equals(String.valueOf(outer.get("event")))
                || Json.num(doc.get("timestamp")).longValue() != Json.num(outer.get("timestamp")).longValue()) {
            throw new DecryptException("webhook envelope does not match its content");
        }
        return d.text();
    }
}
