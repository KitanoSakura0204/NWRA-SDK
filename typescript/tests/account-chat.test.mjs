import test from 'node:test';
import assert from 'node:assert/strict';
import { AccountClient, ChatClient, NoworkClient, ApiError, TransportError } from '../dist/index.js';

test('portal client isolates JWT and open-server role binding', async () => {
  const client = new AccountClient('https://account.example', { token: 'site-jwt', fetch: async (url, init) => {
    assert.equal(url, 'https://account.example/api/account/portal/servers/3/roles');
    assert.equal(init.headers.Authorization, 'Bearer site-jwt');
    assert.equal(init.headers['X-Server-Key'], undefined);
    assert.equal(init.redirect, 'error');
    assert.deepEqual(JSON.parse(init.body), { roleName: 'Alex' });
    return new Response(JSON.stringify({ code: 0, data: { id: 3, requiresWhitelist: false } }));
  } });
  assert.equal((await client.bindRole(3, { roleName: 'Alex' })).requiresWhitelist, false);
});

test('portal preserves business errors and rejects broken envelopes', async () => {
  for (const [body, status, error] of [[{}, 200, TransportError], [{ code: 0, data: [] }, 503, TransportError],
      [{ code: 403, message: 'denied' }, 403, ApiError]]) {
    const client = new AccountClient('https://account.example', { token: 'jwt', fetch: async () => new Response(JSON.stringify(body), { status }) });
    await assert.rejects(client.servers(), error);
  }
});

test('chat relay acknowledgments use done rather than bot event ids', async () => {
  const client = new NoworkClient('https://account.example', { serverKey: 'key', fetch: async (url, init) => {
    assert.equal(url.pathname, '/server/v1/chat/relay/ack');
    assert.deepEqual(JSON.parse(init.body), { done: [9], failed: [] });
    return new Response(JSON.stringify({ code: 0, data: { updated: 1 } }));
  } });
  assert.equal(await client.ackChatRelay([9]), 1);
});

test('chat session tokens, nonce and empty read responses stay separate from portal JWTs', async () => {
  const client = new ChatClient('https://account.example', { token: 'cs_session', fetch: async (url, init) => {
    assert.equal(init.headers.Authorization, 'Chat cs_session');
    assert.equal(init.headers['X-Server-Key'], undefined);
    if (url.endsWith('/read')) return new Response(JSON.stringify({ code: 0, data: null }));
    assert.equal(url, 'https://account.example/api/chat/channels/3/messages');
    assert.deepEqual(JSON.parse(init.body), { content: 'hello', nonce: 'retry-1' });
    return new Response(JSON.stringify({ code: 0, data: { id: 9, ownReactions: ['heart'] } }));
  } });
  assert.deepEqual((await client.sendMessage(3, 'hello', { nonce: 'retry-1' })).ownReactions, ['heart']);
  await client.markRead(3, 9);
});
