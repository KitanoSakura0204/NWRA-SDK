import assert from 'node:assert/strict';
import test, { before } from 'node:test';
import { webcrypto } from 'node:crypto';
import { base64url, createLoginClient, decodeJWTUnverified, parseAuthorizationResponse, pkceChallenge, verifyIdToken } from '../index.js';

const issuer = 'https://accounts.example.test';
const clientId = 'public-example';
const callback = 'https://app.example.test/callback';
let signingKey;
let jwks;
before(async () => {
  signingKey = await webcrypto.subtle.generateKey({ name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256', modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]) }, true, ['sign', 'verify']);
  jwks = { keys: [{ ...await webcrypto.subtle.exportKey('jwk', signingKey.publicKey), kid: 'test-1', alg: 'RS256', use: 'sig' }] };
});

async function signedToken(overrides = {}, header = {}) {
  const now = Math.floor(Date.now() / 1000);
  const claims = { iss: issuer, sub: 'owner-user', aud: clientId, exp: now + 300, iat: now, nonce: 'bound-nonce', ...overrides };
  const body = `${base64url(new TextEncoder().encode(JSON.stringify({ alg: 'RS256', typ: 'JWT', kid: 'test-1', ...header })))}.${base64url(new TextEncoder().encode(JSON.stringify(claims)))}`;
  const signature = await webcrypto.subtle.sign('RSASSA-PKCS1-v1_5', signingKey.privateKey, new TextEncoder().encode(body));
  return `${body}.${base64url(signature)}`;
}

const verification = () => ({ issuer, clientId, jwks, nonce: 'bound-nonce' });
const transaction = () => ({ issuer, clientId, redirectUri: callback, state: 'bound-state', nonce: 'bound-nonce', verifier: 'v'.repeat(43), createdAt: Date.now() });

test('S256 uses the RFC 7636 vector and rejects weak verifiers', async () => {
  assert.equal(await pkceChallenge('dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk'), 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM');
  await assert.rejects(pkceChallenge('short'), { code: 'invalid_code_verifier' });
});

test('callbacks reject mismatched origins, state, issuer, duplicates and expired transactions', () => {
  assert.equal(parseAuthorizationResponse(`${callback}?code=code&state=bound-state`, transaction()), 'code');
  for (const raw of [
    'https://app.example.test.attacker.test/callback?code=x&state=bound-state',
    `${callback}?code=x&state=wrong`, `${callback}?code=x&state=bound-state&state=bound-state`,
    `${callback}?code=x&code=y&state=bound-state`, `${callback}?error=access_denied`,
    `${callback}?code=x&error=access_denied&state=bound-state`, `${callback}?code=x&state=bound-state&iss=https://other.test`,
    `${callback}?code=x&state=bound-state#access_token=unsafe`
  ]) assert.throws(() => parseAuthorizationResponse(raw, transaction()));
  assert.throws(() => parseAuthorizationResponse(`${callback}?code=x&state=bound-state`, { ...transaction(), createdAt: Date.now() - 301000 }), { code: 'transaction_expired' });
  assert.throws(() => parseAuthorizationResponse(`${callback}?code=x&state=bound-state`, { ...transaction(), createdAt: undefined }), { code: 'transaction_expired' });
});

test('only a valid signature with issuer, audience, lifetime and nonce creates verified claims', async () => {
  const token = await signedToken();
  assert.equal(decodeJWTUnverified(token).verified, false);
  assert.equal((await verifyIdToken(token, verification())).sub, 'owner-user');
  const tampered = token.split('.');
  tampered[1] = base64url(new TextEncoder().encode(JSON.stringify({ ...decodeJWTUnverified(token).claims, sub: 'attacker' })));
  assert.equal(decodeJWTUnverified(tampered.join('.')).claims.sub, 'attacker');
  await assert.rejects(verifyIdToken(tampered.join('.'), verification()), { code: 'invalid_id_token_signature' });
  for (const claims of [
    { iss: 'https://attacker.test' }, { aud: 'other-app' }, { aud: [clientId, 'other-app'] },
    { azp: 'other-app' }, { exp: 1 }, { nonce: 'wrong' }, { nonce: undefined },
    { sub: '' }, { iat: Math.floor(Date.now() / 1000) + 120 }, { token_use: 'access' }
  ]) await assert.rejects(verifyIdToken(await signedToken(claims), verification()));
  await assert.rejects(verifyIdToken(await signedToken({}, { alg: 'none' }), verification()), { code: 'unsupported_id_token' });
  await assert.rejects(verifyIdToken(token, { ...verification(), jwks: { keys: [jwks.keys[0], jwks.keys[0]] } }), { code: 'signing_key_not_found' });
});

test('at_hash and refresh identity are checked after signature verification', async () => {
  const access = 'opaque-access';
  const digest = new Uint8Array(await webcrypto.subtle.digest('SHA-256', new TextEncoder().encode(access)));
  const token = await signedToken({ at_hash: base64url(digest.slice(0, 16)) });
  await verifyIdToken(token, { ...verification(), accessToken: access });
  await assert.rejects(verifyIdToken(token, { ...verification(), accessToken: 'different' }), { code: 'access_token_hash_mismatch' });
  await verifyIdToken(await signedToken({ nonce: undefined }), { ...verification(), refresh: true, subject: 'owner-user' });
  await assert.rejects(verifyIdToken(await signedToken({ sub: 'another-user' }), { ...verification(), refresh: true, subject: 'owner-user' }), { code: 'subject_mismatch' });
});

function fakeBrowser(t) {
  const previous = globalThis.window;
  const listeners = new Set();
  const storage = new Map();
  const popup = { closed: false, location: { replace: (url) => { browser.authorizeUrl = url; } }, close() { this.closed = true; } };
  const browser = {
    location: { origin: 'https://app.example.test', href: callback, assign: (url) => { browser.authorizeUrl = url; } },
    history: { replaceState: (_data, _unused, path) => { browser.cleanedPath = path; } },
    sessionStorage: { getItem: (key) => storage.get(key) || null, setItem: (key, value) => storage.set(key, value), removeItem: (key) => storage.delete(key) },
    open: () => popup,
    addEventListener: (_type, fn) => listeners.add(fn), removeEventListener: (_type, fn) => listeners.delete(fn),
    dispatch: (event) => { for (const listener of [...listeners]) listener(event); }, storage, popup
  };
  globalThis.window = browser;
  t.after(() => { globalThis.window = previous; });
  return browser;
}

async function until(predicate) {
  for (let i = 0; i < 100; i++) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  assert.fail('condition was not reached');
}

function provider(browser) {
  const calls = [];
  let refreshes = 0;
  return { calls, fetch: async (url, init) => {
    assert.equal(init.credentials, 'omit');
    assert.equal(init.redirect, 'error');
    calls.push({ url, init });
    const json = (data) => new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } });
    if (url.endsWith('/.well-known/openid-configuration')) return json({ issuer, authorization_endpoint: `${issuer}/oauth2/authorize`, token_endpoint: `${issuer}/oauth2/token`, userinfo_endpoint: `${issuer}/oauth2/userinfo`, revocation_endpoint: `${issuer}/oauth2/revoke`, jwks_uri: `${issuer}/jwks`, response_types_supported: ['code'], code_challenge_methods_supported: ['S256'], id_token_signing_alg_values_supported: ['RS256'] });
    if (url.endsWith('/jwks')) return json(jwks);
    if (url.endsWith('/oauth2/token')) {
      const request = new URLSearchParams(init.body);
      assert.equal(request.get('client_id'), clientId);
      assert.equal(request.has('client_secret'), false);
      const authorization = new URL(browser.authorizeUrl);
      if (request.get('grant_type') === 'authorization_code') {
        assert.equal(await pkceChallenge(request.get('code_verifier')), authorization.searchParams.get('code_challenge'));
        assert.equal(request.get('redirect_uri'), callback);
      } else { assert.equal(request.get('refresh_token'), 'refresh-1'); refreshes++; }
      return json({ access_token: `access-${refreshes + 1}`, refresh_token: `refresh-${refreshes + 1}`, token_type: 'Bearer', expires_in: 300,
        id_token: await signedToken({ nonce: authorization.searchParams.get('nonce') }) });
    }
    if (url.endsWith('/oauth2/userinfo')) return json({ sub: 'owner-user', name: 'Owner' });
    if (url.endsWith('/oauth2/revoke')) return new Response('', { status: 200 });
    assert.fail(`unexpected endpoint ${url}`);
  } };
}

