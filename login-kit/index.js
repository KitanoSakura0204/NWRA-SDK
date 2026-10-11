/** NoWork browser login kit. Tokens live in memory; only pending redirect transactions use sessionStorage. */
const CALLBACK_TYPE = 'nowork.oidc.callback.v1';
const TRANSACTION_TTL = 5 * 60 * 1000;
const encoder = new TextEncoder();
const decoder = new TextDecoder('utf-8', { fatal: true });
const labels = {
  zh_CN: '使用 {brand} 登录',
  zh_TW: '使用 {brand} 登入',
  en: 'Sign in with {brand}',
  ja: '{brand} でログイン'
};

export class LoginError extends Error {
  constructor(code, message = code) {
    super(message);
    this.name = 'LoginError';
    this.code = code;
  }
}

function fail(code) { throw new LoginError(code); }
function cryptoAPI() {
  const api = globalThis.crypto;
  if (!api?.subtle || !api.getRandomValues) fail('secure_context_required');
  return api;
}

export function base64url(bytes) {
  let raw = '';
  for (const byte of new Uint8Array(bytes)) raw += String.fromCharCode(byte);
  return btoa(raw).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function fromBase64url(value) {
  if (!/^[A-Za-z0-9_-]+$/.test(value) || value.length % 4 === 1) fail('invalid_jwt');
  try {
    const raw = atob(value.replace(/-/g, '+').replace(/_/g, '/'));
    const result = Uint8Array.from(raw, (c) => c.charCodeAt(0));
    if (base64url(result) !== value) fail('invalid_jwt');
    return result;
  } catch { fail('invalid_jwt'); }
}

export function randomValue() {
  return base64url(cryptoAPI().getRandomValues(new Uint8Array(32)));
}

export async function pkceChallenge(verifier) {
  if (!/^[A-Za-z0-9._~-]{43,128}$/.test(verifier)) fail('invalid_code_verifier');
  return base64url(await cryptoAPI().subtle.digest('SHA-256', encoder.encode(verifier)));
}

function safeURL(raw, { query = true } = {}) {
  let url;
  try { url = new URL(raw); } catch { fail('invalid_url'); }
  const loopback = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname);
  if (url.username || url.password || url.hash || (!query && url.search) ||
      (url.protocol !== 'https:' && !(url.protocol === 'http:' && loopback))) fail('insecure_url');
  return url;
}

function callbackURLMatches(candidate, expected) {
  return candidate.origin === expected.origin && candidate.pathname === expected.pathname;
}

function single(params, key) {
  const all = params.getAll(key);
  if (all.length > 1) fail('duplicate_response_parameter');
  return all[0] || '';
}

/** Decoding is inspection only: this function never establishes an identity or verifies a JWT. */
export function decodeJWTUnverified(raw) {
  if (typeof raw !== 'string' || raw.length > 32768) fail('invalid_jwt');
  const parts = raw.split('.');
  if (parts.length !== 3) fail('invalid_jwt');
  try {
    const header = JSON.parse(decoder.decode(fromBase64url(parts[0])));
    const claims = JSON.parse(decoder.decode(fromBase64url(parts[1])));
    if (!header || typeof header !== 'object' || Array.isArray(header) ||
        !claims || typeof claims !== 'object' || Array.isArray(claims)) fail('invalid_jwt');
    return { verified: false, header, claims };
  } catch { fail('invalid_jwt'); }
}

export function parseAuthorizationResponse(raw, transaction) {
  if (!transaction || !Number.isFinite(transaction.createdAt) || Date.now() - transaction.createdAt > TRANSACTION_TTL ||
      transaction.createdAt > Date.now() + 1000) fail('transaction_expired');
  const url = new URL(raw);
  const expected = safeURL(transaction.redirectUri, { query: false });
  if (!callbackURLMatches(url, expected) || url.username || url.password || url.hash) fail('callback_mismatch');
  const state = single(url.searchParams, 'state');
  if (!state || state !== transaction.state) fail('state_mismatch');
  const issuer = single(url.searchParams, 'iss');
  if (issuer && issuer !== transaction.issuer) fail('issuer_mismatch');
  const error = single(url.searchParams, 'error');
  const code = single(url.searchParams, 'code');
  if (error && code) fail('invalid_authorization_response');
  if (error) throw new LoginError(error);
  if (!code || code.length > 4096) fail('missing_authorization_code');
  return code;
}

