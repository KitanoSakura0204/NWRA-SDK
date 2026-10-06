/** SDK login page helpers (PROTOCOL.md §6): PKCE, the login URL, and reading the result. */

/** Message type the login page posts in postMessage mode. */
export const SDK_LOGIN_MESSAGE = 'nowork:sdk-login';

function b64url(bytes: Uint8Array): string {
  let s = '';
  bytes.forEach((b) => (s += String.fromCharCode(b)));
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/** BASE64URL(SHA256(verifier)) without padding (RFC 7636 S256). */
export async function s256Challenge(verifier: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
  return b64url(new Uint8Array(digest));
}

/** A fresh 43-character verifier and its S256 challenge. Keep the verifier; send only the challenge. */
export async function createPkcePair(): Promise<{ verifier: string; challenge: string }> {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  const verifier = b64url(bytes);
  return { verifier, challenge: await s256Challenge(verifier) };
}

/** A random state value for one login attempt. */
export function createState(): string {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return b64url(bytes);
}

export interface LoginUrlParams {
  /** Game server code, e.g. "survival". */
  serverCode: string;
  /** Exact redirect URI registered for the server (optional in code-display mode). */
  redirectUri?: string;
  /** Echoed back unchanged (optional in code-display mode). */
  state?: string;
  /** S256 challenge (required). */
  codeChallenge: string;
  scope?: string;
  nonce?: string;
  /** zh-CN / zh-TW / en / ja */
  uiLocales?: string;
}

/** The URL a game client opens. `frontend` is the user-centre web address (the SPA origin). */
export function buildLoginUrl(frontend: string, p: LoginUrlParams): string {
  if (!p.serverCode) throw new Error('serverCode is required');
  if (!p.codeChallenge) throw new Error('codeChallenge is required (PKCE S256)');
  const u = new URL(`${frontend.replace(/\/+$/, '')}/sdk/login/${encodeURIComponent(p.serverCode)}`);
  const q: [string, string | undefined][] = [
    ['redirect_uri', p.redirectUri],
    ['state', p.state],
    ['scope', p.scope],
    ['nonce', p.nonce],
    ['ui_locales', p.uiLocales]
  ];
  q.forEach(([k, v]) => v && u.searchParams.set(k, v));
  u.searchParams.set('code_challenge', p.codeChallenge);
  u.searchParams.set('code_challenge_method', 'S256');
  return u.toString();
}

/** An error the login page handed back (access_denied, invalid_request …) or a failed check. */
export class SdkLoginError extends Error {
  constructor(public code: string, public description = '') {
    super(description ? `${code}: ${description}` : code);
    this.name = 'SdkLoginError';
  }
}

export interface SdkLoginResult {
  code: string;
  state: string;
  iss?: string;
}

function check(get: (k: string) => string | null | undefined, wantState: string, wantIssuer?: string): SdkLoginResult {
  const state = get('state') ?? '';
  if (state !== wantState) throw new SdkLoginError('state_mismatch');
  const iss = get('iss') ?? undefined;
  if (wantIssuer && iss && iss !== wantIssuer) throw new SdkLoginError('issuer_mismatch');
  const error = get('error');
  if (error) throw new SdkLoginError(error, get('error_description') ?? '');
  const code = get('code');
  if (!code) throw new SdkLoginError('missing_code');
  return { code, state, iss };
}

/** Redirect mode: reads the callback URL (mygame://auth?code=…&state=…&iss=…). */
export function parseLoginCallback(callbackUrl: string, wantState: string, wantIssuer?: string): SdkLoginResult {
  const q = new URL(callbackUrl).searchParams;
  return check((k) => q.get(k), wantState, wantIssuer);
}

/**
 * postMessage mode: pass every `message` event. Returns undefined for messages that are not the
 * login result (other origins, other types), so a foreign window cannot end the flow. `pageOrigin`
 * is the origin of the login page (the user-centre web address).
 */
export function readLoginMessage(event: { origin: string; data: unknown }, pageOrigin: string, wantState: string, wantIssuer?: string): SdkLoginResult | undefined {
  if (event.origin !== new URL(pageOrigin).origin) return undefined;
  const d = event.data as Record<string, unknown> | null;
  if (!d || typeof d !== 'object' || d.type !== SDK_LOGIN_MESSAGE) return undefined;
  return check((k) => (typeof d[k] === 'string' ? (d[k] as string) : undefined), wantState, wantIssuer);
}

/** Code-display mode: cleans what the player typed ("abcd-2345" → "ABCD2345"). Not for other modes. */
export function normalizeLoginCode(s: string): string {
  return s.replace(/[\s-]+/g, '').toUpperCase();
}
