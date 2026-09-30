// Runs against the compiled output: npm test (tsc, then node --test). No network, no Discord.
import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { loadConfig, parseLevelRoles } from '../dist/examples/discord-role-sync/src/config.js';
import { DiscordError, DiscordRest } from '../dist/examples/discord-role-sync/src/discord.js';
import { LinkStore } from '../dist/examples/discord-role-sync/src/store.js';
import { plan, RoleSync } from '../dist/examples/discord-role-sync/src/sync.js';

const LINKED = '100000000000000001';
const TRUSTED = '100000000000000002'; // trust level 2 and above
const VETERAN = '100000000000000003'; // trust level 3 and above
const ALICE = '200000000000000001';
const ALICE_ALT = '200000000000000002';

/** Discord stand-in: remembers the roles each member holds and every call made. */
function fakeApi() {
  const roles = new Map();
  const of = (id) => roles.get(id) ?? roles.set(id, new Set()).get(id);
  return {
    calls: [],
    held: (id) => [...of(id)].sort(),
    async addRole(id, role) {
      this.calls.push(['add', id, role]);
      of(id).add(role);
    },
    async removeRole(id, role) {
      this.calls.push(['remove', id, role]);
      of(id).delete(role);
    },
  };
}

function newSync(api = fakeApi(), store = new LinkStore('')) {
  const sync = new RoleSync({ api, store, linkedRoleId: LINKED, levelRoles: parseLevelRoles(`2:${TRUSTED},3:${VETERAN}`) });
  return { api, store, sync };
}

let seq = 0;
const ev = (event, data, timestamp) => {
  seq++;
  return { id: `evt-${seq}`, event, timestamp: timestamp ?? 1_000 + seq, data };
};
const bound = (extra = {}) => ({ userId: 1, uid: 100001, provider: 'discord', externalId: ALICE, externalName: 'alice', ...extra });

/** Keeps the expected warnings out of the test output. */
function quiet(t) {
  for (const m of ['info', 'warn', 'error']) t.mock.method(console, m, () => {});
}

test('link, trust level up and down, unlink', async (t) => {
  quiet(t);
  const { api, store, sync } = newSync();
  await sync.handle(ev('connection.bound', bound({ user: { security_level: 2 } })));
  assert.deepEqual(api.held(ALICE), [LINKED, TRUSTED]);

  await sync.handle(ev('user.level_changed', { userId: 1, from: 2, to: 3 }));
  assert.deepEqual(api.held(ALICE), [LINKED, TRUSTED, VETERAN]);
  await sync.handle(ev('user.level_changed', { userId: 1, from: 3, to: 1 }));
  assert.deepEqual(api.held(ALICE), [LINKED]);
  assert.equal(store.get(1).level, 1);

  await sync.handle(ev('connection.unbound', { userId: 1, provider: 'discord', externalId: ALICE }));
  assert.deepEqual(api.held(ALICE), []);
  assert.equal(store.get(1).discordId, '');
  assert.equal(store.size, 0);
});

test('an unknown level leaves the level roles alone', async (t) => {
  quiet(t);
  const { api, sync } = newSync();
  await sync.handle(ev('connection.bound', bound())); // profile does not map security_level
  assert.deepEqual(api.held(ALICE), [LINKED]);
  assert.ok(api.calls.every(([, , role]) => role === LINKED), 'no level role touched');
  await sync.handle(ev('user.level_changed', { userId: 1, from: 1, to: 2 }));
  assert.deepEqual(api.held(ALICE), [LINKED, TRUSTED]);
});

test('a late retry cannot undo a newer event', async (t) => {
  quiet(t);
  const { api, sync } = newSync();
  const stale = ev('connection.bound', bound({ user: { security_level: 3 } }), 2_000);
  await sync.handle(ev('user.deleted', { userId: 1, uid: 100001, username: 'alice', reason: 'self-service' }, 2_001));
  await sync.handle(stale);
  assert.deepEqual(api.held(ALICE), []);
  assert.equal(api.calls.length, 0, 'never linked, nothing to remove, the stale bind is ignored');
});

