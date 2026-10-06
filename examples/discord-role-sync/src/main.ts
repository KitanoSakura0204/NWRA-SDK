/**
 * Discord role sync for NoWork Account: a webhook endpoint that keeps Discord roles in step with
 * the user center.
 *
 * - connection.bound (provider discord): the Discord account gets LINKED_ROLE_ID and the
 *   LEVEL_ROLES its trust level earns
 * - user.level_changed: the level roles follow the new trust level
 * - connection.unbound / user.deleted: every managed role is removed
 *
 * Signature, JWE and de-duplication come from ../webhook-receiver-node/src/receiver.ts.
 * Configuration comes from the environment; see README.md.
 */
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
// explicit: with lib DOM the global setTimeout may be typed as returning a number (no unref)
import { setTimeout } from 'node:timers';

import { createReceiver, json, loadKeys } from '../../webhook-receiver-node/src/receiver.js';
import { loadConfig } from './config.js';
import { DiscordRest, DryRunApi } from './discord.js';
import { LinkStore } from './store.js';
import { EVENTS, RoleSync } from './sync.js';

function setup() {
  const cfg = loadConfig();
  const keys = loadKeys(cfg.keysPath);
  const store = new LinkStore(cfg.linksFile);
  const api = cfg.dryRun ? new DryRunApi() : new DiscordRest(cfg.botToken, cfg.guildId);
  const sync = new RoleSync({ api, store, linkedRoleId: cfg.linkedRoleId, levelRoles: cfg.levelRoles });
  const receive = createReceiver({
    secret: cfg.secret,
    tolerance: cfg.tolerance,
    keys,
    capacity: cfg.seenCapacity,
    dispatch: (ev) => sync.handle(ev),
  });
  return { cfg, keys, store, receive };
}

const { cfg, keys, store, receive } = (() => {
  try {
    return setup();
  } catch (e) {
    console.error((e as Error).message);
    return process.exit(1);
  }
})();

async function route(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (url.pathname === '/healthz') return json(res, 200, { ok: true });
  if (url.pathname !== cfg.path) return json(res, 404, { error: 'not found' });
  return receive(req, res);
}

const server = createServer((req, res) => {
  route(req, res).catch((e) => {
    console.error('unhandled error:', e);
    if (!res.headersSent) json(res, 500, { error: 'internal error' });
  });
});
server.requestTimeout = 10_000;
server.listen(cfg.port, cfg.host, () => {
  console.info(
    `listening on http://${cfg.host}:${cfg.port}${cfg.path}: guild ${cfg.guildId}, linked role ${cfg.linkedRoleId || '-'}, ` +
      `${cfg.levelRoles.length} level role(s), ${store.size} linked account(s), ${keys.length} decryption key(s)` +
      `${cfg.dryRun ? ', DRY RUN' : ''}; subscribe the webhook to ${EVENTS.join(', ')}`,
  );
});

for (const sig of ['SIGINT', 'SIGTERM'] as const) {
  process.on(sig, () => {
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(0), 5_000).unref();
  });
}
