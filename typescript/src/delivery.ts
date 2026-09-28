/**
 * Delivery helpers (SDK/docs/DELIVERY.md): decrypt what the user centre sends when an app
 * registered an encryption key. RSA-OAEP-256 or ECDH-ES+A256KW (P-256 / X25519), enc A256GCM.
 * WebCrypto only (Node 18+ / browsers; X25519 needs Node 20+ or a current browser).
 */

export class DecryptError extends Error {
  constructor(message = 'cannot decrypt the JWE') {
    super(message);
    this.name = 'DecryptError';
  }
}

/** A private JWK of the app (RSA, EC P-256 or OKP X25519). */
export type PrivateJwk = Record<string, string>;

const enc = new TextEncoder();
const dec = new TextDecoder();

function subtle(): SubtleCrypto {
  const s = (globalThis as { crypto?: Crypto }).crypto?.subtle;
  if (!s) throw new DecryptError('WebCrypto (crypto.subtle) is not available');
  return s;
}

function b64u(s: string): Uint8Array {
  const b = atob(s.replace(/-/g, '+').replace(/_/g, '/') + '==='.slice((s.length + 3) % 4));
  const out = new Uint8Array(b.length);
  for (let i = 0; i < b.length; i++) out[i] = b.charCodeAt(i);
  return out;
}

/** A fresh ArrayBuffer copy (keeps WebCrypto's BufferSource typing simple). */
function ab(u: Uint8Array): ArrayBuffer {
  const out = new ArrayBuffer(u.byteLength);
  new Uint8Array(out).set(u);
  return out;
}

function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let o = 0;
  for (const p of parts) {
    out.set(p, o);
    o += p.length;
  }
  return out;
}

function u32(n: number): Uint8Array {
  return new Uint8Array([(n >>> 24) & 255, (n >>> 16) & 255, (n >>> 8) & 255, n & 255]);
}

/** JWA §4.6.2 Concat KDF for a 256-bit key (one SHA-256 round). */
async function concatKdf(z: Uint8Array, alg: string, apu: Uint8Array, apv: Uint8Array): Promise<Uint8Array> {
  const a = enc.encode(alg);
  const info = concat(u32(a.length), a, u32(apu.length), apu, u32(apv.length), apv, u32(256));
  return new Uint8Array(await subtle().digest('SHA-256', ab(concat(u32(1), z, info))));
}

/** Keep only the members WebCrypto needs (alg / use / key_ops of the file must not interfere). */
function only(jwk: PrivateJwk, names: string[]): JsonWebKey {
  const out: Record<string, string> = {};
  for (const n of names) if (jwk[n] !== undefined) out[n] = jwk[n];
  return out as JsonWebKey;
}

async function contentKey(hdr: Record<string, any>, ek: Uint8Array, jwk: PrivateJwk): Promise<CryptoKey> {
  const s = subtle();
  if (hdr.alg === 'RSA-OAEP-256' && jwk.kty === 'RSA') {
    const priv = await s.importKey('jwk', only(jwk, ['kty', 'n', 'e', 'd', 'p', 'q', 'dp', 'dq', 'qi']), { name: 'RSA-OAEP', hash: 'SHA-256' }, false, ['decrypt']);
    const cek = await s.decrypt({ name: 'RSA-OAEP' }, priv, ab(ek));
    return s.importKey('raw', cek, 'AES-GCM', false, ['decrypt']);
  }
  if (hdr.alg !== 'ECDH-ES+A256KW' || typeof hdr.epk !== 'object') throw new DecryptError();
  const epk = hdr.epk as Record<string, string>;
  let z: ArrayBuffer;
  if (jwk.kty === 'EC' && jwk.crv === 'P-256' && epk.kty === 'EC' && epk.crv === 'P-256') {
    const algo = { name: 'ECDH', namedCurve: 'P-256' };
    const priv = await s.importKey('jwk', only(jwk, ['kty', 'crv', 'x', 'y', 'd']), algo, false, ['deriveBits']);
    const pub = await s.importKey('jwk', only(epk, ['kty', 'crv', 'x', 'y']), algo, false, []);
    z = await s.deriveBits({ name: 'ECDH', public: pub } as EcdhKeyDeriveParams, priv, 256);
  } else if (jwk.kty === 'OKP' && jwk.crv === 'X25519' && epk.kty === 'OKP' && epk.crv === 'X25519') {
    const priv = await s.importKey('jwk', only(jwk, ['kty', 'crv', 'x', 'd']), { name: 'X25519' }, false, ['deriveBits']);
    const pub = await s.importKey('jwk', only(epk, ['kty', 'crv', 'x']), { name: 'X25519' }, false, []);
    z = await s.deriveBits({ name: 'X25519', public: pub } as unknown as EcdhKeyDeriveParams, priv, 256);
  } else {
    throw new DecryptError();
  }
  const kek = await concatKdf(new Uint8Array(z), hdr.alg, hdr.apu ? b64u(hdr.apu) : new Uint8Array(), hdr.apv ? b64u(hdr.apv) : new Uint8Array());
  const kw = await s.importKey('raw', ab(kek), 'AES-KW', false, ['unwrapKey']);
  return s.unwrapKey('raw', ab(ek), kw, 'AES-KW', 'AES-GCM', false, ['decrypt']);
}

export interface Decrypted {
  text: string;
  bytes: Uint8Array;
  header: Record<string, any>;
}

/**
 * Open a compact JWE. With several keys (a rotation overlap) the header kid picks one; without a
 * kid every key is tried. The protected header is returned for ctx / iat checks.
 */
