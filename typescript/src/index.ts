/** Lounge Account SDK — Server API (whitelist / heartbeat) and OIDC helpers. Requires global fetch (Node 18+). */

export class TransportError extends Error {
  constructor(
    message: string,
    public cause?: unknown
  ) {
    super(`lounge: transport error: ${message}`);
    this.name = 'TransportError';
  }
}

export class ApiError extends Error {
  constructor(
    public code: number,
    message: string
  ) {
    super(`lounge: api error ${code}: ${message}`);
    this.name = 'ApiError';
  }
}

export class OAuthError extends Error {
  constructor(
    public error: string,
    public description = '',
    public status = 0
  ) {
    super(`lounge: oauth error ${error}: ${description}`);
    this.name = 'OAuthError';
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
  player?: WhitelistEntry | null;
}

export interface LoungeClientOptions {
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
export class LoungeClient {
  private readonly base: string;
  private readonly key: string;
  private readonly timeout: number;
  private readonly f: typeof fetch;

  constructor(base: string, opts: LoungeClientOptions) {
    this.base = base.replace(/\/+$/, '');
    this.key = opts.serverKey;
    this.timeout = opts.timeoutMs ?? 10_000;
    this.f = opts.fetch ?? fetch;
  }

  private async call<T>(
    method: string,
    path: string,
    query?: Record<string, string | undefined>,
    body?: unknown
  ): Promise<T> {
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
    let env: Envelope<T>;
    try {
      // The timeout covers reading the body as well, so the timer is only cleared afterwards.
      env = (await res.json()) as Envelope<T>;
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
    if (env.data === undefined || env.data === null) {
      throw new TransportError(`invalid response (${res.status}): missing data`);
    }
    return env.data as T;
  }

  /** Full whitelist for this server. */
  whitelist(): Promise<Whitelist> {
    return this.call('GET', '/server/v1/whitelist');
  }

  /** Join check by name and/or uuid. */
  checkWhitelist(name?: string, uuid?: string): Promise<CheckResult> {
    return this.call('GET', '/server/v1/whitelist/check', { name, uuid });
  }

  /** Report live status (about every 60s). */
  heartbeat(online: number, max = 0, version = ''): Promise<void> {
    return this.call('POST', '/server/v1/heartbeat', undefined, { online, max, version });
  }
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
export class LoungeOIDC {
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
  const ts = Number.parseInt(get('X-Lounge-Timestamp').trim(), 10);
  if (!Number.isFinite(ts)) throw new WebhookError('missing or invalid X-Lounge-Timestamp');
  if (toleranceSeconds > 0 && Math.abs(Date.now() / 1000 - ts) > toleranceSeconds) {
    throw new WebhookError('webhook timestamp outside tolerance');
  }
  const expected = await webhookSignature(secret, ts, body);
  if (!timingSafeEqual(expected, get('X-Lounge-Signature').trim())) throw new WebhookError('invalid webhook signature');
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
