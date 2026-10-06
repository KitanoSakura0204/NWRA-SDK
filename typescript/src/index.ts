/** NoWork Account SDK — Server API (whitelist / heartbeat) and OIDC helpers. Requires global fetch (Node 18+). */

import type { BanOptions, BanTarget, KickEntry, ServerBan, UserData, UserSchema, WriteOptions } from './serverusers.js';

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
 * A Server API reply was encrypted and could not be used (no decryption key configured). The data
 * was never read: treat it as a configuration error, never fail open on it (an unreadable
 * whitelist is not a disabled whitelist).
 */
export class EncryptionError extends Error {
  constructor(
    public route: string,
    public reason: string
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
  /** X-NoWork-Encryption is set or data is exactly {"jwe": "<string>"}. */
  encrypted: boolean;
}

function isJweData(data: unknown): boolean {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return false;
  const keys = Object.keys(data);
  return keys.length === 1 && typeof (data as Record<string, unknown>).jwe === 'string';
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

export interface NoworkClientOptions {
  serverKey: string;
  timeoutMs?: number;
  fetch?: typeof fetch;
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

  constructor(base: string, opts: NoworkClientOptions) {
    this.base = base.replace(/\/+$/, '');
    this.key = opts.serverKey;
    this.timeout = opts.timeoutMs ?? 10_000;
    this.f = opts.fetch ?? fetch;
  }

  /**
   * Call a Server API route (e.g. '/server/v1/whitelist') and return the reply without decoding
   * data. Transport failures and business errors (code != 0) still throw.
   */
  raw(method: string, path: string, query?: Record<string, string | undefined>, body?: unknown): Promise<RawResponse> {
    return this.send(method, path, query, body);
  }

  /** Decodes data; an encrypted reply throws EncryptionError instead (fail closed). */
  private async call<T>(
    method: string,
    path: string,
    query?: Record<string, string | undefined>,
    body?: unknown
  ): Promise<T> {
    const res = await this.send(method, path, query, body);
    if (res.encrypted) throw new EncryptionError(path.replace(/^\/server\/v1\//, ''), 'no decryption key configured');
    if (res.data === undefined || res.data === null) {
      throw new TransportError(`invalid response (${res.status}): missing data`);
    }
    return res.data as T;
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
      env = (await res.json()) as Envelope<unknown>;
    } catch (e) {
      throw new TransportError(`invalid json (${res.status})`, e);
    } finally {
      clearTimeout(timer);
    }
    if (!env || typeof env !== 'object' || typeof env.code !== 'number') {
      throw new TransportError(`invalid response (${res.status}): missing code`);
    }
    if (env.code !== 0) throw new ApiError(env.code, env.message ?? '');
    if (res.status >= 400) throw new TransportError(`http ${res.status}`);
    const headers: Headers | undefined = res.headers;
    return {
      status: res.status,
      headers,
      message: env.message ?? '',
      data: env.data,
      envelope: env as unknown as Record<string, unknown>,
      encrypted: !!headers?.get?.(HEADER_ENCRYPTION) || isJweData(env.data)
    };
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
   * kits) in one call. Omitting options keeps the stored lists. Entries the server cannot accept
   * are skipped and listed, never a failure for the whole call.
   */
  publishGMCatalog(actions: GMAction[], options?: Record<string, GMOption[]>): Promise<GMPublishResult> {
    return this.publish('/server/v1/gm/actions', options ? { actions, options } : { actions });
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

/** Minimal OIDC relying-party helper. */
export class NoworkOIDC {
  private readonly issuer: string;

  constructor(
    issuer: string,
    private readonly clientId: string,
    private readonly clientSecret = '',
    private readonly redirectUri = '',
    private readonly f: typeof fetch = fetch
  ) {
    this.issuer = issuer.replace(/\/+$/, '');
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
    const body = new URLSearchParams();
    for (const [k, v] of Object.entries(form)) {
      if (v) body.set(k, v);
    }
    body.set('client_id', this.clientId);
    if (this.clientSecret) body.set('client_secret', this.clientSecret);
    let res: Response;
    try {
      res = await this.f(this.issuer + path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded', Accept: 'application/json' },
        body
      });
    } catch (e) {
      throw new TransportError(String(e), e);
    }
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
    let res: Response;
    try {
      res = await this.f(this.issuer + '/oauth2/userinfo', {
        headers: { Authorization: `Bearer ${accessToken}`, Accept: 'application/json' }
      });
    } catch (e) {
      throw new TransportError(String(e), e);
    }
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
