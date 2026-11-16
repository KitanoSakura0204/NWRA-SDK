# 游戏服务器示例（Go）

> English version below.

一个最小的游戏服务器，用 Go SDK（`../../go`）接入休息室账号的全部服务器侧能力：

- 白名单：启动时与每 5 分钟全量同步，玩家加入时实时校验；用户中心不可达时按 `OUTAGE_POLICY` 降级（`cache` 用上次同步的名单，`deny` 全部拒绝，`allow` 全部放行）。
- 加密响应（DELIVERY.md §5）：配置私钥后自动解密并核对 `ctx`（`server:<服务器代码>:<路由>`），此时明文响应同样拒绝（防降级）；无法打开的加密响应一律拒绝，任何降级策略都不会放行。
- 心跳（PROTOCOL §2.3）：每 60 秒上报在线人数、TPS（按实际 tick 计算）与在线玩家列表。
- 服务器封禁（[SERVER_USERS.md](../../docs/SERVER_USERS.md)）：每次心跳后拉取踢出列表，断开刚被封禁的在线玩家；进服被拒时显示封禁原因（`message`）。
- GM 面板（PROTOCOL §2.4）：启用 `server-gm` 模块时发布类型化操作清单（`player` / `number` / `select` 参数、分类、危险等级、多语言标题）和具名选项列表 `worlds`，按 `GM_POLL_SECONDS` 拉取命令并回执；`stop` 为 `critical` 操作。
- Webhook（PROTOCOL §5）：校验签名、解密加密投递、按事件 ID 去重；账号被冻结、自行锁定或删除时立即踢出在线玩家，游戏角色审核与信任等级变化后重新同步白名单。完整事件处理见 `../webhook-receiver-go`。
- SDK 登录页（PROTOCOL §6）：`game-server-go login` 演示游戏客户端一侧：PKCE S256、独立 nonce、本机回环回调或登录码（code-display）、验证签名 ID Token 后读取 UserInfo 并核对 `sub`。

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

需要 Go 1.21+。从 SDK 仓库根目录执行 `cd examples/game-server-go`；`go.mod` 中的 `replace ... => ../../go` 始终使用本仓库更新后的 SDK，无需依赖新版本已公开发布。不要单独复制示例目录后仍保留失效的相对替换路径。

```bash
export NOWORK_BASE=https://account.example.com   # 换成实际 NoWork issuer
export NOWORK_SERVER_CODE=survival              # 换成已配置 SDK 登录页的服务器代码
export NOWORK_CLIENT_ID=game-client              # 后台「游戏服务 → SDK 登录页」为该服务器接入的公开客户端
export NOWORK_REDIRECT_URI=http://127.0.0.1:47823/cb   # 与登记的回调地址逐字符一致；留空则使用登录码模式
go run . login
```

PowerShell：

```powershell
Set-Location D:\Project\Admin\SDK\examples\game-server-go
$env:NOWORK_BASE = 'https://account.example.com'
$env:NOWORK_SERVER_CODE = 'survival'
$env:NOWORK_CLIENT_ID = 'game-client'
$env:NOWORK_REDIRECT_URI = 'http://127.0.0.1:47823/cb'
go run . login
```

这些值都是待替换的配置示例，没有真实凭据。`login` 子命令不需要 `NOWORK_SERVER_KEY`，也不发送 `NOWORK_CLIENT_SECRET`；它使用公开客户端和 S256 PKCE，与默认游戏服务器模式的服务器密钥认证分开。`NOWORK_FRONTEND` 可指定独立的用户中心前端地址（默认 `NOWORK_BASE`），`NOWORK_UI_LOCALES` 可选。请求范围为 `openid profile`。

登录前使用 `Discover` 校验 issuer；换码后通过固定的 `/jwks.json` 获取公开签名密钥，调用 `VerifyIDTokenWithNonce` 校验 RS256 签名、issuer、audience、时间和独立 nonce，再校验非空 subject、`azp`（多 audience 时必需）以及 `UserInfo.sub` 一致。ID Token 已过期时不调用 UserInfo。全部成功后终端才显示 `Signed in`；回环页面收到 code 只提示返回游戏继续验证。令牌不写入日志或保存为会话。

