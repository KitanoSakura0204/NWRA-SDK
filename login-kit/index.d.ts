export type LoginLocale = 'zh_CN' | 'zh_TW' | 'en' | 'ja';
export interface Tokens { access_token: string; token_type: string; expires_in: number; refresh_token?: string; id_token?: string; scope?: string }
export interface VerifiedClaims extends Record<string, unknown> { iss: string; sub: string; aud: string | string[]; exp: number; iat: number; nonce?: string }
export interface PublicJWK extends JsonWebKey { kid?: string }
export interface LoginResult { tokens: Readonly<Tokens>; claims: Readonly<VerifiedClaims> }
export interface Transaction { issuer: string; clientId: string; redirectUri: string; state: string; nonce: string; verifier: string; createdAt: number }
export interface LoginConfig {
  issuer: string;
  clientId: string;
  redirectUri: string;
  scope?: string;
  timeoutMs?: number;
  trustedEndpointOrigins?: string[];
  authorizationParams?: Partial<Record<'prompt' | 'login_hint' | 'ui_locales' | 'max_age', string>>;
  fetch?: typeof fetch;
}
export class LoginError extends Error { readonly code: string; constructor(code: string, message?: string) }
export function randomValue(): string;
export function base64url(bytes: ArrayBuffer | ArrayBufferView): string;
export function pkceChallenge(verifier: string): Promise<string>;
export function decodeJWTUnverified(raw: string): { verified: false; header: Record<string, unknown>; claims: Record<string, unknown> };
export function parseAuthorizationResponse(raw: string, transaction: Transaction): string;
export function verifyIdToken(raw: string, options: { issuer: string; clientId: string; nonce: string; jwks: { keys: PublicJWK[] }; accessToken?: string; subject?: string; refresh?: boolean }): Promise<Readonly<VerifiedClaims>>;
export function createLoginClient(config: LoginConfig): {
  loginPopup(): Promise<LoginResult>;
  loginRedirect(): Promise<void>;
  completeRedirect(raw?: string): Promise<LoginResult>;
  userinfo(): Promise<Record<string, unknown>>;
  refresh(): Promise<LoginResult>;
  revoke(): Promise<void>;
  clear(): void;
};
export function completePopupCallback(): void;
export function createLoginButton(options?: { brand?: string; locale?: LoginLocale; logoUrl?: string; onClick?: (event: MouseEvent) => void }): HTMLButtonElement;
