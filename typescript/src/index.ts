/** NoWork Account SDK — Server API (whitelist / heartbeat) and OIDC helpers. Requires global fetch (Node 18+). */

/** SDK release this package implements (see CHANGELOG.md). */
export const SDK_VERSION = '1.3.0';

import type { BanOptions, BanTarget, KickEntry, ServerBan, UserData, UserSchema, WriteOptions } from './serverusers.js';
import { decryptJwe, type PrivateJwk } from './delivery.js';

export class TransportError extends Error {
  constructor(
    message: string,
    public cause?: unknown
  ) {
    super(`nowork: transport error: ${message}`);
    this.name = 'TransportError';
  }
}

export class ApiError extends Error {
  constructor(
    public code: number,
    message: string
  ) {
    super(`nowork: api error ${code}: ${message}`);
    this.name = 'ApiError';
  }
}

export class OAuthError extends Error {
  constructor(
    public error: string,
    public description = '',
    public status = 0
  ) {
    super(`nowork: oauth error ${error}: ${description}`);
    this.name = 'OAuthError';
  }
}

/** Header marking a Server API reply whose data is the {"jwe": "..."} envelope (DELIVERY.md §5). */
export const HEADER_ENCRYPTION = 'X-NoWork-Encryption';

/**
 * An encrypted Server API reply could not be authenticated/decoded, or a whitelist reply was
 * plaintext while decryption keys were configured. Never fail open on this error: an unreadable
 * whitelist is not a disabled whitelist.
 */
export class EncryptionError extends Error {
  constructor(
    public route: string,
    public reason: string,
    public cause?: unknown
  ) {
    super(`nowork: encrypted response on ${route}: ${reason}`);
    this.name = 'EncryptionError';
  }
}

/** A Server API reply as received, data left undecoded (pass `envelope` to openServerData). */
export interface RawResponse {
  status: number;
  headers: Headers | undefined;
  message: string;
  data: unknown;
  /** The whole parsed body {code, message, data}. */
  envelope: Record<string, unknown>;
  /** X-NoWork-Encryption is present or data has only a jwe member (even if malformed). */
  encrypted: boolean;
}

function isJweData(data: unknown): boolean {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return false;
  const keys = Object.keys(data);
  return keys.length === 1 && keys[0] === 'jwe';
}

const SERVER_RESPONSE_LIMIT = 8 * 1024 * 1024;
const SEALED_ROUTES = new Set(['whitelist', 'whitelist/check']);

function serverRoute(path: string): string {
  return path.split(/[?#]/, 1)[0].replace(/^\/server\/v1\//, '');
}

function isServerCode(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && !/[:\s\u0000-\u001f\u007f]/.test(value);
}

/** Bound wire bytes before JSON/JWE decoding, including responses without Content-Length. */
async function readServerEnvelope(response: Response, signal: AbortSignal): Promise<unknown> {
  const tooLarge = () => new TransportError(`response exceeds ${SERVER_RESPONSE_LIMIT}-byte limit`);
  const declared = response.headers?.get?.('Content-Length');
  if (declared && /^\d+$/.test(declared) && Number(declared) > SERVER_RESPONSE_LIMIT) {
    void response.body?.cancel().catch(() => {});
    throw tooLarge();
  }
  if (signal.aborted) {
    void response.body?.cancel(signal.reason).catch(() => {});
    throw signal.reason;
  }
  if (!response.body) {
    // Retain compatibility with custom fetch adapters that expose json() without a stream.
    const value: unknown = await response.json();
    const text = JSON.stringify(value);
    if (text && (text.length > SERVER_RESPONSE_LIMIT || new TextEncoder().encode(text).length > SERVER_RESPONSE_LIMIT)) {
      throw tooLarge();
    }
    return value;
  }
  const reader = response.body.getReader();
  const cancel = () => { void reader.cancel(signal.reason).catch(() => {}); };
  signal.addEventListener('abort', cancel, { once: true });
  const decoder = new TextDecoder('utf-8', { fatal: true });
  const chunks: string[] = [];
  let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (signal.aborted) throw signal.reason;
      if (done) break;
      size += value.byteLength;
      if (size > SERVER_RESPONSE_LIMIT) throw tooLarge();
      chunks.push(decoder.decode(value, { stream: true }));
    }
    chunks.push(decoder.decode());
    return JSON.parse(chunks.join(''));
  } catch (error) {
    void reader.cancel(error).catch(() => {});
    throw error;
  } finally {
    signal.removeEventListener('abort', cancel);
    reader.releaseLock();
  }
}

export interface WhitelistEntry {
  name: string;
  uuid: string;
  uid: number;
  nickname: string;
  securityLevel: number;
  since: string;
}

