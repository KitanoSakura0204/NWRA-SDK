/**
 * NoWork account -> Discord account links, learned from connection.bound and kept in a small JSON
 * file: user.level_changed and user.deleted carry no Discord id, so the bot needs them to know whose
 * roles to change.
 *
 * `at` is the timestamp of the last event applied to the account. An older event (a retry that
 * arrives late) is ignored, so a stale connection.bound cannot re-grant roles after the account
 * unlinked; unlinked and deleted accounts keep a small tombstone for that reason.
 */
import { existsSync, readFileSync, renameSync, writeFileSync } from 'node:fs';

export interface Link {
  /** Empty once the account unlinked Discord or was deleted (tombstone). */
  discordId: string;
  /** Last known trust level (security_level); undefined until an event carried it. */
  level?: number;
  /** Unix seconds of the last event applied. */
  at: number;
}

export class LinkStore {
  private links: Record<string, Link> = {};

  /** file '' keeps the links in memory only (tests, dry runs). */
  constructor(private readonly file: string) {
    if (!file || !existsSync(file)) return;
    let data: unknown;
    try {
      data = JSON.parse(readFileSync(file, 'utf8'));
    } catch (e) {
      throw new Error(`${file}: ${(e as Error).message}`);
    }
    if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error(`${file}: not a JSON object`);
    this.links = data as Record<string, Link>;
  }

  get(userId: number): Link | undefined {
    return this.links[String(userId)];
  }

  set(userId: number, link: Link): void {
    this.links[String(userId)] = link;
    this.save();
  }

  /** Linked accounts (tombstones excluded). */
  get size(): number {
    return Object.values(this.links).filter((l) => l.discordId).length;
  }

  private save(): void {
    if (!this.file) return;
    // write, then rename: a crash mid-write never leaves a truncated file behind
    const tmp = `${this.file}.tmp`;
    writeFileSync(tmp, JSON.stringify(this.links, null, 2));
    renameSync(tmp, this.file);
  }
}
