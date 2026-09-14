/**
 * Lounge Account webhook receiver on plain `node:http` (no framework).
 *
 * - verifies X-Lounge-Signature / X-Lounge-Timestamp with the TypeScript SDK
 * - de-duplicates deliveries by event id (in-memory LRU)
 * - handles game_role.reviewed, user.deleted and server.offline
 *
 * Configuration comes from the environment; see README.md.
 */
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';

import { verifyWebhook, WebhookError, type WebhookEvent } from '../../../typescript/src/index.js';
import { LruSet } from './lru.js';

const secret = process.env.LOUNGE_WEBHOOK_SECRET ?? '';
if (!secret) {
  console.error('set LOUNGE_WEBHOOK_SECRET (admin console: 开发者 -> Webhook 事件推送 -> 签名密钥)');
  process.exit(1);
}
const host = process.env.HOST ?? '0.0.0.0';
const port = Number(process.env.PORT ?? 8091);
const webhookPath = process.env.WEBHOOK_PATH ?? '/lounge/webhook';
const tolerance = Number(process.env.WEBHOOK_TOLERANCE ?? 300);
const seen = new LruSet(Number(process.env.SEEN_CAPACITY ?? 10000));
const MAX_BODY = 1 << 20; // 1 MiB, same cap as the SDKs

/* ---- event payloads (field names as emitted by the user center) ---- */

const RoleStatus = { Pending: 0, Approved: 1, Rejected: 2, Banned: 3 } as const;

interface GameRoleReviewed {
  roleId: number;
  userId: number;
  serverId: number;
  roleName: string;
  roleUuid: string;
  status: number;
  reviewerId: number;
}

interface UserDeleted {
  userId: number;
  uid: number;
  username: string;
  reason: string;
}

interface ServerOffline {
  serverId: number;
  code: string;
  name: string;
  lastHeartbeat: string | null; // "2006-01-02 15:04:05" or null
}

/* ---- http plumbing ---- */

function readBody(req: IncomingMessage): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let size = 0;
    req.on('data', (chunk: Buffer) => {
      size += chunk.length;
      if (size > MAX_BODY) {
        reject(new Error('payload too large'));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

function json(res: ServerResponse, status: number, body: unknown): void {
  const text = JSON.stringify(body);
  res.writeHead(status, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(text) });
  res.end(text);
}

function header(req: IncomingMessage, name: string): string {
  const v = req.headers[name.toLowerCase()];
  return Array.isArray(v) ? (v[0] ?? '') : (v ?? '');
}

/**
 * Status codes matter: anything outside 2xx (or a timeout) makes the user center
 * retry with back-off, so bad signatures get 401 and handler failures get 500.
 */
async function handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (url.pathname === '/healthz') return json(res, 200, { ok: true });
  if (url.pathname !== webhookPath) return json(res, 404, { error: 'not found' });
  if (req.method !== 'POST') {
    res.setHeader('Allow', 'POST');
    return json(res, 405, { error: 'method not allowed' });
  }
  const delivery = header(req, 'X-Lounge-Delivery');

  let body: Buffer;
  try {
    body = await readBody(req);
  } catch (e) {
    console.warn(`delivery=${delivery} rejected: ${(e as Error).message}`);
    return json(res, 413, { error: 'payload too large' });
  }

  // Verify against the raw bytes: the signature covers "<timestamp>.<body>".
  let ev: WebhookEvent;
  try {
    ev = await verifyWebhook(secret, req.headers, body, tolerance);
  } catch (e) {
    if (e instanceof WebhookError) {
      console.warn(`delivery=${delivery} rejected: ${e.message}`);
      // a malformed body will not get better with retries -> 400; signature/timestamp -> 401
      return json(res, e.message.includes('body') ? 400 : 401, { error: e.message });
    }
    throw e;
  }
  if (!ev.id) return json(res, 400, { error: 'missing event id' });

  // Idempotency: the same id may arrive again (retry after a timeout, manual
  // redelivery from the console). Acknowledge without running the handler twice.
  if (seen.has(ev.id)) {
    console.info(`delivery=${delivery} event=${ev.event} id=${ev.id} duplicate, ignored`);
    return json(res, 200, { status: 'duplicate' });
  }

  try {
    dispatch(ev);
  } catch (e) {
    console.error(`delivery=${delivery} event=${ev.event} id=${ev.id} handler failed:`, e);
    return json(res, 500, { error: 'handler failed' });
  }
  seen.add(ev.id);
  return json(res, 200, { status: 'ok' });
}

/* ---- business logic ---- */

/** Keep this short (the sender waits at most 10 s); queue anything heavier. */
function dispatch(ev: WebhookEvent): void {
  const at = new Date(ev.timestamp * 1000).toISOString();
  switch (ev.event) {
    case 'game_role.reviewed': {
      const d = ev.data as unknown as GameRoleReviewed;
      const who = `role="${d.roleName}" uuid=${d.roleUuid} user=${d.userId} server=${d.serverId} reviewer=${d.reviewerId}`;
      switch (d.status) {
        case RoleStatus.Approved:
          console.info(`[${at}] role APPROVED: ${who}`);
          // e.g. add the player to the server whitelist / grant a rank
          break;
        case RoleStatus.Rejected:
          console.info(`[${at}] role REJECTED: ${who}`);
          break;
        case RoleStatus.Banned:
          console.info(`[${at}] role BANNED: ${who}`);
          // e.g. kick the player if online
          break;
        case RoleStatus.Pending:
          console.info(`[${at}] role back to PENDING: ${who}`);
          break;
        default:
          console.info(`[${at}] role status ${d.status} (unknown): ${who}`);
      }
      break;
    }
    case 'user.deleted': {
      const d = ev.data as unknown as UserDeleted;
      console.info(`[${at}] user DELETED: userId=${d.userId} uid=${d.uid} username="${d.username}" reason="${d.reason}"`);
      // e.g. purge local data tied to this account
      break;
    }
    case 'server.offline': {
      const d = ev.data as unknown as ServerOffline;
      console.info(`[${at}] server OFFLINE: code=${d.code} name="${d.name}" id=${d.serverId} lastHeartbeat=${d.lastHeartbeat ?? 'null'}`);
      // e.g. page the on-call channel
      break;
    }
    case 'ping':
      // sent by the "测试" button in the admin console
      console.info(`[${at}] ping from console: ${JSON.stringify(ev.data)}`);
      break;
    default:
      console.info(`[${at}] event ${ev.event} ignored (id=${ev.id})`);
  }
}

/* ---- bootstrap ---- */

const server = createServer((req, res) => {
  handle(req, res).catch((e) => {
    console.error('unhandled error:', e);
    if (!res.headersSent) json(res, 500, { error: 'internal error' });
  });
});
server.requestTimeout = 10_000;
server.listen(port, host, () => {
  console.info(`listening on http://${host}:${port}${webhookPath} (timestamp tolerance ${tolerance}s)`);
});

for (const sig of ['SIGINT', 'SIGTERM'] as const) {
  process.on(sig, () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 5_000).unref();
  });
}
