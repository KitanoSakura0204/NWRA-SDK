/** Player data and per-server bans (module "server-users", SDK/docs/SERVER_USERS.md). */

/** Business codes of the server-users routes (ApiError.code); other refusals use code 1. */
export const CODE_NOT_ON_ROSTER = 404;
/** The data changed since the version you read: read again, then write. */
export const CODE_VERSION_CONFLICT = 409;
/** A ban an administrator placed: only an administrator can change it. */
export const CODE_ADMIN_BAN = 403;

export interface UserFieldOption {
  value: string;
  label?: string;
}

/** One entry of the data schema the server owner defined. */
export interface UserField {
  key: string;
  label: string;
  type: 'text' | 'number' | 'bool' | 'select' | 'json';
  description?: string;
  /** Shown read-only to the player in the user centre. */
  visible: boolean;
  default?: unknown;
  /** Characters (text) or bytes (json). */
  maxLength?: number;
  min?: number;
  max?: number;
  integer?: boolean;
  options?: UserFieldOption[];
}

export interface UserSchema {
  fields: UserField[];
  /** Cap on one account's stored values (JSON bytes). */
  maxUserBytes: number;
}

/**
 * One account's data on this server: every schema field with its stored value, else the field
 * default, else null. version is the optimistic lock (0 = nothing stored yet).
 */
export interface UserData {
  uid: number;
  data: Record<string, unknown>;
  version: number;
  /** "2006-01-02 15:04:05" server time; null before the first write. */
  updatedAt: string | null;
  updatedBy?: string;
  size: number;
}

/** Names an account: its uid, or the name / uuid of one of its bindings on this server. */
export interface BanTarget {
  uid?: number;
  name?: string;
  uuid?: string;
}

/** expireTime ("2006-01-02 15:04:05" or RFC 3339) or durationMinutes; neither = permanent. */
export interface BanOptions {
  reason?: string;
  expireTime?: string;
  durationMinutes?: number;
}

/** An active ban on this server with the account's bindings there. */
export interface ServerBan {
  uid: number;
  names: string[];
  uuids: string[];
  reason: string;
  /** null = permanent. */
  expireTime: string | null;
  createTime: string;
  source: 'admin' | 'owner' | 'plugin' | string;
}

/** An online player (from the last heartbeat) whose account is banned here. */
export interface KickEntry {
  name: string;
  uuid?: string;
  uid: number;
  reason: string;
  expireTime: string | null;
}

/** Write options: version is the one you last read; leave it out to write unconditionally. */
export interface WriteOptions {
  version?: number;
}

/** Whether a data write was refused because of a stale version. */
export function isVersionConflict(e: unknown): boolean {
  return !!e && typeof e === 'object' && (e as { name?: unknown }).name === 'ApiError' && (e as { code?: unknown }).code === CODE_VERSION_CONFLICT;
}
