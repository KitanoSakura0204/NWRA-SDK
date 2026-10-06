# 游戏服务器示例（Go）

> English version below.

一个最小的游戏服务器，用 Go SDK（`../../go`）接入休息室账号的全部服务器侧能力：

- 白名单：启动时与每 5 分钟全量同步，玩家加入时实时校验；用户中心不可达时按 `OUTAGE_POLICY` 降级（`cache` 用上次同步的名单，`deny` 全部拒绝，`allow` 全部放行）。
- 加密响应（DELIVERY.md §5）：配置私钥后自动解密并核对 `ctx`（`server:<服务器代码>:<路由>`），此时明文响应同样拒绝（防降级）；无法打开的加密响应一律拒绝，任何降级策略都不会放行。
- 心跳（PROTOCOL §2.3）：每 60 秒上报在线人数、TPS（按实际 tick 计算）与在线玩家列表。
- 服务器封禁（[SERVER_USERS.md](../../docs/SERVER_USERS.md)）：每次心跳后拉取踢出列表，断开刚被封禁的在线玩家；进服被拒时显示封禁原因（`message`）。
- GM 面板（PROTOCOL §2.4）：启用 `server-gm` 模块时发布类型化操作清单（`player` / `number` / `select` 参数、分类、危险等级、多语言标题）和具名选项列表 `worlds`，按 `GM_POLL_SECONDS` 拉取命令并回执；`stop` 为 `critical` 操作。
- Webhook（PROTOCOL §5）：校验签名、解密加密投递、按事件 ID 去重；账号被冻结、自行锁定或删除时立即踢出在线玩家，游戏角色审核与信任等级变化后重新同步白名单。完整事件处理见 `../webhook-receiver-go`。
- SDK 登录页（PROTOCOL §6）：`game-server-go login` 演示游戏客户端一侧：PKCE S256、本机回环回调或登录码（code-display）、换取令牌并读取 userinfo。

## 运行

```bash
export NOWORK_BASE=https://account.example.com
export NOWORK_SERVER_KEY=lsk_xxx                 # 后台「游戏服务 → 服务器管理」生成，只显示一次
export NOWORK_SERVER_CODE=survival               # 用于核对加密信封的 ctx 与登录页地址
go run .
```

控制台输入 `join Steve 069a79f4-44e9-4726-a5be-fca90e38aaf5`、`leave Steve`、`list`、`quit` 模拟玩家进出。

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `NOWORK_BASE` | `http://localhost:8081` | 用户中心地址（即 OIDC issuer） |
| `NOWORK_SERVER_KEY` | 必填 | 服务器密钥 |
| `NOWORK_SERVER_CODE` | 空 | 服务器代码；为空时只核对 `ctx` 的类型与路由 |
| `NOWORK_DECRYPTION_KEYS` | 空 | 私钥 JWK 文件，逗号分隔；轮换期间同时列出新旧两把 |
| `OUTAGE_POLICY` | `cache` | `cache` / `deny` / `allow` |
| `MAX_PLAYERS` | `20` | 最大人数 |
| `HEARTBEAT_SECONDS` / `GM_POLL_SECONDS` / `WHITELIST_SYNC_MINUTES` | `60` / `15` / `5` | 定时任务间隔 |
| `NOWORK_WEBHOOK_SECRET` | 空 | 设置后在 `WEBHOOK_ADDR`（默认 `127.0.0.1:8095`）的 `WEBHOOK_PATH`（默认 `/nowork/webhook`）接收 Webhook |

Webhook 端点应订阅 `user.frozen`、`user.locked`、`user.deleted`、`game_role.reviewed`、`user.level_changed`，并绑定 public 投递规则（pairwise 端点不带 `uid`，无法对应在线玩家）。对外暴露时请放在 HTTPS 反向代理之后。

### 游戏客户端登录

```bash
export NOWORK_CLIENT_ID=game-client              # 后台「游戏服务 → SDK 登录页」为该服务器接入的公开客户端
export NOWORK_REDIRECT_URI=http://127.0.0.1:47823/cb   # 与登记的回调地址逐字符一致；留空则使用登录码模式
go run . login
```

## 测试

```bash
go vet ./... && go test ./...
```

测试不访问网络：白名单与降级、加密响应（共享向量 `../../go/testdata/vectors.json`，含路由互换、服务器代码不符、错误私钥与明文降级）、GM 命令、Webhook 踢人与去重、加密投递。

---

# Game server example (Go)

A minimal game server using the Go SDK for every server-side feature of NoWork Account: whitelist sync and join checks with an outage policy (`cache`, `deny`, `allow`); encrypted replies opened with `NOWORK_DECRYPTION_KEYS` and checked against `server:<code>:<route>` (plain replies are refused once keys are set, and an encrypted reply that cannot be opened always rejects); heartbeat with measured TPS and the player list, then the per-server ban kick list (SERVER_USERS.md; the join check shows the ban reason); the GM channel (typed catalogue, the `worlds` option list, polling and results); webhooks that kick frozen, locked or deleted accounts at once and resync the whitelist after reviews; and `game-server-go login`, the game-client side of the SDK login page (PKCE S256, loopback redirect or code-display, token exchange, userinfo).

Run with `NOWORK_BASE`, `NOWORK_SERVER_KEY` and `NOWORK_SERVER_CODE` set, then `go run .`; type `join <name> [uuid]`, `leave <name>`, `list`, `quit`. The table above lists every variable. Subscribe the webhook endpoint to `user.frozen`, `user.locked`, `user.deleted`, `game_role.reviewed` and `user.level_changed` with a public delivery profile, and put it behind an HTTPS reverse proxy. Tests: `go vet ./... && go test ./...` (no network; shared vectors).
