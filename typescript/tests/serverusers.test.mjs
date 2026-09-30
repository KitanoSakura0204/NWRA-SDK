import test from 'node:test';
import assert from 'node:assert/strict';
import { LoungeClient, ApiError, isVersionConflict } from '../dist/index.js';

const json = (data, status = 200) => new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } });
const stored = { uid: 100001, data: { coins: 12, rank: null }, version: 2, updatedAt: '2026-09-29 10:00:00', updatedBy: 'plugin', size: 11 };

function client(answer) {
  const calls = [];
  const c = new LoungeClient('https://account.example', {
    serverKey: 'k',
    fetch: async (url, init) => {
      calls.push([init.method, url.pathname + url.search, init.body ? JSON.parse(init.body) : undefined]);
      return json(answer(init.method, url.pathname));
    }
  });
  return { c, calls };
}

test('player data: schema, read, replace with a version, patch, stale version', async () => {
  const { c, calls } = client((method, path) => {
    if (path === '/server/v1/users/schema') return { code: 0, data: { fields: [{ key: 'coins', label: 'Coins', type: 'number', visible: true }], maxUserBytes: 8192 } };
    if (path === '/server/v1/users/100002/data') return { code: 409, message: 'stale' };
    return { code: 0, data: stored };
  });
  assert.equal((await c.userSchema()).maxUserBytes, 8192);
  assert.deepEqual(await c.userData(100001), stored);
  await c.putUserData(100001, { coins: 1 }, { version: 2 });
  await c.patchUserData(100001, { rank: null });
  await c.putUserData(100001, {}, { version: 0 });
  assert.deepEqual(calls.slice(2), [
    ['PUT', '/server/v1/users/100001/data', { data: { coins: 1 }, version: 2 }],
    ['PATCH', '/server/v1/users/100001/data', { data: { rank: null } }],
    ['PUT', '/server/v1/users/100001/data', { data: {}, version: 0 }]
  ]);
  await assert.rejects(c.putUserData(100002, { coins: 1 }, { version: 1 }), e => e instanceof ApiError && isVersionConflict(e));
  await assert.rejects(c.userData(0), TypeError);
  assert.equal(isVersionConflict(new ApiError(404, 'x')), false);
});

test('bans: list with a filter, kick list, ban and unban bodies', async () => {
  const ban = { uid: 100001, names: ['Steve'], uuids: [], reason: 'x-ray', expireTime: null, createTime: '2026-09-29 10:00:00', source: 'plugin' };
  const { c, calls } = client((method, path) => {
    if (path === '/server/v1/bans' && method === 'GET') return { code: 0, data: { count: 1, bans: [ban] } };
    if (path === '/server/v1/bans/kick') return { code: 0, data: { count: 0, players: [] } };
    if (path === '/server/v1/bans') return { code: 0, data: { uid: 100001, ban } };
    return { code: 0, data: { uid: 100001, lifted: true } };
  });
  assert.deepEqual(await c.bans({ name: 'Steve' }), [ban]);
  assert.deepEqual(await c.kickList(), []);
  assert.deepEqual(await c.ban({ name: 'Steve' }, { reason: 'x-ray', durationMinutes: 1440 }), { uid: 100001, ban });
  assert.deepEqual(await c.unban({ uid: 100001 }, 'appeal'), { uid: 100001, lifted: true });
  assert.deepEqual(calls, [
    ['GET', '/server/v1/bans?name=Steve', undefined],
    ['GET', '/server/v1/bans/kick', undefined],
    ['POST', '/server/v1/bans', { name: 'Steve', reason: 'x-ray', durationMinutes: 1440 }],
    ['POST', '/server/v1/bans/lift', { uid: 100001, reason: 'appeal' }]
  ]);
});

test('join check carries the ban message', async () => {
  const { c } = client(() => ({ code: 0, data: { allowed: false, bound: true, reason: 'banned', message: 'x-ray', player: null } }));
  const r = await c.checkWhitelist('Steve');
  assert.equal(r.reason, 'banned');
  assert.equal(r.message, 'x-ray');
});
