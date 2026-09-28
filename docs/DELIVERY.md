# 数据投递（DELIVERY）

不同应用 / 游戏需要的用户数据不一样，有的还要求"加密后由服务端解密"。功能模块 `delivery`（后台「开放平台 → 数据投递」，权限 `delivery:view` / `delivery:manage`）按**投递配置**决定：下发哪些属性、字段叫什么名字、附加哪些固定值、用公开还是成对匿名的账号标识，以及是否用应用的公钥加密（JWE）。

**没有绑定投递配置的应用、服务器与 Webhook，输出与本功能上线前逐字节相同**（后端 `golden_test.go` 证明）。

## 1. 概念

| 概念 | 说明 |
| --- | --- |
| 属性 | 用户中心能下发的一项数据，带敏感级别：`public`（公开）/ `personal`（个人）/ `sensitive`（敏感）。敏感属性必须在配置里「放行敏感属性」中逐项允许才会下发。实名的证件号码与真实姓名**不是属性**，任何配置都拿不到 |
| 投递配置 | 一份 JSON：`subject`、`sector`、`allowSensitive`，以及 `oidc` / `server` / `webhook` 三个渠道各自的属性规则（`attr` → `claim`）与固定值 |
| 绑定 | 一个配置可绑定多个 OAuth 客户端、游戏服务器、Webhook 端点；每个对象最多绑定一个配置 |
| 自定义属性 | 应用自己按用户写入的键值（`entitlement.<键>` / `entitlements`），见 §8 |

内置属性：`sub` `uid` `username` `nickname` `avatar` `alias` `gender` `birthdate` `updated_at` `created_at` `region` `email` `email_verified` `phone`（脱敏，如 `138****0000`）`phone_number`（完整，敏感）`phone_number_verified` `security_level` `security_level_name` `realname_verified` `realname_region`（证件签发地区，敏感）`realname_doc_type`（证件类型，敏感）`game_roles` `game_roles_here`（仅本配置绑定的服务器上的角色）`entitlements`，以及社区资料 `profile_*`（只在管理员设为公开、用户也未隐藏时下发）。其他模块可在 `init()` 里 `delivery.Register(...)` 注册自己的属性，无需改核心代码。

`gender`、`birthdate` 与 `profile_*` 遵循用户的资料可见性：用户设为"仅自己可见"、或管理员未在资料设置里设为公开的字段，**不会下发给任何应用**（OIDC 内置输出同样如此）。生日按管理员的显示方式：只显示月日时为 OIDC 省略年份的 `0000-MM-DD`，只显示年龄时不下发。

## 2. 账号标识：public 与 pairwise

- `public`（默认）：`sub` 为账号的 OpenID，所有应用看到的相同。
- `pairwise`（OIDC Core §8.1）：`sub = UUIDv8(HMAC-SHA256(部署盐, sector ‖ 0x00 ‖ OpenID)[0:16])`，不同扇区的应用无法凭 `sub` 关联同一用户。同一游戏的客户端与服务器应设置相同的 `sector`，这样 SDK 登录拿到的 `sub` 与服务器插件看到的一致；不设则每个配置单独成一个扇区。
- pairwise 配置下，访问令牌与 introspection 不再携带 `uid` / `username`；Webhook 事件去掉 `userId` `uid` `username` `ip` `externalId` `externalName` `email` `phone`，改由 `data.user.sub` 标识用户；自定义属性接口只接受 `sub`。
- 若在 pairwise 配置里再放行 `uid`、`username`、`email` 等全局标识，后台会提示"可被跨应用关联"。
- 部署盐加密存放、一经生成不再改变；若 `jwt.secret` 变更导致无法解密，成对投递会停止（报错）而不是悄悄换出一套新 `sub`。

## 3. 加密算法与公钥

| 项 | 取值 |
| --- | --- |
| 密钥管理 `alg` | `RSA-OAEP-256`（RSA 2048–8192 位）或 `ECDH-ES+A256KW`（EC P-256 或 X25519，RFC 8037） |
| 内容加密 `enc` | `A256GCM` |
| 格式 | JWE Compact Serialization（RFC 7516），`kid` 在受保护头里 |