test('popup ignores wrong origins/sources, consumes state, rotates and revokes without storage', async (t) => {
  const browser = fakeBrowser(t);
  const op = provider(browser);
  const client = createLoginClient({ issuer, clientId, redirectUri: callback, fetch: op.fetch });
  const pending = client.loginPopup();
  await until(() => browser.authorizeUrl);
  const state = new URL(browser.authorizeUrl).searchParams.get('state');
  const data = { type: 'nowork.oidc.callback.v1', responseUrl: `${callback}?code=real-code&state=${state}` };
  browser.dispatch({ origin: 'https://attacker.test', source: browser.popup, data });
  browser.dispatch({ origin: browser.location.origin, source: {}, data });
  assert.equal(op.calls.filter((call) => call.url.endsWith('/oauth2/token')).length, 0);
  browser.dispatch({ origin: browser.location.origin, source: browser.popup, data });
  const result = await pending;
  assert.equal(result.claims.sub, 'owner-user');
  assert.equal(browser.popup.closed, true);
  assert.equal(browser.storage.size, 0);
  assert.equal((await client.userinfo()).sub, 'owner-user');
  assert.equal((await client.refresh()).tokens.refresh_token, 'refresh-2');
  await client.revoke();
  assert.equal(op.calls.filter((call) => call.url.endsWith('/oauth2/revoke')).length, 2);
  await assert.rejects(client.userinfo(), { code: 'login_required' });
});