export interface Whitelist {
  server: string;
  mode: 0 | 1 | 2;
  count: number;
  entries: WhitelistEntry[];
}

export interface CheckResult {
  allowed: boolean;
  bound: boolean;
  reason: string;
  /** Extra text for some refusals, e.g. the reason of a per-server ban (reason "banned"). */
  message?: string;
  player?: WhitelistEntry | null;
}

/** Character identity only: GM authorization remains the relying party's responsibility. */
export interface GMSession {
  serverId: number;
  serverCode: string;
  subject: string;
  character: { id: number; roleName: string; roleUuid: string };
  authenticatedAt: number;
  scope: 'character:login';
}

export interface ChatTicketInput {
  accessToken?: string; uid?: number; playerName?: string; playerUuid?: string; channels?: number[];
}
export interface ChatTicket {
  ticket: string; expiresAt: string; channels: number[]; embedPath: string; wsPath: string;
}
export interface ChatRelayItem {
  id: number; kind: string; channelId: number; payload: Record<string, unknown>; attempts: number;
}
export interface ChatPost { player: string; text: string; playerUuid?: string; uid?: number; kind?: 'chat' | 'system' }

export interface NoworkClientOptions {
  serverKey: string;
  timeoutMs?: number;
  fetch?: typeof fetch;
  /** Private JWK(s); nonempty keys require encrypted whitelist replies. Keep both keys during rotation. */
  decryptionKeys?: PrivateJwk | readonly PrivateJwk[];
  /** Expected server code in the authenticated ctx. Recommended when keys are shared by servers. */
  serverCode?: string;
}

interface Envelope<T> {
  code: number;
  message?: string;
  data?: T;
}

/** Server API client authenticated with X-Server-Key. */
export class NoworkClient {
  private readonly base: string;
  private readonly key: string;
  private readonly timeout: number;
  private readonly f: typeof fetch;
  private readonly decryptionKeys: PrivateJwk[];
  private readonly serverCode?: string;

  constructor(base: string, opts: NoworkClientOptions) {
    this.base = base.replace(/\/+$/, '');
    this.key = opts.serverKey;
    this.timeout = opts.timeoutMs ?? 10_000;
    this.f = opts.fetch ?? fetch;
    if (!Number.isSafeInteger(this.timeout) || this.timeout <= 0 || this.timeout > 2_147_483_647) {
      throw new RangeError('timeoutMs must be a positive 32-bit integer');
    }
    if (opts.serverCode !== undefined && !isServerCode(opts.serverCode)) throw new TypeError('invalid serverCode');
    this.serverCode = opts.serverCode;
    const keys: readonly PrivateJwk[] = opts.decryptionKeys === undefined ? []
      : Array.isArray(opts.decryptionKeys) ? opts.decryptionKeys : [opts.decryptionKeys as PrivateJwk];
    this.decryptionKeys = keys.map(key => {
      if (!key || typeof key !== 'object' || Array.isArray(key) || !['RSA', 'EC', 'OKP'].includes(key.kty)
        || typeof key.d !== 'string' || !key.d) throw new TypeError('decryptionKeys must contain private JWK objects');
      // Later mutation of the caller's key list must not disable the downgrade guard.
      return Object.freeze({ ...key });
    });
  }

  /**
   * Call a Server API route (e.g. '/server/v1/whitelist') and return the reply without decoding
   * data. Transport failures and business errors (code != 0) still throw. This low-level accessor
   * skips automatic decryption and the plaintext guard; use openServerData with an expected ctx.
   */
  raw(method: string, path: string, query?: Record<string, string | undefined>, body?: unknown): Promise<RawResponse> {
    return this.send(method, path, query, body);
  }

  /** Decodes data, opening encrypted replies with the configured keys or failing closed. */
  private async call<T>(
    method: string,
    path: string,
    query?: Record<string, string | undefined>,
    body?: unknown
  ): Promise<T> {
    const res = await this.send(method, path, query, body);
    const route = serverRoute(path);
    let data = res.data;
    if (res.encrypted) {
      if (!this.decryptionKeys.length) throw new EncryptionError(route, 'no decryption key configured');
      data = await this.open(data, route);
    } else if (this.decryptionKeys.length && SEALED_ROUTES.has(route)) {
      throw new EncryptionError(route, 'plain reply while decryption keys are configured');
    }
    if (data === undefined || data === null) {
      throw new TransportError(`invalid response (${res.status}): missing data`);
    }
    return data as T;
  }