应用在后台上传**公钥 JWK**（私钥、对称密钥与 JWK Set 会被拒绝）。未写 `kid` 时取 RFC 7638 指纹（X25519 按 RFC 8037 §2）。后台的"测试解密"会用该公钥加密一段样例，管理员在浏览器里粘贴私钥解密核对，私钥不会上传。

## 4. OIDC

按 OIDC Dynamic Client Registration §2 的字段名写在配置的 `oidc` 里：

| 字段 | 效果 |
| --- | --- |
| `id_token_encrypted_response_alg` / `_enc` | ID Token 先签名（RS256）再加密：JWE 头 `cty: "JWT"`，解密后照常验签 |
| `userinfo_signed_response_alg` | `RS256`：userinfo 返回 `application/jwt` 的签名 JWT，含 `iss`、`aud` |
| `userinfo_encrypted_response_alg` / `_enc` | userinfo 返回 `application/jwt` 的 JWE（与签名同时开启时为先签后加密） |

两者需使用同一种 `alg`（同一时间只有一把生效的公钥）。模块启用时，发现文档增加 `subject_types_supported: ["public","pairwise"]` 与 `id_token_encryption_*`、`userinfo_signing_*`、`userinfo_encryption_*` 各项。属性规则可按 `scope` 限定，并用 `in` 只放进 `id_token` 或 `userinfo`。自定义配置时，授权确认接口会多返回 `delivered` 列表，说明各权限范围实际下发哪些字段。

退出登录（`BASE/oauth2/logout`，OIDC RP-Initiated Logout）：`post_logout_redirect_uri` 必须与 `client_id` 或 `id_token_hint` 所属应用登记的地址**逐字符一致**（或是本站路径），否则退出后回到站点首页；`state` 原样带回。过期的 ID Token 也可作为 `id_token_hint`；若 ID Token 是加密的，请先解密再把内层签名 JWT 作为提示传入。

## 5. Server API 加密信封

配置 `server.encrypt = true` 时，`GET /server/v1/whitelist` 与 `GET /server/v1/whitelist/check` 的响应变为：

```
HTTP/1.1 200 OK
X-Lounge-Encryption: JWE

{ "code": 0, "message": "...", "data": { "jwe": "eyJhbGciOi..." } }
```

- 解密得到的明文就是原本的 `data`（JSON，字段同 PROTOCOL §2.1 / §2.2，字段名与内容按配置的 `server` 规则）。
- 受保护头带 `iat`（Unix 秒）与 `ctx: "server:<服务器代码>:<路由>"`（路由为 `whitelist` 或 `whitelist/check`），插件应核对 `ctx`，防止把一个接口的密文当成另一个接口的。
- 轮换过渡期内，插件可在请求头 `X-Lounge-Encryption-Kid: <kid>` 指定用哪把公钥（生效的或过渡中的）；不带则用生效的那把。
- 开启加密但没有可用公钥时返回业务错误（`code != 0`），**不会退回明文**。插件在收到 `X-Lounge-Encryption` 之外的明文时也应拒绝（SDK 的 `OpenServerData` 如此）。

## 6. Webhook 加密信封

配置 `webhook.encrypt = true` 时，请求体变为：

```json
{ "id": "5c5b…", "event": "user.login", "timestamp": 1757800000, "jwe": "eyJhbGciOi..." }
```

并带请求头 `X-Lounge-Encryption: JWE`。签名规则不变（PROTOCOL §5，对上面这个请求体签名），先验签再解密。JWE 明文是完整的原始文档 `{"id","event","timestamp","data"}`，其中 `id` / `event` / `timestamp` 必须与外层一致；受保护头 `ctx: "webhook:<端点 ID>"`。配置的 `webhook` 规则非空或为 pairwise 时，`data.user` 为按规则生成的用户对象。重试沿用首次加密的密文，过渡期（默认 7 天）远长于最长重试间隔（6 小时）。

## 7. 密钥轮换

公钥状态：`pending`（已上传待启用）→ `active`（生效，每个配置只有一把）→ `retiring`（过渡中，到 `retireAt` 为止）→ `retired`。

1. 应用生成新密钥对，**先在自己一侧同时部署新旧两把私钥**（按 JWE 头的 `kid` 选择，SDK 支持传入多把）。
2. 后台上传新公钥（`pending`），可用"测试解密"核对。第一把公钥上传时若算法与配置相符会直接生效。
3. 点"启用"并填过渡期（0–90 天，默认 7）：新钥立即生效，OIDC 与 Webhook 立刻改用新钥；旧钥进入 `retiring`，过渡期内插件仍可用 `X-Lounge-Encryption-Kid` 指定它。过渡中的旧钥可再次"启用"以回滚。
4. 过渡期结束后系统自动置为 `retired`，应用即可删除旧私钥。

