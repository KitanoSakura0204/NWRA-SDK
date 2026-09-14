# Webhook 接收端示例（Go）

一个只依赖标准库 + Go SDK 的 Webhook 接收服务：校验 `X-Lounge-Signature`、按事件 `id` 幂等、处理 `game_role.reviewed` / `user.deleted` / `server.offline` 三类事件并打印日志。可直接作为自己接收端的起点。

## 运行

```bash
cd SDK/examples/webhook-receiver-go
export LOUNGE_WEBHOOK_SECRET=whsec_xxx   # 后台「开发者 → Webhook 事件推送」创建端点时显示（只显示一次，可随时轮换）
go run .
# 2026/09/14 20:00:00 listening on :8090/lounge/webhook (timestamp tolerance 5m0s)
```

`go.mod` 通过 `replace ../../go` 直接使用仓库内的 SDK 源码；单独拷走这个目录时，把 `replace` 删掉并 `go get github.com/wangework/lounge-sdk-go` 即可。

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LOUNGE_WEBHOOK_SECRET` | 必填 | 端点签名密钥 |
| `LISTEN_ADDR` | `:8090` | 监听地址 |
| `WEBHOOK_PATH` | `/lounge/webhook` | 接收路径（后台端点 URL 需与之一致，例如 `https://hooks.example.com/lounge/webhook`） |
| `WEBHOOK_TOLERANCE` | `300` | 允许的时间戳偏差（秒），`0` 关闭检查 |
| `SEEN_CAPACITY` | `10000` | 幂等缓存保留的事件 id 数量 |

后台端点设置里勾选需要的事件即可；未订阅的事件不会推送，收到未处理的事件本示例也会返回 200（打印 `ignored`）。

## 行为约定

| 情况 | 响应 | 用户中心的处理 |
| --- | --- | --- |
| 签名正确、首次收到 | `200 {"status":"ok"}` | 标记已投递 |
| 同一 `id` 再次收到 | `200 {"status":"duplicate"}` | 标记已投递（不会重复处理） |
| 签名错误 / 时间戳偏差过大 | `401` | 按 30s / 2m / 10m / 30m / 2h / 6h 退避重试 |
| 请求体不是 JSON | `400` | 同上（重试也不会成功，请检查中间代理是否改写了请求体） |
| 处理逻辑抛错 | `500` | 同上，重试时携带相同的 `id` |

签名是对 `<X-Lounge-Timestamp>.<原始请求体>` 做 HMAC-SHA256，所以**必须用原始字节校验**，不要先反序列化再序列化；反向代理也不能改写请求体。

幂等缓存是进程内 LRU（`lru.go`）。多实例部署时请换成共享存储（Redis `SET NX EX`、数据库唯一索引等），并把 `id` 作为键。

## 事件字段

`data` 字段以用户中心实际推送为准（见 `docs/PROTOCOL.md` 第 5 节）；本示例用到的三类如下。

```jsonc
// game_role.reviewed — 游戏角色审核完成。status: 0 待审核 1 通过 2 拒绝 3 封禁
{ "roleId": 12, "userId": 100, "serverId": 1, "roleName": "Steve",
  "roleUuid": "069a79f4-44e9-4726-a5be-fca90e38aaf5", "status": 1, "reviewerId": 1 }

// user.deleted — 账号被删除
{ "userId": 100, "uid": 100001, "username": "alice", "reason": "self-service" }

// server.offline — 服务器超过 5 分钟没有心跳
{ "serverId": 1, "code": "survival", "name": "生存服", "lastHeartbeat": "2026-09-14 12:00:00" }
```

## 本地自测

后台「Webhook 事件推送」里点“测试”会推送一条 `event = ping`。也可以用 `openssl` 手工签名：

```bash
SECRET=whsec_xxx
TS=$(date +%s)
BODY='{"id":"test-1","event":"game_role.reviewed","timestamp":'$TS',"data":{"roleId":1,"userId":100,"serverId":1,"roleName":"Steve","roleUuid":"069a79f4-44e9-4726-a5be-fca90e38aaf5","status":1,"reviewerId":1}}'
SIG=$(printf '%s.%s' "$TS" "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | sed 's/^.* //')
curl -i -X POST http://127.0.0.1:8090/lounge/webhook \
  -H 'Content-Type: application/json' \
  -H 'X-Lounge-Event: game_role.reviewed' -H 'X-Lounge-Delivery: 1' \
  -H "X-Lounge-Timestamp: $TS" -H "X-Lounge-Signature: sha256=$SIG" \
  --data-binary "$BODY"
```

第二次发送同一个请求会得到 `{"status":"duplicate"}`；改动任意一个字节会得到 `401`。

## 检查

```bash
go vet ./... && go build .
```
