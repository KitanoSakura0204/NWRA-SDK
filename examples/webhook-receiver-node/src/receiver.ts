/**
 * The webhook endpoint as a plain `node:http` handler, independent of the listener
 * so tests can call it directly:
 *
 * - verifies X-Lounge-Signature / X-Lounge-Timestamp over the raw body
 * - opens encrypted deliveries (JWE, SDK/docs/DELIVERY.md section 6)
 * - de-duplicates deliveries by event id (in-memory LRU)
 */
import { readFileSync } from 'node:fs';
import type { IncomingMessage, ServerResponse } from 'node:http';

import { DecryptError, openWebhook, verifyWebhook, WebhookError, type PrivateJwk, type WebhookEvent } from '../../../typescript/src/index.js';
import { LruSet } from './lru.js';

const MAX_BODY = 1 << 20; // 1 MiB, same cap as the SDKs

export interface ReceiverOptions {
  secret: string;
  /** Allowed timestamp skew in seconds; 0 turns the replay check off. */
  tolerance: number;
  /** Private JWKs; empty means the endpoint is not encrypted. */
  keys: PrivateJwk[];
  /** Event ids kept for de-duplication. */
  capacity: number;
  /** Business logic; throw to answer 500 and have the delivery retried. */
  dispatch: (ev: WebhookEvent) => void | Promise<void>;
}

/** Signed by the user center but not readable here (key rotation or configuration mismatch). */
class OpenError extends Error {}

/**
 * Read private JWKs from a comma-separated list of files. During a rotation list the new
 * and the old key: the JWE header's kid picks the right one (DELIVERY.md section 7).
 */
export function loadKeys(paths: string): PrivateJwk[] {
  const keys: PrivateJwk[] = [];
  for (const p of paths.split(',').map((s) => s.trim()).filter(Boolean)) {
    let jwk: PrivateJwk;
    try {
      jwk = JSON.parse(readFileSync(p, 'utf8')) as PrivateJwk;
    } catch (e) {
      throw new Error(`${p}: ${(e as Error).message}`);
    }
    if (!jwk || typeof jwk.d !== 'string') throw new Error(`${p}: not a private JWK (no "d")`);
    keys.push(jwk);
  }
  return keys;
}

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

export function json(res: ServerResponse, status: number, body: unknown): void {
  const text = JSON.stringify(body);
  res.writeHead(status, { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(text) });
  res.end(text);
}

function header(req: IncomingMessage, name: string): string {
  const v = req.headers[name.toLowerCase()];
  return Array.isArray(v) ? (v[0] ?? '') : (v ?? '');
}

/** Detects the encrypted shape even when a proxy dropped the X-Lounge-Encryption header. */
function hasJwe(text: string): boolean {
  try {
    return typeof JSON.parse(text)?.jwe === 'string';
  } catch {
    return false;
  }
}

/**
 * Returns the event to dispatch. A plain delivery is returned as verified; an encrypted one
 * is decrypted (openWebhook also checks that the inner id, event and timestamp repeat the
 * signed outer ones and that the header ctx is "webhook:<endpoint id>").
 */
async function open(req: IncomingMessage, text: string, ev: WebhookEvent, keys: PrivateJwk[]): Promise<WebhookEvent> {
  const encrypted = header(req, 'X-Lounge-Encryption').toUpperCase() === 'JWE' || hasJwe(text);
  if (!encrypted) {
    // Refuse plaintext from an endpoint that should encrypt rather than silently accept it.
    if (keys.length > 0) throw new OpenError('decryption keys configured but the delivery is not encrypted');
    return ev;
  }
  if (keys.length === 0) throw new OpenError('encrypted delivery but no decryption key configured');
  try {
    return await openWebhook(text, keys);
  } catch (e) {
    if (e instanceof DecryptError) throw new OpenError(e.message);
    throw e;
  }
}

/**
 * Builds the endpoint handler. Status codes matter: anything outside 2xx (or a timeout) makes
 * the user center retry with back-off, so bad signatures get 401 and handler failures get 500.
 */
export function createReceiver(opts: ReceiverOptions): (req: IncomingMessage, res: ServerResponse) => Promise<void> {
  const seen = new LruSet(opts.capacity);
  return async (req, res) => {
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

    // Verify against the raw bytes: the signature covers "<timestamp>.<body>" exactly as
    // sent, and an encrypted body is opened only after it verified.
    let ev: WebhookEvent;
    try {
      ev = await verifyWebhook(opts.secret, req.headers, body, opts.tolerance);
      ev = await open(req, body.toString('utf8'), ev, opts.keys);
    } catch (e) {
      if (e instanceof OpenError) {
        // 500 keeps the delivery queued: it succeeds once the right key is in place.
        console.warn(`delivery=${delivery} cannot open: ${e.message}`);
        return json(res, 500, { error: 'cannot decrypt' });
      }
      if (e instanceof WebhookError) {
        console.warn(`delivery=${delivery} rejected: ${e.message}`);
        // a malformed body will not get better with retries -> 400; signature / timestamp -> 401
        return json(res, e.message.includes('body') ? 400 : 401, { error: e.message });
      }
      throw e;
    }
    if (!ev.id) return json(res, 400, { error: 'missing event id' });

    // Idempotency: the same id may arrive again (retry after a timeout, manual redelivery
    // from the console). Acknowledge without running the handler twice.
    if (seen.has(ev.id)) {
      console.info(`delivery=${delivery} event=${ev.event} id=${ev.id} duplicate, ignored`);
      return json(res, 200, { status: 'duplicate' });
    }
    try {
      await opts.dispatch(ev);
    } catch (e) {
      console.error(`delivery=${delivery} event=${ev.event} id=${ev.id} handler failed:`, e);
      return json(res, 500, { error: 'handler failed' });
    }
    seen.add(ev.id);
    return json(res, 200, { status: 'ok' });
  };
}
