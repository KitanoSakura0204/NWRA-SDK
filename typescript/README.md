# @lounge/sdk (TypeScript / Node.js)

零依赖，Node 18+（全局 `fetch`），也可在浏览器/启动器（Electron/Tauri）中使用。

```bash
npm i @lounge/sdk
```

```ts
import { LoungeClient, LoungeOIDC, pkce, TransportError } from '@lounge/sdk';

const client = new LoungeClient('https://account.example.com', { serverKey: process.env.LOUNGE_SERVER_KEY! });
const r = await client.checkWhitelist('Steve', '069a79f4-...');
if (!r.allowed) kick(`未绑定账号: ${r.reason}`);
setInterval(() => client.heartbeat(online, max, version).catch(() => {}), 60_000);

// OIDC（公开客户端 + PKCE）
const oidc = new LoungeOIDC(issuer, clientId, '', redirectUri);
const { verifier, challenge } = await pkce();
location.href = oidc.authorizeUrl(state, nonce, 'openid profile email game_roles', challenge);
// 回调页
const token = await oidc.exchange(code, verifier);
const me = await oidc.userInfo(token.access_token);
```
