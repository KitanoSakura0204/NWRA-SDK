# Discord 身份组同步示例（Node.js / TypeScript）

> English version below.

一个 Webhook 接收端：用户在休息室账号里绑定 Discord 后，自动在你的 Discord 服务器里给他发身份组；信任等级变化时按等级调整；解绑或注销时收回。只用 Node 自带模块和 Discord REST API（`fetch`），没有运行期依赖；验签、解密（JWE）和幂等直接复用 `../webhook-receiver-node/src/receiver.ts`。

| 事件 | 动作 |
| --- | --- |
| `connection.bound`（`provider = discord`） | 发 `LINKED_ROLE_ID`，并按信任等级发 `LEVEL_ROLES` |
| `user.level_changed` | 按新等级增减 `LEVEL_ROLES` |
| `connection.unbound` / `user.deleted` | 收回本示例管理的全部身份组 |

只会增删 `LINKED_ROLE_ID` 与 `LEVEL_ROLES` 里列出的身份组，其他身份组不受影响。

## 目录

```
discord-role-sync/
├── src/main.ts               入口：读取配置、监听端口、/healthz
├── src/config.ts             环境变量解析与校验（启动时出错即退出）
├── src/sync.ts               同步逻辑：事件 -> 应有的身份组（幂等、按账号串行、忽略过期事件）
├── src/discord.ts            Discord REST：加 / 删成员身份组（限流重试一次、不在服务器内时跳过）
├── src/store.ts              账号 -> Discord 账号的对应关系（links.json，原子写入）
├── scripts/send-event.mjs    手工签名并发送一条事件（本地自测，其他示例也能用）
└── tests/sync.test.mjs       node:test 测试（不联网，Discord 用内存替身）
```

单独拷走本目录时要连同 `../webhook-receiver-node/src/{receiver,lru}.ts` 一起拷走，并把导入改成 `@lounge/sdk`（见该示例的 README）。

## 配置步骤

1. **Discord 应用与机器人**：在 Discord Developer Portal 新建应用 → Bot → Reset Token，得到 `DISCORD_BOT_TOKEN`。用 OAuth2 URL Generator 勾选 scope `bot` 与权限 `Manage Roles` 邀请进服务器。在服务器「身份组」设置里把机器人自己的身份组**拖到要管理的身份组之上**，否则 Discord 返回 403。
2. **拿到 ID**：Discord 设置 → 高级 → 开启开发者模式；右键服务器「复制服务器 ID」得到 `DISCORD_GUILD_ID`，右键身份组「复制身份组 ID」。
3. **让用户能绑定 Discord**：后台「开放平台 → 登录身份源」启用 Discord 预设（代码 `discord`），用户即可在账号里绑定 Discord。
4. **Webhook**：后台「开放平台 → Webhook」新建端点，URL 指向本服务（例如 `https://bot.example.com/lounge/webhook`），订阅 `connection.bound` / `connection.unbound` / `user.level_changed` / `user.deleted`，保存后复制签名密钥。这个端点**不能**绑定 `subject = pairwise` 的投递配置：pairwise 事件不带 `userId` / `externalId`，无法对应到 Discord 账号（本示例会记录错误并跳过）。
5. **可选：绑定时就按等级发身份组**：`connection.bound` 本身不带信任等级。给端点绑定一个投递配置（「开放平台 → 数据投递」），在 `webhook` 规则里下发 `security_level`，事件里就会有 `data.user.security_level`，绑定当下即可发等级身份组；不配置的话，等级身份组要等下一次 `user.level_changed` 才会发放。

## 运行

需要 Node 20+。

