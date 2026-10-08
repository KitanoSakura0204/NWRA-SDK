import test from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, TransportError } from '../dist/index.js';
import { GameDataClient, GameDataPublicClient, GameDataAdminClient, isGameDataApiError, gameFieldPath } from '../dist/gamedata.js';

const site = 'https://account.example';
const policy = { visibility: 'owner', exportable: false };
const source = { source_id: 'fixture', name: 'Official source', kind: 'official', auth_scope: 'role', fetched_at: '2026-10-08T00:00:00Z' };
const snapshot = {
  game_id: 'arknights', fetched_at: source.fetched_at, expires_at: '2026-10-08T01:00:00Z', next_cursor: 'page/+?=2',
  sections: [{ id: 'characters', name: { en: 'Characters' }, status: 'partial', reason: 'upstream_omitted_fields',
    fields: [], policy, source, entities: [{ id: 'char_1', kind: 'character', name: { en: 'Operator' }, policy,
      fields: [{ key: 'level', name: { en: 'Level' }, kind: 'number', value: 0, status: 'ok', policy, source }] }] }],
};
function ok(data) { return new Response(JSON.stringify({ code: 0, data })); }

test('miHoYo and Hypergryph snapshot paths preserve source, absent ownership, zero values and opaque pagination', async () => {
  const seen = [];
  const client = new GameDataClient(site, { token: 'site-jwt', fetch: async (url, init) => {
    seen.push(url);
    assert.equal(init.headers.Authorization, 'Bearer site-jwt');
    assert.equal(init.headers['X-Server-Key'], undefined);
    assert.equal(init.redirect, 'error');
    assert.equal(url.searchParams.get('roleId'), '17');
    assert.equal(url.searchParams.get('sections'), 'characters,note');
    assert.equal(url.searchParams.get('cursor'), 'page/+?=2');
    assert.equal(url.searchParams.get('locale'), 'zh_TW');
    assert.equal(url.searchParams.get('limit'), '100');
    return ok(snapshot);
  } });
  const query = { roleId: 17, sections: ['characters', 'note'], cursor: snapshot.next_cursor, locale: 'zh_TW', limit: 100 };
  for (const provider of ['mihoyo', 'hypergryph']) {
    const result = await client.snapshot(provider, 'arknights', query);
    assert.deepEqual(result, snapshot);
    assert.equal(result.sections[0].entities[0].fields[0].value, 0);
    assert.equal(result.sections[0].entities[0].owned, undefined);
  }
  assert.equal(seen.length, 2, 'the client must not auto-retry or drain all pages');
  assert.equal(seen[0].pathname, '/api/account/mihoyo/games/arknights/data');
  assert.equal(seen[1].pathname, '/api/account/hypergryph/games/arknights/snapshot');
});

test('provider descriptors and role discovery keep their actual wire shapes', async () => {
  const mihoyoRole = { id: 7, gameBiz: 'hk4e_cn', gameUid: '123456789', region: 'cn_gf01' };
  const hgRole = { id: 'arknights:123456789', game_id: 'arknights', uid: '123456789', region: 'cn', policy };
  const discovery = { games: [], roles: [hgRole], discovery: { status: 'partial', reason: 'one_binding_unavailable' }, selectedRoleId: hgRole.id };
  const client = new GameDataClient(site, { token: 'jwt', fetch: async url => {
    if (url.pathname === '/api/account/mihoyo/games') return ok([]);
    if (url.pathname === '/api/account/mihoyo/status') return ok({ roles: [mihoyoRole], bound: true });
    assert.equal(url.pathname, '/api/account/hypergryph/games');
    return ok(discovery);
  } });
  assert.deepEqual(await client.games('mihoyo'), []);
  assert.deepEqual(await client.games('hypergryph'), discovery);
  assert.deepEqual(await client.roles('mihoyo'), [mihoyoRole]);
  assert.deepEqual(await client.roles('hypergryph'), [hgRole]);
});

test('public Hypergryph references cannot receive a JWT, server key or private role selector', async () => {
  let count = 0;
  const client = new GameDataPublicClient(site, { fetch: async (url, init) => {
    count++;
    assert.equal(init.headers.Authorization, undefined);
    assert.equal(init.headers['X-Server-Key'], undefined);
    assert.equal(url.pathname, '/api/hypergryph/games/popucom/snapshot');
    assert.equal(url.searchParams.has('roleId'), false);
    return ok(snapshot);
  } });
  await client.snapshot('hypergryph', 'popucom', { sections: ['reference'] });
  assert.throws(() => client.snapshot('hypergryph', 'popucom', { roleId: 'private' }), TypeError);
  assert.throws(() => client.games('mihoyo'), RangeError);
  assert.throws(() => new GameDataPublicClient(site, { token: 'jwt' }), TypeError);
  assert.throws(() => new GameDataPublicClient(site, { serverKey: 'key' }), TypeError);
  assert.throws(() => new GameDataClient(site, { token: 'jwt', serverKey: 'key' }), TypeError);
  assert.throws(() => new GameDataClient(site, { token: '' }), TypeError);
  assert.equal(count, 1);
});

