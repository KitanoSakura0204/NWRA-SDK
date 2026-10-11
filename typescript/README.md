# @nowork/sdk (TypeScript / Node.js)

零依赖，Node 18+（全局 `fetch`），也可在浏览器/启动器（Electron/Tauri）中使用。

```bash
npm i @nowork/sdk
```

```ts
import { NoworkClient, NoworkOIDC, pkce, TransportError } from '@nowork/sdk';

const client = new NoworkClient('https://account.example.com', { serverKey: process.env.NOWORK_SERVER_KEY! });
const r = await client.checkWhitelist('Steve', '069a79f4-...');
if (!r.allowed) kick(`未绑定账号: ${r.reason}`);
setInterval(() => client.heartbeat(online, max, version).catch(() => {}), 60_000);

// OIDC（公开客户端 + PKCE）
const oidc = new NoworkOIDC(issuer, clientId, '', redirectUri);
const { verifier, challenge } = await pkce();
location.href = oidc.authorizeUrl(state, nonce, 'openid profile email game_roles', challenge);
// 回调页
const token = await oidc.exchange(code, verifier);
const me = await oidc.userInfo(token.access_token);
```

## Server API 自动解密

服务器启用数据投递加密后，把对应的私钥 JWK 传给 `NoworkClient`。复用
[DELIVERY.md](../docs/DELIVERY.md) 的 `PrivateJwk` 类型和算法：RSA-OAEP-256、P-256，
以及运行时支持的 X25519（Node 20+）。私钥只用于本地解密，不会随请求发送。

```ts
import { NoworkClient, EncryptionError, type PrivateJwk } from '@nowork/sdk';

// 从服务器的密钥存储加载；轮换过渡期内同时保留新旧私钥。
declare const currentJwk: PrivateJwk;
declare const previousJwk: PrivateJwk;
const server = new NoworkClient('https://account.example.com', {
  serverKey: process.env.NOWORK_SERVER_KEY!,
  serverCode: 'survival',
  decryptionKeys: [currentJwk, previousJwk], // 单把 PrivateJwk 也可直接传入
  timeoutMs: 10_000,
});
const whitelist = await server.whitelist();
const joined = await server.checkWhitelist('Steve');
```

配置私钥后，`whitelist()` 和 `checkWhitelist()` 必须收到加密响应；明文回退会抛出
`EncryptionError`。其他接口（心跳、GM、玩家数据、聊天）继续接受普通明文响应，并在收到
加密响应时自动解密。未配置密钥或传入空列表时保留原有行为：明文可用，加密响应抛错。
收到 `EncryptionError` 时应拒绝本次入服校验，不能将其当成“白名单关闭”。
解密后的白名单必须明确给出 `mode: 0 | 1 | 2`，入服校验必须明确给出布尔值 `allowed`；
字段缺失或类型错误同样会抛出 `EncryptionError`，不会补成允许加入的默认值。

客户端核对经过认证的 `ctx` 为 `server:<服务器代码>:<实际路由>`，并按 `kid` 选择轮换密钥。
`serverCode` 可省略；此时仍严格校验类型、非空服务器代码和完整路由，但不能限定具体服务器。
共享密钥时应总是设置它。配置和密钥在创建客户端时复制，轮换更新时创建新的客户端。

`raw()` 保留原始信封，跳过自动解密和明文拒绝策略；自行处理时把 `raw.envelope` 交给
`openServerData(raw.envelope, keys, 'server:survival:whitelist')`。业务错误仍为 `ApiError`；网络、
无效外层 JSON、HTTP 错误或超过 **8 MiB** 的响应仍为 `TransportError`。超时覆盖请求和响应体读取。
本设置只影响 Server API，不改变账号/游戏数据客户端的鉴权或归档分页。ServerKey 与私钥留在可信服务端；
账号站点 JWT 使用独立的 `AccountClient` / `GameDataClient`。

### English

Set `decryptionKeys` to one private JWK or a rotation key list, and set `serverCode` to pin the authenticated
context to your server. Keys require encrypted success responses only on `whitelist` and `whitelist/check`;
heartbeat, GM, player-data and chat methods still accept plaintext. Every typed Server API method decrypts
encrypted replies and rejects wrong keys, context, malformed envelopes or invalid decrypted data with
`EncryptionError`. Without keys, encrypted replies fail closed. An omitted `serverCode` checks the context
kind and exact route, but does not pin the server identity. `raw()` returns the original envelope for manual
handling. Transport reads are limited to 8 MiB and remain subject to `timeoutMs`; business errors stay `ApiError`.
Decrypted whitelist decisions require an explicit mode of 0, 1 or 2, or an explicit boolean `allowed` for join checks.
Server keys and decryption keys are independent from site JWT clients and are never used for their requests.

Offline verification: build with `npm run build`, then run `node --test tests/*.test.mjs`.
Node 18 may need `--experimental-global-webcrypto`; X25519 requires a runtime that implements it.
