import test from 'node:test';
import assert from 'node:assert/strict';
import { LoungeClient, LoungeOIDC, ApiError, OAuthError, TransportError } from '../dist/index.js';

const json = (data, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });

test('Server API sends its key and encodes player names', async () => {
  const client = new LoungeClient('https://account.example/', {
    serverKey: 'test-key',
    fetch: async (url, init) => {
      assert.equal(init.headers['X-Server-Key'], 'test-key');
      assert.equal(url.pathname, '/server/v1/whitelist/check');
      assert.equal(url.searchParams.get('name'), 'Player & One');
      return json({ code: 0, data: { allowed: true, bound: false, reason: 'whitelist disabled' } });
    }
  });
  const result = await client.checkWhitelist('Player & One');
  assert.equal(result.allowed, true);
  assert.equal(result.bound, false);
});

test('Server API rejects missing envelopes and HTTP failures', async () => {
  for (const [body, status] of [[{}, 200], [{ code: 0 }, 200], [{ code: 0, data: { allowed: true } }, 503]]) {
    const client = new LoungeClient('https://account.example', { serverKey: 'test-key', fetch: async () => json(body, status) });
    await assert.rejects(client.whitelist(), TransportError);
  }
  const client = new LoungeClient('https://account.example', { serverKey: 'test-key', fetch: async () => json({ code: 401, message: 'invalid server key' }, 401) });
  await assert.rejects(client.whitelist(), error => error instanceof ApiError && error.code === 401);
});

test('Server API timeout includes reading the response body', async () => {
  const client = new LoungeClient('https://account.example', {
    serverKey: 'test-key', timeoutMs: 10,
    fetch: async (_url, { signal }) => ({
      status: 200, ok: true,
      json: () => new Promise((_resolve, reject) => signal.addEventListener('abort', () => reject(new Error('aborted')), { once: true }))
    })
  });
  await assert.rejects(client.whitelist(), TransportError);
});

test('OIDC code exchange encodes client authentication and PKCE', async () => {
  const oidc = new LoungeOIDC('https://account.example/', 'client', 'secret', 'https://app.example/callback', async (url, init) => {
    assert.equal(url, 'https://account.example/oauth2/token');
    assert.equal(init.body.get('client_id'), 'client');
    assert.equal(init.body.get('client_secret'), 'secret');
    assert.equal(init.body.get('grant_type'), 'authorization_code');
    assert.equal(init.body.get('code_verifier'), 'verifier');
    assert.equal(init.body.get('redirect_uri'), 'https://app.example/callback');
    return json({ access_token: 'access', token_type: 'Bearer', expires_in: 3600 });
  });
  const url = new URL(oidc.authorizeUrl('state & +', 'nonce', 'openid security_level', 'challenge'));
  assert.equal(url.searchParams.get('state'), 'state & +');
  assert.equal(url.searchParams.get('code_challenge_method'), 'S256');
  assert.equal((await oidc.exchange('code', 'verifier')).access_token, 'access');
});

test('OIDC accepts the backend empty revocation response', async () => {
  const oidc = new LoungeOIDC('https://account.example', 'client', 'secret', '', async () => new Response('', { status: 200 }));
  await oidc.revoke('access');
  await assert.rejects(oidc.exchange('code'), TransportError);
});

test('OIDC separates protocol errors from invalid gateway bodies', async () => {
  const oauth = new LoungeOIDC('https://account.example', 'client', '', '', async () => json({ error: 'invalid_grant', error_description: 'expired' }, 400));
  await assert.rejects(oauth.exchange('code'), error => error instanceof OAuthError && error.status === 400);
  const gateway = new LoungeOIDC('https://account.example', 'client', '', '', async () => new Response('<html>unavailable</html>', { status: 502 }));
  await assert.rejects(gateway.exchange('code'), TransportError);
});

test('OIDC timeout aborts a pending request', async () => {
  const oidc = new LoungeOIDC('https://account.example', 'client', '', '', async (_url, { signal }) => new Promise((_resolve, reject) => {
    signal.addEventListener('abort', () => reject(new Error('aborted')), { once: true });
  }), 10);
  await assert.rejects(oidc.userInfo('access'), TransportError);
});
