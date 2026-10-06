/**
 * The two Discord REST calls the bot needs (API v10, plain fetch). The bot needs the "Manage
 * Roles" permission, and its own role must sit above every role it manages in the role list.
 */
const API = 'https://discord.com/api/v10';
/** A Discord id (user, role, server). */
export const SNOWFLAKE = /^\d{17,20}$/;
// Discord JSON error codes that mean "this account is not in the server": nothing to change.
const NOT_A_MEMBER = new Set([10007 /* Unknown Member */, 10013 /* Unknown User */]);

export interface RoleApi {
  addRole(discordId: string, roleId: string, reason: string): Promise<void>;
  removeRole(discordId: string, roleId: string, reason: string): Promise<void>;
}

/** A Discord call failed; the webhook answers 500 and the user center retries later. */
export class DiscordError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
    this.name = 'DiscordError';
  }
}

export type Fetch = (url: string, init: RequestInit) => Promise<Response>;

export class DiscordRest implements RoleApi {
  constructor(
    private readonly token: string,
    private readonly guildId: string,
    private readonly fetchImpl: Fetch = (url, init) => fetch(url, init),
  ) {}

  addRole(discordId: string, roleId: string, reason: string): Promise<void> {
    return this.call('PUT', discordId, roleId, reason, false);
  }

  removeRole(discordId: string, roleId: string, reason: string): Promise<void> {
    return this.call('DELETE', discordId, roleId, reason, false);
  }

  private async call(method: 'PUT' | 'DELETE', discordId: string, roleId: string, reason: string, retried: boolean): Promise<void> {
    // both ids end up in the URL: accept Discord snowflakes only
    if (!SNOWFLAKE.test(discordId) || !SNOWFLAKE.test(roleId)) throw new DiscordError(0, `not a Discord id: ${discordId} / ${roleId}`);
    const res = await this.fetchImpl(`${API}/guilds/${this.guildId}/members/${discordId}/roles/${roleId}`, {
      method,
      headers: {
        Authorization: `Bot ${this.token}`,
        'User-Agent': 'DiscordBot (https://github.com/wangework/nowork-sdk, 1.0)',
        'X-Audit-Log-Reason': encodeURIComponent(reason),
      },
      signal: AbortSignal.timeout(8_000),
    });
    if (res.status === 204) return;
    const text = await res.text().catch(() => '');
    let info: { code?: number; message?: string; retry_after?: number } = {};
    try {
      info = JSON.parse(text) ?? {};
    } catch {
      // not JSON: keep the raw text for the error
    }
    if (res.status === 404 && info.code !== undefined && NOT_A_MEMBER.has(info.code)) {
      // Not (or no longer) in the server. The next event after they join sets the roles.
      console.info(`discord: ${discordId} is not a member of the server, ${method} role ${roleId} skipped`);
      return;
    }
    if (res.status === 429 && !retried && Number(info.retry_after) <= 5) {
      // rate limited: wait once when it is short, otherwise let the user center retry later
      await new Promise<void>((resolve) => setTimeout(() => resolve(), Math.ceil(Number(info.retry_after) * 1000)));
      return this.call(method, discordId, roleId, reason, true);
    }
    const hint = res.status === 403 ? ' (the bot needs Manage Roles and a role above this one)' : '';
    throw new DiscordError(res.status, `${method} role ${roleId} for ${discordId}: HTTP ${res.status} ${info.message ?? text.slice(0, 200)}${hint}`);
  }
}

/** DRY_RUN=1: log what would change, call nothing. */
export class DryRunApi implements RoleApi {
  async addRole(discordId: string, roleId: string, reason: string): Promise<void> {
    console.info(`[dry run] add role ${roleId} to ${discordId} (${reason})`);
  }

  async removeRole(discordId: string, roleId: string, reason: string): Promise<void> {
    console.info(`[dry run] remove role ${roleId} from ${discordId} (${reason})`);
  }
}