  private async open(data: unknown, route: string): Promise<unknown> {
    const jwe = isJweData(data)
      ? (data as Record<string, unknown>).jwe : undefined;
    if (typeof jwe !== 'string' || !jwe) throw new EncryptionError(route, 'encrypted reply without a JWE');
    try {
      const opened = await decryptJwe(jwe, this.decryptionKeys);
      const ctx = typeof opened.header.ctx === 'string' ? opened.header.ctx.split(':') : [];
      if (ctx.length !== 3 || ctx[0] !== 'server' || !isServerCode(ctx[1]) || ctx[2] !== route
        || (this.serverCode !== undefined && ctx[1] !== this.serverCode)) {
        throw new EncryptionError(route, 'envelope ctx mismatch');
      }
      const plain: unknown = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(opened.bytes));
      if (!plain || typeof plain !== 'object' || Array.isArray(plain) !== (route === 'gm/commands')) {
        throw new EncryptionError(route, 'decrypted data has an invalid shape');
      }
      const fields = plain as Record<string, unknown>;
      if (route === 'whitelist' && (!Object.prototype.hasOwnProperty.call(fields, 'mode')
        || (fields.mode !== 0 && fields.mode !== 1 && fields.mode !== 2))) {
        throw new EncryptionError(route, 'decrypted whitelist mode is missing or invalid');
      }
      if (route === 'whitelist/check' && (!Object.prototype.hasOwnProperty.call(fields, 'allowed')
        || typeof fields.allowed !== 'boolean')) {
        throw new EncryptionError(route, 'decrypted whitelist allowed flag is missing or invalid');
      }
      return plain;
    } catch (error) {
      if (error instanceof EncryptionError) throw error;
      throw new EncryptionError(route, 'the JWE did not open as valid JSON data', error);
    }
  }

  private async send(
    method: string,
    path: string,
    query?: Record<string, string | undefined>,
    body?: unknown
  ): Promise<RawResponse> {
    const url = new URL(this.base + path);
    for (const [k, v] of Object.entries(query ?? {})) {
      if (v) url.searchParams.set(k, v);
    }
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), this.timeout);
    let res: Response;
    try {
      res = await this.f(url, {
        method,
        redirect: 'error',
        credentials: 'omit',
        headers: {
          Accept: 'application/json',
          'X-Server-Key': this.key,
          ...(body ? { 'Content-Type': 'application/json' } : {})
        },
        body: body ? JSON.stringify(body) : undefined,
        signal: ctrl.signal
      });
    } catch (e) {
      clearTimeout(timer);
      throw new TransportError(String(e), e);
    }
    let env: Envelope<unknown>;
    try {
      // The timeout covers reading the body as well, so the timer is only cleared afterwards.
      env = (await readServerEnvelope(res, ctrl.signal)) as Envelope<unknown>;
      if (ctrl.signal.aborted) throw new TransportError('request timed out', ctrl.signal.reason);
    } catch (e) {
      if (e instanceof TransportError) throw e;
      throw new TransportError(ctrl.signal.aborted ? 'request timed out' : `invalid json (${res.status})`, e);
    } finally {
      clearTimeout(timer);
    }
    if (!env || typeof env !== 'object' || Array.isArray(env) || !Number.isSafeInteger(env.code)) {
      throw new TransportError(`invalid response (${res.status}): missing code`);
    }
    const message = typeof env.message === 'string' ? env.message : '';
    if (env.code !== 0) throw new ApiError(env.code, message);
    if (res.status >= 400) throw new TransportError(`http ${res.status}`);
    const headers: Headers | undefined = res.headers;
    return {
      status: res.status,
      headers,
      message,
      data: env.data,
      envelope: env as unknown as Record<string, unknown>,
      encrypted: headers?.get?.(HEADER_ENCRYPTION) != null || isJweData(env.data)
    };
  }

  /** Backend-only exchange; verify callback state against the GM browser session first. */
  exchangeGMCode(code: string, codeVerifier: string, redirectUri: string): Promise<GMSession> {
    return this.call('POST', '/server/v1/gm/sso/exchange', undefined, { code, codeVerifier, redirectUri });
  }

  /** Supply exactly one of accessToken or uid; UID mode requires server configuration. */
  issueChatTicket(input: ChatTicketInput): Promise<ChatTicket> {
    return this.call('POST', '/server/v1/chat/tickets', undefined, input);
  }
  async pollChatRelay(): Promise<ChatRelayItem[]> {
    const out = await this.call<{ items: ChatRelayItem[] }>('GET', '/server/v1/chat/relay');
    return out.items ?? [];
  }
  /** failed means permanent failure; leave transient failures pending for redelivery. */
  async ackChatRelay(done: number[], failed: { id: number; error: string }[] = []): Promise<number> {
    const out = await this.call<{ updated: number }>('POST', '/server/v1/chat/relay/ack', undefined, { done, failed });
    return out.updated;
  }
  postChatMessage(input: ChatPost): Promise<Record<string, unknown>> {
    return this.call('POST', '/server/v1/chat/messages', undefined, input);
  }

  /** Full whitelist for this server. */
  whitelist(): Promise<Whitelist> {
    return this.call('GET', '/server/v1/whitelist');
  }

  /** Join check by name and/or uuid. */
  checkWhitelist(name?: string, uuid?: string): Promise<CheckResult> {
    return this.call('GET', '/server/v1/whitelist/check', { name, uuid });
  }

  /**
   * Report live status (about every 60s). Pass a Heartbeat object to add the optional tps and
   * players (PROTOCOL §2.3); a tps that is not a finite number > 0 is left out.
   */
  heartbeat(status: Heartbeat): Promise<void>;
  heartbeat(online: number, max?: number, version?: string): Promise<void>;
  heartbeat(online: number | Heartbeat, max = 0, version = ''): Promise<void> {
    const hb: Heartbeat = typeof online === 'number' ? { online, max, version } : online;
    const body: Record<string, unknown> = { online: hb.online, max: hb.max ?? 0, version: hb.version ?? '' };
    if (typeof hb.tps === 'number' && Number.isFinite(hb.tps) && hb.tps > 0) body.tps = hb.tps;
    if (hb.players?.length) body.players = hb.players.map(p => (p.uuid ? { name: p.name, uuid: p.uuid } : { name: p.name }));
    return this.call('POST', '/server/v1/heartbeat', undefined, body);
  }

  // ---- Feature modules / GM channel (PROTOCOL §2.4) ----

  /** Modules enabled for this server, keyed by module name, with their per-server config. */
  async modules(): Promise<Record<string, unknown>> {
    const d = await this.call<{ modules?: Record<string, unknown> }>('GET', '/server/v1/modules');
    return d.modules ?? {};
  }

  /** Replace the GM action catalogue shown in the admin panel (call on start-up and on change). */
  publishGMActions(actions: GMAction[]): Promise<GMPublishResult> {
    return this.publish('/server/v1/gm/actions', actions);
  }

  /**
   * Replace the actions and the named option lists (select / suggestion sources such as worlds or
   * kits) in one call. Null actions or omitted options keep the stored values; empty collections clear them. Entries the server cannot accept
   * are skipped and listed, never a failure for the whole call.
   */
  publishGMCatalog(actions: GMAction[] | null, options?: Record<string, GMOption[]>): Promise<GMPublishResult> {
    return this.publish('/server/v1/gm/actions', {
      ...(actions === null ? {} : { actions }), ...(options === undefined ? {} : { options }),
    });
  }

  /** Replace only the named option lists (the published actions stay), e.g. when a world loads. */
  publishGMOptions(options: Record<string, GMOption[]>): Promise<GMPublishResult> {
    return this.publish('/server/v1/gm/options', options);
  }

  private async publish(path: string, body: unknown): Promise<GMPublishResult> {
    const d = await this.call<Partial<GMPublishResult>>('PUT', path, undefined, body);
    return { count: d.count ?? 0, skipped: d.skipped ?? [] };
  }

  /** Fetch queued GM commands (each becomes "sent"); poll alongside the heartbeat. */
  pollGMCommands(): Promise<GMCommand[]> {
    return this.call('GET', '/server/v1/gm/commands');
  }

  /** Report how a command went; result is free text (<= 4000 chars). */
  reportGMResult(id: number, ok: boolean, result = ''): Promise<{ id: number; status: string }> {
    return this.call('POST', `/server/v1/gm/commands/${id}/result`, undefined, { ok, result });
  }

  // ---- Player data and per-server bans (SERVER_USERS.md) ----

  /** The data schema the server owner defined (needs the module on for this server). */
  userSchema(): Promise<UserSchema> {
    return this.call('GET', '/server/v1/users/schema');
  }

  /** One account's data (uid from the join check or the whitelist). */
  async userData(uid: number): Promise<UserData> {
    return this.call('GET', userPath(uid));
  }

  /**
   * Replace the account's stored values (fields left out are unset). Pass the version you last
   * read to refuse a concurrent change (ApiError 409, see isVersionConflict). Returns the stored data.
   */
  async putUserData(uid: number, data: Record<string, unknown>, opts: WriteOptions = {}): Promise<UserData> {
    return this.call('PUT', userPath(uid), undefined, writeBody(data, opts));
  }

  /** Merge values into the stored ones; a null value deletes that field. */
  async patchUserData(uid: number, data: Record<string, unknown>, opts: WriteOptions = {}): Promise<UserData> {
    return this.call('PATCH', userPath(uid), undefined, writeBody(data, opts));
  }

  /**
   * Active bans on this server: all of them (at most 5000, newest first), or those of the accounts
   * matching the target. Works with the module off too.
   */
  async bans(only: BanTarget = {}): Promise<ServerBan[]> {
    const q: Record<string, string | undefined> = { uid: only.uid ? String(only.uid) : undefined, name: only.name, uuid: only.uuid };
    const d = await this.call<{ bans?: ServerBan[] }>('GET', '/server/v1/bans', q);
    return d.bans ?? [];
  }

  /** Online players (as of the last heartbeat) whose account is banned here: disconnect them. */
  async kickList(): Promise<KickEntry[]> {
    const d = await this.call<{ players?: KickEntry[] }>('GET', '/server/v1/bans/kick');
    return d.players ?? [];
  }

  /**
   * Ban an account on this server (module on and its pluginBans setting allowed). A ban an
   * administrator placed cannot be replaced (ApiError 403).
   */
  async ban(target: BanTarget, opts: BanOptions = {}): Promise<{ uid: number; ban: ServerBan | null }> {
    const d = await this.call<{ uid: number; ban?: ServerBan | null }>('POST', '/server/v1/bans', undefined, { ...target, ...opts });
    return { uid: d.uid, ban: d.ban ?? null };
  }

  /** Lift the account's active ban on this server (not one an administrator placed). */
  unban(target: BanTarget, reason?: string): Promise<{ uid: number; lifted: boolean }> {
    return this.call('POST', '/server/v1/bans/lift', undefined, reason ? { ...target, reason } : { ...target });
  }
}