test('switching Discord accounts moves the roles', async (t) => {
  quiet(t);
  const { api, sync } = newSync();
  await sync.handle(ev('connection.bound', bound({ user: { security_level: 2 } })));
  await sync.handle(ev('connection.bound', bound({ externalId: ALICE_ALT })));
  assert.deepEqual(api.held(ALICE), []);
  assert.deepEqual(api.held(ALICE_ALT), [LINKED, TRUSTED], 'the stored level carries over');
});

test('a deleted account loses every managed role', async (t) => {
  quiet(t);
  const { api, store, sync } = newSync();
  await sync.handle(ev('connection.bound', bound({ user: { security_level: 3 } })));
  assert.deepEqual(api.held(ALICE), [LINKED, TRUSTED, VETERAN]);
  await sync.handle(ev('user.deleted', { userId: 1, uid: 100001, username: 'alice', reason: 'self-service' }));
  assert.deepEqual(api.held(ALICE), []);
  assert.equal(store.size, 0);
});

test('other providers, pairwise endpoints and other events are ignored', async (t) => {
  quiet(t);
  const { api, store, sync } = newSync();
  await sync.handle(ev('connection.bound', bound({ provider: 'github', externalId: '12345' })));
  await sync.handle(ev('connection.bound', { provider: 'discord', user: { sub: '2f1c8e0a' } })); // pairwise: no userId
  await sync.handle(ev('connection.bound', bound({ externalId: 'not-a-snowflake' })));
  await sync.handle(ev('user.login', { userId: 1 }));
  await sync.handle(ev('user.level_changed', { userId: 2, from: 1, to: 3 })); // never linked
  assert.equal(api.calls.length, 0);
  assert.equal(store.get(1), undefined);
  assert.equal(store.get(2), undefined);
});

test('a Discord failure is retried with the store unchanged', async (t) => {
  quiet(t);
  const api = fakeApi();
  const { store, sync } = newSync(api);
  const add = api.addRole;
  api.addRole = async () => {
    throw new DiscordError(500, 'Discord is down');
  };
  const e = ev('connection.bound', bound());
  await assert.rejects(sync.handle(e), DiscordError);
  assert.equal(store.get(1), undefined);
  api.addRole = add;
  await sync.handle(e); // the user center's retry: same id and timestamp
  assert.deepEqual(api.held(ALICE), [LINKED]);
});

test('events for one account never interleave', async (t) => {
  quiet(t);
  const api = fakeApi();
  const add = api.addRole;
  api.addRole = async function (...args) {
    await new Promise((resolve) => setTimeout(resolve, 20)); // a slow Discord call
    return add.apply(this, args);
  };
  const { sync } = newSync(api);
  await Promise.all([
    sync.handle(ev('connection.bound', bound())),
    sync.handle(ev('connection.unbound', { userId: 1, provider: 'discord', externalId: ALICE })),
  ]);
  assert.deepEqual(api.held(ALICE), [], 'the unlink ran after the link finished');
});