本示例支持**签名 RS256 ID Token + 明文 JSON UserInfo**。不支持加密 ID Token、签名/加密 UserInfo、刷新令牌或持久会话；启用相应 Delivery 配置前需另行接入解密与校验（SDK 提供 `DecryptIDTokenWithNonce`，仍会验证内部签名）。回环模式最多等待五分钟；登录码模式等待终端输入，由服务端检查登录码有效期。示例不负责真实游戏会话与玩家连接的绑定、退出和撤销。部署时应使用 HTTPS issuer，核实公开客户端、回调地址、服务器绑定与 Delivery 配置。

## 测试

```bash
go vet ./... && go test ./...
```

测试仅使用回环 mock 服务，不访问外部网络或真实登录：白名单与降级、加密响应（共享向量 `../../go/testdata/vectors.json`，含路由互换、服务器代码不符、错误私钥与明文降级）、GM 命令、Webhook 踢人与去重、加密投递，以及登录成功、nonce/签名/issuer/audience/有效期/subject 错误和 UserInfo subject 不一致。未声称通过真实部署认证验证。

---

# Game server example (Go)

A minimal game server using the Go SDK for every server-side feature of NoWork Account: whitelist sync and join checks with an outage policy (`cache`, `deny`, `allow`); encrypted replies opened with `NOWORK_DECRYPTION_KEYS` and checked against `server:<code>:<route>` (plain replies are refused once keys are set, and an encrypted reply that cannot be opened always rejects); heartbeat with measured TPS and the player list, then the per-server ban kick list (SERVER_USERS.md; the join check shows the ban reason); the GM channel (typed catalogue, the `worlds` option list, polling and results); webhooks that kick frozen, locked or deleted accounts at once and resync the whitelist after reviews; and `game-server-go login`, the game-client side of the SDK login page (PKCE S256, independent nonce, loopback redirect or code-display, verified ID Token and matching UserInfo subject).

Run with `NOWORK_BASE`, `NOWORK_SERVER_KEY` and `NOWORK_SERVER_CODE` set, then `go run .`; type `join <name> [uuid]`, `leave <name>`, `list`, `quit`. The table above lists the server variables. Subscribe the webhook endpoint to `user.frozen`, `user.locked`, `user.deleted`, `game_role.reviewed` and `user.level_changed` with a public delivery profile, and put it behind an HTTPS reverse proxy.

For client login, enter `examples/game-server-go` from the SDK repository root, set `NOWORK_BASE`, `NOWORK_SERVER_CODE`, `NOWORK_CLIENT_ID`, and optionally `NOWORK_REDIRECT_URI`, then run `go run . login` (commands above). `NOWORK_FRONTEND` defaults to `NOWORK_BASE`; `NOWORK_UI_LOCALES` is optional. Omit the redirect for code-display mode. This subcommand uses a public client and requires no server key or client secret. Go 1.21+ and the local `../../go` module replacement are required; the commands do not assume a new SDK package has been published.

Login validates discovery, fetches public JWKS, calls `VerifyIDTokenWithNonce`, checks a nonempty subject and authorized party, rejects expired tokens, then requires matching `UserInfo.sub`. Only verified results are reported as signed in. Configure signed RS256 ID Tokens and plain JSON UserInfo; encrypted ID Tokens and signed/encrypted UserInfo need a separate implementation (the SDK provides `DecryptIDTokenWithNonce`). The example does not retain tokens, create a persistent game session, refresh tokens, or implement logout/revocation. Loopback callbacks time out after five minutes; code-display waits for terminal input and the provider enforces code expiry. Production game sessions must bind the verified identity to the correct player connection.

Tests: run `go vet ./...` and `go test ./...`. They use shared vectors and loopback mock providers, including generated signing keys and negative login cases. No real credentials or live provider authentication are used or claimed.
