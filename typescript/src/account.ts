import { ApiError, TransportError } from './index.js';

export interface PortalRole {
  id: number; serverId: number; serverName: string; serverCode: string;
  roleName: string; roleUuid: string; status: number; remark: string;
  createTime: string; reviewTime: string | null;
}

export interface PortalServer {
  id: number; code: string; name: string; description: string; gameType: string;
  icon: string; address: string; version: string; status: number;
  announcement: string; maintenanceMessage: string; whitelistMode: number;
  requiresWhitelist: boolean; userIsWhitelisted: boolean; userHasApplied: boolean;
  serverGmEnabled: boolean; serverGmLoginEnabled: boolean; playerDataEnabled: boolean;
  roleNameLabel: string; roleIdLabel: string; requireRoleId: boolean;
  minSecurityLevel: number; requireRealName: boolean; maxRolesPerUser: number;
  canBind: boolean; bindTip: string; ban: Record<string, unknown> | null; roles: PortalRole[];
}

export interface PortalRoleInput { roleName: string; roleUuid?: string; remark?: string }

/** Account portal uses the site's login JWT. A server key or OAuth access token is not a substitute. */
class JSONClient {
  private readonly base: string;
  private readonly token: string;
  private readonly timeout: number;
  private readonly fetchImpl: typeof fetch;

  constructor(base: string, opts: { token: string; timeoutMs?: number; fetch?: typeof fetch },
    private readonly prefix = '/api/account', private readonly scheme = 'Bearer') {
    this.base = base.replace(/\/+$/, '');
    this.token = opts.token;
    this.timeout = opts.timeoutMs ?? 10_000;
    this.fetchImpl = opts.fetch ?? fetch;
    if (!Number.isFinite(this.timeout) || this.timeout <= 0) throw new RangeError('timeoutMs must be positive');
  }

  protected async call<T>(method: string, path: string, body?: unknown, allowEmpty = false): Promise<T> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);
    try {
      const response = await this.fetchImpl(this.base + this.prefix + path, {
        method, signal: controller.signal, redirect: 'error',
        headers: { ...(this.token ? { Authorization: `${this.scheme} ${this.token}` } : {}), Accept: 'application/json',
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      const env = await response.json() as { code: number; message?: string; data?: T };
      if (!env || !Number.isInteger(env.code)) throw new TransportError('invalid API envelope');
      if (env.code !== 0) throw new ApiError(env.code, env.message ?? '');
      if (!response.ok || (!allowEmpty && (env.data === undefined || env.data === null))) throw new TransportError(`invalid API response (${response.status})`);
      return env.data as T;
    } catch (error) {
      if (error instanceof ApiError || error instanceof TransportError) throw error;
      throw new TransportError('API request failed', error);
    } finally { clearTimeout(timer); }
  }
}

/** Site-JWT portal client; independent from server keys and chat-session tokens. */
export class AccountClient extends JSONClient {
  constructor(base: string, opts: { token: string; timeoutMs?: number; fetch?: typeof fetch }) { super(base, opts); }

  servers(): Promise<PortalServer[]> { return this.call('GET', '/portal/servers'); }
  whitelistStatus(serverId: number): Promise<PortalServer> { return this.call('GET', `/portal/servers/${id(serverId)}/whitelist-status`); }
  whitelistApplications(): Promise<PortalRole[]> { return this.call('GET', '/whitelist/applications'); }
  applyWhitelist(serverId: number, input: PortalRoleInput): Promise<PortalServer> {
    return this.call('POST', `/portal/servers/${id(serverId)}/whitelist-apply`, input);
  }
  /** Also supports servers whose whitelist is disabled. */
  bindRole(serverId: number, input: PortalRoleInput): Promise<PortalServer> {
    return this.call('POST', `/portal/servers/${id(serverId)}/roles`, input);
  }
  launchGM(serverId: number, roleId: number): Promise<{ startUrl: string; expiresIn: number }> {
    return this.call('POST', `/gm/servers/${id(serverId)}/launch`, { roleId: id(roleId) });
  }
}

export interface ChatSession {
  token: string; expiresAt: string; via: string; channels: number[]; serverId: number; player: string;
}

export interface ChatMessage {
  id: number; channelId: number; source: string; author: string; kind: string; content: string;
  state: string; createTime: string; nonce?: string; ownReactions?: string[];
  [field: string]: unknown;
}

/** Chat REST client. Defaults to Authorization: Chat; siteLogin uses a site JWT. */
export class ChatClient extends JSONClient {
  constructor(base: string, opts: { token?: string; siteLogin?: boolean; timeoutMs?: number; fetch?: typeof fetch } = {}) {
    super(base, { ...opts, token: opts.token ?? '' }, '/api/chat', opts.siteLogin ? 'Bearer' : 'Chat');
  }
  redeemTicket(ticket: string, server = ''): Promise<ChatSession> { return this.call('POST', '/session', { ticket, server }); }
  siteSession(): Promise<ChatSession> { return this.call('POST', '/session/site', {}); }
  endSession(): Promise<void> { return this.call('DELETE', '/session', undefined, true); }
  me(): Promise<Record<string, unknown>> { return this.call('GET', '/me'); }
  channels(): Promise<Record<string, unknown>[]> { return this.call('GET', '/channels'); }
  history(channelId: number, opts: { before?: number; after?: number; limit?: number } = {}): Promise<ChatMessage[]> {
    const query = new URLSearchParams({ before: String(opts.before ?? 0), after: String(opts.after ?? 0), limit: String(opts.limit ?? 50) });
    return this.call('GET', `/channels/${id(channelId)}/messages?${query}`);
  }
  /** Reuse nonce on retries of the same message, and change it for new messages. */
  sendMessage(channelId: number, content: string, opts: { kind?: string; extra?: string; replyTo?: number; nonce?: string } = {}): Promise<ChatMessage> {
    return this.call('POST', `/channels/${id(channelId)}/messages`, { ...opts, content });
  }
  markRead(channelId: number, messageId: number): Promise<void> {
    return this.call('POST', `/channels/${id(channelId)}/read`, { id: id(messageId) }, true);
  }
}

function id(value: number): number {
  if (!Number.isSafeInteger(value) || value <= 0) throw new RangeError('id must be a positive integer');
  return value;
}
