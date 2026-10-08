import test from 'node:test';
import assert from 'node:assert/strict';
import { NoworkClient, ApiError, parseLoginCallback } from '../dist/index.js';

test('GM exchange sends private JSON and preserves server authorization errors', async () => {
  for (const denied of [false, true]) {
    const client = new NoworkClient('https://account.example', {
      serverKey: 'test-key',
      fetch: async (url, init) => {
        assert.equal(url.pathname, '/server/v1/gm/sso/exchange');
        assert.equal(url.search, '');
        assert.equal(init.headers['X-Server-Key'], 'test-key');
        assert.deepEqual(JSON.parse(init.body), { code: 'one-use', codeVerifier: 'private', redirectUri: 'https://gm.example/cb' });
        return new Response(JSON.stringify(denied ? { code: 403, message: 'denied' } : {
          code: 0, data: { serverId: 1, subject: 'gm-role:7', character: { id: 7 }, scope: 'character:login' }
        }), { status: denied ? 403 : 200 });
      }
    });
    const promise = client.exchangeGMCode('one-use', 'private', 'https://gm.example/cb');
    if (denied) await assert.rejects(promise, e => e instanceof ApiError && e.code === 403);
    else assert.equal((await promise).subject, 'gm-role:7');
  }
});

test('login callbacks reject missing state and duplicate parameters', () => {
  assert.throws(() => parseLoginCallback('mygame://cb?code=x', ''));
  assert.throws(() => parseLoginCallback('mygame://cb?state=s&state=bad&code=x', 's'));
  assert.throws(() => parseLoginCallback('mygame://cb?state=s&code=x&code=y', 's'));
});

test('catalog null preserves actions while an empty array clears them', async () => {
  const bodies = [];
  const client = new NoworkClient('https://account.example', { serverKey: 'key', fetch: async (_url, init) => {
    bodies.push(JSON.parse(init.body));
    return new Response(JSON.stringify({ code: 0, data: { count: 0 } }));
  } });
  await client.publishGMCatalog(null, {});
  await client.publishGMCatalog([], {});
  assert.deepEqual(bodies, [{ options: {} }, { actions: [], options: {} }]);
});
