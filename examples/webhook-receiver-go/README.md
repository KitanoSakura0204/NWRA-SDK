# Webhook 接收端示例（Go）

> English version below.

一个只依赖标准库 + Go SDK 的 Webhook 接收服务：校验 `X-NoWork-Signature`、解密加密投递（JWE）、按事件 `id` 幂等，处理 `game_role.reviewed` / `user.deleted` / `server.offline` / `user.level_changed` / `connection.bound` / `connection.unbound` 并打印日志。可直接作为自己接收端的起点。

## 运行

```bash
cd SDK/examples/webhook-receiver-go
export NOWORK_WEBHOOK_SECRET=whsec_xxx   # 后台「开放平台 → Webhook」新建 Webhook 时显示（只显示一次，可「重置密钥」）
go run .
# 2026/09/29 20:00:00 listening on :8090/nowork/webhook (timestamp tolerance 5m0s, 0 decryption key(s))
```

`go.mod` 通过 `replace ../../go` 直接使用仓库内的 SDK 源码；单独拷走这个目录时，把 `replace` 删掉并 `go get github.com/wangework/nowork-sdk-go` 即可。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `NOWORK_WEBHOOK_SECRET` | 必填 | 端点签名密钥 |
| `LISTEN_ADDR` | `:8090` | 监听地址 |
| `WEBHOOK_PATH` | `/nowork/webhook` | 接收路径（后台端点 URL 需与之一致，例如 `https://hooks.example.com/nowork/webhook`） |
| `WEBHOOK_TOLERANCE` | `300` | 允许的时间戳偏差（秒），`0` 关闭检查 |
| `SEEN_CAPACITY` | `10000` | 幂等缓存保留的事件 id 数量 |
| `NOWORK_DECRYPTION_KEYS` | 空 | 私钥 JWK 文件路径，逗号分隔（轮换期间填新旧两把）；留空表示端点未加密 |

在 Webhook 的「订阅事件」里勾选需要的事件即可；未订阅的事件不会推送，收到未处理的事件本示例也会返回 200（打印 `ignored`）。

## 行为约定

| 情况 | 响应 | 用户中心的处理 |
| --- | --- | --- |
| 签名正确、首次收到 | `200 {"status":"ok"}` | 标记已投递 |
| 同一 `id` 再次收到 | `200 {"status":"duplicate"}` | 标记已投递（不会重复处理） |
| 签名错误 / 时间戳偏差过大 | `401` | 按 30s / 2m / 10m / 30m / 2h / 6h 退避重试 |
| 请求体不是 JSON | `400` | 同上（重试也不会成功，请检查中间代理是否改写了请求体） |
| 处理逻辑抛错 | `500` | 同上，重试时携带相同的 `id` |
| 签名正确但无法解密（缺私钥、密钥不符，或配置了私钥却收到明文） | `500` | 同上；补上私钥后重试即可成功 |
| 请求体超过 1 MiB | `413` | 同上 |

签名是对 `<X-NoWork-Timestamp>.<原始请求体>` 做 HMAC-SHA256，所以**必须用原始字节校验**，不要先反序列化再序列化；反向代理也不能改写请求体。

幂等缓存是进程内 LRU（`lru.go`）。多实例部署时请换成共享存储（Redis `SET NX EX`、数据库唯一索引等），并把 `id` 作为键。

## 加密投递（JWE）

Webhook 绑定的投递配置开启 `webhook.encrypt` 后（后台「开放平台 → 数据投递」，见 `SDK/docs/DELIVERY.md` 第 6 节），请求体变为 `{"id","event","timestamp","jwe"}` 并带 `X-NoWork-Encryption: JWE`。本示例（`jwe.go`）：

1. 先对原始请求体验签（签名覆盖的就是这个外层请求体）；
2. 再用 `nowork.OpenWebhook` 解密，它会核对内层的 `id` / `event` / `timestamp` 与外层一致、受保护头 `ctx` 为 `webhook:<端点 ID>`；
3. 配置了私钥却收到明文时拒绝（`500`），避免把本该加密的数据当明文接收。

