# NoWork login kit

Dependency-free browser helpers for OpenID Connect authorization code + S256 PKCE. Four button languages: `zh_CN`, `zh_TW`, `en`, `ja`. No build step is required: serve `index.js`, `index.d.ts` and `button.css` as your own static files. This directory is not published to a package registry.

The built-in transport supports **public clients only**. Never put a client secret or signing private key in browser code. Confidential web applications exchange codes and keep credentials on their backend, using an OIDC library for their framework.

## Public-client popup login

Register a SPA/native public client, enable the authorization-code grant and register the exact callback URL. Your callback must be on the same origin as the page opening the popup. HTTPS is required; HTTP is allowed only on localhost/loopback for development.

```html
<link rel="stylesheet" href="/assets/login-kit/button.css">
<div id="login"></div>
<p id="identity"></p>
<script type="module" src="/login.js"></script>
```

```js
// /login.js — these are public registration values, never a client secret.
import { createLoginClient, createLoginButton } from '/assets/login-kit/index.js';

const login = createLoginClient({
  issuer: 'https://accounts.example.com', // exact discovery issuer
  clientId: 'your-registered-public-client-id',
  redirectUri: 'https://app.example.com/oidc/callback',
  scope: 'openid profile email offline_access',
  authorizationParams: { ui_locales: 'en' }
});

document.querySelector('#login').append(createLoginButton({
  brand: 'NoWork',
  locale: 'en',
  // Optional: an existing logo you have permission to use. The kit creates no logo.
  // logoUrl: '/brand/logo.png',
  onClick: async () => {
    try {
      const { claims } = await login.loginPopup();
      // Returned claims passed signature, issuer, audience, time and nonce checks.
      document.querySelector('#identity').textContent = claims.sub;
    } catch (error) {
      document.querySelector('#identity').textContent = error.code || 'login_failed';
    }
  }
}));
```

The registered callback page runs this external module, with no analytics or third-party scripts:

```js
// /oidc/callback.js
import { completePopupCallback } from '/assets/login-kit/index.js';
completePopupCallback();
```

Serve the callback with `Cache-Control: no-store` and `Referrer-Policy: no-referrer`. Its script removes the authorization response from the address bar before messaging the opener. The opener checks the exact callback origin, the precise popup `Window`, one-use state, issuer when supplied, and the registered path. It closes the popup after handling the response. Popup isolation headers that sever `window.opener` require the redirect flow instead.

## Redirect login

Create the client with the same static public configuration on your login page and callback page:

```js
// Login button handler:
await login.loginRedirect();

// Registered callback page, after creating the same configured client:
const { claims } = await login.completeRedirect();
document.querySelector('#identity').textContent = claims.sub;
```

Redirect mode stores only the pending state/nonce/PKCE verifier and public registration context in `sessionStorage`, expires it after five minutes, and consumes it before exchange. It never persists access, refresh or ID tokens. Do not run the popup callback helper and redirect completion on the same response.

## Tokens, userinfo and clearing

```js
const user = await login.userinfo(); // HTTPS response; sub must match verified ID token
const refreshed = await login.refresh(); // rotated refresh token replaces the old one
await login.revoke(); // revoke refresh/access tokens, clear local memory
login.clear();       // clear local memory only; does not revoke server credentials
```

`refresh()` needs `offline_access` and a granted refresh token. A failed refresh clears the local session: an ambiguous network failure may already have consumed the old refresh token. `revoke()` also clears local tokens if the network request fails, and reports the failure; remote revocation is not claimed in that case. A page reload discards token state. The kit does not create your application's server session or implement application authorization.

## Verification and transport contract

- Pin the exact issuer. Discovery must return that issuer, advertise code/S256/RS256, and supply trusted endpoint URLs.
- HTTPS endpoints default to the issuer origin. For an operator-controlled provider with intentionally separate endpoint hosts, list each exact origin in `trustedEndpointOrigins`. Never derive this list from untrusted query parameters or token headers.
- Token, userinfo and JWKS fetches omit cookies, reject HTTP redirects, use no-store, have bounded response sizes and timeouts, and require provider CORS support for your application origin.
- ID tokens require RS256, one matching public RSA signing key (2048–4096 bits), a valid signature, exact issuer and audience, authorized party where applicable, subject, expiry/issued-at/not-before, and the original nonce. `at_hash` is checked when present. A refreshed ID token must keep the subject; any returned nonce must match the original.
- JWKS are cached in memory for up to five minutes; one refresh is attempted for an unknown signing key. Public keys are fetched only from the discovery endpoint validated against configured origins. Token-supplied `jku`, `jwk` and `x5u` are rejected.
- `decodeJWTUnverified()` is only for inspection and returns `verified: false`. A decoded payload is never proof of identity. The verified login APIs do not use an unverified payload to authorize the application.
- The kit expects JSON token/userinfo responses and signed RS256 ID tokens. It does not decrypt JWE, accept unsigned/HMAC ID tokens, implement implicit/hybrid flows, or verify tokens for a backend resource server. Use a capable server library when an app's delivery profile enables encrypted or signed userinfo.
- `authorizationParams` accepts only `prompt`, `login_hint`, `ui_locales` and `max_age`; callers cannot override state, nonce, redirect, response type or PKCE.

The separate account-centre Playground is an owner-authenticated debugging tool. It can accept an owner's temporary secret/assertion through its normal backend client-authentication chain. That debugger is not this public-client kit and does not justify embedding a secret in a production browser application.

## Error handling and verification status

`LoginError.code` carries a stable code, such as `state_mismatch`, `nonce_mismatch`, `popup_blocked`, `invalid_id_token_signature` or a standard OAuth error. Display a localized explanation; do not log token response bodies. The kit does not emit tokens to the console.

Tests are in `tests/login-kit.test.mjs`, using Node's built-in test runner and WebCrypto, with no install step. They exercise real RSA signatures, rejected identity claims, callback correlation, popup source/origin checks, refresh rotation, revocation and one-use redirect storage. Tests were authored by this workstream; only the root verification report establishes execution results. No external provider or browser deployment has been claimed as tested here.

中文：按钮通过 `locale` 支持简体中文、繁体中文、英语和日语。SDK 仅适用于公开客户端，不接收客户端密钥。签名验证成功后的 `claims` 才能用于显示登录身份；`decodeJWTUnverified()` 只是查看内容。重定向模式仅暂存 state、nonce 与 PKCE 证明，不保存令牌。正式机密应用应在服务端交换授权码。