test('redirect storage contains only one-use proof, never returned tokens', async (t) => {
  const browser = fakeBrowser(t);
  const op = provider(browser);
  const config = { issuer, clientId, redirectUri: callback, fetch: op.fetch };
  await createLoginClient(config).loginRedirect();
  const stored = JSON.parse([...browser.storage.values()][0]);
  assert.ok(stored.verifier);
  assert.equal(stored.access_token, undefined);
  const raw = `${callback}?code=redirect-code&state=${stored.state}`;
  const afterNavigation = createLoginClient(config);
  assert.equal((await afterNavigation.completeRedirect(raw)).claims.sub, 'owner-user');
  assert.equal(browser.storage.size, 0);
  assert.equal(browser.cleanedPath, '/callback');
  await assert.rejects(afterNavigation.completeRedirect(raw), { code: 'transaction_not_found' });
});

test('provider metadata cannot change the configured issuer or send tokens to another origin', async (t) => {
  fakeBrowser(t);
  const client = createLoginClient({ issuer, clientId, redirectUri: callback,
    fetch: async () => new Response(JSON.stringify({ issuer: 'https://attacker.test' }), { status: 200 }) });
  await assert.rejects(client.loginRedirect(), { code: 'issuer_mismatch' });
  assert.throws(() => createLoginClient({ issuer, clientId, redirectUri: 'https://other.test/callback' }), { code: 'callback_origin_mismatch' });
});

test('a bounded streamed discovery response fails before it can configure endpoints', async (t) => {
  fakeBrowser(t);
  const client = createLoginClient({ issuer, clientId, redirectUri: callback,
    fetch: async () => new Response('x'.repeat(65537), { status: 200 }) });
  await assert.rejects(client.loginRedirect(), { code: 'response_too_large' });
});
