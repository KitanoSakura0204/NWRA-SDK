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

test('Webhook signature verification', async () => {
  const { verifyWebhook, webhookSignature, WebhookError } = await import('../dist/index.js');
  const body = JSON.stringify({ id: 'e1', event: 'user.login', timestamp: 1, data: { userId: 7 } });
  const ts = Math.floor(Date.now() / 1000);
  const sig = await webhookSignature('whsec_test', ts, body);
  const ev = await verifyWebhook('whsec_test', { 'x-lounge-timestamp': String(ts), 'X-Lounge-Signature': sig }, body);
  assert.equal(ev.event, 'user.login');
  assert.equal(ev.data.userId, 7);
  await assert.rejects(verifyWebhook('wrong', { 'X-Lounge-Timestamp': String(ts), 'X-Lounge-Signature': sig }, body), WebhookError);
  const old = ts - 3600;
  const oldSig = await webhookSignature('whsec_test', old, body);
  await assert.rejects(verifyWebhook('whsec_test', new Headers({ 'X-Lounge-Timestamp': String(old), 'X-Lounge-Signature': oldSig }), body), WebhookError);
  assert.equal((await verifyWebhook('whsec_test', { 'X-Lounge-Timestamp': String(old), 'X-Lounge-Signature': oldSig }, body, 0)).id, 'e1');
});

test('Server API fails closed on an encrypted reply', async () => {
  const { EncryptionError, HEADER_ENCRYPTION } = await import('../dist/index.js');
  const sealed = { code: 0, message: 'ok', data: { jwe: 'aGVhZA.a2V5.aXY.Y2lwaGVy.dGFn' } };
  for (const header of ['JWE', '']) { // a proxy may drop the header: the body shape still counts
    const client = new LoungeClient('https://account.example', {
      serverKey: 'k',
      fetch: async () => new Response(JSON.stringify(sealed), { headers: header ? { [HEADER_ENCRYPTION]: header } : {} })
    });
    await assert.rejects(client.whitelist(), e => e instanceof EncryptionError && e.route === 'whitelist');
    await assert.rejects(client.checkWhitelist('Steve'), e => e instanceof EncryptionError && e.route === 'whitelist/check');
    await assert.rejects(client.pollGMCommands(), EncryptionError);
    const raw = await client.raw('GET', '/server/v1/whitelist');
    assert.equal(raw.encrypted, true);
    assert.deepEqual(raw.envelope, sealed);
  }
  const plain = new LoungeClient('https://account.example', { serverKey: 'k', fetch: async () => json({ code: 0, data: { jwe: 'x', mode: 1, count: 0, entries: [] } }) });
  assert.equal((await plain.whitelist()).mode, 1);
});

test('Heartbeat sends tps and players only when valid', async () => {
  const bodies = [];
  const client = new LoungeClient('https://account.example', {
    serverKey: 'k',
    fetch: async (_url, init) => { bodies.push(JSON.parse(init.body)); return json({ code: 0, data: { status: 'ok' } }); }
  });
  await client.heartbeat(2, 20, '1.21.1');
  await client.heartbeat({ online: 2, max: 20, version: '1.21.1', tps: 19.8, players: [{ name: 'Steve', uuid: '069a79f4-44e9-4726-a5be-fca90e38aaf5' }, { name: 'Alex' }] });
  await client.heartbeat({ online: 0, tps: NaN, players: [] });
  assert.deepEqual(bodies, [
    { online: 2, max: 20, version: '1.21.1' },
    { online: 2, max: 20, version: '1.21.1', tps: 19.8, players: [{ name: 'Steve', uuid: '069a79f4-44e9-4726-a5be-fca90e38aaf5' }, { name: 'Alex' }] },
    { online: 0, max: 0, version: '' }
  ]);
});

test('GM catalogue publish sends the object form and lists skipped entries', async () => {
  const calls = [];
  const client = new LoungeClient('https://account.example', {
    serverKey: 'k',
    fetch: async (url, init) => {
      calls.push([init.method, url.pathname, JSON.parse(init.body)]);
      return json({ code: 0, data: url.pathname.endsWith('/options') ? { count: 1 } : { count: 1, skipped: [{ key: 'bad key', reason: 'invalid key' }] } });
    }
  });
  const kick = { key: 'kick', title: 'Kick', category: 'players', danger: 'confirm',
    params: [{ name: 'player', type: 'player', required: true }, { name: 'world', type: 'select', source: 'worlds', default: 'overworld' }] };
  const worlds = { worlds: [{ value: 'overworld', label: 'Overworld' }] };
  const res = await client.publishGMCatalog([kick], worlds);
  assert.deepEqual(res, { count: 1, skipped: [{ key: 'bad key', reason: 'invalid key' }] });
  assert.deepEqual((await client.publishGMOptions(worlds)).skipped, []);
  await client.publishGMCatalog([kick]);
  assert.deepEqual(calls, [
    ['PUT', '/server/v1/gm/actions', { actions: [kick], options: worlds }],
    ['PUT', '/server/v1/gm/options', worlds],
    ['PUT', '/server/v1/gm/actions', { actions: [kick] }]
  ]);
});