/** Verify RS256 signature and OIDC claims against a pinned issuer and the transaction nonce. */
export async function verifyIdToken(raw, options) {
  const decoded = decodeJWTUnverified(raw);
  const { header, claims } = decoded;
  // No algorithm negotiation based on attacker-controlled token data.
  if (header.alg !== 'RS256' || typeof header.kid !== 'string' || !header.kid || header.crit || header.jku || header.jwk || header.x5u ||
      (header.typ !== undefined && header.typ !== 'JWT') || (claims.token_use !== undefined && claims.token_use !== 'id')) fail('unsupported_id_token');
  const keys = options.jwks?.keys;
  if (!Array.isArray(keys) || keys.length > 32) fail('invalid_jwks');
  const matches = keys.filter((key) => key && key.kid === header.kid && key.kty === 'RSA' &&
    (!key.alg || key.alg === 'RS256') && (!key.use || key.use === 'sig') &&
    (!key.key_ops || (Array.isArray(key.key_ops) && key.key_ops.includes('verify'))) &&
    !['d', 'p', 'q', 'dp', 'dq', 'qi', 'oth'].some((field) => field in key));
  if (matches.length !== 1) fail('signing_key_not_found');
  const jwk = matches[0];
  if (typeof jwk.n !== 'string' || fromBase64url(jwk.n).length < 256 || fromBase64url(jwk.n).length > 512 || typeof jwk.e !== 'string') fail('invalid_signing_key');
  let valid = false;
  try {
    const key = await cryptoAPI().subtle.importKey('jwk', jwk, { name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' }, false, ['verify']);
    const parts = raw.split('.');
    valid = await cryptoAPI().subtle.verify('RSASSA-PKCS1-v1_5', key, fromBase64url(parts[2]), encoder.encode(`${parts[0]}.${parts[1]}`));
  } catch { fail('invalid_id_token_signature'); }
  if (!valid) fail('invalid_id_token_signature');
  const now = Math.floor(Date.now() / 1000);
  const audience = Array.isArray(claims.aud) ? claims.aud : [claims.aud];
  if (claims.iss !== options.issuer || !audience.length || audience.some((a) => typeof a !== 'string') || !audience.includes(options.clientId)) fail('id_token_audience_mismatch');
  if ((audience.length > 1 || claims.azp !== undefined) && claims.azp !== options.clientId) fail('id_token_audience_mismatch');
  if (typeof claims.sub !== 'string' || !claims.sub ||
      !Number.isFinite(claims.exp) || claims.exp <= now ||
      !Number.isFinite(claims.iat) || claims.iat > now + 60 || claims.exp <= claims.iat ||
      (claims.nbf !== undefined && (!Number.isFinite(claims.nbf) || claims.nbf > now + 60))) fail('invalid_id_token_claims');
  if (!options.nonce || (claims.nonce !== options.nonce && !(options.refresh && claims.nonce === undefined))) fail('nonce_mismatch');
  if (options.subject && claims.sub !== options.subject) fail('subject_mismatch');
  if (claims.at_hash !== undefined) {
    if (!options.accessToken) fail('access_token_hash_mismatch');
    const digest = new Uint8Array(await cryptoAPI().subtle.digest('SHA-256', encoder.encode(options.accessToken)));
    if (claims.at_hash !== base64url(digest.slice(0, digest.length / 2))) fail('access_token_hash_mismatch');
  }
  return Object.freeze({ ...claims });
}

async function limitedResponseText(response, maxBytes) {
  const advertised = Number(response.headers.get('Content-Length') || 0);
  if (advertised > maxBytes) fail('response_too_large');
  if (!response.body?.getReader) {
    const body = await response.text();
    if (encoder.encode(body).byteLength > maxBytes) fail('response_too_large');
    return body;
  }
  const reader = response.body.getReader();
  const chunks = [];
  let length = 0;
  try {
    for (;;) {
      const part = await reader.read();
      if (part.done) break;
      length += part.value.byteLength;
      if (length > maxBytes) { await reader.cancel(); fail('response_too_large'); }
      chunks.push(part.value);
    }
  } finally { reader.releaseLock(); }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
  try { return decoder.decode(bytes); } catch { fail('invalid_json_response'); }
}

async function jsonResponse(response, maxBytes = 65536) {
  if (!response.ok) {
    let code = 'http_error';
    try {
      const error = JSON.parse(await limitedResponseText(response, maxBytes));
      if (typeof error.error === 'string' && /^[a-z_]{1,64}$/.test(error.error)) code = error.error;
    } catch { /* Never surface server response bodies, which may contain credentials. */ }
    throw new LoginError(code);
  }
  const body = await limitedResponseText(response, maxBytes);
  try { return JSON.parse(body); } catch { fail('invalid_json_response'); }
}

/** The built-in transport is for public clients. Confidential clients exchange codes on their server. */
export function createLoginClient(config) {
  const browser = globalThis.window;
  if (!browser) fail('browser_required');
  safeURL(config.issuer, { query: false });
  const issuer = config.issuer;
  if (typeof config.clientId !== 'string' || !config.clientId.trim()) fail('client_id_required');
  const redirectUri = safeURL(config.redirectUri, { query: false }).href;
  if (new URL(redirectUri).origin !== browser.location.origin) fail('callback_origin_mismatch');
  const clientId = config.clientId;
  const scopes = [...new Set(['openid', ...(config.scope || 'profile').split(/\s+/).filter(Boolean)])];
  const endpointOrigins = new Set([new URL(issuer).origin, ...(config.trustedEndpointOrigins || [])]);
  for (const origin of endpointOrigins) if (safeURL(origin, { query: false }).origin !== origin) fail('invalid_trusted_origin');
  const fetcher = config.fetch || globalThis.fetch.bind(globalThis);
  const timeout = Math.max(1000, Math.min(config.timeoutMs || TRANSACTION_TTL, TRANSACTION_TTL));
  const storageKey = `nowork.oidc.transaction:${issuer}:${clientId}:${redirectUri}`;
  let discovery;
  let jwks;
  let jwksAt = 0;
  let current = null;
  let running = false;
  let operationEpoch = 0;

  function endpoint(value) {
    const url = safeURL(value);
    if (!endpointOrigins.has(url.origin)) fail('untrusted_endpoint_origin');
    return url.href;
  }

  async function requestJSON(url, init = {}) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 15000);
    try {
      return await jsonResponse(await fetcher(endpoint(url), {
        ...init, signal: controller.signal, credentials: 'omit', redirect: 'error', cache: 'no-store',
        headers: { Accept: 'application/json', ...(init.headers || {}) }
      }));
    } catch (error) {
      if (error instanceof LoginError) throw error;
      throw new LoginError(error?.name === 'AbortError' ? 'request_timeout' : 'network_error');
    } finally { clearTimeout(timer); }
  }

  async function configuration() {
    if (discovery) return discovery;
    const document = await requestJSON(`${issuer.replace(/\/$/, '')}/.well-known/openid-configuration`);
    if (document.issuer !== issuer) fail('issuer_mismatch');
    if (!document.code_challenge_methods_supported?.includes('S256') ||
        !document.response_types_supported?.includes('code') ||
        !document.id_token_signing_alg_values_supported?.includes('RS256')) fail('unsupported_provider');
    for (const name of ['authorization_endpoint', 'token_endpoint', 'jwks_uri']) endpoint(document[name]);
    if (document.userinfo_endpoint) endpoint(document.userinfo_endpoint);
    if (document.revocation_endpoint) endpoint(document.revocation_endpoint);
    discovery = document;
    return document;
  }

  async function signingKeys(force = false) {
    const document = await configuration();
    if (force || !jwks || Date.now() - jwksAt > 300000) {
      jwks = await requestJSON(document.jwks_uri);
      jwksAt = Date.now();
    }
    return jwks;
  }

  async function checkedTokens(value, transaction, refresh = false) {
    if (!value || typeof value.access_token !== 'string' || !value.access_token ||
        String(value.token_type).toLowerCase() !== 'bearer' ||
        !Number.isFinite(value.expires_in) || value.expires_in <= 0 ||
        (value.refresh_token !== undefined && typeof value.refresh_token !== 'string')) fail('invalid_token_response');
    let claims = current?.claims;
    if (value.id_token) {
      const options = { issuer, clientId, nonce: transaction.nonce, accessToken: value.access_token,
        refresh, subject: refresh ? current?.claims.sub : undefined, jwks: await signingKeys() };
      try { claims = await verifyIdToken(value.id_token, options); }
      catch (error) {
        if (error.code !== 'signing_key_not_found') throw error;
        options.jwks = await signingKeys(true);
        claims = await verifyIdToken(value.id_token, options);
      }
    } else if (!refresh) fail('missing_id_token');
    if (!claims) fail('missing_verified_identity');
    return Object.freeze({ tokens: Object.freeze({ ...value }), claims, nonce: transaction.nonce });
  }

  async function tokenRequest(parameters) {
    const document = await configuration();
    return requestJSON(document.token_endpoint, { method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: new URLSearchParams({ client_id: clientId, ...parameters }).toString() });
  }

  async function prepare() {
    const document = await configuration();
    const transaction = { issuer, clientId, redirectUri, state: randomValue(), nonce: randomValue(),
      verifier: randomValue(), createdAt: Date.now() };
    const url = new URL(document.authorization_endpoint);
    const extra = config.authorizationParams || {};
    const allowed = new Set(['prompt', 'login_hint', 'ui_locales', 'max_age']);
    for (const [key, value] of Object.entries(extra)) {
      if (!allowed.has(key) || typeof value !== 'string') fail('unsupported_authorization_parameter');
      url.searchParams.set(key, value);
    }
    for (const [key, value] of Object.entries({ client_id: clientId, redirect_uri: redirectUri,
      response_type: 'code', response_mode: 'query', scope: scopes.join(' '), state: transaction.state,
      nonce: transaction.nonce, code_challenge_method: 'S256', code_challenge: await pkceChallenge(transaction.verifier) })) {
      url.searchParams.set(key, value);
    }
    return { transaction, url: url.href };
  }

  async function exchange(raw, transaction, epoch) {
    const code = parseAuthorizationResponse(raw, transaction);
    const response = await tokenRequest({ grant_type: 'authorization_code', code,
      redirect_uri: redirectUri, code_verifier: transaction.verifier });
    const result = await checkedTokens(response, transaction);
    if (epoch !== operationEpoch) fail('login_cancelled');
    current = result;
    return { tokens: result.tokens, claims: result.claims };
  }

  return {
    async loginPopup() {
      if (running) fail('operation_in_progress');
      running = true;
      const epoch = ++operationEpoch;
      let popup;
      let cleanup = () => {};
      try {
        // Open before awaiting network/crypto so browsers retain the user gesture.
        popup = browser.open('about:blank', `nowork-oidc-${randomValue()}`, 'popup,width=520,height=720');
        if (!popup) fail('popup_blocked');
        const { transaction, url } = await prepare();
        if (epoch !== operationEpoch || popup.closed) fail('login_cancelled');
        const raw = await new Promise((resolve, reject) => {
          let settled = false;
          const finish = (error, value) => {
            if (settled) return;
            settled = true;
            cleanup();
            if (error) reject(error); else resolve(value);
          };
          const listener = (event) => {
            if (event.origin !== new URL(redirectUri).origin || event.source !== popup ||
                event.data?.type !== CALLBACK_TYPE || typeof event.data.responseUrl !== 'string') return;
            try {
              parseAuthorizationResponse(event.data.responseUrl, transaction);
              finish(null, event.data.responseUrl);
            } catch (error) { finish(error); }
          };
          const timer = setTimeout(() => finish(new LoginError('login_timeout')), timeout);
          const closed = setInterval(() => {
            if (popup.closed || epoch !== operationEpoch) finish(new LoginError('login_cancelled'));
          }, 250);
          cleanup = () => { browser.removeEventListener('message', listener); clearTimeout(timer); clearInterval(closed); };
          browser.addEventListener('message', listener);
          popup.location.replace(url);
        });
        return await exchange(raw, transaction, epoch);
      } finally {
        cleanup();
        popup?.close();
        running = false;
      }
    },

    async loginRedirect() {
      if (running) fail('operation_in_progress');
      running = true;
      ++operationEpoch;
      try {
        const { transaction, url } = await prepare();
        browser.sessionStorage.setItem(storageKey, JSON.stringify(transaction));
        browser.location.assign(url);
      } finally { running = false; }
    },

    async completeRedirect(raw = browser.location.href) {
      if (running) fail('operation_in_progress');
      const stored = browser.sessionStorage.getItem(storageKey);
      // Consume before network I/O; a callback cannot be replayed locally.
      browser.sessionStorage.removeItem(storageKey);
      if (!stored) fail('transaction_not_found');
      let transaction;
      try { transaction = JSON.parse(stored); } catch { fail('invalid_transaction'); }
      if (transaction.issuer !== issuer || transaction.clientId !== clientId || transaction.redirectUri !== redirectUri) fail('transaction_mismatch');
      running = true;
      const epoch = ++operationEpoch;
      try {
        parseAuthorizationResponse(raw, transaction);
        browser.history.replaceState(null, '', new URL(redirectUri).pathname);
        return await exchange(raw, transaction, epoch);
      } finally { running = false; }
    },

    async userinfo() {
      if (!current) fail('login_required');
      const snapshot = current;
      const document = await configuration();
      if (!document.userinfo_endpoint) fail('userinfo_unsupported');
      const claims = await requestJSON(document.userinfo_endpoint, {
        headers: { Authorization: `Bearer ${snapshot.tokens.access_token}` }
      });
      if (claims.sub !== snapshot.claims.sub) fail('subject_mismatch');
      if (snapshot !== current) fail('login_cancelled');
      return claims;
    },

    async refresh() {
      if (running) fail('operation_in_progress');
      if (!current?.tokens.refresh_token) fail('refresh_token_unavailable');
      running = true;
      const epoch = operationEpoch;
      const snapshot = current;
      try {
        const value = await tokenRequest({ grant_type: 'refresh_token', refresh_token: snapshot.tokens.refresh_token });
        const result = await checkedTokens(value, snapshot, true);
        if (epoch !== operationEpoch) fail('login_cancelled');
        current = Object.freeze({ ...result, tokens: Object.freeze({ ...result.tokens,
          refresh_token: result.tokens.refresh_token || snapshot.tokens.refresh_token }) });
        return { tokens: current.tokens, claims: current.claims };
      } catch (error) {
        // A rotated refresh token cannot safely be retried after any ambiguous failure.
        current = null;
        throw error;
      } finally { running = false; }
    },

    async revoke() {
      if (running) fail('operation_in_progress');
      if (!current) return;
      running = true;
      const snapshot = current;
      current = null;
      ++operationEpoch;
      try {
        const document = await configuration();
        if (!document.revocation_endpoint) fail('revocation_unsupported');
        for (const [token, hint] of [[snapshot.tokens.refresh_token, 'refresh_token'], [snapshot.tokens.access_token, 'access_token']]) {
          if (!token) continue;
          const response = await fetcher(endpoint(document.revocation_endpoint), { method: 'POST', credentials: 'omit', redirect: 'error', cache: 'no-store',
            signal: AbortSignal.timeout(15000), headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({ client_id: clientId, token, token_type_hint: hint }).toString() });
          if (!response.ok) await jsonResponse(response);
        }
      } finally { running = false; }
    },

    clear() {
      current = null;
      ++operationEpoch;
      browser.sessionStorage.removeItem(storageKey);
    }
  };
}