function userPath(uid: number): string {
  if (!Number.isSafeInteger(uid) || uid <= 0) throw new TypeError(`nowork: uid must be a positive integer, got ${uid}`);
  return `/server/v1/users/${uid}/data`;
}

function writeBody(data: Record<string, unknown>, opts: WriteOptions): Record<string, unknown> {
  // the server wants a data object, even an empty one
  return opts.version === undefined ? { data: data ?? {} } : { data: data ?? {}, version: opts.version };
}

/** Live status for heartbeat(); tps and players are optional (PROTOCOL §2.3). */
export interface Heartbeat {
  online: number;
  max?: number;
  version?: string;
  /** Current ticks per second; left out unless a finite number > 0. */
  tps?: number;
  /** Players online now (name required, uuid optional); should match online. */
  players?: OnlinePlayer[];
}

/** One entry of the heartbeat player list. */
export interface OnlinePlayer {
  name: string;
  uuid?: string;
}

/** One choice of a select parameter or of a named option list. */
export interface GMOption {
  value: string;
  label?: string;
}

/**
 * One input of a GM action. Only name / type / required / label existed before 2026-09-28; every
 * other field is optional (PROTOCOL §2.4). Aliases int / integer / bool / textarea are accepted too.
 */
export interface GMActionParam {
  name: string;
  type: 'string' | 'text' | 'number' | 'boolean' | 'select' | 'player' | 'int' | 'integer' | 'bool' | 'textarea';
  required?: boolean;
  label?: string;
  description?: string;
  placeholder?: string;
  /** Ignored by the server when it does not fit the definition. */
  default?: string | number | boolean;
  /** number only */
  min?: number;
  max?: number;
  step?: number;
  integer?: boolean;
  /** Text length limit (server default 1000, text 4000, player 64). */
  maxLength?: number;
  /** Inline choices of a select (1-200, unique values). */
  options?: GMOption[];
  /** Names an option list published with publishGMCatalog / publishGMOptions. */
  source?: string;
}

