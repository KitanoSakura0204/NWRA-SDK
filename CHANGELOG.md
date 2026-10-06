# Changelog

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