/** Run only on the registered same-origin callback page. The opener validates source, origin and state. */
export function completePopupCallback() {
  const browser = globalThis.window;
  if (!browser?.opener || browser.opener === browser) fail('popup_opener_unavailable');
  const url = new URL(browser.location.href);
  safeURL(`${url.origin}${url.pathname}`, { query: false });
  if (url.hash || !single(url.searchParams, 'state')) fail('invalid_authorization_response');
  const raw = url.href;
  browser.history.replaceState(null, '', url.pathname);
  browser.opener.postMessage({ type: CALLBACK_TYPE, responseUrl: raw }, url.origin);
  // The opener closes the popup after it has received and checked the response.
}

/** Text and image URLs are assigned through DOM properties, never interpolated into HTML. */
export function createLoginButton({ brand = 'NoWork', locale = 'en', logoUrl, onClick } = {}) {
  const button = document.createElement('button');
  button.type = 'button';
  button.className = 'nowork-login-button';
  if (logoUrl) {
    const logo = document.createElement('img');
    logo.src = safeURL(new URL(logoUrl, location.href).href).href;
    logo.alt = '';
    logo.width = logo.height = 22;
    logo.referrerPolicy = 'no-referrer';
    button.append(logo);
  }
  const label = document.createElement('span');
  label.textContent = (Object.hasOwn(labels, locale) ? labels[locale] : labels.en).replace('{brand}', String(brand));
  button.append(label);
  if (onClick) button.addEventListener('click', onClick);
  return button;
}
