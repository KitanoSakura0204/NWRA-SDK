import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { constants, createCipheriv, createPublicKey, generateKeyPairSync, publicEncrypt, randomBytes, webcrypto } from 'node:crypto';
import { NoworkClient, ApiError, EncryptionError, TransportError, HEADER_ENCRYPTION } from '../dist/index.js';

globalThis.crypto ??= webcrypto;
const fixtures = JSON.parse(readFileSync(new URL('../../go/testdata/vectors.json', import.meta.url), 'utf8'));
const serverVector = fixtures.vectors.find(vector => vector.kind === 'server');
const rsa = fixtures.keys.rsa;
const previous = { ...generateKeyPairSync('rsa', { modulusLength: 2048 }).privateKey.export({ format: 'jwk' }), kid: 'previous' };
const base = 'https://account.example';
const expectedCode = 'survival';
const wireLimit = 8 * 1024 * 1024;

function response(data, { marker = true, status = 200, code = 0, message = 'ok' } = {}) {
  return new Response(JSON.stringify({ code, message, data }), {
    status, headers: marker ? { [HEADER_ENCRYPTION]: 'JWE' } : {},
  });
}

// Offline RSA fixture producer using Node crypto, independently of the SDK's delivery decoder.
function seal(data, ctx = `server:${expectedCode}:whitelist`, key = rsa, extra = {}, bytes) {
  const header = { alg: 'RSA-OAEP-256', enc: 'A256GCM', kid: key.kid, ctx, ...extra };
  const protectedHeader = Buffer.from(JSON.stringify(header)).toString('base64url');
  const cek = randomBytes(32);
  const iv = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', cek, iv);
  cipher.setAAD(Buffer.from(protectedHeader));
  const encrypted = Buffer.concat([cipher.update(bytes ?? Buffer.from(JSON.stringify(data))), cipher.final()]);
  const publicKey = createPublicKey({ key: { kty: 'RSA', n: key.n, e: key.e }, format: 'jwk' });
  const wrapped = publicEncrypt({ key: publicKey, padding: constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' }, cek);
  return { jwe: [protectedHeader, wrapped.toString('base64url'), iv.toString('base64url'),
    encrypted.toString('base64url'), cipher.getAuthTag().toString('base64url')].join('.') };
}

function encryptedClient(data, options = {}, responseOptions = {}) {
  return new NoworkClient(base, {
    serverKey: 'server-key', serverCode: expectedCode, decryptionKeys: rsa,
    fetch: async () => response(data, responseOptions), ...options,
  });
}

test('main client opens backend shared vector with and without the marker, keeping raw encrypted', async () => {
  for (const marker of [true, false]) {
    const envelope = JSON.parse(serverVector.response);
    const client = encryptedClient(envelope.data, {
      decryptionKeys: [fixtures.keys.rsa, fixtures.keys.x25519, fixtures.keys.ec],
      fetch: async (url, init) => {
        assert.equal(url.pathname, '/server/v1/whitelist');
        assert.equal(init.headers['X-Server-Key'], 'server-key');
        assert.equal(init.headers.Authorization, undefined);
        assert.equal(init.redirect, 'error');
        assert.equal(init.credentials, 'omit');
        assert.equal(JSON.stringify(init).includes(fixtures.keys.ec.d), false);
        return new Response(serverVector.response, { headers: marker ? { [HEADER_ENCRYPTION]: 'JWE' } : {} });
      },
    });
    assert.deepEqual(await client.whitelist(), JSON.parse(serverVector.plaintext));
    const raw = await client.raw('GET', '/server/v1/whitelist');
    assert.equal(raw.encrypted, true);
    assert.deepEqual(raw.envelope, envelope);
    assert.deepEqual(raw.data, envelope.data);
  }
});

const whitelist = { server: expectedCode, mode: 1, count: 0, entries: [] };
const checked = { allowed: false, bound: true, reason: 'banned', message: 'appeal pending', player: null };
const stored = { uid: 7, data: { coins: 8 }, version: 2 };
const banned = { uid: 7, reason: 'x-ray' };
const command = { id: 9, action: 'say', params: { text: 'Hello' }, status: 'pending' };
const session = { serverId: 1, serverCode: expectedCode, subject: 'subject', character: { id: 7, roleName: 'Alex', roleUuid: '' },
  authenticatedAt: 1, scope: 'character:login' };
const ticket = { ticket: 'chat-ticket', expiresAt: '2026-10-09T00:00:00Z', channels: [1], embedPath: '/chat', wsPath: '/ws' };
const relay = { id: 1, kind: 'message', channelId: 1, payload: { text: 'Hello' }, attempts: 1 };
const methods = [
  ['whitelist', 'GET', c => c.whitelist(), whitelist, whitelist],
  ['whitelist/check', 'GET', c => c.checkWhitelist('Player & One'), checked, checked],
  ['heartbeat', 'POST', c => c.heartbeat({ online: 1, tps: 20, players: [{ name: 'Alex' }] }), { status: 'ok' }],
  ['modules', 'GET', c => c.modules(), { modules: { gm: { enabled: true } } }, { gm: { enabled: true } }],
  ['gm/actions', 'PUT', c => c.publishGMActions([]), { count: 0, skipped: [] }, { count: 0, skipped: [] }],
  ['gm/actions', 'PUT', c => c.publishGMCatalog([], {}), { count: 0, skipped: [] }, { count: 0, skipped: [] }],
  ['gm/options', 'PUT', c => c.publishGMOptions({ worlds: [] }), { count: 0 }, { count: 0, skipped: [] }],
  ['gm/commands', 'GET', c => c.pollGMCommands(), [command], [command]],
  ['gm/commands/9/result', 'POST', c => c.reportGMResult(9, true, 'sent'), { id: 9, status: 'done' }, { id: 9, status: 'done' }],
  ['users/schema', 'GET', c => c.userSchema(), { fields: [], maxUserBytes: 8192 }, { fields: [], maxUserBytes: 8192 }],
  ['users/7/data', 'GET', c => c.userData(7), stored, stored],
  ['users/7/data', 'PUT', c => c.putUserData(7, { coins: 8 }, { version: 1 }), stored, stored],
  ['users/7/data', 'PATCH', c => c.patchUserData(7, { coins: 8 }), stored, stored],
  ['bans', 'GET', c => c.bans(), { bans: [banned] }, [banned]],
  ['bans/kick', 'GET', c => c.kickList(), { players: [{ uid: 7, name: 'Alex' }] }, [{ uid: 7, name: 'Alex' }]],
  ['bans', 'POST', c => c.ban({ uid: 7 }), { uid: 7, ban: banned }, { uid: 7, ban: banned }],
  ['bans/lift', 'POST', c => c.unban({ uid: 7 }), { uid: 7, lifted: true }, { uid: 7, lifted: true }],
  ['gm/sso/exchange', 'POST', c => c.exchangeGMCode('code', 'verifier', 'https://gm.example/callback'), session, session],
  ['chat/tickets', 'POST', c => c.issueChatTicket({ uid: 7 }), ticket, ticket],
  ['chat/relay', 'GET', c => c.pollChatRelay(), { items: [relay] }, [relay]],
  ['chat/relay/ack', 'POST', c => c.ackChatRelay([1]), { updated: 1 }, 1],
  ['chat/messages', 'POST', c => c.postChatMessage({ player: 'Alex', text: 'Hello' }), { id: 1 }, { id: 1 }],
];

test('every typed Server API method opens authenticated data on its own route, including void writes', async () => {
  for (const [route, method, invoke, data, expected] of methods) {
    const client = encryptedClient({}, {
      fetch: async (url, init) => {
        assert.equal(url.pathname, `/server/v1/${route}`);
        assert.equal(init.method, method);
        assert.equal(init.headers.Authorization, undefined);
        if (route === 'whitelist/check') assert.equal(url.searchParams.get('name'), 'Player & One');
        return response(seal(data, `server:${expectedCode}:${route}`));
      },
    });
    const actual = await invoke(client);
    if (expected !== undefined) assert.deepEqual(actual, expected, `${method} ${route}`);
  }
});

test('no keys fail closed on every typed method even when a proxy removes the encryption header', async () => {
  const sealed = seal(whitelist);
  for (const marker of [true, false]) {
    const client = encryptedClient(sealed, { decryptionKeys: undefined }, { marker });
    for (const [route, , invoke] of methods) {
      await assert.rejects(invoke(client), error => error instanceof EncryptionError && error.route === route);
    }
    assert.equal((await client.raw('GET', '/server/v1/whitelist')).encrypted, true);
  }
});

test('keys reject plaintext whitelist replies while all ordinary heartbeat/GM/user/chat replies remain supported', async () => {
  for (const [route, , invoke, data, expected] of methods) {
    const client = encryptedClient(data, {}, { marker: false });
    if (route === 'whitelist' || route === 'whitelist/check') {
      await assert.rejects(invoke(client), error => error instanceof EncryptionError && /plain reply/.test(error.reason));
      assert.deepEqual((await client.raw('GET', `/server/v1/${route}`)).data, data);
    } else {
      const actual = await invoke(client);
      if (expected !== undefined) assert.deepEqual(actual, expected, route);
    }
  }
  for (const decryptionKeys of [undefined, []]) {
    assert.deepEqual(await encryptedClient(whitelist, { decryptionKeys }, { marker: false }).whitelist(), whitelist);
  }
  // A regular data field called jwe is not a delivery envelope.
  const plain = { ...whitelist, jwe: 'application-data' };
  assert.deepEqual(await encryptedClient(plain, { decryptionKeys: [] }, { marker: false }).whitelist(), plain);
});

test('rotation selects current/previous keys by kid and tries every key when kid is absent', async () => {
  const ring = [previous, rsa];
  for (const key of ring) {
    const sealed = seal(whitelist, `server:${expectedCode}:whitelist`, key);
    assert.deepEqual(await encryptedClient(sealed, { decryptionKeys: ring }).whitelist(), whitelist);
  }
  const kidless = seal(whitelist, `server:${expectedCode}:whitelist`, rsa, { kid: undefined });
  assert.deepEqual(await encryptedClient(kidless, { decryptionKeys: ring }).whitelist(), whitelist);
  const wrongKey = { ...previous, kid: rsa.kid };
  await assert.rejects(encryptedClient(seal(whitelist), { decryptionKeys: wrongKey }).whitelist(), EncryptionError);
  const unknownKid = seal(whitelist, `server:${expectedCode}:whitelist`, rsa, { kid: 'unknown' });
  await assert.rejects(encryptedClient(unknownKid, { decryptionKeys: ring }).whitelist(), EncryptionError);
});

test('client snapshots decryption keys and the expected server so later option mutation cannot disable checks', async () => {
  const key = { ...rsa };
  const keys = [key];
  const options = { serverKey: 'server-key', serverCode: expectedCode, decryptionKeys: keys,
    fetch: async () => response(seal(whitelist)) };
  const client = new NoworkClient(base, options);
  keys.splice(0);
  key.d = 'broken';
  options.serverCode = 'other';
  assert.deepEqual(await client.whitelist(), whitelist);
  const guards = [{ ...rsa }];
  const guarded = encryptedClient(whitelist, { decryptionKeys: guards }, { marker: false });
  guards.splice(0);
  await assert.rejects(guarded.whitelist(), EncryptionError);
});

test('authenticated context must name the exact route and the configured server', async () => {
  const bad = [undefined, null, 1, [], '', 'server::whitelist', 'server: :whitelist', 'webhook:survival:whitelist',
    'server:survival:whitelist/check', 'server:survival:whitelist?name=Alex', 'server:survival:whitelist:extra',
    'server:other:whitelist'];
  for (const ctx of bad) {
    const sealed = seal(whitelist, '', rsa, { ctx });
    await assert.rejects(encryptedClient(sealed).whitelist(), error => error instanceof EncryptionError
      && error.route === 'whitelist' && /ctx/.test(error.reason), String(ctx));
    if (ctx !== 'server:other:whitelist') {
      await assert.rejects(encryptedClient(sealed, { serverCode: undefined }).whitelist(), EncryptionError, String(ctx));
    }
  }
  const other = seal(whitelist, 'server:other:whitelist');
  assert.deepEqual(await encryptedClient(other, { serverCode: undefined }).whitelist(), whitelist);
  await assert.rejects(encryptedClient(seal(checked)).checkWhitelist('Alex'), EncryptionError);
});

test('malformed singleton envelopes are encrypted failures even without a marker or configured keys', async () => {
  for (const jwe of ['', null, 7, [], {}, 'a.b.c.d.e']) {
    for (const decryptionKeys of [undefined, rsa]) {
      for (const marker of [true, false]) {
        const client = encryptedClient({ jwe }, { decryptionKeys }, { marker });
        await assert.rejects(client.whitelist(), EncryptionError);
        assert.equal((await client.raw('GET', '/server/v1/whitelist')).encrypted, true);
      }
    }
  }
  for (const data of [undefined, null, [], {}, { ...seal(whitelist), ignored: true }]) {
    await assert.rejects(encryptedClient(data).whitelist(), EncryptionError);
  }
  const emptyMarker = encryptedClient(whitelist, { fetch: async () => new Response(JSON.stringify({ code: 0, data: whitelist }),
    { headers: { [HEADER_ENCRYPTION]: '' } }) });
  await assert.rejects(emptyMarker.whitelist(), EncryptionError);
});

test('tampered JOSE headers, ciphertext, tags and unsupported algorithms are refused', async () => {
  const sealed = seal(whitelist);
  for (const position of [0, 1, 2, 3, 4]) {
    const parts = sealed.jwe.split('.');
    if (position === 0) {
      const header = JSON.parse(Buffer.from(parts[0], 'base64url'));
      parts[0] = Buffer.from(JSON.stringify({ ...header, ctx: 'server:other:whitelist' })).toString('base64url');
    } else {
      const bytes = Buffer.from(parts[position], 'base64url');
      bytes[0] ^= 1;
      parts[position] = bytes.toString('base64url');
    }
    await assert.rejects(encryptedClient({ jwe: parts.join('.') }).whitelist(), EncryptionError);
  }
  for (const extra of [{ alg: 'dir' }, { alg: 'RSA1_5' }, { enc: 'A128GCM' }, { zip: 'DEF' }, { crit: ['ctx'] }]) {
    await assert.rejects(encryptedClient(seal(whitelist, `server:${expectedCode}:whitelist`, rsa, extra)).whitelist(), EncryptionError);
  }
  for (const jwe of ['bnVsbA....', 'W10....', 'MQ....', 'e30....', 'bm90LWpzb24....', 'a.b.c', 'a..b..c', '%.$.^.&.*']) {
    await assert.rejects(encryptedClient({ jwe }).whitelist(), EncryptionError);
  }
});

test('authenticated data still requires valid UTF-8, JSON and the route response shape', async () => {
  for (const plain of [null, false, 0, 'text', []]) {
    await assert.rejects(encryptedClient(seal(plain)).whitelist(), EncryptionError);
  }
  for (const bytes of [Buffer.from('not json'), Buffer.from('{"broken":'), Buffer.from([123, 34, 120, 34, 58, 34, 0xc3, 0x28, 34, 125])]) {
    await assert.rejects(encryptedClient(seal({}, `server:${expectedCode}:whitelist`, rsa, {}, bytes)).whitelist(), EncryptionError);
  }
  const objectCommands = seal({}, `server:${expectedCode}:gm/commands`);
  await assert.rejects(encryptedClient(objectCommands).pollGMCommands(), EncryptionError);
  const emptyCommands = seal([], `server:${expectedCode}:gm/commands`);
  assert.deepEqual(await encryptedClient(emptyCommands).pollGMCommands(), []);
});

test('encrypted whitelist decisions require explicit valid mode/allowed fields instead of defaults', async () => {
  for (const mode of [undefined, null, false, true, '0', -1, 3, 0.5]) {
    const data = mode === undefined ? {} : { mode };
    await assert.rejects(encryptedClient(seal(data)).whitelist(), error => error instanceof EncryptionError
      && /mode/.test(error.reason));
  }
  for (const mode of [0, 1, 2]) {
    assert.deepEqual(await encryptedClient(seal({ ...whitelist, mode })).whitelist(), { ...whitelist, mode });
  }
  for (const allowed of [undefined, null, 0, 1, 'false', 'true', [], {}]) {
    const data = allowed === undefined ? {} : { allowed };
    const client = encryptedClient(seal(data, `server:${expectedCode}:whitelist/check`));
    await assert.rejects(client.checkWhitelist('Alex'), error => error instanceof EncryptionError && /allowed/.test(error.reason));
  }
  for (const allowed of [false, true]) {
    const data = { ...checked, allowed };
    assert.deepEqual(await encryptedClient(seal(data, `server:${expectedCode}:whitelist/check`)).checkWhitelist('Alex'), data);
  }
});

test('business and HTTP transport errors retain their original types before decryption/downgrade checks', async () => {
  for (const marker of [true, false]) {
    const denied = encryptedClient({ jwe: 'broken' }, {}, { marker, code: 403, status: 403, message: 'denied' });
    await assert.rejects(denied.whitelist(), error => error instanceof ApiError && error.code === 403 && /denied/.test(error.message));
    await assert.rejects(denied.raw('GET', '/server/v1/whitelist'), ApiError);
    const gateway = encryptedClient(seal(whitelist), {}, { marker, status: 503 });
    await assert.rejects(gateway.whitelist(), TransportError);
  }
  for (const env of [null, [], {}, { code: 0.5 }, { code: '0' }]) {
    const client = encryptedClient({}, { fetch: async () => new Response(JSON.stringify(env)) });
    await assert.rejects(client.whitelist(), TransportError);
  }
});

test('declared and streamed oversized bodies stop at the wire cap before JSON or JWE decoding', async () => {
  for (const declared of [true, false]) {
    let cancelled = false;
    let reads = 0;
    const chunk = new TextEncoder().encode('界'.repeat(512 * 1024));
    const body = new ReadableStream({
      pull(controller) { reads++; controller.enqueue(chunk); },
      cancel() { cancelled = true; },
    }, { highWaterMark: 0 });
    const client = encryptedClient({}, { fetch: async () => new Response(body, { headers: {
      [HEADER_ENCRYPTION]: 'JWE', ...(declared ? { 'Content-Length': String(wireLimit + 1) } : {}),
    } }) });
    await assert.rejects(client.whitelist(), error => error instanceof TransportError && /byte limit/.test(error.message));
    assert.equal(cancelled, true);
    assert.equal(body.locked, false);
    assert.equal(declared ? reads === 0 : reads <= 6, true);
  }
});

test('custom json-only fetch adapters are bounded and still support valid responses', async () => {
  const client = encryptedClient({}, { fetch: async () => ({ status: 200, json: async () => ({ code: 0, data: seal(whitelist) }) }) });
  assert.deepEqual(await client.whitelist(), whitelist);
  const huge = encryptedClient({}, { fetch: async () => ({ status: 200, json: async () => ({ code: 0, data: { jwe: 'x'.repeat(wireLimit) } }) }) });
  await assert.rejects(huge.whitelist(), error => error instanceof TransportError && /byte limit/.test(error.message));
});

test('response timeout covers stalled streamed encrypted bodies and releases their reader', async () => {
  let cancelled = false;
  const body = new ReadableStream({
    start(controller) { controller.enqueue(new TextEncoder().encode('{"code":0,"data":{"jwe":"')); },
    cancel() { cancelled = true; },
  });
  const client = encryptedClient({}, { timeoutMs: 20, fetch: async () => new Response(body, { headers: { [HEADER_ENCRYPTION]: 'JWE' } }) });
  await assert.rejects(client.whitelist(), error => error instanceof TransportError && /timed out/.test(error.message));
  assert.equal(cancelled, true);
  assert.equal(body.locked, false);
});

test('response byte accounting accepts split UTF-8 and refuses malformed wire UTF-8', async () => {
  const bytes = new TextEncoder().encode(JSON.stringify({ code: 0, data: { status: '在线' } }));
  const unicodeIndex = bytes.indexOf(0xe5);
  const chunks = [bytes.slice(0, unicodeIndex + 1), bytes.slice(unicodeIndex + 1, unicodeIndex + 2), bytes.slice(unicodeIndex + 2)];
  const body = new ReadableStream({ start(controller) { for (const chunk of chunks) controller.enqueue(chunk); controller.close(); } });
  await encryptedClient({}, { fetch: async () => new Response(body) }).heartbeat(1);
  const malformed = new Uint8Array([123, 34, 99, 111, 100, 101, 34, 58, 48, 44, 34, 100, 97, 116, 97, 34, 58, 34, 0xc3, 0x28, 34, 125]);
  const client = encryptedClient({}, { fetch: async () => new Response(malformed) });
  await assert.rejects(client.whitelist(), TransportError);
});

test('invalid decryption/expected-server configuration is rejected before sending requests', () => {
  const fetch = async () => { assert.fail('invalid client options must not send a request'); };
  for (const decryptionKeys of [null, 'json', [{}], [null], [[rsa]], [{ ...rsa, d: '' }], [{ ...rsa, kty: 'oct' }]]) {
    assert.throws(() => new NoworkClient(base, { serverKey: 'key', fetch, decryptionKeys }), TypeError);
  }
  for (const serverCode of ['', ' ', 'server:other', 'survival\n', null, 42]) {
    assert.throws(() => new NoworkClient(base, { serverKey: 'key', fetch, decryptionKeys: rsa, serverCode }), TypeError);
  }
  for (const timeoutMs of [0, -1, NaN, 2_147_483_648]) {
    assert.throws(() => new NoworkClient(base, { serverKey: 'key', fetch, timeoutMs }), RangeError);
  }
});