/** A GM action the plugin offers to the admin panel. */
export interface GMAction {
  /** No colon, no whitespace, at most 64 bytes. */
  key: string;
  title: string;
  description?: string;
  params?: GMActionParam[];
  labels?: Record<string, string>;
  /** Catalogue group. */
  category?: string;
  /** confirm / critical need a confirmation (critical: typing the key) before the command is sent. */
  danger?: 'normal' | 'confirm' | 'critical';
}

/** What a publish stored; entries the server could not accept are skipped and listed. */
export interface GMPublishResult {
  count: number;
  skipped: { key: string; reason: string }[];
}

/** A queued GM command the admin panel wants the plugin to run. */
export interface GMCommand {
  id: number;
  action: string;
  params: Record<string, unknown>;
  status: string;
  /** admin | owner | grant */
  source?: string;
  /** "staff" for admin commands on owner-run servers, the login name otherwise. */
  operator?: string;
}

export interface TokenResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  refresh_token?: string;
  id_token?: string;
  scope: string;
}

export interface UserInfo {
  sub: string;
  uid: number;
  name?: string;
  nickname?: string;
  preferred_username?: string;
  picture?: string;
  alias?: string;
  email?: string;
  email_verified?: boolean;
  phone_number?: string;
  phone_number_verified?: boolean;
  security_level?: number;
  realname_verified?: boolean;
  game_roles?: { server: string; server_name: string; name: string; uuid: string }[];
}

