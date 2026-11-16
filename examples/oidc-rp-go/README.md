# OIDC relying-party example (Go)

A runnable browser login using the checked-out Go SDK and Go's standard library (Go 1.21+). It supports authorization code + S256 PKCE, a signed RS256 ID Token, and plain JSON UserInfo. It needs an OIDC application registration, not a game-server key or an account password.

## Run from this checkout

Register an application on your NoWork issuer with authorization-code flow, `openid profile`, S256 PKCE, and this exact redirect URI for local development:

```text
http://127.0.0.1:8080/callback
```

Configure signed RS256 ID Tokens and plain JSON UserInfo for that application. Use a public client without a secret, or supply `NOWORK_CLIENT_SECRET` for a confidential client. The SDK sends a configured secret in the token request body (`client_secret_post`). A game-server key is never required by this example.

PowerShell (replace the example issuer and client ID with your registration):

```powershell
Set-Location D:\Project\Admin\SDK\examples\oidc-rp-go
$env:NOWORK_ISSUER = 'https://account.example.com'
$env:NOWORK_CLIENT_ID = 'your-registered-client-id'
$env:APP_BASE_URL = 'http://127.0.0.1:8080'
# Set NOWORK_CLIENT_SECRET through your environment only if the client needs it.
go run .
```

Bash, starting at the SDK repository root:

```bash
cd examples/oidc-rp-go
export NOWORK_ISSUER=https://account.example.com
export NOWORK_CLIENT_ID=your-registered-client-id
export APP_BASE_URL=http://127.0.0.1:8080
go run .
```

Open `http://127.0.0.1:8080/`, choose **Sign in**, and finish the issuer's login and consent. The callback redirects to a signed-in home page with a **Sign out locally** button. Starting the process and viewing the signed-out page require no provider connection; `/login` performs discovery. Ctrl+C stops the process.

Both `go run .` and the tests use `replace github.com/wangework/nowork-sdk-go => ../../go` in `go.mod`. Keep the example in this checkout with the updated SDK. The commands do not depend on a published SDK version being available. Copying this directory alone requires changing that replacement to the location of the updated SDK.

| Environment variable | Default | Meaning |
| --- | --- | --- |
| `NOWORK_ISSUER` | Required | Configured NoWork issuer; trailing slashes are normalized. HTTPS required except loopback HTTP. |
| `NOWORK_CLIENT_ID` | Required | The registered OIDC application client ID. |
| `NOWORK_CLIENT_SECRET` | Empty | Optional confidential-client secret; never placed in browser URLs. |
| `APP_BASE_URL` | `http://127.0.0.1:8080` | Browser-facing origin. Scheme/host casing, IP literals and default ports are normalized; redirect URI is the normalized origin plus `/callback`. No path, credentials, query or fragment. |

The listener always binds **`127.0.0.1:8080`**. `APP_BASE_URL` changes the public origin and cookie security, not the listening address. For HTTPS, terminate TLS at a reverse proxy forwarding to this listener and set `APP_BASE_URL` to the public HTTPS origin. Register its normalized `/callback` address: for example, `HTTPS://RP.EXAMPLE:443` becomes `https://rp.example/callback`, and all cookies are Secure. This application-origin normalization does not change the SDK's exact issuer-matching contract. Do not switch between `localhost` and `127.0.0.1` during a login: browser cookies belong to the chosen host.

## Flow and security checks

1. `GET /login` uses `OIDC.Discover` to validate the configured issuer. The SDK uses the issuer's fixed paths (`/.well-known/openid-configuration`, `/oauth2/authorize`, `/oauth2/token`, `/jwks.json`, `/oauth2/userinfo`), and never follows advertised metadata endpoints.
2. Independent random state, nonce and PKCE verifier stay in memory for five minutes. A separate random handle in an HttpOnly, SameSite=Lax cookie binds the attempt to this browser. All cookies use `Secure` with an HTTPS `APP_BASE_URL`.
3. `GET /callback` consumes the attempt once, checks state and any returned `iss`, rejects ambiguous/malformed callback parameters, then exchanges the code with the stored verifier. The example expects a query/GET callback; `form_post` is not supported.
4. `OIDC.JWKS` obtains public signing keys. `VerifyIDTokenWithNonce` checks RS256 signature, issuer, audience, time and nonce. The example also checks the subject and authorized party (`azp`, required for multiple audiences) and rejects already expired tokens before calling UserInfo. It does not alter signed claims or shift clocks.
5. UserInfo must have exactly the signed ID Token's nonempty `sub`. Only then is a new local session created. Its absolute lifetime is at most one hour and no later than the signed token expiry; it never slides on activity. Tokens are not retained in the session or logged. The displayed profile is HTML-escaped.
6. `POST /logout` requires the session's CSRF value in the form body, checks Origin when supplied, invalidates the local session and any pending attempt, and clears both cookies. GET logout is rejected. This does not end the provider's SSO session or revoke provider tokens.

An invalid/denied callback consumes that attempt, so start a fresh login after an error. Starting another login in the same browser replaces its previous pending attempt. Each store holds at most 1,024 entries; saturation returns HTTP 503 without evicting other active users. Expired entries are cleaned on requests and every minute while the process runs.

## Local verification

```powershell
go test ./...
go vet ./...
```

Tests generate temporary RSA keys and use loopback mock providers. They cover successful PKCE login, state/browser/nonce/issuer/audience/signature/sub errors, replay, provider discovery failure, TTLs, bounded stores, HTML escaping and CSRF-protected logout. They make no live authentication requests and need no credentials. These tests are not evidence that a deployed provider or application registration has been validated.

## Deployment limits

This is a single-process example. Sessions and pending logins disappear on restart and are not shared between replicas. Use a shared TTL store with atomic consumption and session invalidation before scaling; add operational rate limits and deployment monitoring. Store counts are bounded, but this is not a complete denial-of-service defense. Use a dedicated browser host; cookies are not isolated by port, and related subdomains can affect cookie handling. Keep client secrets in your deployment's secret store, and ensure reverse-proxy/access logs do not record callback query strings or tokens.

Encrypted ID Tokens, signed/encrypted UserInfo delivery, refresh-token management, remote logout and account/game-server APIs are not implemented here. For an application using encrypted ID Tokens, the SDK's `DecryptIDTokenWithNonce` also verifies the inner signature, but the example intentionally rejects that configuration until it is explicitly adapted with the right decryption keys and delivery handling. Verify issuer configuration, redirect registration, HTTPS/proxy behavior and the intended delivery mode against your own deployment.