防呆：开启加密的配置不能停用唯一生效的公钥；算法与 `id_token_encrypted_response_alg` 等不符的公钥不能启用；没有生效公钥时不能开启加密。

## 8. 自定义属性（按应用、按用户）

绑定了配置的应用可为用户写入键值，配置里用 `entitlement.<键>`（单个）或 `entitlements`（全部，JSON 对象）下发。键：字母开头，字母数字与 `. _ -`，≤ 64；值：任意 JSON，≤ 4096 字节；每用户每配置 ≤ 50 个。

| 调用方 | 地址 | 鉴权 |
| --- | --- | --- |
| 游戏服务器 | `GET` / `PUT` / `DELETE BASE/server/v1/delivery/entitlements` | `X-Server-Key` |
| OAuth 客户端 | `GET` / `PUT` / `DELETE BASE/oauth2/delivery/entitlements` | HTTP Basic `client_id:client_secret`（仅机密客户端） |

`GET` / `DELETE` 用查询参数 `uid` 或 `sub`（`DELETE` 另带 `key`），`PUT` 请求体 `{"uid"|"sub", "key", "value"}`；返回 `{ "sub": "...", ... }`（`sub` 为该应用看到的标识）。pairwise 配置只接受 `sub`。后台「数据投递」的配置抽屉里也可按用户查看与编辑。

## 9. SDK

所有 SDK 共用测试向量 `SDK/go/testdata/vectors.json`（后端 `internal/modules/delivery` 生成：`go test -run TestVectors -update-vectors`）。

| 语言 | 入口 | 支持 | 验证 |
| --- | --- | --- | --- |
| Go | `ParseDecryptionKey` / `DecryptJWE` / `VerifyIDToken` / `DecryptIDToken` / `OpenServerData` / `OpenWebhook` | 全部 | `go test`（7/7 向量 + RFC 3394） |
| TypeScript | `src/delivery.ts`：`decryptJwe` / `verifyIdToken` / `decryptIdToken` / `openServerData` / `openWebhook` | 全部（X25519 需 Node 20+ 或新版浏览器） | `node --test`（7/7） |
| Python | `lounge_sdk.delivery`：`decrypt_jwe` / `verify_id_token` / `decrypt_id_token` / `open_server_data` / `open_webhook` | 全部；需 `pip install cryptography` | `unittest`（7/7，独立于后端实现的交叉核对） |
| Java | `LoungeDelivery.decrypt` / `verifyIdToken` / `decryptIdToken` / `openServerData` / `openWebhook`，密钥 `PrivateJwk.parse(json)` | 全部（Java 17+） | 临时程序跑向量 7/7（仓库内无 JUnit） |
| C# | `LoungeDelivery.Decrypt` / `VerifyIdToken` / `DecryptIdToken` / `OpenServerData` / `OpenWebhook` | RSA-OAEP-256、P-256；**不支持 X25519**（.NET 无此算法） | 临时程序在 .NET 10 上跑向量 5/7，2 个 X25519 向量按预期拒绝；net6.0 目标未单独编译 |
| PHP | `Delivery::decrypt` / `verifyIdToken` / `decryptIdToken` / `openServerData` / `openWebhook` | P-256（ext-openssl）、X25519（ext-sodium）；**不支持 RSA-OAEP-256** | **未验证**（本机无 PHP） |

## 10. 示例

Go 插件读取加密的白名单：

```go
key, _ := lounge.ParseDecryptionKey(privateJWK)
plain, err := lounge.OpenServerData(respBody, "server:survival:whitelist", key)
```

TypeScript 应用解密 ID Token：

```ts
const claims = await decryptIdToken(tokens.id_token, [currentJwk, previousJwk], await (await fetch(BASE + '/jwks.json')).json(), { issuer: BASE, audience: CLIENT_ID });
```

Python 接收加密的 Webhook：

```python
verify_webhook(secret, request.headers, raw_body)
doc = open_webhook(raw_body, [current_jwk, previous_jwk])
```
