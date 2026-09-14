# Webhook 接收端示例（Node.js / TypeScript）

只用 Node 自带的 `node:http`（不依赖 Express），通过相对路径直接引用仓库内的 TypeScript SDK 源码做签名校验；按事件 `id` 幂等；处理 `game_role.reviewed` / `user.deleted` / `server.offline`。

## 目录

```
webhook-receiver-node/
├── src/server.ts   接收端（校验、幂等、事件分发）
├── src/lru.ts      进程内 LRU 幂等缓存
├── tsconfig.json   rootDir 指向仓库根，因此会把 ../../typescript/src 一起编译进 dist/
└── package.json    build / start 脚本
```

`src/server.ts` 里 `import ... from '../../../typescript/src/index.js'` 指向 SDK 源码；编译后对应 `dist/typescript/src/index.js`，所以运行时不需要安装 `@lounge/sdk`。若单独拷走本目录，把导入改成 `from '@lounge/sdk'` 并 `npm i @lounge/sdk`，`tsconfig.json` 的 `rootDir` 改回 `src`、`include` 只保留 `src` 即可。

## 运行

需要 Node 18+（全局 `fetch` / WebCrypto）。

```bash
cd SDK/examples/webhook-receiver-node
npm install            # 只装 typescript 与 @types/node（开发期）
npm run build          # tsc -> dist/
export LOUNGE_WEBHOOK_SECRET=whsec_xxx   # 后台「开发者 → Webhook 事件推送」创建端点时显示（只显示一次）
node dist/examples/webhook-receiver-node/src/server.js
# listening on http://0.0.0.0:8091/lounge/webhook (timestamp tolerance 300s)
```

`npm start` 等价于最后一行。生产环境建议用 `pm2` / `systemd` 托管并放在反向代理（HTTPS）之后。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LOUNGE_WEBHOOK_SECRET` | 必填 | 端点签名密钥 |
| `HOST` / `PORT` | `0.0.0.0` / `8091` | 监听地址 |
| `WEBHOOK_PATH` | `/lounge/webhook` | 接收路径（后台端点 URL 需与之一致） |
| `WEBHOOK_TOLERANCE` | `300` | 允许的时间戳偏差（秒），`0` 关闭检查 |
| `SEEN_CAPACITY` | `10000` | 幂等缓存保留的事件 id 数量 |

## 行为约定

| 情况 | 响应 | 用户中心的处理 |
| --- | --- | --- |
| 签名正确、首次收到 | `200 {"status":"ok"}` | 标记已投递 |
| 同一 `id` 再次收到 | `200 {"status":"duplicate"}` | 标记已投递（不重复处理） |
| 签名错误 / 时间戳偏差过大 | `401` | 按 30s / 2m / 10m / 30m / 2h / 6h 退避重试 |
| 请求体不是 JSON | `400` | 同上 |
| 处理逻辑抛错 | `500` | 同上，重试时 `id` 不变 |

要点：

- 签名覆盖 `<X-Lounge-Timestamp>.<原始请求体>`，所以要把原始 `Buffer` 交给 `verifyWebhook`，不要经过任何 body parser 再序列化。
- `verifyWebhook` 直接接受 `req.headers`（Node 会把头名转成小写，SDK 按不区分大小写查找）。
- 幂等缓存是进程内 LRU；多实例部署时换成 Redis `SET NX EX` 或数据库唯一索引。
- 处理逻辑必须在 10 秒内返回；耗时任务先入队再响应。

## 事件字段

```jsonc
// game_role.reviewed — status: 0 待审核 1 通过 2 拒绝 3 封禁
{ "roleId": 12, "userId": 100, "serverId": 1, "roleName": "Steve",
  "roleUuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5", "status": 1, "reviewerId": 1 }
// user.deleted
{ "userId": 100, "uid": 100001, "username": "alice", "reason": "self-service" }
// server.offline
{ "serverId": 1, "code": "survival", "name": "生存服", "lastHeartbeat": "2026-09-14 12:00:00" }
```

完整事件列表见 `docs/PROTOCOL.md` 第 5 节。

## 本地自测

后台端点页面的“测试”按钮会推送 `event = ping`。也可以自己签名一条：

```bash
node -e '
const c=require("crypto");const s=process.env.LOUNGE_WEBHOOK_SECRET;const ts=Math.floor(Date.now()/1000);
const body=JSON.stringify({id:"test-1",event:"user.deleted",timestamp:ts,data:{userId:100,uid:100001,username:"alice",reason:"self-service"}});
const sig="sha256="+c.createHmac("sha256",s).update(ts+"."+body).digest("hex");
fetch("http://127.0.0.1:8091/lounge/webhook",{method:"POST",headers:{"Content-Type":"application/json","X-Lounge-Event":"user.deleted","X-Lounge-Delivery":"1","X-Lounge-Timestamp":String(ts),"X-Lounge-Signature":sig},body}).then(async r=>console.log(r.status,await r.text()));'
```

再发一次同样的 `id` 会得到 `{"status":"duplicate"}`；改动请求体任意字节会得到 `401`。`discord-role-sync/scripts/send-event.mjs` 提供了更完整的发送工具，也可用于本示例。