export async function decryptJwe(compact: string, keys: PrivateJwk | PrivateJwk[]): Promise<Decrypted> {
  const parts = compact.split('.');
  if (parts.length !== 5) throw new DecryptError();
  let seg: Uint8Array[];
  let hdr: Record<string, any>;
  try {
    seg = parts.map((p) => b64u(p));
    hdr = JSON.parse(dec.decode(seg[0]));
  } catch {
    throw new DecryptError();
  }
  if (hdr.enc !== 'A256GCM' || hdr.zip !== undefined || hdr.crit !== undefined) throw new DecryptError();
  for (const jwk of Array.isArray(keys) ? keys : [keys]) {
    if (hdr.kid && jwk.kid && jwk.kid !== hdr.kid) continue;
    try {
      const cek = await contentKey(hdr, seg[1], jwk);
      const params = { name: 'AES-GCM', iv: ab(seg[2]), additionalData: ab(enc.encode(parts[0])), tagLength: 128 };
      const bytes = new Uint8Array(await subtle().decrypt(params, cek, ab(concat(seg[3], seg[4]))));
      return { bytes, text: dec.decode(bytes), header: hdr };
    } catch {
      // wrong key or tampered: try the next key
    }
  }
  throw new DecryptError();
}

export interface VerifyOptions {
  issuer?: string;
  audience?: string;
  /** Unix seconds (default: now). */
  now?: number;
}

/** Verify an RS256 ID token against the provider JWKS (GET BASE/jwks.json): signature, iss, aud, exp / nbf (60 s leeway). */
export async function verifyIdToken(token: string, jwks: { keys: Record<string, string>[] } | string, opts: VerifyOptions = {}): Promise<Record<string, any>> {
  const parts = token.split('.');
  if (parts.length !== 3) throw new DecryptError('not a signed JWT');
  const hdr = JSON.parse(dec.decode(b64u(parts[0])));
  if (hdr.alg !== 'RS256') throw new DecryptError('ID token must be RS256');
  const set = typeof jwks === 'string' ? JSON.parse(jwks) : jwks;
  const jwk = (set.keys as Record<string, string>[]).find((k) => k.kty === 'RSA' && (!hdr.kid || k.kid === hdr.kid));
  if (!jwk) throw new DecryptError('signing key not in the JWKS');
  const s = subtle();
  const pub = await s.importKey('jwk', only(jwk, ['kty', 'n', 'e']), { name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' }, false, ['verify']);
  if (!(await s.verify('RSASSA-PKCS1-v1_5', pub, ab(b64u(parts[2])), ab(enc.encode(parts[0] + '.' + parts[1]))))) {
    throw new DecryptError('invalid ID token signature');
  }
  const claims = JSON.parse(dec.decode(b64u(parts[1])));
  const now = opts.now ?? Math.floor(Date.now() / 1000);
  if (opts.issuer && claims.iss !== opts.issuer) throw new DecryptError('ID token issuer mismatch');
  const aud = claims.aud;
  if (opts.audience && aud !== opts.audience && !(Array.isArray(aud) && aud.includes(opts.audience))) throw new DecryptError('ID token audience mismatch');
  if (typeof claims.exp !== 'number' || claims.exp + 60 < now) throw new DecryptError('ID token expired');
  if (typeof claims.nbf === 'number' && claims.nbf - 60 > now) throw new DecryptError('ID token not valid yet');
  return claims;
}

/** Open a signed-then-encrypted ID token (id_token_encrypted_response_alg) and verify the inner token. */
export async function decryptIdToken(token: string, keys: PrivateJwk | PrivateJwk[], jwks: { keys: Record<string, string>[] } | string, opts: VerifyOptions = {}): Promise<Record<string, any>> {
  return verifyIdToken((await decryptJwe(token, keys)).text, jwks, opts);
}

/**
 * Open the plugin-API envelope: data {"jwe"} of a response with X-Lounge-Encryption: JWE (the whole
 * body is accepted too). ctx, when set, must equal the header ctx ("server:<code>:<route>"). A plain
 * body is refused: a server that turned encryption on must not silently accept plaintext.
 */
export async function openServerData<T = any>(response: string | Record<string, any>, keys: PrivateJwk | PrivateJwk[], ctx?: string): Promise<T> {
  const obj = typeof response === 'string' ? JSON.parse(response) : response;
  const jwe = obj?.jwe ?? obj?.data?.jwe;
  if (typeof jwe !== 'string') throw new DecryptError('response is not encrypted');
  const out = await decryptJwe(jwe, keys);
  if (ctx && out.header.ctx !== ctx) throw new DecryptError('envelope ctx mismatch');
  return JSON.parse(out.text) as T;
}

/**
 * Open an encrypted webhook body ({"id","event","timestamp","jwe"}); verify the signature over the
 * raw body with verifyWebhook first. The inner document must repeat the outer id, event, timestamp.
 */
export async function openWebhook<T = Record<string, unknown>>(body: string, keys: PrivateJwk | PrivateJwk[]): Promise<{ id: string; event: string; timestamp: number; data: T }> {
  const outer = JSON.parse(body);
  if (typeof outer.jwe !== 'string') throw new DecryptError('webhook is not encrypted');
  const out = await decryptJwe(outer.jwe, keys);
  if (typeof out.header.ctx !== 'string' || !out.header.ctx.startsWith('webhook:')) throw new DecryptError('envelope ctx mismatch');
  const ev = JSON.parse(out.text);
  if (ev.id !== outer.id || ev.event !== outer.event || ev.timestamp !== outer.timestamp) {
    throw new DecryptError('webhook envelope does not match its content');
  }
  return ev;
}
