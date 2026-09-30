/**
 * Role sync: turns Lounge Account events into Discord role changes. Every change converges on
 * the roles an account should hold, so a retried or duplicated delivery is harmless, and only the
 * roles configured here are ever touched.
 */
import type { WebhookEvent } from '../../../typescript/src/index.js';
import type { LevelRole } from './config.js';
import { SNOWFLAKE, type RoleApi } from './discord.js';
import type { LinkStore } from './store.js';

export interface SyncOptions {
  api: RoleApi;
  store: LinkStore;
  /** Held by every linked account; '' for none. */
  linkedRoleId: string;
  /** Each role is held from its level upwards. */
  levelRoles: LevelRole[];
}

/** The events the bot reacts to; subscribe the webhook to exactly these. */
export const EVENTS = ['connection.bound', 'connection.unbound', 'user.level_changed', 'user.deleted'];

/** Fields of those events (docs/PROTOCOL.md section 5). */
interface EventData {
  userId?: number;
  provider?: string;
  externalId?: string;
  to?: number;
  /** Delivery-profile user object: security_level when the profile's webhook rules map it. */
  user?: { sub?: string; security_level?: number };
}

/**
 * The managed roles and whether the account should hold each one. linked false: none. An unknown
 * level (undefined) leaves the level roles alone rather than strip roles the bot cannot judge.
 */
export function plan(opts: Pick<SyncOptions, 'linkedRoleId' | 'levelRoles'>, linked: boolean, level: number | undefined): Array<[string, boolean]> {
  const out: Array<[string, boolean]> = [];
  if (opts.linkedRoleId) out.push([opts.linkedRoleId, linked]);
  for (const r of opts.levelRoles) {
    if (!linked) out.push([r.roleId, false]);
    else if (level !== undefined) out.push([r.roleId, level >= r.level]);
  }
  return out;
}

export class RoleSync {
  /** Per-account queue: two events for one account never interleave their Discord calls. */
  private readonly queue = new Map<number, Promise<void>>();

  constructor(private readonly opts: SyncOptions) {}

  /** Webhook dispatch. Throws when Discord failed: the receiver answers 500 and the event is retried. */
  async handle(ev: WebhookEvent): Promise<void> {
    if (!EVENTS.includes(ev.event)) return;
    const d = ev.data as unknown as EventData;
    if (ev.event.startsWith('connection.') && d.provider !== 'discord') return;
    if (typeof d.userId !== 'number') {
      // A pairwise endpoint carries data.user.sub and no userId / externalId: nothing maps to
      // Discord, and a retry would not change that. Bind this webhook to a public profile.
      console.error(`${ev.event} id=${ev.id} has no userId (pairwise endpoint?): skipped`);
      return;
    }
    const userId = d.userId;
    return this.serialize(userId, () => this.apply(ev, userId, d));
  }

  private async apply(ev: WebhookEvent, userId: number, d: EventData): Promise<void> {
    const store = this.opts.store;
    const link = store.get(userId);
    if (link && ev.timestamp < link.at) {
      console.info(`${ev.event} id=${ev.id} is older than the last change of user ${userId}: ignored`);
      return;
    }
    switch (ev.event) {
      case 'connection.bound': {
        const discordId = d.externalId ?? '';
        if (!SNOWFLAKE.test(discordId)) {
          console.warn(`connection.bound id=${ev.id}: "${discordId}" is not a Discord id, skipped`);
          return;
        }
        const sl = d.user?.security_level;
        const level = typeof sl === 'number' ? sl : link?.level;
        if (link?.discordId && link.discordId !== discordId) {
          // the account switched Discord accounts: the old one loses its roles
          await this.converge(link.discordId, false, undefined, 'Lounge account linked another Discord account');
        }
        await this.converge(discordId, true, level, 'Lounge account linked');
        store.set(userId, { discordId, level, at: ev.timestamp });
        console.info(`user ${userId} linked Discord ${discordId} (trust level ${level ?? 'unknown'})`);
        return;
      }
      case 'connection.unbound': {
        const discordId = SNOWFLAKE.test(d.externalId ?? '') ? (d.externalId as string) : (link?.discordId ?? '');
        if (discordId) await this.converge(discordId, false, undefined, 'Lounge account unlinked Discord');
        store.set(userId, { discordId: '', level: link?.level, at: ev.timestamp });
        console.info(`user ${userId} unlinked Discord ${discordId || '(unknown)'}`);
        return;
      }
      case 'user.level_changed': {
        // Not linked: nothing to change. The level arrives with connection.bound when the
        // endpoint's delivery profile maps security_level, otherwise with the next change.
        if (!link?.discordId) return;
        const level = typeof d.to === 'number' ? d.to : link.level;
        await this.converge(link.discordId, true, level, `trust level ${level ?? 'unknown'}`);
        store.set(userId, { ...link, level, at: ev.timestamp });
        console.info(`user ${userId} trust level ${level ?? 'unknown'}: Discord ${link.discordId} roles updated`);
        return;
      }
      case 'user.deleted': {
        if (link?.discordId) await this.converge(link.discordId, false, undefined, 'Lounge account deleted');
        store.set(userId, { discordId: '', at: ev.timestamp });
        console.info(`user ${userId} deleted${link?.discordId ? `: Discord ${link.discordId} lost its roles` : ''}`);
        return;
      }
    }
  }

  /** Puts or removes each managed role; both calls are idempotent on Discord's side. */
  private async converge(discordId: string, linked: boolean, level: number | undefined, reason: string): Promise<void> {
    for (const [roleId, want] of plan(this.opts, linked, level)) {
      if (want) await this.opts.api.addRole(discordId, roleId, reason);
      else await this.opts.api.removeRole(discordId, roleId, reason);
    }
  }

  private serialize(userId: number, fn: () => Promise<void>): Promise<void> {
    // a failed predecessor must not block the next event: its own caller already saw the error
    const run = (this.queue.get(userId) ?? Promise.resolve()).catch(() => undefined).then(fn);
    this.queue.set(userId, run);
    const clear = (): void => {
      if (this.queue.get(userId) === run) this.queue.delete(userId);
    };
    run.then(clear, clear);
    return run;
  }
}
