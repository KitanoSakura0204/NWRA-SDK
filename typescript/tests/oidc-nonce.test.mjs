import test from 'node:test';
import assert from 'node:assert/strict';
import { constants, createCipheriv, createPublicKey, generateKeyPairSync, publicEncrypt, randomBytes, sign, webcrypto } from 'node:crypto';
import { decryptIdToken, verifyIdToken, DecryptError } from '../dist/index.js';

globalThis.crypto ??= webcrypto;
const signing = generateKeyPairSync('rsa', { modulusLength: 2048 });
const encryption = generateKeyPairSync('rsa', { modulusLength: 2048 });
const privateJwk = encryption.privateKey.export({ format: 'jwk' });
const jwks = { keys: [{ ...signing.publicKey.export({ format: 'jwk' }), kid: 'signing-key', extension: ['retained'] }] };
const opts = { issuer: 'https://account.example', audience: 'client', now: 2_000_000_000 };
const claims = { iss: opts.issuer, aud: opts.audience, exp: opts.now + 300, nbf: opts.now - 1, sub: 'subject', nonce: 'login-nonce' };

// Independent Node crypto fixtures exercise the public WebCrypto verifier and decryptor.
function signed(payload = claims) {
  const input = [Buffer.from(JSON.stringify({ alg: 'RS256', kid: 'signing-key' })).toString('base64url'),
    Buffer.from(JSON.stringify(payload)).toString('base64url')].join('.');
  return input + '.' + sign('RSA-SHA256', Buffer.from(input), signing.privateKey).toString('base64url');
}

function encrypted(plaintext) {
  const header = Buffer.from(JSON.stringify({ alg: 'RSA-OAEP-256', enc: 'A256GCM' })).toString('base64url');
  const cek = randomBytes(32);
  const iv = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', cek, iv);
  cipher.setAAD(Buffer.from(header));
  const ciphertext = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
  const wrapped = publicEncrypt({ key: createPublicKey(encryption.privateKey), padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' }, cek);
  return [header, wrapped.toString('base64url'), iv.toString('base64url'), ciphertext.toString('base64url'),
    cipher.getAuthTag().toString('base64url')].join('.');
}

function badSignature(token) {
  const parts = token.split('.');
  const bytes = Buffer.from(parts[2], 'base64url');
  bytes[0] ^= 1;
  parts[2] = bytes.toString('base64url');
  return parts.join('.');
}

test('signed nonce matches exactly, preserves claims, and remains optional for existing callers', async () => {
  assert.deepEqual(await verifyIdToken(signed(), jwks, { ...opts, expectedNonce: 'login-nonce' }), claims);
  const withoutNonce = { ...claims };
  delete withoutNonce.nonce;
  assert.deepEqual(await verifyIdToken(signed(withoutNonce), jwks, opts), withoutNonce);
});

test('explicit expectedNonce must be a nonempty string', async () => {
  for (const expectedNonce of ['', undefined, null, 7, {}, []]) {
    await assert.rejects(verifyIdToken(signed(), jwks, { ...opts, expectedNonce }),
      error => error instanceof DecryptError && /nonce/i.test(error.message), String(expectedNonce));
  }
});

test('nonce-bound verification refuses missing, empty, mismatched, and non-string signed nonce claims', async () => {
  for (const nonce of [undefined, '', 'other-nonce', 7, null, {}, ['login-nonce']]) {
    await assert.rejects(verifyIdToken(signed({ ...claims, nonce }), jwks, { ...opts, expectedNonce: 'login-nonce' }),
      error => error instanceof DecryptError && /nonce/i.test(error.message));
  }
});

test('signature, issuer, audience, and time validation run before nonce rejection', async () => {
  const invalidNonce = { ...opts, expectedNonce: '' };
  await assert.rejects(verifyIdToken(badSignature(signed()), jwks, invalidNonce), /invalid ID token signature/);
  for (const [payload, message] of [
    [{ ...claims, iss: 'https://other.example' }, /issuer mismatch/],
    [{ ...claims, aud: 'other-client' }, /audience mismatch/],
    [{ ...claims, exp: opts.now - 61 }, /expired/],
    [{ ...claims, nbf: opts.now + 61 }, /not valid yet/],
  ]) {
    await assert.rejects(verifyIdToken(signed(payload), jwks, invalidNonce), message);
  }
});

test('encrypted ID tokens forward expectedNonce and still verify the inner signature', async () => {
  const token = encrypted(signed());
  assert.deepEqual(await decryptIdToken(token, privateJwk, jwks, { ...opts, expectedNonce: 'login-nonce' }), claims);
  assert.deepEqual(await decryptIdToken(token, privateJwk, jwks, opts), claims);
  for (const expectedNonce of ['', undefined, 'other-nonce']) {
    await assert.rejects(decryptIdToken(token, privateJwk, jwks, { ...opts, expectedNonce }), /nonce/i);
  }
  await assert.rejects(decryptIdToken(encrypted(badSignature(signed())), privateJwk, jwks,
    { ...opts, expectedNonce: 'login-nonce' }), /invalid ID token signature/);
});