test('links survive a restart; a corrupt file stops the start', (t) => {
  const dir = mkdtempSync(join(tmpdir(), 'lounge-discord-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const file = join(dir, 'links.json');
  new LinkStore(file).set(1, { discordId: ALICE, level: 2, at: 1 });
  assert.deepEqual(new LinkStore(file).get(1), { discordId: ALICE, level: 2, at: 1 });
  writeFileSync(file, '[]');
  assert.throws(() => new LinkStore(file), /not a JSON object/);
  writeFileSync(file, '{"1":');
  assert.throws(() => new LinkStore(file), /links\.json/);
});

test('configuration is checked at start', () => {
  const base = { LOUNGE_WEBHOOK_SECRET: 'whsec_x', DISCORD_BOT_TOKEN: 'token', DISCORD_GUILD_ID: '300000000000000001', LINKED_ROLE_ID: LINKED };
  const cfg = loadConfig({ ...base, LEVEL_ROLES: `3:${VETERAN}, 2:${TRUSTED}` });
  assert.deepEqual(cfg.levelRoles, [{ level: 2, roleId: TRUSTED }, { level: 3, roleId: VETERAN }]);
  assert.equal(cfg.port, 8093);
  assert.throws(() => loadConfig({ ...base, LOUNGE_WEBHOOK_SECRET: '' }), /LOUNGE_WEBHOOK_SECRET/);
  assert.throws(() => loadConfig({ ...base, DISCORD_BOT_TOKEN: '' }), /DISCORD_BOT_TOKEN/);
  assert.doesNotThrow(() => loadConfig({ ...base, DISCORD_BOT_TOKEN: '', DRY_RUN: '1' }));
  assert.throws(() => loadConfig({ ...base, DISCORD_GUILD_ID: 'my-server' }), /not a Discord id/);
  assert.throws(() => loadConfig({ ...base, LINKED_ROLE_ID: '' }), /no role to manage/);
  assert.throws(() => loadConfig({ ...base, LEVEL_ROLES: '2:abc' }), /<level>:<role id>/);
  assert.throws(() => loadConfig({ ...base, LEVEL_ROLES: `2:${LINKED}` }), /different role/);
  assert.throws(() => loadConfig({ ...base, LEVEL_ROLES: `2:${TRUSTED},3:${TRUSTED}` }), /listed twice/);
});

test('Discord REST: success, not a member, rate limit, missing permission', async (t) => {
  quiet(t);
  const sent = [];
  const replies = [];
  const rest = new DiscordRest('token', '300000000000000001', async (url, init) => {
    sent.push({ url, init });
    return replies.shift();
  });
  const reply = (status, body) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

  replies.push(new Response(null, { status: 204 }));
  await rest.addRole(ALICE, LINKED, 'Lounge account linked');
  assert.equal(sent[0].url, `https://discord.com/api/v10/guilds/300000000000000001/members/${ALICE}/roles/${LINKED}`);
  assert.equal(sent[0].init.method, 'PUT');
  assert.equal(sent[0].init.headers.Authorization, 'Bot token');

  replies.push(reply(404, { code: 10007, message: 'Unknown Member' }));
  await rest.removeRole(ALICE, LINKED, 'unlinked'); // not in the server: nothing to do

  replies.push(reply(429, { retry_after: 0.01, message: 'You are being rate limited.' }), new Response(null, { status: 204 }));
  await rest.addRole(ALICE, TRUSTED, 'trust level 2');
  assert.equal(sent.length, 4, 'one retry after the rate limit');

  replies.push(reply(403, { code: 50013, message: 'Missing Permissions' }));
  await assert.rejects(rest.addRole(ALICE, VETERAN, 'trust level 3'), /Manage Roles/);
  replies.push(reply(404, { code: 10011, message: 'Unknown Role' }));
  await assert.rejects(rest.addRole(ALICE, VETERAN, 'trust level 3'), DiscordError);
  await assert.rejects(rest.addRole('alice', LINKED, 'x'), /not a Discord id/);
});

test('plan: the managed roles an account should hold', () => {
  const opts = { linkedRoleId: LINKED, levelRoles: parseLevelRoles(`2:${TRUSTED},3:${VETERAN}`) };
  assert.deepEqual(plan(opts, true, 2), [[LINKED, true], [TRUSTED, true], [VETERAN, false]]);
  assert.deepEqual(plan(opts, true, undefined), [[LINKED, true]]);
  assert.deepEqual(plan(opts, false, 3), [[LINKED, false], [TRUSTED, false], [VETERAN, false]]);
  assert.deepEqual(plan({ ...opts, linkedRoleId: '' }, true, 3), [[TRUSTED, true], [VETERAN, true]]);
});
