// Runs against the compiled output: npm test (tsc, then node --test).
import test from 'node:test';
import assert from 'node:assert/strict';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Readable } from 'node:stream';

import { createReceiver, loadKeys } from '../dist/examples/webhook-receiver-node/src/receiver.js';
import { who } from '../dist/examples/webhook-receiver-node/src/events.js';
import { webhookSignature } from '../dist/typescript/src/index.js';

const SECRET = 'whsec_test';

function receiver(opts = {}) {
  return createReceiver({ secret: SECRET, tolerance: 300, keys: [], capacity: 100, dispatch: () => {}, ...opts });
}

/** Calls the handler directly with an in-memory request (no listener, no network). */
async function post(handler, body, headers, method = 'POST') {
  const req = Readable.from(body ? [Buffer.from(body)] : []);
  req.method = method;
  req.url = '/lounge/webhook';
  req.headers = Object.fromEntries(Object.entries(headers).map(([k, v]) => [k.toLowerCase(), v]));
  const out = { status: 0, body: {} };
  const res = {
    headersSent: false,
    setHeader() {},
    writeHead(status) {
      out.status = status;
      this.headersSent = true;
    },
    end(text) {
      out.body = text ? JSON.parse(text) : {};
    },
  };
  await handler(req, res);
  return out;
}

async function signed(secret, ts, body) {
  return { 'Content-Type': 'application/json', 'X-Lounge-Timestamp': String(ts), 'X-Lounge-Signature': await webhookSignature(secret, ts, body) };
}

const now = () => Math.floor(Date.now() / 1000);

test('plain delivery, then the same id again', async () => {
  const got = [];
  const h = receiver({ dispatch: (ev) => got.push(ev) });
  const ts = now();
  const body = JSON.stringify({ id: 'evt-1', event: 'user.level_changed', timestamp: ts, data: { userId: 1, uid: 100001, from: 1, to: 2 } });
  assert.deepEqual(await post(h, body, await signed(SECRET, ts, body)), { status: 200, body: { status: 'ok' } });
  assert.deepEqual(await post(h, body, await signed(SECRET, ts, body)), { status: 200, body: { status: 'duplicate' } });
  assert.equal(got.length, 1);
  assert.equal(got[0].data.to, 2);
});

test('rejections', async () => {
  const h = receiver();
  const ts = now();
  const body = JSON.stringify({ id: 'evt-2', event: 'ping', timestamp: ts, data: {} });
  assert.equal((await post(h, body, await signed('wrong', ts, body))).status, 401);
  assert.equal((await post(h, body + ' ', await signed(SECRET, ts, body))).status, 401, 'tampered');
  assert.equal((await post(h, body, await signed(SECRET, ts - 3600, body))).status, 401, 'stale');
  assert.equal((await post(h, body, {})).status, 401, 'unsigned');
  assert.equal((await post(h, 'not json', await signed(SECRET, ts, 'not json'))).status, 400);
  const noId = JSON.stringify({ event: 'ping', timestamp: ts, data: {} });
  assert.equal((await post(h, noId, await signed(SECRET, ts, noId))).status, 400);
  assert.equal((await post(h, '', {}, 'GET')).status, 405);
});

test('a failed handler answers 500 and the retry runs it again', async () => {
  let calls = 0;
  const h = receiver({ dispatch: () => { if (++calls === 1) throw new Error('database down'); } });
  const ts = now();
  const body = JSON.stringify({ id: 'evt-3', event: 'ping', timestamp: ts, data: {} });
  assert.equal((await post(h, body, await signed(SECRET, ts, body))).status, 500);
  assert.deepEqual((await post(h, body, await signed(SECRET, ts, body))).body, { status: 'ok' });
  assert.equal(calls, 2);
});

// The shared SDK test vectors hold a signed, encrypted webhook (SDK/go/testdata/vectors.json).
const vectorsUrl = new URL('../../../go/testdata/vectors.json', import.meta.url);

test('encrypted delivery (shared SDK vector)', { skip: !existsSync(vectorsUrl) && 'SDK vectors not found (example copied out of the repo)' }, async (t) => {
  const set = JSON.parse(readFileSync(vectorsUrl, 'utf8'));
  const v = set.vectors.find((x) => x.kind === 'webhook' && !x.fail);
  assert.ok(v, 'webhook vector missing');
  const dir = mkdtempSync(join(tmpdir(), 'lounge-recv-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const file = join(dir, 'key.jwk');
  writeFileSync(file, JSON.stringify(set.keys[v.key]));
  const keys = loadKeys(` ${file} ,`);
  assert.equal(keys.length, 1);

  // The vector's timestamp is fixed, so the replay window is off (tolerance 0).
  const got = [];
  const h = receiver({ secret: v.secret, tolerance: 0, keys, dispatch: (ev) => got.push(ev) });
  assert.deepEqual((await post(h, v.body, v.headers)).body, { status: 'ok' });
  assert.equal(got[0].id, 'evt-vector');
  assert.equal(got[0].data.user.nickname, '旅行者');

  // signed but not readable here: 500 keeps the delivery queued
  assert.equal((await post(receiver({ secret: v.secret, tolerance: 0 }), v.body, v.headers)).status, 500);
  // keys configured but the delivery is plain: refused
  const ts = now();
  const plain = JSON.stringify({ id: 'evt-4', event: 'ping', timestamp: ts, data: {} });
  assert.equal((await post(receiver({ secret: v.secret, keys }), plain, await signed(v.secret, ts, plain))).status, 500);

  // a public JWK is refused at startup
  const { d, ...pub } = set.keys[v.key];
  assert.ok(d);
  writeFileSync(file, JSON.stringify(pub));
  assert.throws(() => loadKeys(file), /not a private JWK/);
});

test('who() prefers the pairwise sub', () => {
  assert.equal(who(100001, undefined), 'uid=100001');
  assert.equal(who(undefined, { sub: '2f1c8e0a' }), 'sub=2f1c8e0a');
});
