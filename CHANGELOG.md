# Changelog

## 1.0.1 — 2026-09-13

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
