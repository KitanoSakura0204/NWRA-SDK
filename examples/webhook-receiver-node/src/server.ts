/**
 * Lounge Account webhook receiver on plain `node:http` (no framework).
 *
 * - verifies X-Lounge-Signature / X-Lounge-Timestamp with the TypeScript SDK
 * - opens encrypted deliveries (JWE) when LOUNGE_DECRYPTION_KEYS is set
 * - de-duplicates deliveries by event id (in-memory LRU)
 * - handles game_role.reviewed, user.deleted, server.offline, user.level_changed,
 *   connection.bound / connection.unbound and the console's ping
 *
 * The endpoint itself is in receiver.ts, the business logic in events.ts.
 * Configuration comes from the environment; see README.md.
 */
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
// explicit: with lib DOM the global setTimeout may be typed as returning a number (no unref)
import { setTimeout } from 'node:timers';

import { dispatch } from './events.js';
import { createReceiver, json, loadKeys } from './receiver.js';

function fail(message: string): never {
  console.error(message);
  process.exit(1);
}

const secret = process.env.LOUNGE_WEBHOOK_SECRET || fail('set LOUNGE_WEBHOOK_SECRET (admin console: 开放平台 -> Webhook -> 签名密钥)');
const host = process.env.HOST ?? '0.0.0.0';
const port = Number(process.env.PORT ?? 8091);
const webhookPath = process.env.WEBHOOK_PATH ?? '/lounge/webhook';
const tolerance = Number(process.env.WEBHOOK_TOLERANCE ?? 300);
let keys: ReturnType<typeof loadKeys> = [];
try {
  keys = loadKeys(process.env.LOUNGE_DECRYPTION_KEYS ?? '');
} catch (e) {
  fail(`LOUNGE_DECRYPTION_KEYS: ${(e as Error).message}`);
}
const receive = createReceiver({ secret, tolerance, keys, capacity: Number(process.env.SEEN_CAPACITY ?? 10000), dispatch });

async function route(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (url.pathname === '/healthz') return json(res, 200, { ok: true });
  if (url.pathname !== webhookPath) return json(res, 404, { error: 'not found' });
  return receive(req, res);
}

/* ---- bootstrap ---- */

const server = createServer((req, res) => {
  route(req, res).catch((e) => {
    console.error('unhandled error:', e);
    if (!res.headersSent) json(res, 500, { error: 'internal error' });
  });
});
server.requestTimeout = 10_000;
server.listen(port, host, () => {
  console.info(`listening on http://${host}:${port}${webhookPath} (timestamp tolerance ${tolerance}s, ${keys.length} decryption key(s))`);
});

for (const sig of ['SIGINT', 'SIGTERM'] as const) {
  process.on(sig, () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 5_000).unref();
  });
}
