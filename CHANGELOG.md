# Changelog

## 1.3.0 — 2026-11-16

- 六语言核心 OIDC SDK 新增 discovery 和 JWKS 获取：固定配置 issuer 路径、精确 issuer 校验、严格 JSON／UTF-8／结构检查和 1 MiB 响应上限。默认传输限制实际读取；已物化响应的自定义适配器仍需自行限制下载和遵守取消约定。
- 新增 nonce-bound ID Token 验证，包含加密 ID Token 的内层签名与 nonce 检查；旧调用保留兼容。Go 使用新的 WithNonce 函数；Java/C# 的加密入口使用独立 WithNonce 方法，避免旧私钥可变参数调用发生歧义；Python/TypeScript/PHP 按各语言参数约定扩展。
- TypeScript OIDC 补齐可选 timeoutMs／AbortSignal，默认 10 秒 deadline 覆盖 fetch 与响应体读取；需要更长等待的调用者应明确配置。PHP 默认 cURL 另提供有界读取，普通五参数自定义 transport 保持兼容。
- 完整游戏服、OIDC 和全项目接入指南、六语言能力矩阵、可离线使用的全量 OpenAPI 快照／接口索引及生成校验脚本统一位于 docs/。
- 新增完整 Go OIDC RP 示例，并升级既有游戏登录示例的 nonce／ID Token／UserInfo subject 检查；示例使用本地 SDK，不表示包或服务已发布部署。
- 六种核心语言版本统一为 1.3.0；Bot、login-kit 与插件协议继续独立维护版本，不随此版本机械变更。
- 实测运行时、全部执行命令、既有 .NET 文档警告和最低版本未实跑项见 [验证记录](docs/VERIFICATION.md)。

## 1.2.0 — 2026-10-09

- Go / Python：新增权益查询（`entitlements`），含测试。
- 传输加密：Go / Java / TypeScript 新增加密传输回归测试。
- 机器人客户端（Python / TypeScript）新增推送（`push`）模块；NoneBot2 新增投递（`delivery`）；Koishi / Wechaty 插件补测试。
- 新增文档：`docs/ACCOUNT_API.md`、`docs/CHAT.md`、`docs/GM_SSO.md`，以及 GM 一键登录 Python 示例（`examples/gm-sso-python/`）与 `login-kit/`。
- 版本号统一升到 1.2.0。

## 1.1.0 — 2026-09-14

- 五语言新增 Webhook 签名校验助手（`X-NoWork-Signature` = sha256=HMAC-SHA256(secret, `<ts>.<body>`)，含时间戳容差与常量时间比较）；Go / Python / TypeScript 附测试。
- 协议文档新增第 5 节「Webhook」。
- 新增 PHP SDK（`php/`，Composer `nowork/sdk`，PHP 8.1+，仅依赖 `ext-curl` / `ext-json`）：Server API、OIDC（含 PKCE S256 助手）、Webhook 校验（`Webhook::verify` / `verifyRequest`），与其它语言相同的信封校验与错误模型；`php tests/run.php` 无需 PHPUnit，含 `php -S` 端到端用例。

## 1.0.1 — 2026-09-13

- Java：新增 Velocity 代理端插件示例（`examples/velocity/`）。

- Go / Python：拒绝缺少 `code`、缺少或为 `null` 的 `data` 以及 HTTP ≥ 400 的"成功"响应，统一归为 `TransportError`；业务错误（`code != 0`）仍为 `ApiError`。
- Go：未配置服务器密钥时直接返回 `ErrNotConfigured`，不再发起请求。
- Python：`HTTPError` 响应体读取后关闭；OIDC 仅允许 `revoke` 返回空响应体，token / userinfo 空体或非 JSON 归为 `TransportError`，RFC 6749 错误对象仍为 `OAuthError`。
- 新增 Go、Python、TypeScript 协议回归测试（`go test ./...`、`python -m unittest discover -s tests`、`node --test tests/sdk.test.mjs`）。

## 1.0.0 — 2026-09-13

- 首个版本：Go / Java / Python / TypeScript / C# 五种语言 SDK
- Server API：`whitelist`、`checkWhitelist`、`heartbeat`
- OIDC：`authorizeUrl`、`exchange`（含 PKCE）、`refresh`、`clientCredentials`、`userInfo`、`introspect`、`revoke`、`logoutUrl`
- 统一错误模型：TransportError / ApiError / OAuthError
- 协议文档 `docs/PROTOCOL.md`