/** Discovery fields required by this client, with provider extensions retained. */
export interface OIDCMetadata {
  issuer: string;
  authorization_endpoint: string;
  token_endpoint: string;
  jwks_uri: string;
  [key: string]: unknown;
}

/** Standard JWK members and any provider-specific extensions. */
export type JWK = JsonWebKey & Record<string, unknown>;
export interface JWKS {
  keys: JWK[];
  [key: string]: unknown;
}

const OIDC_RESPONSE_LIMIT = 1024 * 1024;

function isOidcObject(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

/** Bound UTF-8 wire bytes before decoding; retain text/json-only fetch adapter support. */
async function readOidcDocument(response: Response, signal: AbortSignal): Promise<unknown> {
  const tooLarge = () => new TransportError(`OIDC response exceeds ${OIDC_RESPONSE_LIMIT}-byte limit`);
  const declared = response.headers?.get?.('Content-Length');
  if (declared && /^\d+$/.test(declared) && Number(declared) > OIDC_RESPONSE_LIMIT) {
    void response.body?.cancel().catch(() => {});
    throw tooLarge();
  }
  if (signal.aborted) {
    void response.body?.cancel(signal.reason).catch(() => {});
    throw signal.reason;
  }
  if (!response.body) {
    // Adapters without a stream cannot expose wire size before materializing their body.
    const text = typeof response.text === 'function' ? await response.text() : JSON.stringify(await response.json());
    if (typeof text !== 'string') throw new TransportError('invalid OIDC JSON');
    if (text.length > OIDC_RESPONSE_LIMIT || new TextEncoder().encode(text).byteLength > OIDC_RESPONSE_LIMIT) throw tooLarge();
    return JSON.parse(text);
  }
  const reader = response.body.getReader();
  const cancel = () => { void reader.cancel(signal.reason).catch(() => {}); };
  signal.addEventListener('abort', cancel, { once: true });
  const decoder = new TextDecoder('utf-8', { fatal: true });
  const chunks: string[] = [];
  let size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (signal.aborted) throw signal.reason;
      if (done) break;
      size += value.byteLength;
      if (size > OIDC_RESPONSE_LIMIT) throw tooLarge();
      chunks.push(decoder.decode(value, { stream: true }));
    }
    chunks.push(decoder.decode());
    return JSON.parse(chunks.join(''));
  } catch (error) {
    void reader.cancel(error).catch(() => {});
    throw error;
  } finally {
    signal.removeEventListener('abort', cancel);
    reader.releaseLock();
  }
}

/** Minimal OIDC relying-party helper. */
export class NoworkOIDC {
  private readonly issuer: string;