test('asset pagination/search is bounded and a catalog page remains distinct from account inventory', async () => {
  const page = { provider: { id: 'catalog' }, cache: { disabled: false }, status: 'stale', list: [], count: 120, page: 2, limit: 50 };
  const client = new GameDataClient(site, { token: 'jwt', fetch: async url => {
    assert.equal(url.pathname, '/api/account/game-assets/catalog');
    assert.equal(url.searchParams.get('q'), 'A&B / icon');
    assert.equal(url.searchParams.get('page'), '2');
    assert.equal(url.searchParams.get('limit'), '50');
    assert.equal(url.searchParams.get('kind'), 'character');
    return ok(page);
  } });
  assert.deepEqual(await client.assets({ provider: 'catalog', q: 'A&B / icon', page: 2, limit: 50, kind: 'character' }), page);
  for (const query of [{ page: 20001 }, { page: 0 }, { limit: 101 }, { limit: NaN }, { q: 'x'.repeat(129) }]) {
    assert.throws(() => client.assets({ provider: 'catalog', ...query }), RangeError);
  }
});

test('role selection and asset state explicitly preserve false and send no unrelated properties', async () => {
  const calls = [];
  const options = { token: 'jwt', fetch: async (url, init) => {
    calls.push({ path: url.pathname, method: init.method, body: JSON.parse(init.body) });
    assert.equal(init.headers.Authorization, 'Bearer jwt');
    return ok({});
  } };
  await new GameDataClient(site, options).selectRole('hypergryph', 'arknights:1');
  const admin = new GameDataAdminClient(site, options);
  await admin.setAssetState('catalog', false, { locale: 'en', version: 'latest', site_id: 900 });
  await admin.refreshAssets('catalog', { locale: 'ja' });
  assert.deepEqual(calls, [
    { path: '/api/account/hypergryph/role', method: 'PUT', body: { roleId: 'arknights:1' } },
    { path: '/api/admin/game-assets/providers/catalog/state', method: 'PUT', body: { locale: 'en', version: 'latest', disabled: false } },
    { path: '/api/admin/game-assets/providers/catalog/refresh', method: 'POST', body: { locale: 'ja' } },
  ]);
});

test('admin account IDs and path parameters cannot escape their routes', async () => {
  let count = 0;
  const admin = new GameDataAdminClient(site, { token: 'jwt', fetch: async url => {
    count++;
    assert.equal(url.pathname, '/api/admin/hypergryph/accounts/7/games/title%2F%3F%23/snapshot');
    return ok(snapshot);
  } });
  await admin.snapshot('hypergryph', 7, 'title/?#');
  for (const id of [0, -1, 0.5, Infinity, Number.MAX_SAFE_INTEGER + 1]) {
    assert.throws(() => admin.accountGames('hypergryph', id), RangeError);
  }
  assert.throws(() => admin.snapshot('hypergryph', 7, '..'), RangeError);
  assert.equal(count, 1);
});

test('business errors keep ApiError codes and rate-limit metadata without retrying challenges', async () => {
  let calls = 0;
  const detail = { status: 'challenge_required', code: 'upstream_challenge' };
  const client = new GameDataClient(site, { token: 'jwt', fetch: async () => {
    calls++;
    return new Response(JSON.stringify({ code: 4291, message: 'Challenge required', data: detail }),
      { status: 429, headers: { 'Retry-After': '60' } });
  } });
  await assert.rejects(client.games('mihoyo'), error => {
    assert.ok(error instanceof ApiError);
    assert.ok(isGameDataApiError(error));
    assert.equal(error.code, 4291);
    assert.equal(error.status, 429);
    assert.equal(error.retryAfter, '60');
    assert.deepEqual(error.data, detail);
    return true;
  });
  assert.equal(calls, 1);
});

test('invalid JSON, malformed envelopes and HTTP failures never become empty success', async () => {
  for (const [body, status] of [['not json', 200], ['[]', 200], ['{"code":"0","data":[]}', 200],
    ['{"code":0}', 200], ['{"code":0,"data":null}', 200], ['{"code":0,"data":[]}', 503]]) {
    const client = new GameDataClient(site, { token: 'jwt', fetch: async () => new Response(body, { status }) });
    await assert.rejects(client.games('mihoyo'), TransportError);
  }
});

test('pre-aborted requests never send credentials or start fetch', async () => {
  const cancellation = new AbortController();
  const reason = new Error('page closed');
  cancellation.abort(reason);
  let calls = 0;
  const client = new GameDataClient(site, { token: 'jwt', fetch: async () => { calls++; return ok([]); } });
  await assert.rejects(client.games('mihoyo', { signal: cancellation.signal }), error => {
    assert.ok(error instanceof TransportError);
    assert.equal(error.cause, reason);
    return true;
  });
  assert.equal(calls, 0);
});

