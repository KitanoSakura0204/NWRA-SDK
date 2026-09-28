# Webhook 接收端示例（Node.js / TypeScript）

> English version below.

只用 Node 自带的 `node:http`（不依赖 Express），通过相对路径直接引用仓库内的 TypeScript SDK 源码：校验签名、解密加密投递（JWE）、按事件 `id` 幂等，处理 `game_role.reviewed` / `user.deleted` / `server.offline` / `user.level_changed` / `connection.bound` / `connection.unbound`。

## 目录

```
webhook-receiver-node/
├── src/server.ts              入口：读取环境变量、监听端口、/healthz
├── src/receiver.ts            端点本身：验签、解密、幂等、状态码（createReceiver）
├── src/events.ts              业务逻辑：事件字段与分发（dispatch）
├── src/lru.ts                 进程内 LRU 幂等缓存
├── tests/receiver.test.mjs    node:test 测试（直接调用处理函数，不监听端口）
├── tsconfig.json              rootDir 指向仓库根，因此会把 ../../typescript/src 一起编译进 dist/
└── package.json               build / start / test 脚本
```

`src/server.ts` 里 `import ... from '../../../typescript/src/index.js'` 指向 SDK 源码；编译后对应 `dist/typescript/src/index.js`，所以运行时不需要安装 `@lounge/sdk`。若单独拷走本目录，把导入改成 `from '@lounge/sdk'` 并 `npm i @lounge/sdk`，`tsconfig.json` 的 `rootDir` 改回 `src`、`include` 只保留 `src` 即可。

## 运行

需要 Node 20+（全局 WebCrypto；X25519 解密需要 20 及以上）。

```bash
cd SDK/examples/webhook-receiver-node
npm install            # 只装 typescript 与 @types/node（开发期）
npm run build          # tsc -> dist/
export LOUNGE_WEBHOOK_SECRET=whsec_xxx   # 后台「开放平台 → Webhook」新建 Webhook 时显示（只显示一次，可「重置密钥」）
node dist/examples/webhook-receiver-node/src/server.js
# listening on http://0.0.0.0:8091/lounge/webhook (timestamp tolerance 300s, 0 decryption key(s))
```

`npm start` 等价于最后一行。生产环境建议用 `pm2` / `systemd` 托管并放在反向代理（HTTPS）之后。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LOUNGE_WEBHOOK_SECRET` | 必填 | 端点签名密钥 |
| `HOST` / `PORT` | `0.0.0.0` / `8091` | 监听地址 |
| `WEBHOOK_PATH` | `/lounge/webhook` | 接收路径（后台端点 URL 需与之一致） |
| `WEBHOOK_TOLERANCE` | `300` | 允许的时间戳偏差（秒），`0` 关闭检查 |
| `SEEN_CAPACITY` | `10000` | 幂等缓存保留的事件 id 数量 |
| `LOUNGE_DECRYPTION_KEYS` | 空 | 私钥 JWK 文件路径，逗号分隔（轮换期间填新旧两把）；留空表示端点未加密 |

## 行为约定

| 情况 | 响应 | 用户中心的处理 |
| --- | --- | --- |
| 签名正确、首次收到 | `200 {"status":"ok"}` | 标记已投递 |
| 同一 `id` 再次收到 | `200 {"status":"duplicate"}` | 标记已投递（不重复处理） |
| 签名错误 / 缺签名 / 时间戳偏差过大 | `401` | 按 30s / 2m / 10m / 30m / 2h / 6h 退避重试 |
| 请求体不是 JSON，或没有 `id` | `400` | 同上（重试也不会成功，请检查中间代理是否改写了请求体） |
| 处理逻辑抛错 | `500` | 同上，重试时 `id` 不变 |
| 签名正确但无法解密（缺私钥、密钥不符，或配置了私钥却收到明文） | `500` | 同上；补上私钥后重试即可成功 |
| 请求体超过 1 MiB | `413` | 同上 |

要点：

- 签名覆盖 `<X-Lounge-Timestamp>.<原始请求体>`，所以要把原始 `Buffer` 交给 `verifyWebhook`，不要经过任何 body parser 再序列化。
- `verifyWebhook` 直接接受 `req.headers`（Node 会把头名转成小写，SDK 按不区分大小写查找）。
- 幂等缓存是进程内 LRU；多实例部署时换成 Redis `SET NX EX` 或数据库唯一索引。
- 处理逻辑必须在 10 秒内返回；耗时任务先入队再响应。自己的业务写在 `src/events.ts` 的 `dispatch` 里（可以是 `async`），抛错即返回 `500` 并由用户中心重试。

## 加密投递（JWE）

Webhook 绑定的投递配置开启 `webhook.encrypt` 后（后台「开放平台 → 数据投递」，见 `SDK/docs/DELIVERY.md` 第 6 节），请求体变为 `{"id","event","timestamp","jwe"}` 并带 `X-Lounge-Encryption: JWE`。`src/receiver.ts` 先对外层原始请求体验签，再用 SDK 的 `openWebhook` 解密（核对内层 `id` / `event` / `timestamp` 与外层一致、受保护头 `ctx` 为 `webhook:<端点 ID>`）；配置了私钥却收到明文时拒绝。私钥只放在接收端，后台只保存公钥 JWK；轮换时先把新私钥加进 `LOUNGE_DECRYPTION_KEYS=new.jwk,old.jwk`，再在后台启用新公钥，过渡期结束后删掉旧私钥。

## 成对匿名标识（pairwise）

投递配置的 `subject` 为 `pairwise` 时，事件里不再有 `userId` / `uid` / `username` / `ip` / `externalId` / `externalName` / `email` / `phone`，改由 `data.user.sub` 标识账号（同一 `sector` 下稳定、跨应用不可关联）。`events.ts` 的 `who()` 两种都能打印；自己的业务按配置选一种作为本地主键，不要混用。需要 `externalId` 的集成（例如 `../discord-role-sync`）要用非 pairwise 的端点。

## 事件字段

```jsonc
// game_role.reviewed — status: 0 待审核 1 通过 2 拒绝 3 封禁
{ "roleId": 12, "userId": 100, "serverId": 1, "roleName": "Steve",
  "roleUuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5", "status": 1, "reviewerId": 1 }