  constructor(
    issuer: string,
    private readonly clientId: string,
    private readonly clientSecret = '',
    private readonly redirectUri = '',
    private readonly f: typeof fetch = fetch,
    private readonly timeoutMs = 10_000
  ) {
    this.issuer = issuer.replace(/\/+$/, '');
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0 || timeoutMs > 2_147_483_647) {
      throw new RangeError('timeoutMs must be a positive 32-bit integer');
    }
  }

  private async request<T>(operation: (signal: AbortSignal) => Promise<T>, signal?: AbortSignal): Promise<T> {
    const ctrl = new AbortController();
    const forwardAbort = () => ctrl.abort(signal?.reason);
    signal?.addEventListener('abort', forwardAbort, { once: true });
    const timer = setTimeout(() => ctrl.abort(), this.timeoutMs);
    let onAbort: () => void = () => {};
    try {
      if (signal?.aborted) forwardAbort();
      if (ctrl.signal.aborted) throw ctrl.signal.reason;
      const aborted = new Promise<never>((_resolve, reject) => {
        onAbort = () => reject(ctrl.signal.reason);
        ctrl.signal.addEventListener('abort', onAbort, { once: true });
      });
      return await Promise.race([operation(ctrl.signal), aborted]);
    } catch (error) {
      if (error instanceof TransportError || error instanceof OAuthError) throw error;
      throw new TransportError(String(error), error);
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', forwardAbort);
      ctrl.signal.removeEventListener('abort', onAbort);
    }
  }

  private getDocument(path: string, signal?: AbortSignal): Promise<Record<string, unknown>> {
    return this.request(async requestSignal => {
      // URL accepts and normalizes some malformed inputs. Check the literal issuer
      // first, then use URL only for validation; never replace the configured value.
      try {
        const authority = /^https?:\/\/([^/]+)/i.exec(this.issuer)?.[1];
        if (!authority || authority.includes('@') || /[\u0000-\u0020\u007f\\?#]/.test(this.issuer)) {
          throw new Error('invalid issuer');
        }
        const configured = new URL(this.issuer);
        if (!configured.hostname || configured.username || configured.password) throw new Error('invalid issuer');
      } catch {
        // URL parser errors can contain the configured URL, including credentials.
        throw new TransportError('invalid metadata issuer URL');
      }
      const res = await this.f(this.issuer + path, {
        method: 'GET', headers: { Accept: 'application/json' },
        credentials: 'omit', redirect: 'error', signal: requestSignal,
      });
      if (res.status < 200 || res.status >= 300 || res.redirected) {
        void res.body?.cancel().catch(() => {});
        throw new TransportError(`http ${res.status}${res.redirected ? ' (redirected)' : ''}`);
      }
      const document = await readOidcDocument(res, requestSignal);
      if (!isOidcObject(document)) throw new TransportError('invalid OIDC JSON object');
      return document;
    }, signal);
  }

  /** Read only the configured issuer's fixed discovery path; never follow advertised URLs. */
  async discover(signal?: AbortSignal): Promise<OIDCMetadata> {
    const document = await this.getDocument('/.well-known/openid-configuration', signal);
    for (const field of ['issuer', 'authorization_endpoint', 'token_endpoint', 'jwks_uri']) {
      if (typeof document[field] !== 'string' || document[field].trim().length === 0) {
        throw new TransportError(`incomplete OIDC discovery: ${field}`);
      }
    }
    if (document.issuer !== this.issuer) throw new TransportError('OIDC discovery issuer mismatch');
    return document as OIDCMetadata;
  }

  /** Read only the configured issuer's /jwks.json; preserve every JWK extension. */
  async jwks(signal?: AbortSignal): Promise<JWKS> {
    const document = await this.getDocument('/jwks.json', signal);
    if (!Array.isArray(document.keys) || !document.keys.every(isOidcObject)) {
      throw new TransportError('invalid JWKS: keys must be an array of objects');
    }
    return document as unknown as JWKS;
  }

  authorizeUrl(state: string, nonce?: string, scope = 'openid profile email', codeChallenge?: string): string {
    const u = new URL(this.issuer + '/oauth2/authorize');
    u.searchParams.set('response_type', 'code');
    u.searchParams.set('client_id', this.clientId);
    u.searchParams.set('redirect_uri', this.redirectUri);
    u.searchParams.set('scope', scope);
    u.searchParams.set('state', state);
    if (nonce) u.searchParams.set('nonce', nonce);
    if (codeChallenge) {
      u.searchParams.set('code_challenge', codeChallenge);
      u.searchParams.set('code_challenge_method', 'S256');
    }
    return u.toString();
  }

  private async post<T>(path: string, form: Record<string, string | undefined>): Promise<T> {
    return this.request(async signal => {
      const body = new URLSearchParams();
      for (const [k, v] of Object.entries(form)) {
        if (v) body.set(k, v);
      }
      body.set('client_id', this.clientId);
      if (this.clientSecret) body.set('client_secret', this.clientSecret);
      const res = await this.f(this.issuer + path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
        body, signal
      });
      const text = await res.text();
      let json: any = null;
      if (text) {
        try {
          json = JSON.parse(text);
        } catch {
          json = null;
        }
      }
      if (res.status >= 400) {
        if (json && typeof json === 'object' && json.error) {
          throw new OAuthError(String(json.error), json.error_description ?? '', res.status);
        }
        throw new TransportError(`http ${res.status}`);
      }
      if (!text) {
        // only revocation may answer with an empty body
        if (path === '/oauth2/revoke') return {} as T;
        throw new TransportError(`empty response (${res.status})`);
      }
      if (!json || typeof json !== 'object') throw new TransportError(`invalid json (${res.status})`);
      return json as T;
    });
  }

  exchange(code: string, codeVerifier?: string): Promise<TokenResponse> {
    return this.post('/oauth2/token', {
      grant_type: 'authorization_code',
      code,
      redirect_uri: this.redirectUri,
      code_verifier: codeVerifier
    });
  }

  refresh(refreshToken: string): Promise<TokenResponse> {
    return this.post('/oauth2/token', { grant_type: 'refresh_token', refresh_token: refreshToken });
  }

  clientCredentials(scope = ''): Promise<TokenResponse> {
    return this.post('/oauth2/token', { grant_type: 'client_credentials', scope });
  }

  introspect(token: string): Promise<{ active: boolean; [k: string]: unknown }> {
    return this.post('/oauth2/introspect', { token });
  }

  async revoke(token: string): Promise<void> {
    await this.post('/oauth2/revoke', { token });
  }

  async userInfo(accessToken: string): Promise<UserInfo> {
    return this.request(async signal => {
      const res = await this.f(this.issuer + '/oauth2/userinfo', {
        headers: { Authorization: `Bearer ${accessToken}`, Accept: 'application/json' }, signal
      });
      let json: any;
      try {
        json = await res.json();
      } catch (e) {
        throw new TransportError(`invalid json (${res.status})`, e);
      }
      if (res.status >= 400) {
        if (json && typeof json === 'object' && json.error) {
          throw new OAuthError(String(json.error), json.error_description ?? '', res.status);
        }
        throw new TransportError(`http ${res.status}`);
      }
      return json as UserInfo;
    });
  }

  logoutUrl(idToken?: string, postLogoutRedirectUri?: string, state?: string): string {
    const u = new URL(this.issuer + '/oauth2/logout');
    u.searchParams.set('client_id', this.clientId);
    if (idToken) u.searchParams.set('id_token_hint', idToken);
    if (postLogoutRedirectUri) u.searchParams.set('post_logout_redirect_uri', postLogoutRedirectUri);
    if (state) u.searchParams.set('state', state);
    return u.toString();
  }
}