```bash
cd SDK/examples/discord-role-sync
npm install && npm run build
export LOUNGE_WEBHOOK_SECRET=whsec_xxx
export DISCORD_BOT_TOKEN=xxx DISCORD_GUILD_ID=300000000000000001
export LINKED_ROLE_ID=100000000000000001                              # 已绑定
export LEVEL_ROLES=2:100000000000000002,3:100000000000000003          # 等级 2 及以上、3 及以上
npm start
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `LOUNGE_WEBHOOK_SECRET` | 必填 | 端点签名密钥 |
| `DISCORD_BOT_TOKEN` | 必填（`DRY_RUN=1` 时可省） | 机器人令牌，只放在环境变量里 |
| `DISCORD_GUILD_ID` | 必填 | 服务器 ID |
| `LINKED_ROLE_ID` | 空 | 所有已绑定账号都有的身份组 |
| `LEVEL_ROLES` | 空 | `等级:身份组ID`，逗号分隔；每个身份组从该等级起一直持有（累加）。与 `LINKED_ROLE_ID` 至少填一个 |
| `LINKS_FILE` | `links.json` | 账号对应关系的存放位置 |
| `DRY_RUN` | 空 | `1`：只打印将要做的改动，不调用 Discord |
| `HOST` / `PORT` | `0.0.0.0` / `8093` | 监听地址 |
| `WEBHOOK_PATH` | `/lounge/webhook` | 接收路径 |
| `WEBHOOK_TOLERANCE` / `SEEN_CAPACITY` | `300` / `10000` | 时间戳容差（秒）/ 幂等缓存大小 |
| `LOUNGE_DECRYPTION_KEYS` | 空 | 端点开启加密投递时的私钥 JWK 文件，逗号分隔 |

## 行为与限制

- **幂等**：每次都把本示例管理的身份组设成"应有状态"（逐个 PUT / DELETE，Discord 侧两者都幂等），重复投递无害。
- **失败重试**：Discord 调用失败时返回 `500`，用户中心按 30s / 2m / 10m / 30m / 2h / 6h 重试；`links.json` 只在调用成功后才更新。
- **过期事件**：按账号记录最后一次生效事件的时间戳，更早的事件（例如解绑之后才到达的、之前失败的绑定重试）直接忽略，不会把身份组发回去。同一账号的事件串行处理，不会交错。
- **等级未知**：还没收到过等级时只处理 `LINKED_ROLE_ID`，不动等级身份组。
- **不在服务器内**：用户绑定时还没加入 Discord 服务器，Discord 返回 Unknown Member，本次跳过；等他加入后的下一次事件（重新绑定、等级变化）才会发放。要在"加入服务器"时立即发放，需要一个连接 Gateway 的机器人（`GUILD_MEMBER_ADD`，特权 intent），不在本示例范围内。
- **已有的绑定**：机器人只从 `connection.bound` 学到对应关系。上线前已经绑定的用户，让他们重新绑定一次，或按 `{"<userId>": {"discordId": "<Discord 用户 ID>", "at": 0}}` 预先写入 `links.json`。

## 安全

- Webhook 端点靠签名（HMAC-SHA256，可叠加 JWE）鉴权，签名不对一律 `401`；`/healthz` 不鉴权，只返回 `{"ok":true}`。请放在 HTTPS 反向代理之后。
- 机器人令牌只放在环境变量里；机器人只需要 `Manage Roles`，而且 Discord 只允许它管理排在它自己身份组之下的身份组。
- `links.json` 记录账号 ID 与 Discord 用户 ID 的对应关系，属于个人数据：已在 `.gitignore` 中排除，文件权限请只给服务账号；账号注销后只保留一条不含 Discord ID 的时间戳记录（用于挡住过期事件）。

## 测试与本地自测

```bash
npm test   # tsc 编译后运行 tests/sync.test.mjs，不联网
```

覆盖：绑定 / 等级升降 / 解绑；等级未知；过期重试；更换 Discord 账号；注销；忽略其他身份源、pairwise 与无关事件；Discord 失败后重试且存储不变；同一账号串行；`links.json` 持久化与损坏；启动配置校验；Discord REST 的 204 / 不在服务器 / 限流重试 / 403 / 未知身份组 / 非法 ID；身份组规划。

不连 Discord 的演练（两个终端）：

```bash
DRY_RUN=1 LOUNGE_WEBHOOK_SECRET=whsec_test DISCORD_GUILD_ID=300000000000000001 \
  LINKED_ROLE_ID=100000000000000001 LINKS_FILE=/tmp/links.json npm start

LOUNGE_WEBHOOK_SECRET=whsec_test node scripts/send-event.mjs connection.bound \
  '{"userId":100,"uid":100001,"provider":"discord","externalId":"200000000000000001","externalName":"alice"}'