私钥只放在接收端，后台只上传公钥 JWK。轮换时先把新私钥加进 `NOWORK_DECRYPTION_KEYS`（`new.jwk,old.jwk`），再在后台启用新公钥；过渡期结束后删掉旧私钥。

## 成对匿名标识（pairwise）

投递配置的 `subject` 为 `pairwise` 时，事件里不再有 `userId` / `uid` / `username` / `ip` / `externalId` / `externalName` / `email` / `phone`，改由 `data.user.sub` 标识账号（同一 `sector` 下稳定、跨应用不可关联）。本示例的 `who()` 两种都能打印；自己的业务应按配置选一种作为本地主键，不要混用。需要 `externalId` 的集成（例如 Discord 身份组同步）要用非 pairwise 的端点。

## 事件字段

`data` 字段以用户中心实际推送为准（见 `docs/PROTOCOL.md` 第 5 节）；本示例用到的事件如下。

```jsonc
// game_role.reviewed — 游戏角色审核完成。status: 0 待审核 1 通过 2 拒绝 3 封禁
{ "roleId": 12, "userId": 100, "serverId": 1, "roleName": "Steve",
  "roleUuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5", "status": 1, "reviewerId": 1 }

// user.deleted — 账号被删除
{ "userId": 100, "uid": 100001, "username": "alice", "reason": "self-service" }

// server.offline — 服务器超过 5 分钟没有心跳
{ "serverId": 1, "code": "survival", "name": "生存服", "lastHeartbeat": "2026-09-14 12:00:00" }

// user.level_changed — 信任等级变化
{ "userId": 100, "uid": 100001, "username": "alice", "from": 1, "to": 2 }

// connection.bound / connection.unbound — 绑定 / 解绑第三方账号（externalName 仅 bound 有）
{ "userId": 100, "uid": 100001, "provider": "discord", "externalId": "80351110224678912", "externalName": "alice" }

// pairwise 配置下，以上事件的账号字段被 data.user 取代，例如：
{ "user": { "sub": "2f1c8e0a-8f3b-8c4d-9a6e-1b2c3d4e5f60" }, "from": 1, "to": 2 }
```

## 本地自测

后台「开放平台 → Webhook」里点“测试”会推送一条 `event = ping`。也可以用 `openssl` 手工签名：

```bash
SECRET=whsec_xxx
TS=$(date +%s)
BODY='{"id":"test-1","event":"game_role.reviewed","timestamp":'$TS',"data":{"roleId":1,"userId":100,"serverId":1,"roleName":"Steve","roleUuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","status":1,"reviewerId":1}}'
SIG=$(printf '%s.%s' "$TS" "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')
curl -i -X POST http://127.0.0.1:8090/nowork/webhook \
  -H 'Content-Type: application/json' \
  -H 'X-NoWork-Event: game_role.reviewed' -H 'X-NoWork-Delivery: 1' \
  -H "X-NoWork-Timestamp: $TS" -H "X-NoWork-Signature: sha256=$SIG" \
  --data-binary "$BODY"
```

第二次发送同一个请求会得到 `{"status":"duplicate"}`；改动任意一个字节会得到 `401`。

## 检查

```bash
go vet ./... && go test ./... && go build .
```

`main_test.go` 直接调用处理函数（不监听端口）：明文投递与重复、错误签名 / 篡改 / 过期 / 非 JSON / 缺 `id` / 非 POST，以及用仓库共享测试向量（`SDK/go/testdata/vectors.json`，X25519）验证加密投递、缺私钥与"配置了私钥却收到明文"三种情况。

---

# Webhook receiver example (Go)

A webhook endpoint that needs only the standard library and the Go SDK. It verifies `X-NoWork-Signature`, opens encrypted deliveries (JWE), is idempotent by event `id`, and logs `game_role.reviewed`, `user.deleted`, `server.offline`, `user.level_changed`, `connection.bound` and `connection.unbound`. Use it as the starting point for your own receiver.

## Run