/** PKCE helpers (Web Crypto). */
export async function pkce(): Promise<{ verifier: string; challenge: string }> {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  const verifier = base64url(bytes);
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
  return { verifier, challenge: base64url(new Uint8Array(digest)) };
}

function base64url(bytes: Uint8Array): string {
  let s = '';
  bytes.forEach((b) => (s += String.fromCharCode(b)));
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/* ---------------- Webhooks ---------------- */

export interface WebhookEvent<T = Record<string, unknown>> {
  id: string;
  event: string;
  timestamp: number;
  data: T;
}

export class WebhookError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'WebhookError';
  }
}

const hex = (buf: ArrayBuffer) => Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, '0')).join('');

/** sha256=HMAC_SHA256(secret, "<ts>.<body>") using Web Crypto (Node 18+, Deno, Bun, workers). */
export async function webhookSignature(secret: string, ts: number | string, body: string | Uint8Array): Promise<string> {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey('raw', enc.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
  const bodyBytes = typeof body === 'string' ? enc.encode(body) : body;
  const prefix = enc.encode(`${ts}.`);
  const msg = new Uint8Array(prefix.length + bodyBytes.length);
  msg.set(prefix, 0);
  msg.set(bodyBytes, prefix.length);
  return 'sha256=' + hex(await crypto.subtle.sign('HMAC', key, msg));
}

function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) return false;
  let diff = 0;
  for (let i = 0; i < a.length; i++) diff |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return diff === 0;
}

/**
 * Verify an incoming webhook. `headers` can be a Headers instance or a plain object (case-insensitive).
 * `toleranceSeconds` bounds replay (0 disables).
 */
export async function verifyWebhook<T = Record<string, unknown>>(
  secret: string,
  headers: Headers | Record<string, string | string[] | undefined>,
  body: string | Uint8Array,
  toleranceSeconds = 300
): Promise<WebhookEvent<T>> {
  const get = (name: string): string => {
    if (typeof (headers as Headers).get === 'function') return (headers as Headers).get(name) ?? '';
    const map = headers as Record<string, string | string[] | undefined>;
    const key = Object.keys(map).find((k) => k.toLowerCase() === name.toLowerCase());
    const v = key ? map[key] : undefined;
    return Array.isArray(v) ? v[0] ?? '' : v ?? '';
  };
  const ts = Number.parseInt(get('X-NoWork-Timestamp').trim(), 10);
  if (!Number.isFinite(ts)) throw new WebhookError('missing or invalid X-NoWork-Timestamp');
  if (toleranceSeconds > 0 && Math.abs(Date.now() / 1000 - ts) > toleranceSeconds) {
    throw new WebhookError('webhook timestamp outside tolerance');
  }
  const expected = await webhookSignature(secret, ts, body);
  if (!timingSafeEqual(expected, get('X-NoWork-Signature').trim())) throw new WebhookError('invalid webhook signature');
  const text = typeof body === 'string' ? body : new TextDecoder().decode(body);
  let payload: any;
  try {
    payload = JSON.parse(text);
  } catch {
    throw new WebhookError('invalid webhook body');
  }
  if (!payload || typeof payload !== 'object') throw new WebhookError('invalid webhook body');
  return { id: String(payload.id ?? ''), event: String(payload.event ?? ''), timestamp: Number(payload.timestamp ?? 0), data: (payload.data ?? {}) as T };
}

// SDK login page helpers (PROTOCOL.md §6).
export * from './sdklogin.js';
export * from './delivery.js';
export * from './serverusers.js';
export * from './account.js';
export * from './gamedata.js';