# 200 {"status":"ok"}；机器人打印 [dry run] add role 100000000000000001 to 200000000000000001 (Lounge account linked)
```

---

# Discord role sync example (Node.js / TypeScript)

A webhook endpoint that keeps Discord roles in step with Lounge Account: when a user links Discord, their Discord account gets roles in your server; when their trust level changes the level roles follow; when they unlink or delete the account the roles are removed. Node built-ins and the Discord REST API (`fetch`) only, no runtime dependencies; signature, JWE and idempotency come from `../webhook-receiver-node/src/receiver.ts` (copy `receiver.ts` and `lru.ts` along if you move this directory, and import from `@lounge/sdk`).

`connection.bound` (provider `discord`) grants `LINKED_ROLE_ID` plus the `LEVEL_ROLES` the trust level earns; `user.level_changed` adjusts the level roles; `connection.unbound` and `user.deleted` remove every managed role. Only the configured roles are ever touched.

## Setup

1. Discord Developer Portal: new application, Bot, Reset Token (`DISCORD_BOT_TOKEN`). Invite it with scope `bot` and the `Manage Roles` permission, then drag the bot's own role **above** the roles it manages (otherwise Discord answers 403).
2. Turn on Developer Mode in Discord; right-click the server for `DISCORD_GUILD_ID` and each role for its id.
3. In the admin console enable the Discord preset (code `discord`) under 开放平台 → 登录身份源 (Open Platform → Identity Providers) so users can link Discord.
4. Under 开放平台 → Webhook create an endpoint pointing at this service, subscribe `connection.bound`, `connection.unbound`, `user.level_changed`, `user.deleted`, and copy the signing secret. Do **not** bind it to a `subject: pairwise` delivery profile: pairwise events carry no `userId` / `externalId`, so nothing maps to Discord (the bot logs an error and skips them).
5. Optional: `connection.bound` carries no trust level. Bind the endpoint to a delivery profile (开放平台 → 数据投递) whose `webhook` rules include `security_level` and the event carries `data.user.security_level`, so level roles are granted at link time; otherwise they follow the next `user.level_changed`.

Node 20+: `npm install && npm run build && npm start`. Variables: `LOUNGE_WEBHOOK_SECRET`, `DISCORD_BOT_TOKEN` (not needed with `DRY_RUN=1`), `DISCORD_GUILD_ID` (required); `LINKED_ROLE_ID`, `LEVEL_ROLES` (`level:roleId,...`, each role held from its level upwards; set at least one of the two); `LINKS_FILE` (`links.json`), `DRY_RUN` (`1` logs instead of calling Discord), `HOST` / `PORT` (`0.0.0.0` / `8093`), `WEBHOOK_PATH`, `WEBHOOK_TOLERANCE`, `SEEN_CAPACITY`, `LOUNGE_DECRYPTION_KEYS`.

## Behaviour, limits, security

- Every change converges on the desired state of the managed roles (idempotent PUT / DELETE), so redeliveries are harmless. A Discord failure answers `500` and the user centre retries; `links.json` changes only after Discord succeeded.
- Each account remembers the timestamp of the last event applied; older events (a late retry) are ignored, so a delayed `connection.bound` cannot re-grant roles after an unlink. Events for one account run one at a time.
- Unknown trust level: only `LINKED_ROLE_ID` is handled. A user not yet in the server (Discord: Unknown Member) is skipped and gets the roles at their next event; granting on join needs a Gateway bot (`GUILD_MEMBER_ADD`), out of scope here. Links made before the bot ran are unknown: ask users to re-link, or seed `links.json` with `{"<userId>": {"discordId": "<Discord user id>", "at": 0}}`.
- The webhook is authenticated by its signature (HMAC-SHA256, optionally JWE); `/healthz` is open and returns only `{"ok":true}`. Serve it behind HTTPS. Keep the bot token in the environment. `links.json` maps account ids to Discord ids (personal data): it is git-ignored, keep it readable by the service user only; a deleted account keeps a tombstone with no Discord id.

## Test

`npm test` compiles and runs `tests/sync.test.mjs` offline (Discord is an in-memory fake): link, level up and down, unlink; unknown level; a stale retry; switching Discord accounts; deletion; other providers, pairwise and unrelated events ignored; a Discord failure retried with the store unchanged; per-account ordering; `links.json` persistence and corruption; start-up config checks; the REST client on 204 / Unknown Member / 429 / 403 / Unknown Role / a bad id; the role plan. For a dry run against a real listener see the two-terminal recipe above (`DRY_RUN=1` plus `scripts/send-event.mjs`).