```bash
cd SDK/examples/webhook-receiver-go
export NOWORK_WEBHOOK_SECRET=whsec_xxx   # shown once when you create the webhook in 开放平台 → Webhook (Open Platform → Webhooks); 重置密钥 rotates it
go run .
```

`go.mod` uses the SDK source in this repo through `replace ../../go`. If you copy the directory elsewhere, delete the `replace` line and run `go get github.com/wangework/nowork-sdk-go`.

| Variable | Default | Meaning |
| --- | --- | --- |
| `NOWORK_WEBHOOK_SECRET` | required | the endpoint's signing secret |
| `LISTEN_ADDR` | `:8090` | listen address |
| `WEBHOOK_PATH` | `/nowork/webhook` | path; the webhook URL in the console must match it |
| `WEBHOOK_TOLERANCE` | `300` | allowed timestamp skew in seconds, `0` turns the check off |
| `SEEN_CAPACITY` | `10000` | event ids kept for de-duplication |
| `NOWORK_DECRYPTION_KEYS` | empty | comma-separated private JWK files (new and old during a rotation); empty means the endpoint is not encrypted |

Subscribe to the events you need on the webhook; unsubscribed events are never sent, and events this example does not handle still get 200 (logged as `ignored`).

## Responses

| Case | Response | What the user centre does |
| --- | --- | --- |
| valid signature, first delivery | `200 {"status":"ok"}` | marks it delivered |
| same `id` again | `200 {"status":"duplicate"}` | marks it delivered (no second run) |
| bad signature or timestamp too far off | `401` | retries after 30s / 2m / 10m / 30m / 2h / 6h |
| body is not JSON, or has no `id` | `400` | same (a retry will not help; check that no proxy rewrites the body) |
| handler error | `500` | same, with the same `id` |
| signed but cannot be opened (no key, wrong key, or plaintext while keys are set) | `500` | same; succeeds once the key is in place |
| body over 1 MiB | `413` | same |

The signature is HMAC-SHA256 over `<X-NoWork-Timestamp>.<raw body>`, so verify the raw bytes: never decode and re-encode first, and make sure no reverse proxy rewrites the body. The de-duplication cache is an in-process LRU (`lru.go`); with several instances use a shared store keyed by `id` (Redis `SET NX EX`, a unique DB column).

## Encrypted deliveries (JWE)

When the webhook's delivery profile turns on `webhook.encrypt` (开放平台 → 数据投递, see `SDK/docs/DELIVERY.md` section 6), the body becomes `{"id","event","timestamp","jwe"}` with `X-NoWork-Encryption: JWE`. `jwe.go` verifies the signature over that outer body first, then opens it with `nowork.OpenWebhook`, which checks that the inner `id` / `event` / `timestamp` repeat the outer ones and that the header `ctx` is `webhook:<endpoint id>`. With keys configured, a plaintext delivery is refused. Keep the private key on the receiver only; the console stores the public JWK. To rotate, add the new private key first (`NOWORK_DECRYPTION_KEYS=new.jwk,old.jwk`), then activate the new public key in the console, and drop the old key after the overlap.

## Pairwise subjects

With `subject: pairwise`, events carry no `userId` / `uid` / `username` / `ip` / `externalId` / `externalName` / `email` / `phone`; `data.user.sub` identifies the account instead (stable within one `sector`, unlinkable across apps). `who()` prints either; pick one as your local key per profile and do not mix them. Integrations that need `externalId` (the Discord role sync, for example) need a non-pairwise endpoint.

## Test and check

```bash
go vet ./... && go test ./... && go build .
```

`main_test.go` calls the handler directly (no listener): a plain delivery and its duplicate; wrong secret, tampered body, stale timestamp, non-JSON, missing `id` and non-POST; and, with the shared SDK vectors (`SDK/go/testdata/vectors.json`, X25519), an encrypted delivery, the same without a key, and a plaintext delivery while keys are set. The admin console's 测试 (Test) button sends `event = ping`; the Chinese section above has an `openssl` + `curl` recipe for a hand-signed request.
