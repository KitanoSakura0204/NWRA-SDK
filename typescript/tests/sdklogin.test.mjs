import test from 'node:test';
import assert from 'node:assert/strict';
import { s256Challenge, createPkcePair, createState, buildLoginUrl, parseLoginCallback, readLoginMessage, normalizeLoginCode, SdkLoginError, SDK_LOGIN_MESSAGE } from '../dist/index.js';

test('PKCE S256 matches RFC 7636 and pairs are fresh', async () => {
  assert.equal(await s256Challenge('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk'), 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM');
  const a = await createPkcePair();
  const b = await createPkcePair();
  assert.equal(a.verifier.length, 43);
  assert.equal(a.challenge, await s256Challenge(a.verifier));
  assert.notEqual(a.verifier, b.verifier);
  assert.match(createState(), /^[A-Za-z0-9_-]{22}$/);
});

test('buildLoginUrl encodes the request and requires PKCE', () => {
  const u = new URL(buildLoginUrl('https://account.example.com/', { serverCode: 's 1', redirectUri: 'mygame://auth', state: 'a&b', codeChallenge: 'ch', uiLocales: 'en' }));
  assert.equal(u.pathname, '/sdk/login/s%201');
  assert.equal(u.searchParams.get('redirect_uri'), 'mygame://auth');
  assert.equal(u.searchParams.get('state'), 'a&b');
  assert.equal(u.searchParams.get('code_challenge_method'), 'S256');
  assert.equal(u.searchParams.has('scope'), false);
  assert.throws(() => buildLoginUrl('https://a', { serverCode: 's1', codeChallenge: '' }));
});

test('parseLoginCallback checks state and issuer and surfaces errors', () => {
  const ok = parseLoginCallback('mygame://auth?code=abc&state=st&iss=https%3A%2F%2Fid.example', 'st', 'https://id.example');
  assert.deepEqual(ok, { code: 'abc', state: 'st', iss: 'https://id.example' });
  assert.throws(() => parseLoginCallback('mygame://auth?code=abc&state=x', 'st'), (e) => e instanceof SdkLoginError && e.code === 'state_mismatch');
  assert.throws(() => parseLoginCallback('mygame://auth?code=abc&state=st&iss=https%3A%2F%2Fevil', 'st', 'https://id.example'), SdkLoginError);
  assert.throws(() => parseLoginCallback('mygame://auth?error=access_denied&state=st', 'st'), (e) => e.code === 'access_denied');
});

test('readLoginMessage ignores foreign origins and other messages', () => {
  const data = { type: SDK_LOGIN_MESSAGE, code: 'abc', state: 'st', iss: 'https://id.example' };
  assert.equal(readLoginMessage({ origin: 'https://evil.example', data }, 'https://account.example.com', 'st'), undefined);
  assert.equal(readLoginMessage({ origin: 'https://account.example.com', data: { type: 'other' } }, 'https://account.example.com', 'st'), undefined);
  assert.equal(readLoginMessage({ origin: 'https://account.example.com', data }, 'https://account.example.com/sdk/login/s1', 'st').code, 'abc');
  assert.throws(() => readLoginMessage({ origin: 'https://account.example.com', data: { ...data, state: 'x' } }, 'https://account.example.com', 'st'), SdkLoginError);
  assert.equal(normalizeLoginCode(' abcd-2345 '), 'ABCD2345');
});