// user.deleted
{ "userId": 100, "uid": 100001, "username": "alice", "reason": "self-service" }
// server.offline
{ "serverId": 1, "code": "survival", "name": "生存服", "lastHeartbeat": "2026-09-14 12:00:00" }
// user.level_changed — 信任等级变化
{ "userId": 100, "uid": 100001, "username": "alice", "from": 1, "to": 2 }
// connection.bound / connection.unbound（externalName 仅 bound 有）
{ "userId": 100, "uid": 100001, "provider": "discord", "externalId": "80351110224678912", "externalName": "alice" }
// pairwise 配置下，账号字段被 data.user 取代
{ "user": { "sub": "2f1c8e0a-8f3b-8c4d-9a6e-1b2c3d4e5f60" }, "from": 1, "to": 2 }
```

完整事件列表见 `docs/PROTOCOL.md` 第 5 节。

## 本地自测

后台「开放平台 → Webhook」里点“测试”会推送一条 `event = ping`。也可以自己签名一条：

```bash
node -e '
const c=require("crypto");const s=process.env.LOUNGE_WEBHOOK_SECRET;const ts=Math.floor(Date.now()/1000);
const body=JSON.stringify({id:"test-1",event:"user.deleted",timestamp:ts,data:{userId:100,uid:100001,username:"alice",reason:"self-service"}});
const sig="sha256="+c.createHmac("sha256",s).update(ts+"."+body).digest("hex");
fetch("http://127.0.0.1:8091/lounge/webhook",{method:"POST",headers:{"Content-Type":"application/json","X-Lounge-Event":"user.deleted","X-Lounge-Delivery":"1","X-Lounge-Timestamp":String(ts),"X-Lounge-Signature":sig},body}).then(async r=>console.log(r.status,await r.text()));'
```

再发一次同样的 `id` 会得到 `{"status":"duplicate"}`；改动请求体任意字节会得到 `401`。`../discord-role-sync/scripts/send-event.mjs` 提供了更完整的发送工具，也可用于本示例。

## 检查

```bash
npm test   # tsc 编译后运行 tests/receiver.test.mjs
```

测试直接调用处理函数（不监听端口）：明文投递与重复、错误签名 / 篡改 / 过期 / 缺签名 / 非 JSON / 缺 `id` / 非 POST、处理失败后重试，以及用仓库共享测试向量（`SDK/go/testdata/vectors.json`）验证加密投递、缺私钥、"配置了私钥却收到明文"和误填公钥四种情况。单独拷走本目录时加密用例会自动跳过。

---

# Webhook receiver example (Node.js / TypeScript)

A webhook endpoint on Node's own `node:http` (no Express) that imports the TypeScript SDK source in this repo by relative path. It verifies signatures, opens encrypted deliveries (JWE), is idempotent by event `id`, and logs `game_role.reviewed`, `user.deleted`, `server.offline`, `user.level_changed`, `connection.bound` and `connection.unbound`.

`src/server.ts` is the entry point (environment, listener, `/healthz`), `src/receiver.ts` the endpoint (`createReceiver`: signature, decryption, idempotency, status codes), `src/events.ts` the business logic (`dispatch`), `src/lru.ts` the in-process de-duplication cache. The imports point at `../../../typescript/src/index.js`, compiled to `dist/typescript/src/index.js`, so no `@lounge/sdk` install is needed. To copy the directory out of the repo, import from `@lounge/sdk` instead, `npm i @lounge/sdk`, and set `rootDir` back to `src` with `include: ["src"]`.

## Run

Node 20+ (global WebCrypto; X25519 decryption needs 20 or later).

```bash
cd SDK/examples/webhook-receiver-node
npm install && npm run build
export LOUNGE_WEBHOOK_SECRET=whsec_xxx   # shown once when you create the webhook in 开放平台 → Webhook (Open Platform → Webhooks)
npm start
```

Variables: `LOUNGE_WEBHOOK_SECRET` (required), `HOST` / `PORT` (`0.0.0.0` / `8091`), `WEBHOOK_PATH` (`/lounge/webhook`, must match the URL in the console), `WEBHOOK_TOLERANCE` (`300` seconds, `0` turns the check off), `SEEN_CAPACITY` (`10000`), `LOUNGE_DECRYPTION_KEYS` (comma-separated private JWK files, new and old during a rotation; empty means the endpoint is not encrypted). Run it under `pm2` / `systemd` behind an HTTPS reverse proxy.

## Responses

`200 {"status":"ok"}` on first delivery, `200 {"status":"duplicate"}` for an `id` already handled, `401` for a bad or missing signature or a timestamp too far off, `400` for a body that is not JSON or has no `id`, `500` when `dispatch` throws or a signed delivery cannot be opened (no key, wrong key, or plaintext while keys are set), `413` over 1 MiB. Anything but 2xx is retried after 30s / 2m / 10m / 30m / 2h / 6h with the same `id`.

- The signature covers `<X-Lounge-Timestamp>.<raw body>`: hand the raw `Buffer` to `verifyWebhook`, never a parsed and re-serialised body. `req.headers` can be passed as is (the SDK looks names up case-insensitively).
- Return within 10 s; queue anything slow. Put your logic in `dispatch` (it may be `async`); a throw answers `500` and the delivery is retried.
- The LRU is per process; with several instances use Redis `SET NX EX` or a unique DB column keyed by `id`.

## Encrypted deliveries and pairwise subjects

With `webhook.encrypt` on the webhook's delivery profile (开放平台 → 数据投递, `SDK/docs/DELIVERY.md` section 6) the body becomes `{"id","event","timestamp","jwe"}` with `X-Lounge-Encryption: JWE`. The receiver verifies the outer body, then opens it with `openWebhook`, which checks that the inner `id` / `event` / `timestamp` repeat the outer ones and that the header `ctx` is `webhook:<endpoint id>`; with keys configured a plaintext delivery is refused. To rotate, add the new private key first (`new.jwk,old.jwk`), activate the new public key in the console, then drop the old one. With `subject: pairwise`, `data.user.sub` replaces `userId` / `uid` / `username` / `externalId` and the other identifying fields; `who()` prints either. The Discord role sync needs `externalId`, so it needs a non-pairwise endpoint.

## Test and check

`npm test` compiles and runs `tests/receiver.test.mjs`, which calls the handler directly: a plain delivery and its duplicate; wrong secret, tampered, stale, unsigned, non-JSON, missing `id` and non-POST; a failing handler and its retry; and, with the shared vectors in `SDK/go/testdata/vectors.json`, an encrypted delivery, the same without a key, plaintext while keys are set, and a public JWK refused at startup (skipped when the directory is copied out of the repo). The console's 测试 (Test) button sends `event = ping`; the `node -e` recipe above signs a request by hand.
