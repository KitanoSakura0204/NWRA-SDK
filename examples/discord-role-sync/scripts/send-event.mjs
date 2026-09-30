#!/usr/bin/env node
// Signs an event the way the user center does and POSTs it to a receiver (local tests only):
//
//   LOUNGE_WEBHOOK_SECRET=whsec_xxx node scripts/send-event.mjs connection.bound \
//     '{"userId":100,"uid":100001,"provider":"discord","externalId":"80351110224678912","externalName":"alice"}'
//
// WEBHOOK_URL picks the receiver (default this bot, http://127.0.0.1:8093/lounge/webhook; the
// other examples listen on 8090 Go, 8091 Node, 8092 Python). EVENT_ID resends a fixed id to
// try the de-duplication. Standard library only.
import { createHmac, randomUUID } from 'node:crypto';

const [event, data = '{}'] = process.argv.slice(2);
const secret = process.env.LOUNGE_WEBHOOK_SECRET;
if (!event || !secret) {
  console.error('usage: LOUNGE_WEBHOOK_SECRET=... node scripts/send-event.mjs <event> [data JSON]');
  process.exit(2);
}
let payload;
try {
  payload = JSON.parse(data);
} catch (e) {
  console.error(`data is not JSON: ${e.message}`);
  process.exit(2);
}
const url = process.env.WEBHOOK_URL ?? 'http://127.0.0.1:8093/lounge/webhook';
const ts = Math.floor(Date.now() / 1000);
const body = JSON.stringify({ id: process.env.EVENT_ID ?? randomUUID(), event, timestamp: ts, data: payload });
// X-Lounge-Signature: sha256=HMAC_SHA256(secret, "<timestamp>.<body>") (docs/PROTOCOL.md section 5)
const signature = 'sha256=' + createHmac('sha256', secret).update(`${ts}.${body}`).digest('hex');

const res = await fetch(url, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-Lounge-Event': event,
    'X-Lounge-Delivery': 'manual',
    'X-Lounge-Timestamp': String(ts),
    'X-Lounge-Signature': signature,
  },
  body,
});
console.log(res.status, await res.text());