test('caller cancellation reaches an in-flight response body and removes its listener', async () => {
  const cancellation = new AbortController();
  let removed = 0;
  const remove = cancellation.signal.removeEventListener.bind(cancellation.signal);
  cancellation.signal.removeEventListener = (...args) => { removed++; return remove(...args); };
  let bodyStarted;
  const started = new Promise(resolve => { bodyStarted = resolve; });
  const client = new GameDataClient(site, { token: 'jwt', fetch: async (_url, init) => ({
    ok: true, status: 200, headers: new Headers(), json: () => new Promise((_resolve, reject) => {
      init.signal.addEventListener('abort', () => reject(init.signal.reason), { once: true });
      bodyStarted();
    }),
  }) });
  const pending = client.games('mihoyo', { signal: cancellation.signal });
  await started;
  const reason = new Error('screen changed');
  cancellation.abort(reason);
  await assert.rejects(pending, error => error instanceof TransportError && error.cause === reason);
  assert.equal(removed, 1);
});

test('timeout covers reading the response body', async () => {
  const client = new GameDataClient(site, { token: 'jwt', timeoutMs: 10, fetch: async (_url, init) => ({
    ok: true, status: 200, headers: new Headers(), json: () => new Promise((_resolve, reject) => {
      init.signal.addEventListener('abort', () => reject(init.signal.reason), { once: true });
    }),
  }) });
  await assert.rejects(client.games('mihoyo'), error => error instanceof TransportError && error.cause.name === 'TimeoutError');
});

test('pagination and section validation fail before a network request', () => {
  const client = new GameDataClient(site, { token: 'jwt', fetch: async () => assert.fail('unexpected request') });
  for (const query of [{ limit: 101 }, { limit: 0 }, { cursor: 'x'.repeat(2049) },
    { sections: Array(17).fill('note') }, { sections: ['note,characters'] }, { locale: 'unsupported' }]) {
    assert.throws(() => client.snapshot('hypergryph', 'arknights', query), RangeError);
  }
  assert.equal(gameFieldPath('characters/owned', 'level~value', 'char/1'), '/sections/characters~1owned/entities/char~11/fields/level~0value');
  assert.equal(gameFieldPath('note', 'ap'), '/sections/note/fields/ap');
  assert.throws(() => gameFieldPath('', 'level'), RangeError);
});

test('admin field-policy writes require explicit revisions, preserve false and reject conflicting edits', async () => {
  const path = gameFieldPath('characters', 'level', 'operator/1');
  let calls = 0;
  const admin = new GameDataAdminClient(site, { token: 'admin-jwt', fetch: async (url, init) => {
    calls++;
    assert.equal(url.pathname, '/api/admin/mihoyo/accounts/9/archive/policies');
    assert.equal(init.headers.Authorization, 'Bearer admin-jwt');
    assert.equal(init.method, 'PUT');
    const body = JSON.parse(init.body);
    assert.deepEqual(body, { game_id: 'genshin', path, visibility: 'owner', exportable: false, revision: 0 });
    assert.equal(body.actor_id, undefined, 'the actor must be derived by the server');
    return new Response(JSON.stringify({ code: 409, message: 'conflict' }));
  } });
  await assert.rejects(admin.setFieldPolicy('mihoyo', 9, 'genshin', {
    path, visibility: 'owner', exportable: false, revision: 0, actor_id: 999,
  }), error => error instanceof ApiError && error.code === 409);
  for (const input of [{ revision: undefined }, { revision: -1 }, { revision: Number.MAX_SAFE_INTEGER + 1 },
    { path: '/sections/a/fields/b~2' }, { path: '/sections//fields/b' }]) {
    assert.throws(() => admin.setFieldPolicy('mihoyo', 9, 'genshin', {
      path, visibility: 'owner', exportable: false, revision: 0, ...input,
    }), RangeError);
  }
  assert.equal(calls, 1, 'revision conflicts must be returned, not retried with an unguarded write');
});

test('archive metadata, policy reads and history preserve owner scope and before cursors', async () => {
  const routes = [];
  const client = new GameDataClient(site, { token: 'owner-jwt', fetch: async (url, init) => {
    routes.push(url.pathname);
    assert.equal(init.headers.Authorization, 'Bearer owner-jwt');
    assert.equal(init.headers['X-Server-Key'], undefined);
    if (url.pathname.endsWith('/history')) {
      assert.equal(url.searchParams.get('before'), '41');
      assert.equal(url.searchParams.get('limit'), '25');
    }
    if (url.pathname.endsWith('/policies')) assert.equal(url.searchParams.get('game_id'), 'arknights');
    return ok([]);
  } });
  assert.deepEqual(await client.archiveSnapshots('hypergryph', 7), []);
  assert.deepEqual(await client.fieldPolicies('hypergryph', 7, 'arknights'), []);
  assert.deepEqual(await client.syncHistory('hypergryph', 7, { before: 41, limit: 25 }), []);
  assert.deepEqual(routes, ['/api/account/hypergryph/accounts/7/archive/snapshots',
    '/api/account/hypergryph/accounts/7/archive/policies', '/api/account/hypergryph/accounts/7/archive/history']);
  assert.equal(client.setFieldPolicy, undefined, 'policy mutation belongs only to the admin facade');
});
