# 全项目接口索引

契约核对日期：2026-11-16。共 **1629 个文档操作、1335 个路径、1368 个数据结构**；其中 1623 个操作有当前源码注册位置，6 个仅作为已弃用契约保留。

本文由 `scripts/generate_integration_reference.py` 生成。每个接口的全部参数、请求体、响应、错误、权限说明和数据结构见同目录内可离线使用的 [完整 OpenAPI](reference/openapi.json)。不要将该完整管理接口文档放入网页公开静态目录；SDK 包默认不发布这个参考目录。

先读 [游戏服务器指南](GAME_SERVER_INTEGRATION.md)、[OIDC 指南](OIDC_INTEGRATION.md) 或 [项目对接总指南](PROJECT_INTEGRATION.md)。表中的鉴权列只是协议凭据入口，角色权限、数据范围、站点与用户状态仍以操作的 `description` 和后端检查为准；接口存在不表示当前账号或站点可使用。`*` 表示必填参数。

源契约 SHA-256：`e8acd4d0d99814103840dc2497d997cfcc092b41df647b006944d1bbd2c6e9ea`。版本和来源见 [manifest](reference/manifest.json)。

## 鉴权方案

| 方案 | 类型 | 说明 |
| --- | --- | --- |
| `AppsR15DeviceSignature` | apiKey | Ed25519 proof without bearer authorization; exact token-hash purpose and device enrollment depend on each operation. |
| `AppsR15NetdiagPingTicket` | apiKey | Finite host/IP/Origin-bound netdiag measurement capability from POST /api/netdiag/session; not an account credential. |
| `AppsR15RealtimeSignature` | apiKey | Device Ed25519 signature over the sakura-native-realtime-v1 ticket tuple; not the ordinary HTTP request proof. |
| `AppsR15RealtimeTicket` | apiKey | nrt_ one-use native realtime ticket, <=30 seconds and capped by current access expiry. Requires separate X-Native-Realtime-Signature and timestamp. |
| `AuthR15APIKey` | http | Opaque API key presented once in Authorization: Bearer. Checked against current creator rights, site membership, scopes, expiry, quotas, IP/origin restrictions and optional proof on every request. Query-string credentials are rejected; this is not a browser/native/OAuth token. |
| `AuthR15DCRInitial` | http | Optional dcri_ initial access token authorizing registration; site/expiry/revocation/use count checked. |
| `AuthR15DCRManagement` | http | Per-registration opaque dcrr_ management token returned by DCR; not a client secret, OAuth token or browser session. Replaced after every update. |
| `NotifyR15AlertProof` | apiKey | 43-character base64url capability from the notification-alert mail fragment; bound to one site/session, expires within 24 hours and the session deadline. GET only reviews; POST confirm sends proof in JSON. |
| `OpsAutomationSignature` | apiKey | Automation inbound callback signature; see the operation's canonical message and timestamp rules. |
| `OpsBotAuthorization` | apiKey | Bot <token> or Bearer <token>, interpreted by the bot API token authenticator. |
| `OpsBotEventToken` | apiKey | Bot token alternative accepted by the events polling endpoint. |
| `OpsLoginSetup` | apiKey | Short-lived setup grant issued when login policy requires additional account security setup. |
| `R15RestrictedBrowserCookie` | apiKey | Matching browser secret for the separately approved brs_ delegation. HttpOnly/SameSite=Strict, Path=/api; a cookie alone does not authenticate the request. |
| `R15RestrictedBrowserToken` | http | Restricted browser delegation, accepted only together with its matching HttpOnly sakura_restricted_browser cookie and the original native family's live read/write scope. Not a full browser session or native-device signature credential. |
| `bearerAuth` | http | Session token from `/api/login`, `/api/register`, `/api/login/challenge` or an upstream callback. Admin routes additionally require an admin-level role plus the per-route authority. |
| `chatBot` | apiKey | Case-sensitive Bot cb_... credential issued for a chat bot. |
| `chatSession` | apiKey | Chat cs_... session credential; site Bearer authentication is a separate accepted alternative. |
| `clientBasic` | http | OAuth client authentication (`client_id:client_secret`, URL-encoded). `client_id`/`client_secret` form fields are equivalent; public clients send only `client_id`. |
| `connectorHmac` | apiKey | Connector webhook signature, computed over the request according to the operation contract. |
| `connectorScim` | http | Opaque lsc_... SCIM connector token; not a user session or OAuth access token. |
| `merchantWechatAuth` | apiKey | WECHATPAY2-SHA256-RSA2048 mchid="...",serial_no="...",nonce_str="...",timestamp="...",signature="..."; RSA-SHA256 canonical method/URI/timestamp/nonce/body. |
| `nativeAccessToken` | http | Native session access credential; only the originating account/app/device approved capabilities are available, with per-request native proof. |
| `nativeAdminGrant` | apiKey | Independently browser-confirmed short-lived native administrator grant; required together with the matching native access token and signed native proof. |
| `nativeBrowserIntentCookie` | apiKey | HttpOnly original-browser intent cookie; polling and passkey continuations also require the pollToken from intent creation. |
| `nativeRefreshToken` | http | nsr_... rotating native-session refresh credential, with required X-Native proof headers. |
| `oidcAccessToken` | http | OAuth access token issued by `/oauth2/token` (RS256, `token_use=access`). |
| `pluginToken` | apiKey | Token of an enabled external-app plugin (`lpt_…`, issued on install and by `POST /api/admin/plugins/{id}/rotate`); limited to 10 requests per second per plugin (burst 20) and to the capabilities the plugin was granted. |
| `probeHeader` | apiKey | Monitor probe-node token alternative. |
| `probeToken` | http | Monitor probe-node reporting token; not a user session. |
| `serverKey` | apiKey | Game-server plugin key (issued on server create / rotate). `?key=` is accepted as a fallback. The authenticated server entity selects its owning site; an unavailable owning site or disabled game feature rejects access. |
| `ssoBrowser` | apiKey | SSO browser session cookie established by the SSO session endpoint. |
| `ssoGatewayKey` | apiKey | Application gateway key; forward-auth additionally requires the user's browser/session credential. |

## 分组目录

- [account](#group-001)（55 个操作）
- [account/api-keys](#group-002)（9 个操作）
- [account/developer-webhooks](#group-003)（10 个操作）
- [account/notification-preferences](#group-004)（3 个操作）
- [account/permissions](#group-005)（1 个操作）
- [admin/account-security](#group-006)（6 个操作）
- [admin/analytics](#group-007)（1 个操作）
- [admin/api-keys](#group-008)（4 个操作）
- [admin/content](#group-009)（14 个操作）
- [admin/dashboard](#group-010)（1 个操作）
- [admin/game](#group-011)（12 个操作）
- [admin/identity](#group-012)（8 个操作）
- [admin/membership](#group-013)（6 个操作）
- [admin/notification-templates](#group-014)（8 个操作）
- [admin/oauth](#group-015)（12 个操作）
- [admin/oidc-policy](#group-016)（8 个操作）
- [admin/payments](#group-017)（52 个操作）
- [admin/permissions](#group-018)（6 个操作）
- [admin/realname](#group-019)（10 个操作）
- [admin/rules](#group-020)（9 个操作）
- [admin/sessions](#group-021)（2 个操作）
- [admin/settings](#group-022)（16 个操作）
- [admin/system](#group-023)（7 个操作）
- [admin/users](#group-024)（22 个操作）
- [admin/webhooks](#group-025)（8 个操作）
- [auth](#group-026)（18 个操作）
- [devportal](#group-027)（17 个操作）
- [federation-contracts](#group-028)（62 个操作）
- [install](#group-029)（6 个操作）
- [membership](#group-030)（10 个操作）
- [merchant-protocol](#group-031)（46 个操作）
- [modules](#group-032)（4 个操作）
- [modules/applications](#group-033)（34 个操作）
- [modules/apps](#group-034)（170 个操作）
- [modules/automation](#group-035)（21 个操作）
- [modules/batch-operations](#group-036)（1 个操作）
- [modules/binding-policy](#group-037)（5 个操作）
- [modules/bots](#group-038)（37 个操作）
- [modules/branding](#group-039)（7 个操作）
- [modules/chat](#group-040)（85 个操作）
- [modules/custom-fields](#group-041)（42 个操作）
- [modules/dbmaint](#group-042)（6 个操作）
- [modules/delivery](#group-043)（24 个操作）
- [modules/devaccess](#group-044)（7 个操作）
- [modules/devdocs](#group-045)（1 个操作）
- [modules/feature-switches](#group-046)（1 个操作）
- [modules/forced-actions](#group-047)（10 个操作）
- [modules/game-archive](#group-048)（26 个操作）
- [modules/game-assets](#group-049)（6 个操作）
- [modules/geolocation](#group-050)（4 个操作）
- [modules/helpcenter](#group-051)（10 个操作）
- [modules/hypergryph](#group-052)（22 个操作）
- [modules/identity-access](#group-053)（10 个操作）
- [modules/invites](#group-054)（10 个操作）
- [modules/legal](#group-055)（15 个操作）
- [modules/login-policy](#group-056)（17 个操作）
- [modules/media-library](#group-057)（3 个操作）
- [modules/messages](#group-058)（12 个操作）
- [modules/mihoyo](#group-059)（19 个操作）
- [modules/monitor](#group-060)（34 个操作）
- [modules/netdiag](#group-061)（12 个操作）
- [modules/onboarding](#group-062)（10 个操作）
- [modules/outboundproxy](#group-063)（9 个操作）
- [modules/pagebuilder](#group-064)（21 个操作）
- [modules/payment-risk](#group-065)（17 个操作）
- [modules/performance](#group-066)（4 个操作）
- [modules/platformlogin](#group-067)（1 个操作）
- [modules/pluginmarket](#group-068)（28 个操作）
- [modules/plugins](#group-069)（30 个操作）
- [modules/points](#group-070)（12 个操作）
- [modules/profile](#group-071)（6 个操作）
- [modules/regional-services](#group-072)（4 个操作）
- [modules/requirements](#group-073)（5 个操作）
- [modules/review](#group-074)（5 个操作）
- [modules/roletemplates](#group-075)（7 个操作）
- [modules/sdk-login](#group-076)（16 个操作）
- [modules/security-center](#group-077)（9 个操作）
- [modules/server-gm](#group-078)（40 个操作）
- [modules/server-owner](#group-079)（22 个操作）
- [modules/server-users](#group-080)（41 个操作）
- [modules/sites](#group-081)（50 个操作）
- [modules/telemetry](#group-082)（17 个操作）
- [modules/textoverrides](#group-083)（8 个操作）
- [modules/theme-packages](#group-084)（11 个操作）
- [modules/tickets](#group-085)（20 个操作）
- [modules/user-audit](#group-086)（1 个操作）
- [modules/user-bulk](#group-087)（7 个操作）
- [notification-alerts](#group-088)（2 个操作）
- [oidc](#group-089)（11 个操作）
- [oidc/registration](#group-090)（4 个操作）
- [payments](#group-091)（28 个操作）
- [public](#group-092)（10 个操作）
- [server-api](#group-093)（3 个操作）
- [sso-contracts](#group-094)（33 个操作）
- [system](#group-095)（2 个操作）
- [system-module](#group-096)（26 个操作）
- [system/permissions](#group-097)（5 个操作）

<a id="group-001"></a>

### account

User centre: profile, security, sessions, MFA, passkeys, privacy, game roles, notifications.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| PUT | `/api/account/alias` | Set the public alias (once) | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/authorizations` | Third-party apps the user consented to | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/authorizations/{clientId}` | Read a personally authorized application's details | `bearerAuth` | path:`clientId`*; header:`X-Lang` |
| DELETE | `/api/account/authorizations/{clientId}` | Revoke an app's access | `bearerAuth` | path:`clientId`*; header:`X-Lang` |
| POST | `/api/account/avatar` | Upload an avatar (multipart `file`, ≤ 2 MB, png/jpg/webp/gif) | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/connections` | Linked external identities | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/connections/binding/finish` | Finish the exact provider-verified binding attempt | `bearerAuth` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/connections/{provider}` | Unbind one selected external identity | `bearerAuth` 或 `nativeAccessToken` | path:`provider`*; query:`connectionId`; header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/connections/{provider}/sync` | Refresh one selected external identity | `bearerAuth` 或 `nativeAccessToken` | path:`provider`*; query:`connectionId`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/contact/bind` | Bind or change e-mail / phone | `bearerAuth` 或 `nativeAccessToken` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/contact/{kind}` | Unbind e-mail or phone | `bearerAuth` 或 `nativeAccessToken` | path:`kind`*; header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/deletion` | Account deletion status | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/deletion` | Request account deletion (grace period) | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/account/deletion` | Cancel the pending deletion | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/export` | Export own account data (GDPR-style) | `bearerAuth` | query:`download`; header:`X-Lang` |
| GET | `/api/account/game/roles` | Own game roles（已弃用） | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/game/roles` | Bind a game role / apply for whitelist | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/game/roles/{id}` | Unbind a game role | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/level` | Trust-level ladder | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| POST | `/api/account/lock` | Self-lock the account | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/login-records` | Own login history | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/me` | Masked self view | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/mfa` | Read own TOTP and passkey summary | `bearerAuth` 或 `nativeAccessToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| POST | `/api/account/mfa/totp/begin` | Start or replace pending authenticator enrollment | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/mfa/totp/confirm` | Confirm pending TOTP and show eight recovery codes | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/mfa/totp/disable` | Disable TOTP using authenticator or account password | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/mfa/totp/recovery` | Replace recovery codes after authenticator verification | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/notifications` | In-app notifications | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`unread`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| PUT | `/api/account/notifications/read-all` | Mark every notification read | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/notifications/unread-count` | Unread notification count | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| DELETE | `/api/account/notifications/{id}` | Delete one notification | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| PUT | `/api/account/notifications/{id}/read` | Mark one notification read | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/operation-records` | Own operation history | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/overview` | Account overview | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/passkeys` | List own registered passkey metadata | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/passkeys/begin` | Begin passkey creation ceremony | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/passkeys/finish` | Verify attestation and store own passkey | `bearerAuth` | query:`sessionId`*; query:`name`; header:`X-Lang`; body:application/json |
| PUT | `/api/account/passkeys/{id}` | Rename own passkey | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/passkeys/{id}` | Remove own passkey while preserving a login method | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/realname` | Real-name verification state | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/realname` | Submit real-name verification | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/realname/manual` | Ask for human review of an automatically rejected record | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/realname/photo` | Upload an ID photo | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/realname/photo/{fileId}` | View an own ID photo | `bearerAuth` | path:`fileId`*; header:`X-Lang` |
| GET | `/api/account/realname/state` | Real-name state with the verification channel | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/account/region` | Legacy self-service region edit is disabled | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/required-bindings` | Mandatory bindings checklist (administrators) | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/security` | Security checklist | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/account/sessions` | List own devices / sessions | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/sessions/others` | Log out every other device | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| DELETE | `/api/account/sessions/{id}` | Log out one device | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/settings` | Read private account preferences and supported country codes | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/trusted-devices` | Browsers remembered after two-step verification | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/trusted-devices` | Forget every remembered device | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/trusted-devices/{id}` | Forget one remembered device | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-002"></a>

### account/api-keys

Browser-session management of site-owned personal and service API keys; full secrets are disclosed only on issue or rotation.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/api-keys` | List visible API keys | `bearerAuth` | query:`page`; query:`limit`; query:`userId`; query:`kind`; query:`status`; query:`keywords`; header:`X-Lang` |
| POST | `/api/account/api-keys` | Issue one personal or service API key | `bearerAuth` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| GET | `/api/account/api-keys/scopes` | Read available API-key scopes and fixed limits | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/api-keys/{id}` | Read API-key controls and counters | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/api-keys/{id}` | Replace an owned API key's policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/api-keys/{id}` | Revoke an API key immediately | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/api-keys/{id}/rotate` | Rotate an owned API key with bounded overlap | `bearerAuth` | path:`id`*; header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| GET | `/api/account/api-keys/{id}/usage` | Read API-key admission and lifecycle history | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; header:`X-Lang` |
| GET | `/api/api-keys/current` | Inspect the presented API key and its effective scopes | `AuthR15APIKey` | header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`Origin`; header:`X-Lang` |

<a id="group-003"></a>

### account/developer-webhooks

Current-site, browser-only and developer-owner-scoped webhook receivers, event schemas, synthetic tests and retry history.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/developer-webhooks/apps` | Page apps owned by the webhook developer | `bearerAuth` | query:`page`; query:`limit`; header:`X-Lang` |
| GET | `/api/account/developer-webhooks/apps/{clientId}/endpoints` | List an owned app's webhook receivers | `bearerAuth` | path:`clientId`*; header:`X-Lang` |
| POST | `/api/account/developer-webhooks/apps/{clientId}/endpoints` | Create an app webhook receiver | `bearerAuth` | path:`clientId`*; header:`X-Lang`; body:application/json |
| PUT | `/api/account/developer-webhooks/apps/{clientId}/endpoints/{id}` | Replace an app webhook receiver | `bearerAuth` | path:`clientId`*; path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/developer-webhooks/apps/{clientId}/endpoints/{id}` | Delete an owned app webhook and its history | `bearerAuth` | path:`clientId`*; path:`id`*; header:`X-Lang` |
| GET | `/api/account/developer-webhooks/apps/{clientId}/endpoints/{id}/deliveries` | Page safe delivery history for an owned receiver | `bearerAuth` | path:`clientId`*; path:`id`*; query:`page`; query:`limit`; query:`status`; header:`X-Lang` |
| POST | `/api/account/developer-webhooks/apps/{clientId}/endpoints/{id}/deliveries/{deliveryId}/retry` | Retry a failed owned-app webhook delivery | `bearerAuth` | path:`clientId`*; path:`id`*; path:`deliveryId`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/developer-webhooks/apps/{clientId}/endpoints/{id}/rotate` | Rotate an owned app webhook signing secret | `bearerAuth` | path:`clientId`*; path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/developer-webhooks/apps/{clientId}/endpoints/{id}/test` | Send a catalogue-generated test to an owned receiver | `bearerAuth` | path:`clientId`*; path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/developer-webhooks/catalog` | Read event schemas, samples and webhook limits | `bearerAuth` | header:`X-Lang` |

<a id="group-004"></a>

### account/notification-preferences

Current user's channel/language choices and separately consented linked-site summaries.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/notification-preferences` | Read effective channel and language preferences | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/notification-preferences` | Save current-site notification preferences atomically | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/notification-preferences/site-link` | Resolve an allowed linked notification source | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`origin`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |

<a id="group-005"></a>

### account/permissions

Effective permission sources for the caller within the current site and credential ceiling.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/permissions` | Explain the caller's effective permissions | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |

<a id="group-006"></a>

### admin/account-security

Scope-checked administrative recovery actions and account security inventories.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| DELETE | `/api/admin/user/{id}/connections/{provider}` | Unbind one selected managed-account identity | `bearerAuth` | path:`id`*; path:`provider`*; query:`connectionId`; header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang` |
| DELETE | `/api/admin/user/{id}/deletion` | Cancel a managed account's pending deletion | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/user/{id}/passkeys/{passkeyId}` | Remove a managed account's passkey | `bearerAuth` | path:`id`*; path:`passkeyId`*; header:`X-Lang` |
| GET | `/api/admin/user/{id}/security` | Inspect a managed account's security methods | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/user/{id}/trusted-devices` | Revoke every remembered device of a managed account | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/user/{id}/trusted-devices/{deviceId}` | Revoke one remembered device of a managed account | `bearerAuth` | path:`id`*; path:`deviceId`*; header:`X-Lang` |

<a id="group-007"></a>

### admin/analytics

Aggregate analytics (`analytics:view`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/analytics` | Aggregate figures behind the analytics page | `bearerAuth` | query:`range`; query:`tz`; header:`X-Lang` |

<a id="group-008"></a>

### admin/api-keys

Current-site API-key inspection and revocation within the administrator's managed-user scope.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/api-keys` | List visible API keys | `bearerAuth` | query:`page`; query:`limit`; query:`userId`; query:`kind`; query:`status`; query:`keywords`; header:`X-Lang` |
| GET | `/api/admin/api-keys/{id}` | Read API-key controls and counters | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/api-keys/{id}` | Revoke an API key immediately | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/api-keys/{id}/usage` | Read API-key admission and lifecycle history | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; header:`X-Lang` |

<a id="group-009"></a>

### admin/content

Announcements, content pages and admin notifications (`announcement:*`, `page:*`, `notification:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/announcements` | List announcements (raw rows) | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/announcements` | Create an announcement | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/announcements` | Update an announcement (id in body) | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/announcements/{id}` | Delete an announcement | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/announcements/{id}/status` | Enable / disable an announcement | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/notifications/campaigns` | Push history | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Lang` |
| GET | `/api/admin/notifications/preview-count` | How many users an audience would reach | `bearerAuth` | query:`audience`*; query:`userIds`; header:`X-Lang` |
| POST | `/api/admin/notifications/send` | Push an admin-authored notification | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/pages` | List content pages (raw rows) | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pages` | Create a content page | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/pages` | Update a content page (id in body) | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/pages/{id}` | Load a content page for the editor | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/pages/{id}` | Update a content page (id in path)（已弃用） | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/pages/{id}` | Delete a content page | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-010"></a>

### admin/dashboard

Console overview.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/dashboard` | Console statistics | `bearerAuth` | header:`X-Lang` |

<a id="group-011"></a>

### admin/game

Game servers and role bindings (`game:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/admin/game/role` | Bind a role on behalf of a user | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/role/page` | Page role bindings | `bearerAuth` 或 `AuthR15APIKey` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`serverId`; query:`status`; query:`keywords`; query:`userId`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| DELETE | `/api/admin/game/role/{id}` | Delete a binding | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/role/{id}/review` | Review a binding | `bearerAuth` 或 `AuthR15APIKey` | path:`id`*; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server` | List game servers (scope-filtered) | `bearerAuth` 或 `AuthR15APIKey` | header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| POST | `/api/admin/game/server` | Create a game server (plugin key returned once) | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/game/server` | Update a game server | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/game/server/{id}` | Delete a game server | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/game/server/{id}/key` | Rotate the plugin key | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/players` | Players reported online, matched to accounts | `bearerAuth` 或 `AuthR15APIKey` | path:`id`*; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/stats` | Player curve and summary | `bearerAuth` 或 `AuthR15APIKey` | path:`id`*; query:`range`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/status` | Set online / offline / maintenance by hand | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-012"></a>

### admin/identity

Upstream identity providers (`identity:provider:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/identity/provider` | List identity providers | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/identity/provider` | Create an identity provider | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/identity/provider` | Update an identity provider | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/identity/provider/options` | Compact provider picker options（已弃用） | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/identity/provider/presets` | Provider templates | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/identity/provider/test` | Run discovery against a provider configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/identity/provider/{id}` | Delete an identity provider | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/identity/provider/{id}/sync` | Sync every user bound to a provider | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-013"></a>

### admin/membership

Membership catalog and scoped operator grants

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/membership/plans` | List all current-site membership terms | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/membership/plans` | Save site membership prices and renewal terms | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/membership/plans/{id}` | Save site membership prices and renewal terms | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/membership/plans/{id}` | Delete an unused membership plan | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/membership/subscriptions` | List scoped membership subscription history | `bearerAuth` | query:`page`; query:`limit`; query:`userId`; query:`planId`; query:`status`; header:`X-Lang` |
| POST | `/api/admin/membership/users/{id}/subscription` | Grant an authorized site membership | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-014"></a>

### admin/notification-templates

Site-specific event/channel/locale templates, fixed previews and actor-only delivery tests.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/notification-templates` | Read one event's effective templates | `bearerAuth` | query:`kind`*; header:`X-Lang` |
| PUT | `/api/admin/notification-templates` | Save one revision-checked event template | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/notification-templates` | Reset exactly one current template override | `bearerAuth` | query:`kind`*; query:`channel`*; query:`locale`*; query:`revision`*; header:`X-Lang` |
| GET | `/api/admin/notification-templates/catalog` | Read event/channel/locale and safe-variable catalogue | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/notification-templates/deliveries` | Read current-site delivery metadata | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/notification-templates/deliveries/{id}/retry` | Requeue one failed unexpired notification delivery | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/notification-templates/preview` | Render a template against fixed example data | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/notification-templates/test` | Queue a fixed example only to the acting administrator | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-015"></a>

### admin/oauth

OAuth clients and signing keys (`oauth:client:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/admin/oauth/client` | Create an administrative OAuth client | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/oauth/client` | Replace OAuth-client configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/oauth/client/page` | Page current-site OAuth clients | `bearerAuth` | query:`page`; query:`limit`; query:`keywords`; query:`status`; query:`approved`; header:`X-Lang` |
| DELETE | `/api/admin/oauth/client/{id}` | Delete a current-site OAuth client | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/oauth/client/{id}/review` | Approve or reject a self-registered developer application | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/oauth/client/{id}/secret` | Rotate an OAuth client's shared secret | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/oauth/client/{id}/secret/previous` | Revoke the previous OAuth client secret | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/oauth/client/{id}/status` | Enable/approve or disable an OAuth client | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/oauth/client/{id}/users` | List managed users authorizing this OAuth client | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; header:`X-Lang` |
| DELETE | `/api/admin/oauth/client/{id}/users/{userId}` | Revoke a managed user's consent and token family | `bearerAuth` | path:`id`*; path:`userId`*; header:`X-Lang` |
| GET | `/api/admin/oauth/keys` | OIDC signing keys | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/oauth/keys/rotate` | Rotate the OIDC signing key | `bearerAuth` | header:`X-Lang` |

<a id="group-016"></a>

### admin/oidc-policy

Main-super OIDC policy, initial registration tokens and revision-bound dynamic-client review.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/oidc/initial-access-tokens` | List initial DCR access-token records | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/oidc/initial-access-tokens` | Issue an initial DCR access token once | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/oidc/initial-access-tokens/{id}` | Revoke an initial registration token | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/oidc/policy` | Read OIDC global policy | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/oidc/policy` | Update OIDC global policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/oidc/registrations` | List dynamic registration review records | `bearerAuth` | query:`state`; header:`X-Lang` |
| DELETE | `/api/admin/oidc/registrations/{id}/access-token` | Expire a dynamic registration's management credential | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/oidc/registrations/{id}/review` | Review a displayed dynamic-registration revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-017"></a>

### admin/payments

Payment configuration, reconciliation and operator actions

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/pay/channels` | Payment channels with secret presence flags | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pay/channels` | Save payment channel | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/pay/channels` | Save payment channel | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/pay/channels/{id}` | Delete an unused channel | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/pay/channels/{id}/enabled` | Enable or disable channel | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pay/channels/{id}/test` | Create a one-minor-unit sandbox checkout | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/pay/coupons` | Latest discount codes | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pay/coupons` | Create or update coupon by body id | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/pay/coupons/{id}` | Delete coupon | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pay/logs` | Notification and upstream query log | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`channelId`; query:`result`; query:`orderNo`; header:`X-Lang` |
| GET | `/api/admin/pay/merchant/apps` | List scoped merchant applications and gateway URLs | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/pay/merchant/apps/{id}/protocols/refunds` | List scoped protocol refund callback records | `bearerAuth` | path:`id`*; query:`before`; header:`X-Lang` |
| POST | `/api/admin/pay/merchant/apps/{id}/protocols/refunds/{refundID}/retry` | Reschedule failed refund callback | `bearerAuth` | path:`id`*; path:`refundID`*; header:`X-Lang` |
| PUT | `/api/admin/pay/merchant/apps/{id}/review` | Review scoped merchant application and routing fees | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/pay/merchant/orders` | List merchant relay orders | `bearerAuth` | query:`appId`; query:`page`; header:`X-Lang` |
| POST | `/api/admin/pay/merchant/orders/{id}/retry` | Retry failed merchant order callback | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pay/merchant/withdrawals` | List merchant withdrawals | `bearerAuth` | query:`appId`; query:`page`; header:`X-Lang` |
| GET | `/api/admin/pay/merchant/withdrawals/{id}` | Read scoped withdrawal and decrypted payout destination | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/pay/merchant/withdrawals/{id}` | Record external withdrawal payment or rejection | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/pay/meta` | Gateway capabilities and configuration fields | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/pay/orders` | Scoped order history | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`status`; query:`purpose`; query:`channelId`; query:`userId`; query:`currency`; query:`from`; query:`to`; query:`fulfil`; header:`X-Lang` |
| GET | `/api/admin/pay/orders/{orderNo}` | Order with refunds, notification log and ledger | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/admin/pay/orders/{orderNo}/close` | Close an unpaid order | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/admin/pay/orders/{orderNo}/fulfil` | Retry failed fulfilment | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/admin/pay/orders/{orderNo}/invoice` | Issue or correct a commercial payment document | `bearerAuth` | path:`orderNo`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pay/orders/{orderNo}/invoice/void` | Void the current issued document | `bearerAuth` | path:`orderNo`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pay/orders/{orderNo}/query` | Requery upstream payment status | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| GET | `/api/admin/pay/orders/{orderNo}/receipt` | Payment receipt and immutable invoice revisions | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/admin/pay/orders/{orderNo}/refund` | Refund an order through its gateway or wallet | `bearerAuth` | path:`orderNo`*; header:`Idempotency-Key`; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pay/orders/{orderNo}/resolve` | Resolve underpaid or late crypto order | `bearerAuth` | path:`orderNo`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/pay/products` | Product catalog including disabled products | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pay/products` | Save product catalog entry | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/pay/products` | Save product catalog entry | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/pay/products/{id}` | Delete unused product or disable purchased product | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pay/rates` | Exact exchange rates | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pay/rates` | Upsert exchange rate pair | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/pay/rates/{id}` | Delete stored exchange rate | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pay/receipts/export` | Export payment reconciliation CSV | `bearerAuth` | query:`from`*; query:`to`*; query:`currency`; header:`X-Lang` |
| GET | `/api/admin/pay/reconcile` | Reconcile real payments and outstanding exceptions | `bearerAuth` | query:`from`; query:`to`; header:`X-Lang` |
| GET | `/api/admin/pay/recurring` | List recurring agreements within operator scope | `bearerAuth` | query:`page`; query:`limit`; query:`userId`; header:`X-Lang` |
| GET | `/api/admin/pay/recurring/{id}` | Read one scoped recurring agreement | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/pay/recurring/{id}/cancel` | Reconcile recurring agreement: cancel | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/pay/recurring/{id}/recover` | Reconcile recurring agreement: recover | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pay/recurring/{id}/refresh` | Reconcile recurring agreement: refresh | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pay/refunds` | Scoped refund history | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; query:`orderNo`; header:`X-Lang` |
| POST | `/api/admin/pay/refunds/{refundNo}/resolve` | Record a verified final upstream refund result | `bearerAuth` | path:`refundNo`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/pay/settings` | Payment settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/pay/settings` | Update payment settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/pay/txns` | Scoped wallet ledger | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`userId`; query:`currency`; query:`type`; query:`orderNo`; header:`X-Lang` |
| GET | `/api/admin/pay/wallets` | Scoped wallet balances | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`userId`; query:`currency`; query:`keywords`; header:`X-Lang` |
| POST | `/api/admin/pay/wallets/adjust` | Credit or debit a scoped account wallet | `bearerAuth` | header:`Idempotency-Key`; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/pay/wallets/{id}/locked` | Freeze or unfreeze scoped wallet spending | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-018"></a>

### admin/permissions

Finite current-site permission grants, review, revocation and effective-source inspection.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/permissions/grants` | Read the visible delegation review queue | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/permissions/grants` | Create a bounded temporary permission grant | `bearerAuth` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/permissions/grants/{id}/decision` | Approve or reject a pending permission grant | `bearerAuth` | path:`id`*; header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/permissions/grants/{id}/revoke` | Revoke an active or pending permission grant | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/permissions/options` | Read permission catalogue, grantable set and delegation policy | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/permissions/users/{id}` | Explain a managed account's effective permissions | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-019"></a>

### admin/realname

Real-name review, document types, KYC providers and routing (`realname:*`, `review:manage`, `settings:edit`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/realname/doc-types` | Every document type with its switch | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/realname/doc-types` | Choose the enabled document types | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/realname/enabled` | Switch real-name verification on or off | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/realname/kyc` | Real-name switch, review mode, KYC routing and providers | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/realname/kyc/providers/{key}` | Save one KYC provider | `bearerAuth` | path:`key`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/realname/kyc/routes` | Save the KYC routing | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/realname/page` | Page real-name submissions | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; query:`keywords`; header:`X-Lang` |
| GET | `/api/admin/realname/{id}` | Unmasked record for review | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/realname/{id}/photo/{slot}` | ID photo of a submission | `bearerAuth` | path:`id`*; path:`slot`*; header:`X-Lang` |
| PUT | `/api/admin/realname/{id}/review` | Approve / reject | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-020"></a>

### admin/rules

Trust-level rules (`rule:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/rules` | Trust-level rules plus editor catalogues | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/rules` | Create a rule | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/rules` | Update a rule | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/rules/level-max` | Set the trust-level ceiling | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/rules/level-names` | Set per-language level names | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/rules/preview` | Preview a security-level rule tree | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/rules/recompute` | Re-evaluate every user's level | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/rules/tree-facts` | List facts allowed in security-level trees | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/admin/rules/{id}` | Delete a rule | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-021"></a>

### admin/sessions

Login sessions (`session:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/session/page` | Page login sessions | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`userId`; query:`username`; query:`ip`; query:`active`; header:`X-Lang` |
| DELETE | `/api/admin/session/{id}` | Force one session offline | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-022"></a>

### admin/settings

Site settings, mail, GeoIP, branding (`settings:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/settings` | Settings for the authenticated request site | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/settings` | Save settings for the authenticated request site | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/cache/clear` | Reload the settings cache from persistent settings | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/settings/geoip` | Offline IP database status | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/settings/geoip/reload` | Reload the IP database | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/settings/mail/templates` | Mail templates (kind × locale) | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/settings/mail/templates` | Save mail template overrides | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/mail/templates/preview` | Render a template with sample data | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/mail/templates/reset` | Drop overrides of a kind (one locale when `lang` given) | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/mail/templates/test` | Send the rendered template to the caller's e-mail | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/mail/test` | Send a test e-mail | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/settings/sms/logs` | Page this site's SMS send log | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`scene`; query:`status`; query:`provider`; header:`X-Lang` |
| GET | `/api/admin/settings/sms/templates` | Read the per-scene SMS templates | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/settings/sms/templates` | Save per-scene SMS templates | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/sms/test` | Send one test message through the configured SMS gateway | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/settings/upload` | Upload a logo / favicon (multipart `file`, ≤ 2 MB, png/jpg/webp/gif/svg/ico) | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |

<a id="group-023"></a>

### admin/system

System status and database backups (`settings:view`, `settings:edit`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/system/backup` | Backup availability and schedule | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/system/backup` | Take a manual backup now | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/system/backups` | Catalogued backups (newest first) | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/admin/system/backups/{id}` | Delete a backup file and its record | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/system/backups/{id}/download` | Download a backup | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/system/backups/{id}/restore` | Restore a backup | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/system/status` | Runtime / database / integration / queue snapshot | `bearerAuth` | header:`X-Lang` |

<a id="group-024"></a>

### admin/users

User management and administrators (`user:*`, `admin:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/user` | List users (export) | `bearerAuth` 或 `AuthR15APIKey` | query:`keywords`; query:`status`; query:`securityLevel`; query:`roleId`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| POST | `/api/admin/user` | Create a user | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/user` | Update a user | `bearerAuth` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| POST | `/api/admin/user/admins/demote` | Strip sub-admin roles (super admin only) | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/user/admins/page` | Page administrators with binding status | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`roleId`; header:`X-Lang` |
| POST | `/api/admin/user/admins/promote` | Grant sub-admin roles (super admin only) | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/user/admins/roles` | Sub-administrator roles available for promotion | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/user/admins/{id}/permissions` | Explain a sub-administrator's permission points | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/user/admins/{id}/permissions` | Rewrite a sub-administrator's dedicated role | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/user/batch` | Delete several users | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/user/existence` | Uniqueness check for admin forms | `bearerAuth` 或 `AuthR15APIKey` | query:`field`*; query:`value`*; query:`id`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| PUT | `/api/admin/user/level` | Override a trust level (`level: null` clears it) | `bearerAuth` 或 `AuthR15APIKey` | header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/admin/user/page` | Page users | `bearerAuth` 或 `AuthR15APIKey` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`username`; query:`nickname`; query:`email`; query:`phone`; query:`uid`; query:`status`; query:`securityLevel`; query:`realNameStatus`; query:`roleId`; query:`createTimeStart`; query:`createTimeEnd`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| PUT | `/api/admin/user/password` | Reset a user's password (logs them out everywhere) | `bearerAuth` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/user/status` | Freeze / unfreeze | `bearerAuth` 或 `AuthR15APIKey` | header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/admin/user/{id}` | User detail bundle | `bearerAuth` 或 `AuthR15APIKey` | path:`id`*; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang` |
| DELETE | `/api/admin/user/{id}` | Delete one user | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/user/{id}/sessions` | Force a user offline everywhere | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/user/{id}/settings` | Read the selected managed user's country and display preferences | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/user/{id}/settings` | Update the selected managed user's country and display preferences | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/user/{id}/sync` | Sync all external connections of a user | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/user/{id}/totp` | Reset a user's two-step verification | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-025"></a>

### admin/webhooks

Outgoing webhooks (`webhook:*`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/webhooks` | List current-site administrator receivers and event names | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/webhooks` | Save an administrator webhook receiver | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/webhooks` | Save an administrator webhook receiver | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/webhooks/deliveries` | Page administrator webhook delivery history | `bearerAuth` | query:`page`; query:`limit`; query:`endpointId`; query:`status`; header:`X-Lang` |
| POST | `/api/admin/webhooks/deliveries/{id}/redeliver` | Request redelivery of an administrator webhook record | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/webhooks/{id}` | Delete an administrator receiver and delivery log | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/webhooks/{id}/rotate` | Rotate an administrator webhook signing secret | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/webhooks/{id}/test` | Send a synthetic ping to an administrator receiver | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-026"></a>

### auth

Login, registration, verification codes, upstream (social) login and the current session. Session tokens are HS256 JWTs sent as `Authorization: Bearer`; a rotated token may come back in the response `Authorization` header.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/account/unlock` | Unlock a self-locked account | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| POST | `/api/auth/logout` | Log out the current session (POST alias)（已弃用） | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/auth/logout` | Log out the current session | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/auth/password` | Change own password (logs out other devices) | `bearerAuth` 或 `nativeAccessToken` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/auth/user` | Current user with roles and admin menus | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/auth/user` | Update own profile | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` 或 `AuthR15APIKey` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-API-Key-Timestamp`; header:`X-API-Key-Nonce`; header:`X-API-Key-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/login` | Authenticate password and return session or security handoff | 公开；仍受站点、限流及业务条件约束 | header:`X-Trusted-Device`; header:`X-Client-Env`; header:`X-Lang`; body:application/json |
| POST | `/api/login/challenge` | Complete TOTP or recovery-code login challenge | 公开；仍受站点、限流及业务条件约束 | header:`X-Client-Env`; header:`X-Lang`; body:application/json |
| POST | `/api/login/code` | Authenticate email or SMS code and return security handoff | 公开；仍受站点、限流及业务条件约束 | header:`X-Trusted-Device`; header:`X-Client-Env`; header:`X-Lang`; body:application/json |
| POST | `/api/login/passkey/begin` | Begin discoverable passkey assertion | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| POST | `/api/login/passkey/finish` | Verify passkey assertion for login or pending challenge | 公开；仍受站点、限流及业务条件约束 | query:`sessionId`*; query:`remember`; query:`challengeId`; query:`rememberDevice`; header:`X-Client-Env`; header:`X-Lang`; body:application/json |
| POST | `/api/oidc/apple/callback` | Sign in with Apple form_post relay | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| POST | `/api/oidc/callback` | Finish verified provider response or return pending binding | 可匿名 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/oidc/start/{code}` | Start provider login, signup or selected identity binding | 可匿名 或 `bearerAuth` | path:`code`*; query:`action`; query:`redirect`; query:`remember`; query:`region`; query:`invite`; query:`legal`; query:`connectionId`; query:`platform_challenge`; header:`X-Lang` |
| GET | `/api/oidc/telegram/callback` | Telegram data-auth-url relay | 公开；仍受站点、限流及业务条件约束 | query:`state`; header:`X-Lang` |
| POST | `/api/password/reset` | Reset a forgotten password | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| POST | `/api/register` | Register with explicit region and current legal consent | 公开；仍受站点、限流及业务条件约束 | header:`X-Client-Env`; header:`X-Lang`; body:application/json |
| POST | `/api/verify/send` | Send a verification code | 可匿名 或 `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-027"></a>

### devportal

Self-service OAuth / OIDC apps owned by the caller (`/api/account/apps`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/apps` | List the caller's developer applications | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps` | Create a developer-owned OAuth application | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/developer-policy` | Read current protocol limits and built-in callback | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/policy` | Read developer access and app-creation policy | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/{clientId}` | Get one of own apps（已弃用） | `bearerAuth` | path:`clientId`*; header:`X-Lang` |
| PUT | `/api/account/apps/{clientId}` | Update the caller's OAuth application | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/apps/{clientId}` | Delete an owned application and its issued credentials | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/{clientId}/developer-settings` | Read owned app protocol and logout settings | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/{clientId}/playground/refresh` | Rotate the owner's Playground refresh token | `bearerAuth` | path:`clientId`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/{clientId}/playground/revoke` | Revoke the owner's token through the registered client | `bearerAuth` | path:`clientId`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/{clientId}/playground/start` | Start an owner-bound S256 Playground authorization | `bearerAuth` | path:`clientId`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/{clientId}/playground/token` | Exchange a Playground code or machine grant | `bearerAuth` | path:`clientId`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/{clientId}/playground/userinfo` | Inspect UserInfo for the owner's access token | `bearerAuth` | path:`clientId`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/{clientId}/protocol-errors` | Read recent sanitized OAuth diagnostics | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/{clientId}/rotate` | Rotate a confidential application's shared secret | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/{clientId}/secret/revoke-previous` | End the previous client-secret overlap immediately | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/{clientId}/stats` | Read an owned application's usage counts | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |

<a id="group-028"></a>

### federation-contracts

Connector administration, signed provisioning and SCIM 2.0

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/connectors` | All connector instances with redacted configuration and counters | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/connectors` | Create disabled connector | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/connectors` | Update connector configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/connectors/groups/{id}/role` | Map SCIM group to a local role | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/connectors/jobs` | Synchronization log in operator user data scope | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`connectorId`; query:`status`; query:`direction`; header:`X-Lang` |
| POST | `/api/admin/connectors/jobs/retry` | Requeue visible failed outbound jobs | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/connectors/jobs/{id}/retry` | Requeue one failed outbound job | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/connectors/links/{id}` | Unlink a connector account locally | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/connectors/meta` | Connector kinds, mapping fields and eligible roles | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/connectors/partners` | Paginated partnership records for current site | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`status`; header:`X-Lang` |
| POST | `/api/admin/connectors/partners` | Create partnership record | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/connectors/partners/{pid}` | Replace partnership record | `bearerAuth` | path:`pid`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/connectors/partners/{pid}` | Delete current-site partnership record | `bearerAuth` | path:`pid`*; header:`X-Lang` |
| GET | `/api/admin/connectors/settings` | Connector module settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/connectors/settings` | Partially update connector module settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/connectors/{id}` | Connector configuration and counters | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/connectors/{id}` | Delete connector and local synchronization records | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/connectors/{id}/enabled` | Enable or disable connector | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/connectors/{id}/groups` | Connector groups with member counts | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/connectors/{id}/links` | Paginated linked accounts in operator data scope | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; header:`X-Lang` |
| POST | `/api/admin/connectors/{id}/sync` | Queue full outbound synchronization | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/connectors/{id}/test` | Test configured connector | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/connectors/{id}/token` | Rotate connector bearer token | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/connectors/in/{key}` | Receive an HMAC-signed provisioning event | `connectorHmac` | path:`key`*; header:`X-Nowork-Timestamp`*; header:`X-Nowork-Event`; header:`X-Nowork-Delivery`; header:`X-Lang`; body:application/json |
| POST | `/api/scim/v2/Bulk` | Unsupported SCIM Bulk | `connectorScim` | header:`X-Lang` |
| GET | `/api/scim/v2/Groups` | List SCIM Groups | `connectorScim` | query:`startIndex`; query:`count`; query:`filter`; query:`attributes`; query:`excludedAttributes`; header:`X-Lang` |
| POST | `/api/scim/v2/Groups` | Create SCIM Group | `connectorScim` | query:`attributes`; query:`excludedAttributes`; header:`X-Lang`; body:application/json; body:application/scim+json |
| GET | `/api/scim/v2/Groups/{id}` | Read SCIM Group | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-None-Match`; header:`X-Lang` |
| PUT | `/api/scim/v2/Groups/{id}` | Replace SCIM Group | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| PATCH | `/api/scim/v2/Groups/{id}` | Patch SCIM Group | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| DELETE | `/api/scim/v2/Groups/{id}` | Delete SCIM Group | `connectorScim` | path:`id`*; header:`If-Match`; header:`X-Lang` |
| GET | `/api/scim/v2/Me` | Unsupported SCIM Me | `connectorScim` | header:`X-Lang` |
| GET | `/api/scim/v2/ResourceTypes` | List SCIM ResourceTypes | `connectorScim` | header:`X-Lang` |
| GET | `/api/scim/v2/ResourceTypes/{id}` | Read SCIM ResourceTypes entry | `connectorScim` | path:`id`*; header:`X-Lang` |
| GET | `/api/scim/v2/Schemas` | List SCIM Schemas | `connectorScim` | header:`X-Lang` |
| GET | `/api/scim/v2/Schemas/{id}` | Read SCIM Schemas entry | `connectorScim` | path:`id`*; header:`X-Lang` |
| GET | `/api/scim/v2/ServiceProviderConfig` | SCIM service capabilities | `connectorScim` | header:`X-Lang` |
| GET | `/api/scim/v2/Users` | List SCIM Users | `connectorScim` | query:`startIndex`; query:`count`; query:`filter`; query:`attributes`; query:`excludedAttributes`; header:`X-Lang` |
| POST | `/api/scim/v2/Users` | Create SCIM User | `connectorScim` | query:`attributes`; query:`excludedAttributes`; header:`X-Lang`; body:application/json; body:application/scim+json |
| GET | `/api/scim/v2/Users/{id}` | Read SCIM User | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-None-Match`; header:`X-Lang` |
| PUT | `/api/scim/v2/Users/{id}` | Replace SCIM User | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| PATCH | `/api/scim/v2/Users/{id}` | Patch SCIM User | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| DELETE | `/api/scim/v2/Users/{id}` | Delete SCIM User | `connectorScim` | path:`id`*; header:`If-Match`; header:`X-Lang` |
| POST | `/scim/v2/Bulk` | Unsupported SCIM Bulk | `connectorScim` | header:`X-Lang` |
| GET | `/scim/v2/Groups` | List SCIM Groups | `connectorScim` | query:`startIndex`; query:`count`; query:`filter`; query:`attributes`; query:`excludedAttributes`; header:`X-Lang` |
| POST | `/scim/v2/Groups` | Create SCIM Group | `connectorScim` | query:`attributes`; query:`excludedAttributes`; header:`X-Lang`; body:application/json; body:application/scim+json |
| GET | `/scim/v2/Groups/{id}` | Read SCIM Group | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-None-Match`; header:`X-Lang` |
| PUT | `/scim/v2/Groups/{id}` | Replace SCIM Group | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| PATCH | `/scim/v2/Groups/{id}` | Patch SCIM Group | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| DELETE | `/scim/v2/Groups/{id}` | Delete SCIM Group | `connectorScim` | path:`id`*; header:`If-Match`; header:`X-Lang` |
| GET | `/scim/v2/Me` | Unsupported SCIM Me | `connectorScim` | header:`X-Lang` |
| GET | `/scim/v2/ResourceTypes` | List SCIM ResourceTypes | `connectorScim` | header:`X-Lang` |
| GET | `/scim/v2/ResourceTypes/{id}` | Read SCIM ResourceTypes entry | `connectorScim` | path:`id`*; header:`X-Lang` |
| GET | `/scim/v2/Schemas` | List SCIM Schemas | `connectorScim` | header:`X-Lang` |
| GET | `/scim/v2/Schemas/{id}` | Read SCIM Schemas entry | `connectorScim` | path:`id`*; header:`X-Lang` |
| GET | `/scim/v2/ServiceProviderConfig` | SCIM service capabilities | `connectorScim` | header:`X-Lang` |
| GET | `/scim/v2/Users` | List SCIM Users | `connectorScim` | query:`startIndex`; query:`count`; query:`filter`; query:`attributes`; query:`excludedAttributes`; header:`X-Lang` |
| POST | `/scim/v2/Users` | Create SCIM User | `connectorScim` | query:`attributes`; query:`excludedAttributes`; header:`X-Lang`; body:application/json; body:application/scim+json |
| GET | `/scim/v2/Users/{id}` | Read SCIM User | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-None-Match`; header:`X-Lang` |
| PUT | `/scim/v2/Users/{id}` | Replace SCIM User | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| PATCH | `/scim/v2/Users/{id}` | Patch SCIM User | `connectorScim` | path:`id`*; query:`attributes`; query:`excludedAttributes`; header:`If-Match`; header:`X-Lang`; body:application/json; body:application/scim+json |
| DELETE | `/scim/v2/Users/{id}` | Delete SCIM User | `connectorScim` | path:`id`*; header:`If-Match`; header:`X-Lang` |

<a id="group-029"></a>

### install

First-start install wizard (`internal/install`). While the instance is not installed the server serves only this engine: `/healthz`, a minimal `/api/config` (`installRequired`, `version`), `/api/install/*` and the SPA shell; every other backend path answers 503 with `data.installRequired`. The POST routes need the one-time setup token (printed to the log at startup and kept in the data directory's `setup-token` file) as `token` in the JSON body; 20 wrong tokens per client IP in 10 minutes answer 429. After `finish` the same listener serves the normal app without a restart, and these routes answer 404.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/install/db/test` | Test a database connection | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| POST | `/api/install/env` | Environment report for the wizard | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| POST | `/api/install/finish` | Finish the install | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| POST | `/api/install/mail/test` | Send a test e-mail with the given SMTP settings | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| GET | `/api/install/status` | Install-mode probe | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| POST | `/api/install/verify` | Check the setup token | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |

<a id="group-030"></a>

### membership

Site-scoped membership terms, one-time trials, genuine renewal orders and explicitly consented provider recurring agreements.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/membership/subscription` | Read own membership, renewal and recurring state | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/membership/subscription/cancel` | Disable renewal for the current membership | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/membership/subscription/recurring` | Authorize a priced provider recurring mandate | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Idempotency-Key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/membership/subscription/recurring/options` | List implemented recurring providers for the agreed plan | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/membership/subscription/recurring/refresh` | Reconcile the owned recurring agreement with its provider | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/membership/subscription/renew` | Create or reuse a genuine renewal order | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Idempotency-Key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| PUT | `/api/account/membership/subscription/renewal` | Change renewal reminders or request provider cancellation | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/membership/subscription/subscribe` | Create an idempotent membership purchase at an exact price | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Idempotency-Key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/membership/subscription/trial` | Start an eligible one-time plan trial | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Idempotency-Key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/plans` | List current-site membership prices and trial/renewal terms | 可匿名 或 `bearerAuth` | header:`X-Lang` |

<a id="group-031"></a>

### merchant-protocol

Self-hosted merchant relay: signed protocol compatibility endpoints

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/alipay/gateway.do` | Self-hosted Alipay-shaped trade gateway | 公开；仍受站点、限流及业务条件约束 | query:`app_id`*; query:`sign`*; query:`notify_url`; query:`return_url`; query:`method`*; query:`sign_type`*; query:`charset`; query:`version`; query:`format`; query:`timestamp`*; query:`biz_content`*; header:`X-Lang` |
| POST | `/alipay/gateway.do` | Self-hosted Alipay-shaped trade gateway | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api.php` | EasyPay merchant balance or order query | 公开；仍受站点、限流及业务条件约束 | query:`pid`*; query:`key`*; query:`trade_no`; query:`out_trade_no`; query:`act`*; header:`X-Lang` |
| POST | `/api.php` | EasyPay merchant balance or order query | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api/pay/merchant/alipay/gateway.do` | Self-hosted Alipay-shaped trade gateway | 公开；仍受站点、限流及业务条件约束 | query:`app_id`*; query:`sign`*; query:`notify_url`; query:`return_url`; query:`method`*; query:`sign_type`*; query:`charset`; query:`version`; query:`format`; query:`timestamp`*; query:`biz_content`*; header:`X-Lang` |
| POST | `/api/pay/merchant/alipay/gateway.do` | Self-hosted Alipay-shaped trade gateway | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api/pay/merchant/codepay/creat_order/` | Codepay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | query:`id`*; query:`pay_id`*; query:`name`; query:`notify_url`*; query:`return_url`; query:`param`; query:`sign`*; query:`type`*; query:`price`*; header:`X-Lang` |
| POST | `/api/pay/merchant/codepay/creat_order/` | Codepay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api/pay/merchant/epay/api.php` | EasyPay merchant balance or order query | 公开；仍受站点、限流及业务条件约束 | query:`pid`*; query:`key`*; query:`trade_no`; query:`out_trade_no`; query:`act`*; header:`X-Lang` |
| POST | `/api/pay/merchant/epay/api.php` | EasyPay merchant balance or order query | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api/pay/merchant/epay/mapi.php` | EasyPay signed checkout URL | 公开；仍受站点、限流及业务条件约束 | query:`pid`*; query:`out_trade_no`*; query:`type`*; query:`name`*; query:`notify_url`*; query:`return_url`; query:`param`; query:`sign`*; query:`money`*; query:`sign_type`; header:`X-Lang` |
| POST | `/api/pay/merchant/epay/mapi.php` | EasyPay signed checkout URL | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api/pay/merchant/epay/submit.php` | EasyPay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | query:`pid`*; query:`out_trade_no`*; query:`type`*; query:`name`*; query:`notify_url`*; query:`return_url`; query:`param`; query:`sign`*; query:`money`*; query:`sign_type`; header:`X-Lang` |
| POST | `/api/pay/merchant/epay/submit.php` | EasyPay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/api/pay/merchant/return/{orderNo}` | Return buyer to approved merchant URL | 公开；仍受站点、限流及业务条件约束 | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/pay/merchant/v1/orders` | Create HMAC-authenticated relay order | 公开；仍受站点、限流及业务条件约束 | header:`X-Merchant-Id`*; header:`X-Merchant-Timestamp`*; header:`X-Merchant-Nonce`*; header:`X-Merchant-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/pay/merchant/v1/orders/{externalNo}` | Query HMAC-authenticated relay order | 公开；仍受站点、限流及业务条件约束 | path:`externalNo`*; header:`X-Merchant-Id`*; header:`X-Merchant-Timestamp`*; header:`X-Merchant-Nonce`*; header:`X-Merchant-Signature`*; header:`X-Lang` |
| GET | `/api/pay/merchant/wechatpay/v3/certificates` | Encrypted relay platform certificate | `merchantWechatAuth` | header:`X-Lang` |
| GET | `/api/pay/merchant/wechatpay/v3/pay/transactions/id/{transactionID}` | Query relay transaction by merchant or relay number | `merchantWechatAuth` | path:`transactionID`*; query:`mchid`*; header:`X-Lang` |
| GET | `/api/pay/merchant/wechatpay/v3/pay/transactions/out-trade-no/{externalNo}` | Query relay transaction by merchant or relay number | `merchantWechatAuth` | path:`externalNo`*; query:`mchid`*; header:`X-Lang` |
| POST | `/api/pay/merchant/wechatpay/v3/pay/transactions/out-trade-no/{externalNo}/close` | Close unpaid relay transaction | `merchantWechatAuth` | path:`externalNo`*; header:`X-Lang`; body:application/json |
| POST | `/api/pay/merchant/wechatpay/v3/pay/transactions/{flow}` | Create hosted checkout via WeChat-shaped protocol | `merchantWechatAuth` | path:`flow`*; header:`X-Lang`; body:application/json |
| POST | `/api/pay/merchant/wechatpay/v3/refund/domestic/refunds` | Request merchant relay refund | `merchantWechatAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/pay/merchant/wechatpay/v3/refund/domestic/refunds/{externalNo}` | Query merchant relay refund by out_refund_no | `merchantWechatAuth` | path:`externalNo`*; header:`X-Lang` |
| GET | `/api/pay/notify/{channelId}` | Verify and apply upstream gateway notification | 公开；仍受站点、限流及业务条件约束 | path:`channelId`*; header:`Stripe-Signature`; header:`Wechatpay-Serial`; header:`Wechatpay-Timestamp`; header:`Wechatpay-Nonce`; header:`Wechatpay-Signature`; header:`PAYPAL-AUTH-ALGO`; header:`PAYPAL-CERT-URL`; header:`PAYPAL-TRANSMISSION-ID`; header:`PAYPAL-TRANSMISSION-SIG`; header:`PAYPAL-TRANSMISSION-TIME`; query:`app_id`; query:`seller_id`; query:`out_trade_no`; query:`trade_no`; query:`trade_status`; query:`gmt_payment`; query:`refund_fee`; query:`gmt_refund`; query:`buyer_logon_id`; query:`sign`; query:`sign_type`; query:`alipay_cert_sn`; query:`total_amount`; query:`pid`; query:`timestamp`; query:`money`; query:`id`; query:`pay_id`; query:`pay_no`; query:`pay_time`; query:`type`; query:`param`; query:`price`; query:`payId`; query:`reallyPrice`; header:`X-Lang`; body:application/json; body:application/x-www-form-urlencoded |
| POST | `/api/pay/notify/{channelId}` | Verify and apply upstream gateway notification | 公开；仍受站点、限流及业务条件约束 | path:`channelId`*; header:`Stripe-Signature`; header:`Wechatpay-Serial`; header:`Wechatpay-Timestamp`; header:`Wechatpay-Nonce`; header:`Wechatpay-Signature`; header:`PAYPAL-AUTH-ALGO`; header:`PAYPAL-CERT-URL`; header:`PAYPAL-TRANSMISSION-ID`; header:`PAYPAL-TRANSMISSION-SIG`; header:`PAYPAL-TRANSMISSION-TIME`; query:`app_id`; query:`seller_id`; query:`out_trade_no`; query:`trade_no`; query:`trade_status`; query:`gmt_payment`; query:`refund_fee`; query:`gmt_refund`; query:`buyer_logon_id`; query:`sign`; query:`sign_type`; query:`alipay_cert_sn`; query:`total_amount`; query:`pid`; query:`timestamp`; query:`money`; query:`id`; query:`pay_id`; query:`pay_no`; query:`pay_time`; query:`type`; query:`param`; query:`price`; query:`payId`; query:`reallyPrice`; header:`X-Lang`; body:application/json; body:application/x-www-form-urlencoded |
| GET | `/codepay/creat_order/` | Codepay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | query:`id`*; query:`pay_id`*; query:`name`; query:`notify_url`*; query:`return_url`; query:`param`; query:`sign`*; query:`type`*; query:`price`*; header:`X-Lang` |
| POST | `/codepay/creat_order/` | Codepay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/mapi.php` | EasyPay signed checkout URL | 公开；仍受站点、限流及业务条件约束 | query:`pid`*; query:`out_trade_no`*; query:`type`*; query:`name`*; query:`notify_url`*; query:`return_url`; query:`param`; query:`sign`*; query:`money`*; query:`sign_type`; header:`X-Lang` |
| POST | `/mapi.php` | EasyPay signed checkout URL | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/submit.php` | EasyPay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | query:`pid`*; query:`out_trade_no`*; query:`type`*; query:`name`*; query:`notify_url`*; query:`return_url`; query:`param`; query:`sign`*; query:`money`*; query:`sign_type`; header:`X-Lang` |
| POST | `/submit.php` | EasyPay signed hosted checkout | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/v3/certificates` | Encrypted relay platform certificate | `merchantWechatAuth` | header:`X-Lang` |
| GET | `/v3/pay/transactions/id/{transactionID}` | Query relay transaction by merchant or relay number | `merchantWechatAuth` | path:`transactionID`*; query:`mchid`*; header:`X-Lang` |
| GET | `/v3/pay/transactions/out-trade-no/{externalNo}` | Query relay transaction by merchant or relay number | `merchantWechatAuth` | path:`externalNo`*; query:`mchid`*; header:`X-Lang` |
| POST | `/v3/pay/transactions/out-trade-no/{externalNo}/close` | Close unpaid relay transaction | `merchantWechatAuth` | path:`externalNo`*; header:`X-Lang`; body:application/json |
| POST | `/v3/pay/transactions/{flow}` | Create hosted checkout via WeChat-shaped protocol | `merchantWechatAuth` | path:`flow`*; header:`X-Lang`; body:application/json |
| POST | `/v3/refund/domestic/refunds` | Request merchant relay refund | `merchantWechatAuth` | header:`X-Lang`; body:application/json |
| GET | `/v3/refund/domestic/refunds/{externalNo}` | Query merchant relay refund by out_refund_no | `merchantWechatAuth` | path:`externalNo`*; header:`X-Lang` |
| GET | `/wechatpay/v3/certificates` | Encrypted relay platform certificate | `merchantWechatAuth` | header:`X-Lang` |
| GET | `/wechatpay/v3/pay/transactions/id/{transactionID}` | Query relay transaction by merchant or relay number | `merchantWechatAuth` | path:`transactionID`*; query:`mchid`*; header:`X-Lang` |
| GET | `/wechatpay/v3/pay/transactions/out-trade-no/{externalNo}` | Query relay transaction by merchant or relay number | `merchantWechatAuth` | path:`externalNo`*; query:`mchid`*; header:`X-Lang` |
| POST | `/wechatpay/v3/pay/transactions/out-trade-no/{externalNo}/close` | Close unpaid relay transaction | `merchantWechatAuth` | path:`externalNo`*; header:`X-Lang`; body:application/json |
| POST | `/wechatpay/v3/pay/transactions/{flow}` | Create hosted checkout via WeChat-shaped protocol | `merchantWechatAuth` | path:`flow`*; header:`X-Lang`; body:application/json |
| POST | `/wechatpay/v3/refund/domestic/refunds` | Request merchant relay refund | `merchantWechatAuth` | header:`X-Lang`; body:application/json |
| GET | `/wechatpay/v3/refund/domestic/refunds/{externalNo}` | Query merchant relay refund by out_refund_no | `merchantWechatAuth` | path:`externalNo`*; header:`X-Lang` |

<a id="group-032"></a>

### modules

Feature-module registry and the per-server module whitelist (`docs/MODULES.md`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/game/server/{id}/modules` | Server-scope modules of a server with its switches and configs | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/modules` | Save module switches and configs of a server | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/modules` | Registered feature modules | `bearerAuth` | header:`X-Lang` |
| GET | `/server/v1/modules` | Modules enabled for the calling server, with their configs | `serverKey` | header:`X-Lang` |

<a id="group-033"></a>

### modules/applications

Versioned account requests for invite, whitelist, developer access and custom review, with private forms and existing domain grant checks.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/applications/export` | Export scoped request snapshots | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`typeId`; query:`status`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/applications/managed/export` | Export scoped request snapshots | `bearerAuth` | query:`typeId`; query:`status`; header:`X-Lang` |
| GET | `/api/account/applications/managed/facts` | List safe request requirement facts | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/applications/managed/preview` | Preview a request requirement without a grant | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/applications/managed/servers` | List allowed approved whitelist servers | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/applications/managed/submissions` | List scoped account requests | `bearerAuth` | query:`page`; query:`limit`; query:`typeId`; query:`status`; header:`X-Lang` |
| GET | `/api/account/applications/managed/submissions/{id}` | Read a request snapshot and progress history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/account/applications/managed/submissions/{id}` | Redact terminal request form data | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/applications/managed/submissions/{id}/decision` | Decide or retry request fulfilment | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/applications/managed/submissions/{id}/files/{file}` | Download a submitted private attachment | `bearerAuth` | path:`id`*; path:`file`*; header:`X-Lang` |
| GET | `/api/account/applications/managed/types` | List request types available to this manager | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/applications/managed/types` | Create or version-update an account request type | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/account/applications/managed/types/{id}` | Archive a request type without unfinished submissions | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/applications/submissions` | List scoped account requests | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`typeId`; query:`status`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/applications/submissions` | Submit one idempotent versioned account request | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/applications/submissions/{id}` | Read a request snapshot and progress history | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| DELETE | `/api/account/applications/submissions/{id}` | Redact terminal request form data | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/applications/submissions/{id}/files/{file}` | Download a submitted private attachment | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`file`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/applications/submissions/{id}/withdraw` | Withdraw own pending request | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/applications/types` | List eligible request types and exact form versions | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/applications/types/{id}/files/{file}` | Download own staged application attachment | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`file`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/applications/types/{id}/files/{key}` | Upload a private application-form attachment | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/applications/export` | Export scoped request snapshots | `bearerAuth` | query:`typeId`; query:`status`; header:`X-Lang` |
| GET | `/api/admin/applications/facts` | List safe request requirement facts | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/applications/preview` | Preview a request requirement without a grant | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/applications/servers` | List allowed approved whitelist servers | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/applications/submissions` | List scoped account requests | `bearerAuth` | query:`page`; query:`limit`; query:`typeId`; query:`status`; query:`userId`; header:`X-Lang` |
| GET | `/api/admin/applications/submissions/{id}` | Read a request snapshot and progress history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/applications/submissions/{id}` | Redact terminal request form data | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/applications/submissions/{id}/decision` | Decide or retry request fulfilment | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/applications/submissions/{id}/files/{file}` | Download a submitted private attachment | `bearerAuth` | path:`id`*; path:`file`*; header:`X-Lang` |
| GET | `/api/admin/applications/types` | List request types available to this manager | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/applications/types` | Create or version-update an account request type | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/applications/types/{id}` | Archive a request type without unfinished submissions | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-034"></a>

### modules/apps

Native App beta1 API; device proofs, scoped approval enrollments, release/security policies, support and realtime. See docs/app/API.md and docs/app/SCREENS.md.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/.well-known/apple-app-site-association` | Apple universal-link association | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/.well-known/assetlinks.json` | Android verified-link association | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| POST | `/api/account/apps/action-approvals` | Prepare exact browser action for App approval | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/action-approvals/policy` | Read own App action approval requirements | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/apps/action-approvals/{id}/cancel` | Poll or cancel browser App action approval | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/action-approvals/{id}/poll` | Poll or cancel browser App action approval | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/approval-response/devices` | Read own devices' approval-response eligibility | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/apps/approval-response/devices/{id}/enrollments` | Start device approval-response consent ceremony | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/approval-response/devices/{id}/revoke` | Withdraw approval-response capability | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/approval-response/enrollments/{id}` | Read or cancel own device consent enrollment | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/apps/approval-response/enrollments/{id}/cancel` | Read or cancel own device consent enrollment | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/apps/approvals` | Read or decide eligible account App approvals | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/approvals/{id}` | Read or decide eligible account App approvals | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/approvals/{id}/decision` | Read or decide eligible account App approvals | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/binding` | Read native binding requirement | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/devices` | List registered devices | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/devices` | Bind Ed25519 device | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/devices/challenge` | Start signed device-binding challenge | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/apps/devices/{id}/revoke` | Revoke registered device | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/native/admin-grants` | Approve an independent five-minute native admin grant | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/native/admin-grants/{id}/revoke` | Revoke an owned native admin grant | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/capabilities` | Discover currently available native staff actions | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/notifications/campaigns` | List visible notification campaigns | `nativeAccessToken` + `nativeAdminGrant` | query:`page`; query:`limit`; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/native/admin/notifications/preview` | Preview or send account notification | `nativeAccessToken` + `nativeAdminGrant` | header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/native/admin/notifications/send` | Preview or send account notification | `nativeAccessToken` + `nativeAdminGrant` | header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/admin/reviews/realname` | Page masked real-name review records | `nativeAccessToken` + `nativeAdminGrant` | query:`page`; query:`limit`; query:`status`; query:`keywords`; query:`sort`; query:`order`; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/reviews/realname/{id}` | Read one unmasked real-name review record | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/native/admin/reviews/realname/{id}/decision` | Approve or reject a scoped real-name submission | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/admin/reviews/realname/{id}/photo/{slot}` | Read a private real-name document photo | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; path:`slot`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/servers` | List server summaries within native admin scope | `nativeAccessToken` + `nativeAdminGrant` | query:`reviewStatus`; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/servers/{id}` | Read a scoped server summary | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| PUT | `/api/account/apps/native/admin/servers/{id}/status` | Set a scoped server's operating status | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/admin/tickets` | List accessible support tickets | `nativeAccessToken` + `nativeAdminGrant` | query:`page`; query:`limit`; query:`status`; query:`priority`; query:`category`; query:`assignee`; query:`keyword`; query:`from`; query:`to`; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/tickets/settings` | Read ticket settings | `nativeAccessToken` + `nativeAdminGrant` | header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/tickets/{id}` | Read authorized ticket thread | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; query:`before`; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| PUT | `/api/account/apps/native/admin/tickets/{id}` | Update ticket status, priority, assignment or custom fields | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/admin/tickets/{id}/assignees` | List ticket-eligible assignees | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/native/admin/tickets/{id}/attachments` | Upload pending ticket attachment | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/apps/native/admin/tickets/{id}/attachments/{attachmentId}` | Download authorized ticket attachment | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; path:`attachmentId`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| DELETE | `/api/account/apps/native/admin/tickets/{id}/attachments/{attachmentId}` | Remove own pending ticket attachment | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; path:`attachmentId`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/tickets/{id}/custom-fields` | Read ticket custom-field schema and values | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/native/admin/tickets/{id}/reply` | Reply to ticket or add internal note | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Admin-Grant`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/admin/users` | Page users within the native administrator's data scope | `nativeAccessToken` + `nativeAdminGrant` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`username`; query:`nickname`; query:`email`; query:`phone`; query:`uid`; query:`status`; query:`securityLevel`; query:`realNameStatus`; query:`roleId`; query:`createTimeStart`; query:`createTimeEnd`; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/account/apps/native/admin/users/{id}` | Read a scoped native admin user summary | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| PUT | `/api/account/apps/native/admin/users/{id}/status` | Freeze or restore a scoped user | `nativeAccessToken` + `nativeAdminGrant` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/browser-login/{id}` | Read and bind a browser login intent to the native device | `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/native/browser-login/{id}/decision` | Approve or deny the bound restricted browser login | `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/native/credential-approvals` | Approve one exact native credential change | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/device-policy` | Read or repair native device admission report | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/native/device-policy` | Read or repair native device admission report | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/apps/native/realtime-ticket` | Mint one-use native realtime ticket | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/native/sessions` | List account native session families | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/native/sessions/{id}/revoke` | Revoke native session family | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/native/support/reports` | List private App reports | `bearerAuth` 或 `nativeAccessToken` | query:`page`; query:`limit`; query:`appId`; query:`kind`; query:`state`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/native/support/reports/{id}` | Read or delete a private App report | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| DELETE | `/api/account/apps/native/support/reports/{id}` | Read or delete a private App report | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/native/support/reports/{id}/attachments/{attachmentId}` | Download private structured diagnostic attachment | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; path:`attachmentId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/push/config/{appId}` | Read public push registration configuration | `bearerAuth` 或 `nativeAccessToken` | path:`appId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/push/devices` | List own push registrations | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/apps/push/devices/{id}` | Register or rotate device push target | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/apps/push/devices/{id}` | Withdraw push consent and cancel deliveries | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/push/devices/{id}/challenge` | Start signed push registration challenge | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/push/notifications/deliveries/{id}/ack` | Record exact-device notification ack | `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/account/apps/push/notifications/deliveries/{id}/read` | Record exact-device notification read | `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| PUT | `/api/account/apps/push/notifications/devices/{id}/preference` | Set separate App account-notification consent | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/push/notifications/inbox` | Read own App notification inbox | `bearerAuth` 或 `nativeAccessToken` | query:`page`; query:`pageSize`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/push/notifications/inbox/read` | Mark App notifications read from browser | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/push/notifications/preferences` | Read own App notification consent per device | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/apps/action-policies` | Read site and App action approval policies | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/apps/action-policies/site` | Set site App approval actions | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications` | List native application registrations | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/apps/applications` | Save native application registration | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/apps/applications/{id}` | Save native application registration | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/apps/applications/{id}/action-policy` | Set an App's approval action policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/approval-response-policy` | Read or update App approval-response policy | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/approval-response-policy` | Read or update App approval-response policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/first-party-policy` | Read or update first-party native login policy | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/first-party-policy` | Read or update first-party native login policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/native-admin-policy` | Read an App's native admin policy | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/native-admin-policy` | Save an App's versioned native admin policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/native-browser-policy` | Read an App's restricted browser-login policy | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/native-browser-policy` | Save an App's versioned browser-login policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/policy` | Read App security and remote configuration policy | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/policy` | Validate or save App policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/policy/revisions` | Read App policy history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/apps/applications/{id}/policy/rollback` | Restore an earlier App policy as a new revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/apps/applications/{id}/policy/validate` | Validate or save App policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/release-tracks` | List App release tracks | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/release-tracks/{trackId}` | Adjust App release rollout and pause | `bearerAuth` | path:`id`*; path:`trackId`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/releases` | List App release snapshots | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`arch`; query:`channel`; header:`X-Lang` |
| POST | `/api/admin/apps/applications/{id}/releases` | Save App release draft | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/applications/{id}/releases/{releaseId}` | Read App release snapshot | `bearerAuth` | path:`id`*; path:`releaseId`*; header:`X-Lang` |
| PUT | `/api/admin/apps/applications/{id}/releases/{releaseId}` | Save App release draft | `bearerAuth` | path:`id`*; path:`releaseId`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/apps/applications/{id}/releases/{releaseId}` | Delete unpublished App release | `bearerAuth` | path:`id`*; path:`releaseId`*; query:`revision`*; header:`X-Lang` |
| POST | `/api/admin/apps/applications/{id}/releases/{releaseId}/publish` | Apply App release publish | `bearerAuth` | path:`id`*; path:`releaseId`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/apps/applications/{id}/releases/{releaseId}/rollback` | Apply App release rollback | `bearerAuth` | path:`id`*; path:`releaseId`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/devices` | List registered devices | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/apps/devices/{id}/revoke` | Revoke registered device | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/push/deliveries` | List push delivery attempts | `bearerAuth` | query:`requestId`; header:`X-Lang` |
| GET | `/api/admin/apps/push/notifications` | List visible App notification tasks | `bearerAuth` | query:`page`; query:`pageSize`; query:`appId`; header:`X-Lang` |
| POST | `/api/admin/apps/push/notifications` | Schedule idempotent multilingual App notification | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/push/notifications/options` | Read App notification composition options | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/apps/push/notifications/preview` | Preview App notification eligible devices | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/push/notifications/{id}` | Read visible App notification and statistics | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/apps/push/notifications/{id}/cancel` | Cancel App notification task | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/apps/push/notifications/{id}/deliveries` | Read visible App notification deliveries | `bearerAuth` | path:`id`*; query:`page`; query:`pageSize`; header:`X-Lang` |
| GET | `/api/admin/apps/push/providers` | List redacted push provider configurations | `bearerAuth` | query:`appId`; header:`X-Lang` |
| PUT | `/api/admin/apps/push/providers/{appId}/{provider}` | Save encrypted push provider credentials | `bearerAuth` | path:`appId`*; path:`provider`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/apps/push/settings` | Set global push delivery switch | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/settings` | Read effective native app settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/apps/settings` | Update global native app settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/support/bans` | List App device-key bans | `bearerAuth` | query:`page`; query:`limit`; query:`appId`; header:`X-Lang` |
| DELETE | `/api/admin/apps/support/bans/{id}` | Remove an App device-key ban | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/apps/support/devices` | Search site App devices and ban state | `bearerAuth` | query:`page`; query:`limit`; query:`appId`; query:`search`; header:`X-Lang` |
| POST | `/api/admin/apps/support/devices/{id}/ban` | Ban the App device signing key | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/apps/support/purge` | Purge expired App support reports | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/apps/support/reports` | List private App reports | `bearerAuth` | query:`page`; query:`limit`; query:`appId`; query:`kind`; query:`state`; header:`X-Lang` |
| GET | `/api/admin/apps/support/reports/{id}` | Read or delete a private App report | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/apps/support/reports/{id}` | Read or delete a private App report | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/apps/support/reports/{id}/attachments/{attachmentId}` | Download private structured diagnostic attachment | `bearerAuth` | path:`id`*; path:`attachmentId`*; header:`X-Lang` |
| GET | `/api/admin/apps/support/reports/{id}/download` | Download private App report JSON | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/apps/support/reports/{id}/redact` | Redact private App report content | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/apps/support/settings` | Read or update App support settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/apps/support/settings` | Read or update App support settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/apps/support/statistics` | Read consented App installation and version statistics | `bearerAuth` | query:`days`; query:`appId`; header:`X-Lang` |
| GET | `/api/app/v1/approval-response/{appId}/devices/{deviceId}/enrollments/{id}` | Inspect device consent challenge using its signing key | `AppsR15DeviceSignature` | path:`appId`*; path:`deviceId`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; path:`id`*; header:`X-Lang` |
| POST | `/api/app/v1/approval-response/{appId}/devices/{deviceId}/enrollments/{id}/confirm` | Confirm independent device approval consent | `AppsR15DeviceSignature` | path:`appId`*; path:`deviceId`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/approval-response/{appId}/devices/{deviceId}/requests` | Read eligible exact-device approval requests | `AppsR15DeviceSignature` | path:`appId`*; path:`deviceId`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| GET | `/api/app/v1/approval-response/{appId}/devices/{deviceId}/requests/{id}` | Read eligible exact-device approval requests | `AppsR15DeviceSignature` | path:`appId`*; path:`deviceId`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; path:`id`*; header:`X-Lang` |
| POST | `/api/app/v1/approval-response/{appId}/devices/{deviceId}/requests/{id}/decision` | Sign an eligible approval decision | `AppsR15DeviceSignature` | path:`appId`*; path:`deviceId`*; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/bootstrap` | Discover App policy, native navigation and support | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | query:`appId`*; query:`version`; query:`arch`; query:`channel`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/app/v1/config` | Resolve App config | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | query:`appId`*; query:`platform`*; query:`version`*; query:`arch`; query:`channel`; query:`locale`; query:`deviceId`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/app/v1/crash` | Submit a private App crash report | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/deep-link` | Resolve configured native deep link | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | query:`action`*; query:`value`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/app/v1/device-policy` | Repair enrolled-device admission without a native token | `AppsR15DeviceSignature` | query:`appId`*; query:`deviceId`*; query:`enrollmentId`; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/app/v1/device-policy` | Repair enrolled-device admission without a native token | `AppsR15DeviceSignature` | query:`appId`*; query:`deviceId`*; query:`enrollmentId`; header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/feedback` | Submit a private App feedback report | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/i18n/{locale}` | Read versioned native locale bundle | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | path:`locale`*; header:`If-None-Match`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/app/v1/login-approvals/{id}/cancel` | Continue a browser login App factor | 公开；仍受站点、限流及业务条件约束 | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/login-approvals/{id}/complete` | Continue a browser login App factor | 公开；仍受站点、限流及业务条件约束 | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/login-approvals/{id}/poll` | Continue a browser login App factor | 公开；仍受站点、限流及业务条件约束 | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/browser-login` | Start a native-approved restricted browser login | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| POST | `/api/app/v1/native/browser-login/{id}/passkey/begin` | Begin the approved browser intent's passkey verification | `nativeBrowserIntentCookie` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/browser-login/{id}/passkey/finish` | Verify a passkey only for the approved browser intent | `nativeBrowserIntentCookie` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/browser-login/{id}/poll` | Poll and consume an approved restricted browser login | `nativeBrowserIntentCookie` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/native/capabilities` | Discover native session capabilities | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/app/v1/native/first-party/approval` | Poll and finish the native flow's App approval | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/first-party/authenticate` | Verify password or contact code in native flow | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/first-party/begin` | Create an explicitly consented device login flow | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/first-party/cancel` | Cancel native login flow and pending ceremonies | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/native/first-party/capabilities` | Discover first-party native login methods | 公开；仍受站点、限流及业务条件约束 | query:`appId`*; query:`clientId`*; header:`X-Lang` |
| POST | `/api/app/v1/native/first-party/challenge` | Verify TOTP or recovery code in native flow | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/first-party/code/send` | Send native login code with CAPTCHA policy | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/first-party/passkey/begin` | Begin passkey assertion within the native flow | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/first-party/passkey/finish` | Finish native passkey assertion | `AppsR15DeviceSignature` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| POST | `/api/app/v1/native/refresh` | Rotate single-use native refresh token | `nativeRefreshToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang` |
| POST | `/api/app/v1/native/token` | Exchange OAuth access token for native session | `oidcAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/policy` | Resolve selected App configuration and admission | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | query:`appId`; query:`deviceId`; query:`platform`; query:`version`; query:`arch`; query:`channel`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/app/v1/qr` | Start browser QR login approval | 可匿名 或 `bearerAuth` | header:`X-Lang` |
| POST | `/api/app/v1/qr/{id}/poll` | Poll and consume browser QR approval | 可匿名 或 `bearerAuth` | path:`id`*; cookie:`sakura_app_qr`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/realtime` | Upgrade one-use signed native realtime connection | `AppsR15RealtimeSignature` + `AppsR15RealtimeTicket` | header:`Upgrade`*; header:`Connection`*; header:`Sec-WebSocket-Key`*; header:`Sec-WebSocket-Version`*; header:`Sec-WebSocket-Protocol`*; header:`X-Native-Realtime-Ticket`*; header:`X-Native-Realtime-Signature`*; header:`X-Native-Timestamp`*; header:`X-Lang` |
| GET | `/api/app/v1/support` | Read App support collection capabilities | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/app/v1/telemetry` | Record consented native installation or activity | `nativeAccessToken` | header:`X-Native-Timestamp`*; header:`X-Native-Nonce`*; header:`X-Native-Signature`*; header:`X-Lang`; body:application/json |
| GET | `/api/app/v1/update` | Resolve App update | 可匿名 或 `bearerAuth` 或 `nativeAccessToken` | query:`appId`*; query:`platform`*; query:`version`*; query:`arch`; query:`channel`; query:`locale`; query:`deviceId`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/login-policy/setup/binding-approvals` | Continue scoped setup binding approval | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/binding-approvals/{id}/cancel` | Continue scoped setup binding approval | `OpsLoginSetup` | path:`id`*; header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/binding-approvals/{id}/poll` | Continue scoped setup binding approval | `OpsLoginSetup` | path:`id`*; header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/credential-approvals` | Continue scoped setup credential approval | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/credential-approvals/{id}/cancel` | Continue scoped setup credential approval | `OpsLoginSetup` | path:`id`*; header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/credential-approvals/{id}/poll` | Continue scoped setup credential approval | `OpsLoginSetup` | path:`id`*; header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |

<a id="group-035"></a>

### modules/automation

Permission-checked workflows, immutable revisions, idempotent runs and transaction-backed actions; catalog is authoritative for installed action kinds.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/automation/points` | Read own nonmonetary points and entries | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/automation/actions/mail` | Page automation mail deliveries | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Lang` |
| GET | `/api/admin/automation/actions/plugins` | Page automation plugin-hook deliveries | `bearerAuth` | query:`page`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/automation/actions/reviews` | Page automation reviews deliveries | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Lang` |
| GET | `/api/admin/automation/catalog` | List event, action and safe-input catalogs | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/automation/dry-run` | Prepare workflow without side effects | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/automation/runs/{id}/steps` | List committed run receipts | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/automation/segments` | List saved user segments | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/automation/segments` | Create a user segment | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/automation/segments/{id}/test` | Evaluate a segment for a managed user | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/automation/users/{id}/tags` | List managed user's tags | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/automation/users/{id}/tags` | Replace managed user's tags | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/automation/workflows` | List workflows | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/automation/workflows` | Save workflow and immutable revision | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/automation/workflows/{id}` | Save workflow and immutable revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/automation/workflows/{id}/enabled` | Enable or disable workflow | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/automation/workflows/{id}/inbound-key` | Rotate inbound workflow signing secret | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/automation/workflows/{id}/revisions` | List latest workflow revisions | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/automation/workflows/{id}/run` | Idempotently enqueue a manual workflow run | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/automation/workflows/{id}/runs` | List latest workflow runs | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/automation/inbound/{id}` | Accept signed inbound workflow event | `OpsAutomationSignature` | path:`id`*; header:`X-Automation-Timestamp`*; header:`X-Automation-Event-ID`*; header:`X-Automation-Signature`*; header:`X-Lang`; body:application/json |

<a id="group-036"></a>

### modules/batch-operations

Bounded administrator batch mutations using the original route guards.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/admin/batch` | Process a selected batch in one HTTP request | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-037"></a>

### modules/binding-policy

Provider binding limits, cooldown and requirement trees; QQ permanence is an independent mandatory constraint.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/account/binding-policy/preview` | Explain own selected binding action | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/admin/binding-policy/facts` | List current binding requirement facts | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/binding-policy/policies` | List effective binding policies | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/binding-policy/policies/{provider}` | Save a versioned provider binding policy | `bearerAuth` | path:`provider`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/binding-policy/preview` | Explain binding eligibility for a managed account | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-038"></a>

### modules/bots

Bot management, chat binding, native signed callbacks and framework HTTP/WebSocket delivery. Tokens and platform secrets are write-only except one-time issuance.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/bots` | Read own chat bindings and notification choices | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/bots/bindings/{id}` | Remove own chat binding | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/bots/code` | Issue a single-use chat bind code | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/account/bots/prefs` | Change chat notification opt-ins | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/bots` | List bots | `bearerAuth` | query:`platform`; query:`keyword`; header:`X-Lang` |
| POST | `/api/admin/bots` | Create bot | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/bots` | Update bot configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/bots/bindings` | Page chat bindings within user scope | `bearerAuth` | query:`page`; query:`limit`; query:`platform`; query:`keyword`; header:`X-Lang` |
| DELETE | `/api/admin/bots/bindings/{id}` | Remove scoped chat binding | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/bots/broadcast` | Queue a broadcast to enabled channels | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/bots/channels` | List notification channels | `bearerAuth` | query:`botId`; header:`X-Lang` |
| POST | `/api/admin/bots/channels` | Save notification channel | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/bots/channels` | Save notification channel | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/bots/channels/{id}` | Delete notification channel | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/bots/logs` | Page bot exchanges | `bearerAuth` | query:`page`; query:`limit`; query:`botId`; query:`direction`; query:`failed`; header:`X-Lang` |
| GET | `/api/admin/bots/meta` | Read platform and command configuration catalogs | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/bots/outbox` | Page bot delivery queue | `bearerAuth` | query:`page`; query:`limit`; query:`botId`; query:`status`; header:`X-Lang` |
| POST | `/api/admin/bots/outbox/{id}/retry` | Retry failed bot delivery | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/bots/settings` | Read bot module settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/bots/settings` | Update bot module settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/bots/{id}` | Read bot connection and queue state | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/bots/{id}` | Delete bot and its delivery records | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/bots/{id}/enabled` | Change bot enabled state | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/bots/{id}/test` | Queue a test message | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/bots/{id}/tokens` | List token metadata | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/bots/{id}/tokens` | Issue or rotate framework token | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/bots/{id}/tokens/{tid}` | Revoke bot token | `bearerAuth` | path:`id`*; path:`tid`*; header:`X-Lang` |
| POST | `/api/bot/v1/bind` | Consume user bind code from a framework plugin | `OpsBotAuthorization` | header:`X-Lang`; body:application/json |
| POST | `/api/bot/v1/command` | Execute a recognized chat command | `OpsBotAuthorization` | header:`X-Lang`; body:application/json |
| GET | `/api/bot/v1/commands` | List framework command completion metadata | `OpsBotAuthorization` | header:`X-Lang` |
| GET | `/api/bot/v1/events` | Poll or stream pending framework messages | `OpsBotAuthorization` 或 `OpsBotEventToken` | query:`token`; header:`Upgrade`; header:`Connection`; header:`X-Lang` |
| POST | `/api/bot/v1/events/ack` | Acknowledge delivered framework messages | `OpsBotAuthorization` | header:`X-Lang`; body:application/json |
| GET | `/api/bot/v1/lookup` | Look up minimal bound-account information | `OpsBotAuthorization` | query:`platform`; query:`platformUserId`; header:`X-Lang` |
| POST | `/api/bot/v1/unbind` | Remove platform-account binding | `OpsBotAuthorization` | header:`X-Lang`; body:application/json |
| GET | `/api/bots/in/{key}` | Answer native callback URL challenge | 公开；仍受站点、限流及业务条件约束 | path:`key`*; query:`timestamp`; query:`nonce`; query:`signature`; query:`msg_signature`; query:`echostr`; query:`access_token`; header:`Authorization`; header:`X-Signature`; header:`X-Telegram-Bot-Api-Secret-Token`; header:`X-Signature-Ed25519`; header:`X-Signature-Timestamp`; header:`X-Lark-Request-Timestamp`; header:`X-Lark-Request-Nonce`; header:`X-Lark-Signature`; header:`timestamp`; header:`sign`; header:`X-Nowork-Timestamp`; header:`X-Nowork-Signature`; header:`X-Lang` |
| POST | `/api/bots/in/{key}` | Receive authenticated native bot callback | 公开；仍受站点、限流及业务条件约束 | path:`key`*; query:`timestamp`; query:`nonce`; query:`signature`; query:`msg_signature`; query:`echostr`; query:`access_token`; header:`Authorization`; header:`X-Signature`; header:`X-Telegram-Bot-Api-Secret-Token`; header:`X-Signature-Ed25519`; header:`X-Signature-Timestamp`; header:`X-Lark-Request-Timestamp`; header:`X-Lark-Request-Nonce`; header:`X-Lark-Signature`; header:`timestamp`; header:`sign`; header:`X-Nowork-Timestamp`; header:`X-Nowork-Signature`; header:`X-Lang`; body:application/json; body:application/octet-stream; body:application/xml; body:text/xml |
| GET | `/api/bots/ws/{key}` | Accept native reverse WebSocket | 公开；仍受站点、限流及业务条件约束 | path:`key`*; header:`Upgrade`*; header:`Connection`*; header:`Authorization`; query:`access_token`; header:`X-Lang` |

<a id="group-039"></a>

### modules/branding

Tenant-scoped domain records and image/font uploads; branding:view / branding:edit.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/branding/domains` | List branding domain records | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/branding/domains` | Register branding domain | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/branding/domains/{id}` | Replace branding domain options | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/branding/domains/{id}` | Delete tenant branding domain record | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/branding/domains/{id}/verify` | Verify branding domain TXT | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/branding/upload` | Upload branding image | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| POST | `/api/admin/branding/upload-font` | Upload WOFF2 branding font | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |

<a id="group-040"></a>

### modules/chat

Site-scoped chat sessions, channels, messages, media, moderation, bots and game relay. HTTP business errors use the envelope; WebSocket and file responses have separate wire contracts.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/chat/audits` | Page moderation and command audit events | `bearerAuth` | query:`page`; query:`limit`; query:`actorId`; query:`targetId`; query:`channelId`; query:`action`; header:`X-Lang` |
| GET | `/api/admin/chat/bans` | Page bans and mutes | `bearerAuth` | query:`page`; query:`limit`; query:`active`; query:`userId`; header:`X-Lang` |
| POST | `/api/admin/chat/bans` | Apply a site or channel moderation action | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/bans/{id}` | Lift an existing mute or ban | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/bots` | List all chat bots | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/chat/bots` | Create a chat bot and reveal its token once | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/chat/bots/{id}` | Replace a chat bot's editable configuration | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/bots/{id}` | Delete a chat bot | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/chat/bots/{id}/token` | Rotate a chat bot token | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/bridges` | List platform group/channel bridges | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/chat/bridges` | Create a chat-to-platform bridge | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/bridges/{id}` | Delete a platform bridge | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/channels` | Page administered channels | `bearerAuth` | query:`page`; query:`limit`; query:`kind`; query:`archived`; query:`keyword`; header:`X-Lang` |
| POST | `/api/admin/chat/channels` | Create a channel | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/chat/channels/{id}` | Replace editable channel fields and unarchive | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/channels/{id}` | Archive a channel | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/channels/{id}/members` | Page channel members with account names | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`role`; header:`X-Lang` |
| PUT | `/api/admin/chat/channels/{id}/members/{userId}` | Assign a channel member role | `bearerAuth` | path:`id`*; path:`userId`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/chat/logins` | Page chat login audit trail | `bearerAuth` | query:`page`; query:`limit`; query:`serverId`; query:`userId`; query:`event`; query:`result`; query:`player`; header:`X-Lang` |
| GET | `/api/admin/chat/queue` | Page messages awaiting review | `bearerAuth` | query:`page`; query:`limit`; header:`X-Lang` |
| POST | `/api/admin/chat/queue/{id}` | Approve or reject a held message | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/chat/relay` | Page the game message and command log | `bearerAuth` | query:`page`; query:`limit`; query:`serverId`; query:`userId`; query:`channelId`; query:`direction`; query:`kind`; query:`status`; query:`player`; header:`X-Lang` |
| POST | `/api/admin/chat/relay/{id}/retry` | Requeue a failed outbound relay item | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/reports` | Page message reports | `bearerAuth` | query:`page`; query:`limit`; query:`status`; query:`channelId`; header:`X-Lang` |
| POST | `/api/admin/chat/reports/{id}` | Resolve or dismiss reports for a message | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/chat/sessions` | Page active chat sessions | `bearerAuth` | query:`page`; query:`limit`; query:`userId`; header:`X-Lang` |
| DELETE | `/api/admin/chat/sessions/{id}` | Revoke an active chat session | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/settings` | Read the finite chat settings map | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/chat/settings` | Update recognized chat settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/chat/stats` | Read current site chat statistics | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/chat/sticker-packs` | List all sticker packs with their images | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/chat/sticker-packs` | Create a sticker or emoji pack | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/chat/sticker-packs/{id}` | Replace sticker-pack metadata | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/sticker-packs/{id}` | Delete a pack, all stickers and their files | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/chat/sticker-packs/{id}/stickers` | Upload a sticker into a pack | `bearerAuth` | path:`id`*; header:`X-Lang`; body:multipart/form-data |
| PUT | `/api/admin/chat/stickers/{id}` | Change a sticker code and sort position | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/stickers/{id}` | Delete a sticker and its stored file | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/chat/words` | Page word-filter rules | `bearerAuth` | query:`page`; query:`limit`; query:`keyword`; query:`action`; header:`X-Lang` |
| POST | `/api/admin/chat/words` | Create a word-filter rule | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/chat/words/test` | Preview word-filter normalization and result | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/chat/words/{id}` | Replace a word-filter rule | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/chat/words/{id}` | Delete a word-filter rule | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/chat/blocks` | List account IDs blocked by the caller | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| POST | `/api/chat/blocks` | Block another account | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/chat/blocks/{userId}` | Remove an account from the block list | `chatSession` 或 `bearerAuth` | path:`userId`*; header:`X-Lang` |
| PUT | `/api/chat/bot/commands` | Replace the bot slash-command registrations | `chatBot` | header:`X-Lang`; body:application/json |
| GET | `/api/chat/bot/me` | Read the authenticated bot identity and grants | `chatBot` | header:`X-Lang` |
| POST | `/api/chat/bot/messages` | Post a bot text message or card | `chatBot` | header:`X-Lang`; body:application/json |
| GET | `/api/chat/channels` | List channels visible to this viewer | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| GET | `/api/chat/channels/{id}/messages` | Read a channel history window | `chatSession` 或 `bearerAuth` | path:`id`*; query:`before`; query:`after`; query:`limit`; header:`X-Lang` |
| POST | `/api/chat/channels/{id}/messages` | Send text, image or sticker to a channel | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/chat/channels/{id}/notify` | Set this channel's notification preference | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/chat/channels/{id}/online` | List currently subscribed user IDs | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/chat/channels/{id}/pins` | List visible pinned messages | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/chat/channels/{id}/read` | Advance the caller's read marker | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/chat/commands` | List available slash commands for a channel | `chatSession` 或 `bearerAuth` | query:`channelId`*; header:`X-Lang` |
| POST | `/api/chat/commands` | Run a slash command | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/chat/dm` | Find or create a private conversation | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/chat/files/{token}` | Read an image using its signed URL | 公开；仍受站点、限流及业务条件约束 | path:`token`*; query:`e`*; query:`s`*; header:`X-Lang` |
| POST | `/api/chat/groups` | Create a private chat group | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/chat/groups/{id}/members` | Invite accounts into a private group | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/chat/groups/{id}/members/me` | Leave a private chat group | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/chat/interactions` | Invoke a bot card button | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/chat/me` | Read chat identity, capabilities and visible channels | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| PUT | `/api/chat/messages/{id}` | Edit an owned visible text message | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/chat/messages/{id}` | Recall a message | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/chat/messages/{id}/pin` | Pin or unpin a visible message | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/chat/messages/{id}/reactions` | Add or remove the caller's message reaction | `chatSession` 或 `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/chat/moderation` | Mute, ban or kick a channel member | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/chat/prefs` | Read chat preferences | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| PUT | `/api/chat/prefs` | Replace chat preferences | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/chat/reports` | Report a visible message | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/chat/session` | Redeem a game ticket for a chat session | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| DELETE | `/api/chat/session` | End the caller's chat session | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| POST | `/api/chat/session/site` | Open a chat session from the current site login | `bearerAuth` | header:`X-Lang` |
| GET | `/api/chat/stickers` | List enabled sticker packs and images | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| GET | `/api/chat/stickers/{id}/file` | Read public immutable sticker bytes | 公开；仍受站点、限流及业务条件约束 | path:`id`*; header:`X-Lang` |
| GET | `/api/chat/unread` | Read unread counts by channel ID | `chatSession` 或 `bearerAuth` | header:`X-Lang` |
| POST | `/api/chat/uploads` | Upload a private chat image | `chatSession` 或 `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/chat/ws` | Upgrade to the authenticated chat WebSocket protocol | 公开；仍受站点、限流及业务条件约束 | header:`Upgrade`*; header:`Connection`*; header:`Sec-WebSocket-Key`*; header:`Sec-WebSocket-Version`*; header:`X-Lang` |
| GET | `/chat/embed` | Serve the embeddable chat application shell | 公开；仍受站点、限流及业务条件约束 | query:`ticket`; query:`server`; header:`X-Lang` |
| POST | `/server/v1/chat/messages` | Relay an in-game line into the server channel | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |
| GET | `/server/v1/chat/relay` | Pull due outbound game messages | `serverKey` | query:`key`; header:`X-Lang` |
| POST | `/server/v1/chat/relay/ack` | Acknowledge delivered or failed relay items | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |
| POST | `/server/v1/chat/tickets` | Issue a single-use game chat ticket | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |

<a id="group-041"></a>

### modules/custom-fields

Tenant-local revisioned form schemas, typed per-account values, publication consent and encrypted private file uploads/downloads.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/apps/{clientId}/custom-fields` | Read host-record form and visible field values | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/apps/{clientId}/custom-fields` | Save host-record custom fields transactionally | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/apps/{clientId}/custom-fields/export` | Export visible host-record fields | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/apps/{clientId}/custom-fields/files/{file}` | Download one authorized host-field attachment | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; path:`file`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/apps/{clientId}/custom-fields/files/{key}` | Stage a private file for an authorized host field | `bearerAuth` 或 `nativeAccessToken` | path:`clientId`*; path:`key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/custom-fields/files/{file}` | Download authorized private custom-field file | `bearerAuth` | path:`file`*; header:`X-Lang` |
| POST | `/api/account/custom-fields/files/{key}` | Upload private custom-field attachment | `bearerAuth` | path:`key`*; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/custom-fields/profile` | Read custom profile form | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/account/custom-fields/profile` | Merge custom profile values and publication flags | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/custom-fields/profile/complete` | Complete required custom profile fields | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/custom-fields/profile/public` | Preview own published custom profile | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/servers/{id}/custom-fields` | Read host-record form and visible field values | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/custom-fields` | Save host-record custom fields transactionally | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/custom-fields/export` | Export visible host-record fields | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/servers/{id}/custom-fields/files/{file}` | Download one authorized host-field attachment | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`file`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/servers/{id}/custom-fields/files/{key}` | Stage a private file for an authorized host field | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/tickets/{id}/custom-fields` | Read host-record form and visible field values | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/tickets/{id}/custom-fields` | Save host-record custom fields transactionally | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/tickets/{id}/custom-fields/export` | Export visible host-record fields | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/tickets/{id}/custom-fields/files/{file}` | Download one authorized host-field attachment | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`file`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/tickets/{id}/custom-fields/files/{key}` | Stage a private file for an authorized host field | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; path:`key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/custom-fields/schema/{entity}` | Read tenant form schema | `bearerAuth` | path:`entity`*; header:`X-Lang` |
| PUT | `/api/admin/custom-fields/schema/{entity}` | Save revisioned form definition | `bearerAuth` | path:`entity`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/custom-fields/users/{id}` | Read custom profile form | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/custom-fields/users/{id}` | Merge custom profile values and publication flags | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/custom-fields/users/{id}/files/{file}` | Download authorized private custom-field file | `bearerAuth` | path:`id`*; path:`file`*; header:`X-Lang` |
| POST | `/api/admin/custom-fields/users/{id}/files/{key}` | Upload private custom-field attachment | `bearerAuth` | path:`id`*; path:`key`*; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/game/server/{id}/custom-fields` | Read host-record form and visible field values | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/custom-fields` | Save host-record custom fields transactionally | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/custom-fields/export` | Export visible host-record fields | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/custom-fields/files/{file}` | Download one authorized host-field attachment | `bearerAuth` | path:`id`*; path:`file`*; header:`X-Lang` |
| POST | `/api/admin/game/server/{id}/custom-fields/files/{key}` | Stage a private file for an authorized host field | `bearerAuth` | path:`id`*; path:`key`*; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/oauth/client/{id}/custom-fields` | Read host-record form and visible field values | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/oauth/client/{id}/custom-fields` | Save host-record custom fields transactionally | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/oauth/client/{id}/custom-fields/export` | Export visible host-record fields | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/oauth/client/{id}/custom-fields/files/{file}` | Download one authorized host-field attachment | `bearerAuth` | path:`id`*; path:`file`*; header:`X-Lang` |
| POST | `/api/admin/oauth/client/{id}/custom-fields/files/{key}` | Stage a private file for an authorized host field | `bearerAuth` | path:`id`*; path:`key`*; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/tickets/{id}/custom-fields` | Read host-record form and visible field values | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/tickets/{id}/custom-fields` | Save host-record custom fields transactionally | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/tickets/{id}/custom-fields/export` | Export visible host-record fields | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/tickets/{id}/custom-fields/files/{file}` | Download one authorized host-field attachment | `bearerAuth` | path:`id`*; path:`file`*; header:`X-Lang` |
| POST | `/api/admin/tickets/{id}/custom-fields/files/{key}` | Stage a private file for an authorized host field | `bearerAuth` | path:`id`*; path:`key`*; header:`X-Lang`; body:multipart/form-data |

<a id="group-042"></a>

### modules/dbmaint

Database maintenance (`dbmaint`): after every migration the backend verifies that each registered model's table and columns exist and records a run row (`dbmaint:view`). Orphan tables and columns - objects no registered model owns - are reported but never dropped automatically. Dialect-specific maintenance actions run on demand (`dbmaint:maintain`); orphan cleanup is super-admin only and demands the typed target name, a recent successful backup and identity confirmation (`password`, plus a two-factor `code` when enabled).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/admin/dbmaint/actions` | Run one maintenance action | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/dbmaint/orphans` | Report orphan tables and columns | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/dbmaint/orphans/clean` | Drop one orphan table or column | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/dbmaint/runs` | Page the schema-verification run history | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Lang` |
| GET | `/api/admin/dbmaint/status` | Schema verification overview | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/dbmaint/tables` | List every table with its row count | `bearerAuth` | header:`X-Lang` |

<a id="group-043"></a>

### modules/delivery

Delivery profile rules, public encryption keys and per-application JSON entitlements.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/delivery/attributes` | List registered delivery attributes | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/delivery/default-spec` | Read built-in delivery rules | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/delivery/preview` | Preview masked shaped payload | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/delivery/profiles` | List delivery profiles | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/delivery/profiles` | Create delivery profile | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/delivery/profiles/{id}` | Read delivery profile | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/delivery/profiles/{id}` | Replace delivery profile | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/delivery/profiles/{id}` | Delete profile and restore target default delivery | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/delivery/profiles/{id}/entitlements` | Page scoped entitlement rows | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`uid`; header:`X-Lang` |
| PUT | `/api/admin/delivery/profiles/{id}/entitlements` | Upsert user's entitlement as administrator | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/delivery/profiles/{id}/entitlements` | Delete user's entitlement as administrator | `bearerAuth` | path:`id`*; query:`userId`*; query:`key`*; header:`X-Lang` |
| POST | `/api/admin/delivery/profiles/{id}/keys` | Upload recipient public encryption JWK | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/delivery/profiles/{id}/keys/{keyId}` | Delete pending or retired key | `bearerAuth` | path:`id`*; path:`keyId`*; header:`X-Lang` |
| PUT | `/api/admin/delivery/profiles/{id}/keys/{keyId}/activate` | Activate key with rotation overlap | `bearerAuth` | path:`id`*; path:`keyId`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/delivery/profiles/{id}/keys/{keyId}/retire` | Retire encryption key immediately | `bearerAuth` | path:`id`*; path:`keyId`*; header:`X-Lang` |
| POST | `/api/admin/delivery/profiles/{id}/keys/{keyId}/test` | Seal fixed sample using recipient public key | `bearerAuth` | path:`id`*; path:`keyId`*; header:`X-Lang` |
| GET | `/api/admin/delivery/targets` | List bindable clients, servers and webhook endpoints | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/delivery/users` | Find preview users | `bearerAuth` | query:`keywords`; header:`X-Lang` |
| GET | `/oauth2/delivery/entitlements` | Read application-scoped user entitlements | `clientBasic` | query:`uid`; query:`sub`; header:`X-Lang` |
| PUT | `/oauth2/delivery/entitlements` | Upsert application-scoped user entitlement | `clientBasic` | header:`X-Lang`; body:application/json |
| DELETE | `/oauth2/delivery/entitlements` | Delete application-scoped user entitlement | `clientBasic` | query:`uid`; query:`sub`; query:`key`*; header:`X-Lang` |
| GET | `/server/v1/delivery/entitlements` | Read application-scoped user entitlements | `serverKey` | query:`uid`; query:`sub`; header:`X-Lang` |
| PUT | `/server/v1/delivery/entitlements` | Upsert application-scoped user entitlement | `serverKey` | header:`X-Lang`; body:application/json |
| DELETE | `/server/v1/delivery/entitlements` | Delete application-scoped user entitlement | `serverKey` | query:`uid`; query:`sub`; query:`key`*; header:`X-Lang` |

<a id="group-044"></a>

### modules/devaccess

Developer access (`devaccess`): the developer portal (`/api/account/apps`) opens only for users an administrator granted access, usually by approving a ticket of category `developer`; without a grant every portal write and the stats call answer 403. Revoking closes the portal but keeps the user's apps working; owners of apps from before the gate got a grant once at startup (`devaccess:manage`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/devaccess` | The caller's developer access | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/devaccess/applications` | Page developer application tickets | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; query:`keywords`; query:`ticketId`; header:`X-Lang` |
| POST | `/api/admin/devaccess/category` | Add the developer category to the ticket settings | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/devaccess/grants` | Page grants | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; query:`keywords`; header:`X-Lang` |
| POST | `/api/admin/devaccess/grants` | Grant developer access | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/devaccess/grants/{userId}/revoke` | Revoke developer access | `bearerAuth` | path:`userId`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/devaccess/summary` | Grant counters and what the operator may do | `bearerAuth` | header:`X-Lang` |

<a id="group-045"></a>

### modules/devdocs

Admin interface reference (`devdocs`): this very document, served as JSON to the admin devdocs page (`devdocs:view`). Gated like any other admin page because the full contract maps every `/api/admin` route and its permission requirements; the public subset (`openapi-public.yaml`, no admin routes) stays a public static file.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/devdocs/openapi.json` | Download the full OpenAPI 3.1 document (admin console included) | `bearerAuth` | header:`X-Lang` |

<a id="group-046"></a>

### modules/feature-switches

Feature switches (`feature-switches`): one admin screen listing every user-facing switch with its current value, so an operator flips them without hunting through each module's own settings page (`feature-switch:view`). Writes keep using `PUT /api/admin/settings`, so every validator and the super-admin-only guard of the settings API apply unchanged (`feature-switch:manage`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/feature-switches` | List every user-facing feature switch with its current value | `bearerAuth` | header:`X-Lang` |

<a id="group-047"></a>

### modules/forced-actions

Site-scoped mandatory actions backed by actual account evidence, with bounded grace periods and audited recovery.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/forced-actions` | Read mandatory action checklist | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`event`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/forced-actions/email/send` | Send verification for a required email action | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/forced-actions/email/verify` | Verify the required email contact | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/forced-actions/recovery` | Recover an unfulfillable mandatory policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/forced-actions/facts` | List mandatory action requirement facts | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/forced-actions/options` | Read available forced-action choices | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/forced-actions/policies` | List mandatory action policies | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/forced-actions/policies` | Create or version-update mandatory actions | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/forced-actions/policies/{id}` | Delete the matching mandatory policy revision | `bearerAuth` | path:`id`*; query:`version`*; header:`X-Lang` |
| POST | `/api/admin/forced-actions/preview` | Preview mandatory actions against managed account evidence | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-048"></a>

### modules/game-archive

Normalized game snapshots, bounded history, field policies and redacted exports. Access is resolved through the current provider binding and admin user scope.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| DELETE | `/api/account/hypergryph/accounts/{id}/archive` | Clear snapshot and synchronization data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/hypergryph/accounts/{id}/archive/export` | Export one binding's permitted archive data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/hypergryph/accounts/{id}/archive/history` | Read bounded synchronization history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/account/hypergryph/accounts/{id}/archive/policies` | Read a game's field restrictions | `bearerAuth` | path:`id`*; query:`game_id`*; header:`X-Lang` |
| GET | `/api/account/hypergryph/accounts/{id}/archive/policy-audit` | Read policy change history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/account/hypergryph/accounts/{id}/archive/snapshots` | List snapshot metadata or read one latest snapshot | `bearerAuth` | path:`id`*; query:`game_id`; query:`role_id`; header:`X-Lang` |
| DELETE | `/api/account/mihoyo/accounts/{id}/archive` | Clear snapshot and synchronization data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/mihoyo/accounts/{id}/archive/export` | Export one binding's permitted archive data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/mihoyo/accounts/{id}/archive/history` | Read bounded synchronization history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/account/mihoyo/accounts/{id}/archive/policies` | Read a game's field restrictions | `bearerAuth` | path:`id`*; query:`game_id`*; header:`X-Lang` |
| GET | `/api/account/mihoyo/accounts/{id}/archive/policy-audit` | Read policy change history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/account/mihoyo/accounts/{id}/archive/snapshots` | List snapshot metadata or read one latest snapshot | `bearerAuth` | path:`id`*; query:`game_id`; query:`role_id`; header:`X-Lang` |
| DELETE | `/api/admin/hypergryph/accounts/{id}/archive` | Clear snapshot and synchronization data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}/archive/export` | Export one binding's permitted archive data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}/archive/history` | Read bounded synchronization history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}/archive/policies` | Read a game's field restrictions | `bearerAuth` | path:`id`*; query:`game_id`*; header:`X-Lang` |
| PUT | `/api/admin/hypergryph/accounts/{id}/archive/policies` | Restrict a game field with an expected revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/hypergryph/accounts/{id}/archive/policy-audit` | Read policy change history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}/archive/snapshots` | List snapshot metadata or read one latest snapshot | `bearerAuth` | path:`id`*; query:`game_id`; query:`role_id`; header:`X-Lang` |
| DELETE | `/api/admin/mihoyo/accounts/{id}/archive` | Clear snapshot and synchronization data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/mihoyo/accounts/{id}/archive/export` | Export one binding's permitted archive data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/mihoyo/accounts/{id}/archive/history` | Read bounded synchronization history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/mihoyo/accounts/{id}/archive/policies` | Read a game's field restrictions | `bearerAuth` | path:`id`*; query:`game_id`*; header:`X-Lang` |
| PUT | `/api/admin/mihoyo/accounts/{id}/archive/policies` | Restrict a game field with an expected revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/mihoyo/accounts/{id}/archive/policy-audit` | Read policy change history | `bearerAuth` | path:`id`*; query:`before`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/mihoyo/accounts/{id}/archive/snapshots` | List snapshot metadata or read one latest snapshot | `bearerAuth` | path:`id`*; query:`game_id`; query:`role_id`; header:`X-Lang` |

<a id="group-049"></a>

### modules/game-assets

Public game reference providers, provenance and site-scoped cached catalogs. Requires the game feature. No player inventory or credentials.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/game-assets/catalog` | Search a public game catalog | `bearerAuth` | query:`provider`*; query:`locale`; query:`version`; query:`q`; query:`kind`; query:`page`; query:`limit`; header:`If-None-Match`; header:`X-Lang` |
| GET | `/api/account/game-assets/providers` | List public game providers and cache variants | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/game-assets/catalog` | Search a public game catalog | `bearerAuth` | query:`provider`*; query:`locale`; query:`version`; query:`q`; query:`kind`; query:`page`; query:`limit`; header:`If-None-Match`; header:`X-Lang` |
| GET | `/api/admin/game-assets/providers` | List public game providers and cache variants | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/game-assets/providers/{id}/refresh` | Refresh a provider cache variant | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/game-assets/providers/{id}/state` | Set a cache variant's disabled state | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-050"></a>

### modules/geolocation

Location audit (`geolocation`): records the coordinates a signed-in visitor's browser returns, or the fact that the visitor refused the prompt, for administrators to review (`geolocation:view`), with a site-wide collection switch and retention window (`geolocation:manage`). Coordinates are rounded to about one metre, rows belong to the account and are purged by the retention window, and the account side never receives a coordinate back.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/account/geolocation` | Report the signed-in visitor's browser location, or that they refused it | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/geolocation/config` | Read the location-audit collection settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/geolocation/config` | Change the location-audit collection settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/geolocation/page` | Page the location audit records | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`userId`; query:`keywords`; query:`region`; query:`denied`; query:`from`; query:`to`; header:`X-Lang` |

<a id="group-051"></a>

### modules/helpcenter

Tenant-local help articles, locale overrides, feedback and revision history.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/account/help/articles/{slug}/feedback` | Save own helpfulness vote | `bearerAuth` 或 `nativeAccessToken` | path:`slug`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/admin/help/articles` | List accessible help articles | `bearerAuth` | query:`q`; query:`audience`; header:`X-Lang` |
| GET | `/api/admin/help/articles/{slug}` | Read accessible help article | `bearerAuth` | path:`slug`*; header:`X-Lang` |
| POST | `/api/admin/help/articles/{slug}/feedback` | Save own helpfulness vote | `bearerAuth` | path:`slug`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/help/manage` | List editable help documents and feedback totals | `bearerAuth` | query:`q`; query:`audience`; header:`X-Lang` |
| PUT | `/api/admin/help/manage` | Save versioned help article override | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/help/manage/{slug}` | Restore bundled article for current locale | `bearerAuth` | path:`slug`*; query:`version`*; header:`X-Lang` |
| GET | `/api/admin/help/manage/{slug}/history` | Read recent article revision history | `bearerAuth` | path:`slug`*; header:`X-Lang` |
| GET | `/api/help/articles` | List accessible help articles | 可匿名 或 `bearerAuth` | query:`q`; header:`X-Lang` |
| GET | `/api/help/articles/{slug}` | Read accessible help article | 可匿名 或 `bearerAuth` | path:`slug`*; header:`X-Lang` |

<a id="group-052"></a>

### modules/hypergryph

Hypergryph/Skland account binding, role discovery and game snapshots. Sections and fields report actual upstream capabilities and machine statuses; public catalog entries do not imply player ownership. Private data requires the caller's bound account/role. Source policy and current archive restrictions remove secret fields even for owners; sharing cannot broaden upstream visibility. Credentials and raw transport errors are never returned.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/hypergryph` | Read binding switches, help URL and the caller's account | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/hypergryph` | Remove the caller's Skland binding | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/hypergryph/games` | Read game descriptors and the caller's discovered roles | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/hypergryph/games/{gameId}/snapshot` | Read a bound game's policy-filtered snapshot | `bearerAuth` | path:`gameId`*; query:`limit`; query:`roleId`; query:`locale`; query:`cursor`; query:`sections`; header:`X-Lang` |
| PUT | `/api/account/hypergryph/role` | Select one of the caller's discovered roles | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/account/hypergryph/share` | Set the caller's server-sharing consent | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/hypergryph/sms/bind` | Bind with an upstream phone verification code | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/hypergryph/sms/send` | Send a Hypergryph binding SMS | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/hypergryph/sync` | Synchronize the caller's Skland account | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/hypergryph/token/bind` | Bind with an upstream account token | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/hypergryph/accounts` | Page bindings in administrator user scope | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`status`; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}` | Read a scoped binding with policy-filtered roles | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/hypergryph/accounts/{id}` | Remove a scoped Skland binding | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}/games` | Read a scoped binding's discovered game roles | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/hypergryph/accounts/{id}/games/{gameId}/snapshot` | Read a bound game's policy-filtered snapshot | `bearerAuth` | path:`id`*; path:`gameId`*; query:`limit`; query:`roleId`; query:`locale`; query:`cursor`; query:`sections`; header:`X-Lang` |
| POST | `/api/admin/hypergryph/accounts/{id}/sync` | Synchronize a scoped Skland binding | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/hypergryph/games` | List registered game capability descriptors | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/hypergryph/settings` | Read Skland module configuration | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/hypergryph/settings` | Save Skland module configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/hypergryph/games` | List registered game capability descriptors | 可匿名 或 `bearerAuth` | header:`X-Lang` |
| GET | `/api/hypergryph/games/{gameId}/snapshot` | Read public game reference snapshot | 可匿名 或 `bearerAuth` | path:`gameId`*; query:`limit`; query:`roleId`; query:`locale`; query:`cursor`; query:`sections`; header:`X-Lang` |
| GET | `/server/v1/hypergryph/roles` | Read consented Skland role data for an approved server player | `serverKey` | query:`uid`*; header:`X-Lang` |

<a id="group-053"></a>

### modules/identity-access

Site-scoped selected identity binding administration and OAuth/OIDC upstream brokering. Credentials and raw upstream profiles are never returned.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/identity-access/connections` | Page selected users' upstream identity bindings | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`userId`; query:`providers`; query:`keywords`; header:`X-Lang` |
| GET | `/api/admin/identity-access/options` | Read redacted provider and application picker options | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/identity-access/qq-force/grants` | List root-issued QQ force-unlink grants | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/identity-access/qq-force/grants` | Grant or immediately revoke QQ force-unlink authority | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/identity-access/relay/{client}` | Read this registered application's identity provider restriction | `bearerAuth` | path:`client`*; header:`X-Lang` |
| PUT | `/api/admin/identity-access/relay/{client}` | Version-update an application's allowed upstream providers | `bearerAuth` | path:`client`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/identity-access/unbind` | Unlink explicitly selected identities with per-record outcomes | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/identity-access/users/{user}/connections/{connection}` | Read a selected binding's saved platform profile | `bearerAuth` | path:`user`*; path:`connection`*; header:`X-Lang` |
| POST | `/api/admin/identity-access/users/{user}/connections/{connection}/query` | Read the bound platform profile through its configured adapter | `bearerAuth` | path:`user`*; path:`connection`*; header:`X-Lang` |
| GET | `/api/identity/relay-options` | Read login-provider choices for a validated OAuth authorization request | 公开；仍受站点、限流及业务条件约束 | query:`client_id`*; query:`redirect_uri`; query:`response_type`*; query:`scope`; query:`state`; query:`nonce`; query:`code_challenge`; query:`code_challenge_method`; query:`prompt`; query:`max_age`; query:`response_mode`; query:`login_hint`; query:`ui_locales`; query:`idp_hint`; query:`id_token_hint`; query:`reauth`; header:`X-Lang` |

<a id="group-054"></a>

### modules/invites

Invite codes (`invites`): personal codes in the user centre; batch generation, revocation, redemptions and CSV export in the admin panel (`invite:view`, `invite:manage`). With `auth.invite_required` on, registration needs a code.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/invites` | Own personal invite codes and the quota | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/invites` | Generate one personal invitation | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/invites/{id}` | Withdraw an own code nobody has used yet | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/invites` | Page invite codes | `bearerAuth` | query:`page`; query:`limit`; query:`keywords`; query:`status`; query:`source`; header:`X-Lang` |
| POST | `/api/admin/invites` | Generate a batch of invite codes | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/invites/export` | Export the filtered codes as CSV | `bearerAuth` | query:`keywords`; query:`status`; query:`source`; header:`X-Lang` |
| POST | `/api/admin/invites/revoke` | Revoke codes | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/invites/settings` | Save current site's invite requirement and personal quota | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/invites/summary` | Invite settings and counters | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/invites/{id}/redemptions` | Accounts that signed up with a code (latest 500) | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-055"></a>

### modules/legal

Site-owned legal publications, explicit acceptance evidence and optional-collection choices. No legal text is prefilled or accepted on a user's behalf.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/legal` | Read current site's legal and consent state | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`locale`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/legal/accept` | Accept the exact current legal versions | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/legal/history` | List own immutable legal choice history | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`kind`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/legal/telemetry` | Record an explicit optional-collection choice | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Origin`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/legal/versions/{id}` | Read one immutable legal text version | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/legal/acceptances` | Audit legal evidence within managed-user scope | `bearerAuth` | query:`page`; query:`limit`; query:`kind`; query:`userId`; header:`X-Lang` |
| GET | `/api/admin/legal/config` | Read current site's legal configuration | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/legal/config` | Save versioned legal configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/legal/policies/{kind}` | Read legal draft and current publication | `bearerAuth` | path:`kind`*; header:`X-Lang` |
| PUT | `/api/admin/legal/policies/{kind}/draft` | Save the four-language legal draft | `bearerAuth` | path:`kind`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/legal/policies/{kind}/publish` | Publish all legal translations atomically | `bearerAuth` | path:`kind`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/legal/versions` | List immutable site legal versions | `bearerAuth` | query:`page`; query:`limit`; query:`kind`; query:`locale`; header:`X-Lang` |
| GET | `/api/legal` | Read current site's legal and consent state | 可匿名 或 `bearerAuth` | query:`locale`; header:`X-Lang` |
| POST | `/api/legal/browser-consent` | Record an explicit optional-collection choice | 可匿名 或 `bearerAuth` | header:`Origin`; header:`X-Lang`; body:application/json |
| GET | `/api/legal/versions/{id}` | Read one immutable legal text version | 可匿名 或 `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-056"></a>

### modules/login-policy

Site-scoped user/role login policies and fifteen-minute restricted setup authorization. Setup credentials never authorize account/admin API calls.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/login-policy-options` | Read selectable login methods, providers and roles | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/login-policy/{subject}/{id}` | Read explicit and effective login policy | `bearerAuth` | path:`subject`*; path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/login-policy/{subject}/{id}` | Set login policy with second-factor confirmation | `bearerAuth` | path:`subject`*; path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/login-policy/{subject}/{id}` | Remove explicit login policy | `bearerAuth` | path:`subject`*; path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/login-policy/setup` | Read remaining login setup requirements | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang` |
| POST | `/api/login-policy/setup/bind` | Start setup-specific identity-provider binding | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/binding/finish` | Finish verified binding in restricted login setup | `OpsLoginSetup` | header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/callback` | Complete setup-specific provider binding | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/contact` | Bind a missing permitted login contact | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/contact/send` | Send enrollment contact verification code | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/finish` | Consume completed setup and issue login session | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang` |
| POST | `/api/login-policy/setup/passkey/begin` | Begin permitted passkey enrollment | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang` |
| POST | `/api/login-policy/setup/passkey/finish` | Finish permitted passkey enrollment | `OpsLoginSetup` | header:`X-Login-Setup`*; query:`sessionId`*; query:`name`; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/password` | Enroll a missing permitted password | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/totp/begin` | Begin required TOTP enrollment | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang` |
| POST | `/api/login-policy/setup/totp/confirm` | Confirm required TOTP and return recovery codes | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |
| POST | `/api/login-policy/setup/verify` | Verify existing TOTP or recovery code | `OpsLoginSetup` | header:`X-Login-Setup`*; header:`X-Lang`; body:application/json |

<a id="group-057"></a>

### modules/media-library

Public media upload inventory and uploader attribution. Private identity documents and attachments are never indexed.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/media` | List current-site public media | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keyword`; query:`userId`; query:`source`; query:`kind`; header:`X-Lang` |
| POST | `/api/admin/media/upload` | Upload a public media file and return its stable URL | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| DELETE | `/api/admin/media/{id}` | Delete one scoped public media asset | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-058"></a>

### modules/messages

Official messages (`messages`): a private message to one user or a system notice to an audience (all, trust level, role), sanitised Markdown, normal or important, optional link, recall and read counts (`message:view`, `message:send`). Users read them in the notification centre as official notices signed with the site name, never the administrator's; sub-administrators only message users inside their data scope.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/official-messages/pinned` | Unread important official notices (newest first, at most 5) | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/official-messages/{id}` | One official notice of the caller | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/messages` | Page sent messages | `bearerAuth` | query:`page`; query:`limit`; query:`kind`; query:`status`; query:`keywords`; header:`X-Lang` |
| GET | `/api/admin/messages/audience-count` | How many users an audience reaches | `bearerAuth` | query:`type`*; query:`minLevel`; query:`maxLevel`; query:`roleId`; header:`X-Lang` |
| POST | `/api/admin/messages/broadcast` | Send a system notice to an audience | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/messages/direct` | Send a private message to one user | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/messages/recipients` | Find a recipient (up to 10) | `bearerAuth` | query:`keywords`; header:`X-Lang` |
| GET | `/api/admin/messages/roles` | Roles a system notice can target | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/messages/settings` | Message limits | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/messages/settings` | Save message limits | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/messages/{id}` | One sent message with read counts | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/messages/{id}/recall` | Recall a message | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-059"></a>

### modules/mihoyo

Miyoushe/HoYoLAB account binding and source-backed game records. QR binding supports the CN region; configured cookie binding supports cn and os. Sections and fields report actual upstream capabilities and machine statuses; public catalog entries do not imply player ownership. Private data requires the caller's bound account/role. Source policy and current archive restrictions remove secret fields even for owners; sharing cannot broaden upstream visibility. Credentials and raw transport errors are never returned.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| DELETE | `/api/account/mihoyo` | Unbind the caller's Miyoushe account | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/mihoyo/cookie` | Bind using an accepted CN or overseas account cookie | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/mihoyo/games` | List game descriptors and supported capabilities | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/mihoyo/games/{gameID}/data` | Read normalized game data for an owned role or public section | `bearerAuth` | path:`gameID`*; query:`roleId`; query:`limit`; query:`locale`; query:`cursor`; query:`sections`; header:`X-Lang` |
| GET | `/api/account/mihoyo/qr/poll` | Poll and complete an owned QR binding | `bearerAuth` | query:`ticket`; header:`X-Lang` |
| POST | `/api/account/mihoyo/qr/start` | Create a CN app QR binding ticket | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/mihoyo/record/{gameBiz}/{uid}` | Read a legacy owned-role record summary | `bearerAuth` | path:`gameBiz`*; path:`uid`*; header:`X-Lang` |
| GET | `/api/account/mihoyo/record/{gameBiz}/{uid}/{kind}` | Read a legacy owned-role section | `bearerAuth` | path:`gameBiz`*; path:`uid`*; path:`kind`*; header:`X-Lang` |
| GET | `/api/account/mihoyo/status` | Read binding state, roles, supported games and recovery action | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/mihoyo/sync` | Synchronize the caller's bound roles and profile | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/mihoyo/verify` | Read the caller's parked Miyoushe risk challenge | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/mihoyo/verify` | Answer a parked Miyoushe risk challenge | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/mihoyo/accounts` | Page bound accounts in administrator user scope | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`status`; header:`X-Lang` |
| DELETE | `/api/admin/mihoyo/accounts/{id}` | Remove a scoped Miyoushe binding and its stored data | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/mihoyo/accounts/{id}/sync` | Synchronize a scoped Miyoushe binding | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/mihoyo/catalog` | List the evidence-backed game catalog | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/mihoyo/games` | List game descriptors and supported capabilities | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/mihoyo/settings` | Read Miyoushe settings with masked salt overrides | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/mihoyo/settings` | Save validated Miyoushe settings | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-060"></a>

### modules/monitor

Probe ingestion, monitoring configuration, public status and private environment inventory.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/monitor/environment` | Inspect authorized backend environment | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/monitor/alerts` | List monitoring alerts | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/monitor/alerts` | Save monitoring alerts | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/monitor/alerts/{id}` | Save monitoring alerts | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/monitor/alerts/{id}` | Delete monitoring alerts | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/checks` | List monitoring checks | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/monitor/checks` | Save monitoring checks | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/monitor/checks/{id}` | Save monitoring checks | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/monitor/checks/{id}` | Delete monitoring checks | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/checks/{id}/results` | List recent service check results | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/checks/{id}/task` | Read current probe-check scheduling state | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/environment` | Inspect authorized backend environment | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/monitor/events` | List recent monitor alert events | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/monitor/events/{id}/retry` | Retry a recoverable monitor alert notification | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/incidents` | List monitoring incidents | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/monitor/incidents` | Save monitoring incidents | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/monitor/incidents/{id}` | Save monitoring incidents | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/monitor/incidents/{id}` | Delete monitoring incidents | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/nodes` | List tenant monitoring nodes | `bearerAuth` | query:`keyword`; query:`group`; header:`X-Lang` |
| POST | `/api/admin/monitor/nodes` | Create monitoring node and initial token | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/monitor/nodes/{id}` | Read tenant monitoring node | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/monitor/nodes/{id}` | Replace node display settings | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/monitor/nodes/{id}` | Delete node and stored samples | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/nodes/{id}/inventory` | Read private node inventory and latest metric sample | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/monitor/nodes/{id}/public` | Set public node visibility | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/monitor/nodes/{id}/samples` | Read bounded metric history | `bearerAuth` | path:`id`*; query:`rollup`; header:`X-Lang` |
| POST | `/api/admin/monitor/nodes/{id}/token` | Rotate probe node token | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/monitor/settings` | Read global monitor settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/monitor/settings` | Update allowlisted global monitor settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/monitor/ping` | Read backend timestamp | 可匿名 或 `bearerAuth` | header:`X-Lang` |
| POST | `/api/probe/v1/report` | Ingest node metrics with per-node token | `probeToken` 或 `probeHeader` | header:`X-Lang`; body:application/json |
| POST | `/api/probe/v1/tasks/claim` | Claim one finite probe check task | `probeToken` 或 `probeHeader` | header:`X-Lang`; body:application/json |
| POST | `/api/probe/v1/tasks/{id}/result` | Commit one leased probe check result | `probeToken` 或 `probeHeader` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/status` | Read public server and service status | 可匿名 或 `bearerAuth` | header:`X-Lang` |

<a id="group-061"></a>

### modules/netdiag

Finite browser transfer measurements and administrator network diagnostics; explicit availability, quotas and SSRF boundaries.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/netdiag/config` | Read site diagnostics settings and runtime availability | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/netdiag/config` | Save site diagnostics policy and permitted private ranges | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/netdiag/dns` | Resolve bounded diagnostic DNS records | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/netdiag/http` | Measure guarded HTTP HEAD redirects and TLS | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/netdiag/icmp` | Run finite ICMP probes | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/netdiag/tcp` | Run finite TCP probes | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/netdiag/traceroute` | Read explicit traceroute unavailability | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/netdiag/capabilities` | Read browser diagnostics availability and quota | 可匿名 或 `bearerAuth` | header:`X-Lang` |
| GET | `/api/netdiag/down` | Stream a bounded random download for browser measurement | 可匿名 或 `bearerAuth` | query:`size`*; header:`X-Lang` |
| GET | `/api/netdiag/ping` | Measure fixed DB-free ping response | `AppsR15NetdiagPingTicket` | query:`token`*; header:`X-Lang` |
| POST | `/api/netdiag/session` | Reserve ping samples and issue finite measurement ticket | 可匿名 或 `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/netdiag/up` | Discard a bounded browser upload and report byte count | 可匿名 或 `bearerAuth` | query:`size`*; header:`X-Lang`; body:application/octet-stream |

<a id="group-062"></a>

### modules/onboarding

Mandatory first-use profile/TAG setup and ticket-backed server access. Cosmetic labels never authorize access.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/onboarding` | Read own first-use status, TAG choices and profile schema | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/onboarding/complete` | Finalize initial TAG choice | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/onboarding/requests` | Request one locked server through an ordinary ticket | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/onboarding` | Read site TAGs, allowed server choices and visibility policies | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/onboarding/config` | Configure first-use setup and initial TAG limit | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/onboarding/requests` | Read reviewable server access applications | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/onboarding/requests/{id}/review` | Approve, decline or revoke a server-only grant | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/onboarding/servers/{id}` | Explicitly set a server public or TAG-restricted | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/onboarding/tags` | Create server audience TAG | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/onboarding/tags/{id}` | Edit or disable server audience TAG | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-063"></a>

### modules/outboundproxy

Main-site outbound proxy configuration. Root-only by default; separate root-issued per-user view/edit grants. API keys, native/restricted credentials and subsite visits are refused.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/outbound-proxy` | Read redacted outbound proxy configuration | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/outbound-proxy` | Update outbound proxy configuration | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/outbound-proxy/diagnostics` | Run detailed saved-proxy connectivity diagnostics | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/outbound-proxy/grants` | List explicit proxy access grants | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/outbound-proxy/grants` | Grant or revoke proxy access | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/outbound-proxy/rules` | Set a module or individual feature route | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/outbound-proxy/servers` | Create or update a named proxy server | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/outbound-proxy/servers/{id}` | Delete an unreferenced proxy server | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/outbound-proxy/test` | Test the saved outbound route | `bearerAuth` | header:`X-Lang` |

<a id="group-064"></a>

### modules/pagebuilder

Versioned block pages with server-side visibility and scheduled publication; existing Markdown pages and default account overview remain the fallback.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/pagebuilder/home` | Read current home widget arrangement | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/pagebuilder/home` | Save a versioned home widget arrangement | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/pagebuilder/home` | Reset the matching home arrangement revision | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`version`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/pagebuilder/assets` | List metadata for public page images | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pagebuilder/assets` | Upload and re-encode a public page image | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| DELETE | `/api/admin/pagebuilder/assets/{id}` | Delete an unreferenced site image | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pagebuilder/facts` | List server-evaluated block visibility facts | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/pagebuilder/home` | Read current home widget arrangement | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/pagebuilder/home` | Save a versioned home widget arrangement | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/pagebuilder/home` | Reset the matching home arrangement revision | `bearerAuth` | query:`version`*; header:`X-Lang` |
| GET | `/api/admin/pagebuilder/pages` | List site block-page draft/publication slots | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/pagebuilder/pages` | Create or version-update a page draft | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/pagebuilder/pages/{id}` | Read page draft and version pointers | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/pagebuilder/pages/{id}` | Delete the exact page version | `bearerAuth` | path:`id`*; query:`version`*; header:`X-Lang` |
| GET | `/api/admin/pagebuilder/pages/{id}/export` | Export a page and its retained revisions | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/pagebuilder/pages/{id}/history` | Read page revision history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/pagebuilder/pages/{id}/publish` | Publish, schedule or roll back a page revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pagebuilder/pages/{id}/unpublish` | Withdraw page publication and schedule | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/pagebuilder/preview` | Validate and render a block document without publication | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/pagebuilder/assets/{id}` | Read a site-owned public block image | 可匿名 或 `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/pagebuilder/pages/{slug}` | Read a published visible block page | 可匿名 或 `bearerAuth` | path:`slug`*; header:`X-Lang` |

<a id="group-065"></a>

### modules/payment-risk

Durable payment assessment, verified disputes, merchant settlement holds and authorized saved-segment prices. Decisions never fabricate payment or refund success.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/riskpay/disputes` | List verified provider disputes | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`status`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/riskpay/freezes` | List merchant settlement holds and measured rates | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`active`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/riskpay/reviews` | List accessible payment assessments | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`status`; query:`orderNo`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/riskpay/audit` | Read scoped audit history for one risk record | `bearerAuth` | query:`page`; query:`limit`; query:`entity`*; query:`entityId`*; header:`X-Lang` |
| GET | `/api/admin/riskpay/blacklists` | List masked risk matching entries | `bearerAuth` | query:`page`; query:`limit`; query:`kind`; header:`X-Lang` |
| POST | `/api/admin/riskpay/blacklists` | Create or update an exact risk matching entry | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/riskpay/blacklists/{id}/enabled` | Enable or disable a risk matching entry | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/riskpay/disputes` | List verified provider disputes | `bearerAuth` | query:`page`; query:`limit`; query:`status`; header:`X-Lang` |
| GET | `/api/admin/riskpay/freezes` | List merchant settlement holds and measured rates | `bearerAuth` | query:`page`; query:`limit`; query:`active`; header:`X-Lang` |
| POST | `/api/admin/riskpay/freezes/{id}/release` | Release a matching merchant hold with a reason | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/riskpay/policy` | Read current payment risk and hold policy | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/riskpay/policy` | Save the exact payment risk policy revision | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/riskpay/prices` | List authorized saved-segment product prices | `bearerAuth` | query:`page`; query:`limit`; header:`X-Lang` |
| POST | `/api/admin/riskpay/prices` | Create or version-update a segment price | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/riskpay/prices/{id}` | Create or version-update a segment price | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/riskpay/reviews` | List accessible payment assessments | `bearerAuth` | query:`page`; query:`limit`; query:`status`; query:`orderNo`; header:`X-Lang` |
| POST | `/api/admin/riskpay/reviews/{id}/decision` | Accept, reject or reopen a payment assessment | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-066"></a>

### modules/performance

Main-site process/SQL/HTTP performance diagnostics. No arbitrary SQL, remote probes or destructive automatic maintenance.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/admin/performance/detect` | Request a lightweight performance sample | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/performance/optimize` | Run the bounded SQLite planner hint | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/performance/report` | Read bounded performance diagnostics | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/performance/settings` | Configure automatic checks and frontend performance mode | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-067"></a>

### modules/platformlogin

Official-platform QR/token adapters as upstream login/linking sources; normal local OIDC issuance remains unchanged.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/oidc/platform/{action}` | Start or advance platform identity verification | 可匿名 或 `bearerAuth` | path:`action`*; header:`X-Lang`; body:application/json |

<a id="group-068"></a>

### modules/pluginmarket

Plugin marketplace publication, publisher keys, review and binary distribution.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/market/plugins` | List owned plugin publications | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/market/plugins/{id}/delist` | Delist or restore plugin | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/market/plugins/{id}/versions/{version}/yank` | Yank or restore published version | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; path:`version`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/market/status` | Read developer marketplace access | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/market/submissions` | List owned package submissions | `bearerAuth` 或 `nativeAccessToken` | query:`pluginId`; query:`page`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/market/submissions` | Submit plugin ZIP for publication review | `bearerAuth` 或 `nativeAccessToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:multipart/form-data |
| POST | `/api/account/market/submissions/{id}/withdraw` | Withdraw pending own submission | `bearerAuth` 或 `nativeAccessToken` | path:`id`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/market/keys` | List public publisher keys and usage | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/market/keys/rotate` | Rotate active publisher signing key | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/market/keys/{keyId}/revoke` | Revoke publisher key and re-sign packages | `bearerAuth` | path:`keyId`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/market/listings` | List all marketplace listings | `bearerAuth` | query:`page`; query:`limit`; query:`status`; query:`keyword`; header:`X-Lang` |
| PUT | `/api/admin/market/listings/{id}` | Set featured flag or category override | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/market/listings/{id}/delist` | Delist or restore plugin | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/market/listings/{id}/versions/{version}/yank` | Yank or restore published version | `bearerAuth` | path:`id`*; path:`version`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/market/settings` | Read marketplace settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/market/settings` | Save marketplace publication settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/market/submissions` | List package review submissions | `bearerAuth` | query:`page`; query:`limit`; query:`status`; query:`keyword`; header:`X-Lang` |
| GET | `/api/admin/market/submissions/{id}` | Read submission manifest and static analysis | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/market/submissions/{id}/approve` | Approve and publish submission | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/market/submissions/{id}/package` | Download uploaded review ZIP | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/market/submissions/{id}/reject` | Reject pending submission | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/market/summary` | Read publication review totals | `bearerAuth` | header:`X-Lang` |
| GET | `/api/market/assets/{id}/{version}/{path}` | Read package icon or screenshot | 可匿名 或 `bearerAuth` | path:`id`*; path:`version`*; path:`path`*; header:`X-Lang` |
| GET | `/api/market/index.json` | Read installer-compatible signed-package index | 可匿名 或 `bearerAuth` | header:`If-None-Match`; header:`X-Lang` |
| GET | `/api/market/packages/{id}/{file}` | Download published plugin ZIP | 可匿名 或 `bearerAuth` | path:`id`*; path:`file`*; header:`If-None-Match`; header:`X-Lang` |
| GET | `/api/market/plugins` | Browse published plugins | 可匿名 或 `bearerAuth` | query:`q`; query:`category`; query:`sort`; query:`featured`; query:`page`; query:`limit`; header:`X-Lang` |
| GET | `/api/market/plugins/{id}` | Read public plugin details and versions | 可匿名 或 `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/market/summary` | Read marketplace categories and totals | 可匿名 或 `bearerAuth` | header:`X-Lang` |

<a id="group-069"></a>

### modules/plugins

Runtime declarative, WebAssembly and external-app plugins. Administrators consent to the manifest's complete capability and permission lists. Packages, settings, server switches, UI bridge calls and external token calls have separate source-enforced boundaries.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| PUT | `/api/account/plugins/fields` | Save the caller's editable plugin fields | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/plugins/home` | Read available plugin pages, links and own visible fields | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/plugins/ui/invoke` | Invoke a plugin UI action | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/plugins/ui/slots` | List enabled plugin UI slots | `bearerAuth` | query:`slot`; query:`plugin`; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/plugins` | Read plugin activation for a scoped server | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/plugins/{plugin}` | Set a server-scoped plugin's activation | `bearerAuth` | path:`id`*; path:`plugin`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/plugins` | List installed plugins and current health | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/plugins/audit` | Page plugin execution and management audit rows | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`plugin`; query:`kind`; query:`status`; header:`X-Lang` |
| GET | `/api/admin/plugins/catalog` | List available capabilities, data permissions and extension points | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/plugins/config` | Read runtime settings and the plugin trust root | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/plugins/config` | Save plugin runtime limits | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/plugins/install` | Install or update a staged plugin with explicit grants | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/plugins/market` | Browse available market plugins and compatible releases | `bearerAuth` | query:`refresh`; header:`X-Lang` |
| POST | `/api/admin/plugins/market/stage` | Stage an exact market version for consent | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/plugins/trust` | Save trusted publisher keys and market policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/plugins/ui/invoke` | Invoke a plugin UI action | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/plugins/ui/slots` | List enabled plugin UI slots | `bearerAuth` | query:`slot`; query:`plugin`; header:`X-Lang` |
| POST | `/api/admin/plugins/upload` | Stage an uploaded plugin ZIP for consent | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/plugins/users/{uid}/fields` | Read a scoped user's plugin-defined fields | `bearerAuth` | path:`uid`*; header:`X-Lang` |
| PUT | `/api/admin/plugins/users/{uid}/fields` | Save a scoped user's plugin-defined fields | `bearerAuth` | path:`uid`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/plugins/{id}` | Read plugin manifest, settings, jobs and package metadata | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/plugins/{id}` | Uninstall a plugin and associated stored state | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/plugins/{id}/enabled` | Set plugin enabled state | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/plugins/{id}/jobs/{job}/run` | Run a declared enabled-plugin job now | `bearerAuth` | path:`id`*; path:`job`*; header:`X-Lang` |
| PUT | `/api/admin/plugins/{id}/pinned` | Set plugin pinned state | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/plugins/{id}/rotate` | Rotate a plugin event secret or external API token | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/plugins/{id}/settings` | Save plugin settings and explicitly supplied secrets | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/plugin-ui/{ticket}/{file}` | Serve an allowed plugin UI package asset | 公开；仍受站点、限流及业务条件约束 | path:`ticket`*; path:`file`*; header:`X-Lang` |
| POST | `/api/plugin/v1/call/{fn}` | Call one granted external-plugin host function | `pluginToken` | path:`fn`*; header:`X-Lang`; body:application/json |
| GET | `/api/plugin/v1/me` | Read the authenticated external plugin's grants and settings | `pluginToken` | header:`X-Lang` |

<a id="group-070"></a>

### modules/points

Site-local point grants, expiration and one-time entitlement redemption against the existing automation points ledger.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/points` | Read own points, ledger and upcoming expiry | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/points/items` | List enabled point redemption items | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/points/redeem` | Redeem points for one versioned entitlement | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Idempotency-Key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/points/redemptions` | List own point-redemption receipts | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/admin/points/grant` | Grant points with a durable idempotency receipt | `bearerAuth` | header:`Idempotency-Key`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/points/items` | List current-site point items | `bearerAuth` | query:`page`; query:`limit`; header:`X-Lang` |
| POST | `/api/admin/points/items` | Create or version-update a point item | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/points/items/{id}` | Create or version-update a point item | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/points/items/{id}` | Delete the exact point-item revision | `bearerAuth` | path:`id`*; query:`version`*; header:`X-Lang` |
| GET | `/api/admin/points/policy` | Read current site's point policy | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/points/policy` | Save a versioned point earning/spending policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/points/users/{id}` | Read a managed user's point account | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; header:`X-Lang` |

<a id="group-071"></a>

### modules/profile

Community profile (`profile`): administrator-defined profile fields (gender, birthday, location, city, signature, introduction, website, social links, cover) with enabled / required / public flags and per-user visibility (`profile:view`, `profile:edit`). After a password or code sign-up the user must complete the required fields first; OAuth consent, SDK login and the maintenance page are never blocked. Administrators edit the fields through the `profile` object of the user editor.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/profile` | Own profile with the schema | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/profile/complete` | Complete required built-in and module profile fields | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/profile/cover` | Upload a cover image | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/profile/public` | What other people see of the caller | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/profile/settings` | Profile field settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/profile/settings` | Save profile field settings | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-072"></a>

### modules/regional-services

Optional-service restrictions use the account's stored region, with tenant/module/consent checks retained.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/regional-services` | Read own effective regional services | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/regional-services` | Read effective regional service policy | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/regional-services` | Set current site's regional service restrictions | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/regional-services` | Reset regional policy to inherited defaults | `bearerAuth` | header:`X-Lang` |

<a id="group-073"></a>

### modules/requirements

Tenant-scoped invite/whitelist eligibility requirements; facts and explanations reflect current registered providers.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/requirements/eligibility` | Explain current account eligibility | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/requirements/dry-run` | Evaluate a requirement for a managed account | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/requirements/facts` | List localized condition facts | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/requirements/policies` | List scoped eligibility policies | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/requirements/policies` | Create or version-update an eligibility policy | `bearerAuth` | header:`X-Lang`; body:application/json |

<a id="group-074"></a>

### modules/review

Review-mode registry (`review`): each reviewable kind registered with it (`realname`, `server_application` (game-server onboarding), `developer_access` and `oauth_client` (self-registered OAuth apps), each while its module is loaded) is reviewed in one mode chosen on the admin page 审核方式 (`review:manage`): `manual` (a human reviews every submission; always available and the default), `auto` (approve on submit), `webhook` (a signed POST to an admin-configured URL, answered synchronously with `approve` / `reject` / `manual` / `pending` or later through `POST /api/review/callback`; errors and timeouts fall back to manual) or `provider` (real-name only: the KYC providers). A mode switch acts only on submissions filed after it: the backlog is never approved or sent automatically. Decisions go through each kind's existing approve / reject functions and apply only while the submission is still pending.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/review/kinds` | Reviewable kinds with their mode, webhook settings and 30-day stats | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/review/kinds/{kind}` | Save the review mode of a kind | `bearerAuth` | path:`kind`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/review/kinds/{kind}/test` | Test the webhook of a kind | `bearerAuth` | path:`kind`*; header:`X-Lang` |
| GET | `/api/admin/review/requests` | Page review requests | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`kind`; query:`status`; header:`X-Lang` |
| POST | `/api/review/callback` | Asynchronous decision of an external review webhook | 公开；仍受站点、限流及业务条件约束 | header:`X-Nowork-Timestamp`*; header:`X-Nowork-Signature`*; header:`X-Lang`; body:application/json |

<a id="group-075"></a>

### modules/roletemplates

Super-administrator permission templates; resolved against the current menu registry.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/system/role-templates` | List builtin and custom permission templates | `bearerAuth` | header:`X-Lang` |
| POST | `/api/system/role-templates` | Create custom permission template | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/system/role-templates/apply` | Create independent role from template | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/system/role-templates/editor` | Read blank custom-template editor | `bearerAuth` | header:`X-Lang` |
| GET | `/api/system/role-templates/{id}` | Preview template and resolved permissions | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/system/role-templates/{id}` | Update custom permission template | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/system/role-templates/{id}` | Delete custom permission template | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-076"></a>

### modules/sdk-login

Per-server OAuth authorization-code login with mandatory PKCE S256. Configuration and callback allowlists are revalidated on every request. Custom templates run through a sandboxed bridge; unavailable templates fall back to the standard page.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/sdk-login/records` | Page game sign-in attempts | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`serverId`; query:`keywords`; query:`result`; query:`from`; query:`to`; header:`X-Lang` |
| GET | `/api/admin/sdk-login/servers` | List servers in the operator's data scope | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/sdk-login/servers/{id}` | Read server login settings and configuration choices | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/sdk-login/servers/{id}` | Save server login configuration | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sdk-login/servers/{id}/templates` | Read both device template slots and retained versions | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/sdk-login/servers/{id}/templates/{device}/mode` | Select standard or published custom login template | `bearerAuth` | path:`id`*; path:`device`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/sdk-login/servers/{id}/templates/{device}/preview` | Validate and preview unsaved template source | `bearerAuth` | path:`id`*; path:`device`*; query:`lang`; query:`theme`; header:`X-Lang`; body:application/json |
| POST | `/api/admin/sdk-login/servers/{id}/templates/{device}/versions` | Save an immutable template version | `bearerAuth` | path:`id`*; path:`device`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sdk-login/servers/{id}/templates/{device}/versions/{vid}` | Read one saved template version | `bearerAuth` | path:`id`*; path:`device`*; path:`vid`*; header:`X-Lang` |
| DELETE | `/api/admin/sdk-login/servers/{id}/templates/{device}/versions/{vid}` | Delete an unpublished template version | `bearerAuth` | path:`id`*; path:`device`*; path:`vid`*; header:`X-Lang` |
| POST | `/api/admin/sdk-login/servers/{id}/templates/{device}/versions/{vid}/publish` | Publish or roll back to a saved version | `bearerAuth` | path:`id`*; path:`device`*; path:`vid`*; header:`X-Lang` |
| GET | `/api/admin/sdk-login/template-starter` | Read the standard template as editable source | `bearerAuth` | header:`X-Lang` |
| GET | `/api/sdk-login/{code}` | Read validated server login configuration and request state | 可匿名 或 `bearerAuth` | path:`code`*; query:`device`; query:`redirect_uri`; query:`state`; query:`code_challenge`; query:`code_challenge_method`; query:`scope`; query:`nonce`; query:`prompt`; header:`X-Lang` |
| POST | `/api/sdk-login/{code}/authorize` | Authorize the current session or return required consent | `bearerAuth` | path:`code`*; header:`X-Lang`; body:application/json |
| GET | `/api/sdk-login/{code}/template` | Serve the published sandboxed custom template | 可匿名 或 `bearerAuth` | path:`code`*; query:`device`; query:`lang`; query:`theme`; query:`site_origin`; header:`X-Lang` |
| GET | `/sdk/login/{serverCode}` | Serve the server login SPA shell | 公开；仍受站点、限流及业务条件约束 | path:`serverCode`*; query:`redirect_uri`; query:`state`; query:`code_challenge`; query:`code_challenge_method`; query:`scope`; query:`nonce`; query:`prompt`; query:`device`; header:`X-Lang` |

<a id="group-077"></a>

### modules/security-center

Account security check-up, enforced rate-limit metadata, confirmed/revision-aware site security policy and anonymous CSP reports.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/security-checkup` | Score own account security controls | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/security/csp-reports` | List recent in-memory CSP violations | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/admin/security/csp-reports` | Clear in-memory CSP violations | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/security/policy` | Read effective site security policy and revision | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/security/policy` | Confirm and save site security policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/security/policy/audit` | Audit enforced security policy controls | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/security/policy/preview` | Preview policy change without saving | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/security/rate-limits` | Read actual enforced rate-limit metadata | `bearerAuth` | header:`X-Lang` |
| POST | `/api/security/csp-report` | Receive anonymous browser CSP violations | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/csp-report; body:application/reports+json |

<a id="group-078"></a>

### modules/server-gm

Per-server operator catalogue, grants, widgets, command delivery; separate external GM character sign-in with one-use request/code tickets and S256.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/gm/launches` | Servers supporting own character login | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/gm/servers` | Owned and granted operator panels | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/gm/servers/{id}/characters` | Approved characters for external GM login | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/gm/servers/{id}/commands` | Latest visible GM commands | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/gm/servers/{id}/execute` | Execute granted GM action or preset | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/account/gm/servers/{id}/favorites` | Replace own GM favorites | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/gm/servers/{id}/launch` | Create external GM login request | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/gm/servers/{id}/panel` | Operator panel and effective actions | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/gm/servers/{id}/status` | Current GM server status | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/gm/sso/authorize` | Exchange login request for redirect code | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/gm/actions` | Effective GM catalogue | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/servers/{id}/gm/commands` | Recent GM command history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/servers/{id}/gm/execute` | Execute GM action or preset | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/gm/grants` | All server GM grants | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/servers/{id}/gm/grants` | Create GM grant | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/account/servers/{id}/gm/grants/{grantId}` | Update GM grant rights | `bearerAuth` | path:`id`*; path:`grantId`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/servers/{id}/gm/grants/{grantId}` | Revoke server GM grant | `bearerAuth` | path:`id`*; path:`grantId`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/gm/layout` | Save server GM layout and presets | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/servers/{id}/gm/layout` | Reset server GM layout | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/servers/{id}/gm/login-config` | External character login configuration | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/gm/login-config` | Configure external character login | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/gm/actions` | Effective GM catalogue | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/gm/commands` | Recent GM command history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/game/server/{id}/gm/execute` | Execute GM action or preset | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/game/server/{id}/gm/favorites` | Replace administrator's favorites | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/gm/grants` | All server GM grants | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/game/server/{id}/gm/grants` | Create GM grant | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/game/server/{id}/gm/grants/{grantId}` | Update GM grant rights | `bearerAuth` | path:`id`*; path:`grantId`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/game/server/{id}/gm/grants/{grantId}` | Revoke server GM grant | `bearerAuth` | path:`id`*; path:`grantId`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/gm/layout` | Save server GM layout and presets | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/game/server/{id}/gm/layout` | Reset server GM layout | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/gm/login-config` | External character login configuration | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/gm/login-config` | Configure external character login | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/gm/panel` | Administrator GM panel | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/gm/status` | GM live status and queue depth | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/server/v1/gm/actions` | Publish plugin action catalogue | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |
| GET | `/server/v1/gm/commands` | Pull pending plugin commands | `serverKey` | query:`key`; header:`X-Lang` |
| POST | `/server/v1/gm/commands/{id}/result` | Report command outcome | `serverKey` | path:`id`*; query:`key`; header:`X-Lang`; body:application/json |
| PUT | `/server/v1/gm/options` | Replace plugin named option lists | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |
| POST | `/server/v1/gm/sso/exchange` | Consume GM code for selected character identity | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |

<a id="group-079"></a>

### modules/server-owner

Self-service onboarding and ownership-scoped console, review, credentials, whitelist policies, stats and owner-toggleable server modules.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/servers` | Own servers and onboarding policy | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/servers` | Apply to onboard a game server | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}` | Own server details | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}` | Update server details or resubmit application | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/account/servers/{id}` | Withdraw never-live application | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/servers/{id}/dashboard` | Own server dashboard statistics | `bearerAuth` | path:`id`*; query:`range`; header:`X-Lang` |
| POST | `/api/account/servers/{id}/key` | Generate or rotate own plugin API key | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/account/servers/{id}/key` | Revoke own plugin key | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/servers/{id}/modules` | Own server module switches | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/modules` | Atomically toggle owner-allowed modules | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/players` | Own server reported online players | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/policy` | Replace own whitelist policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/roles` | Page own server bindings | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; query:`keywords`; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/roles/{roleId}` | Review or change a role binding | `bearerAuth` | path:`id`*; path:`roleId`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/stats` | Own server heartbeat statistics | `bearerAuth` | path:`id`*; query:`range`; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/status` | Set own online/offline/maintenance status | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/onboarding` | Read onboarding settings and effective review mode | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/game/onboarding` | Update onboarding settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server-registrations` | Server registration review queue | `bearerAuth` | query:`status`; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/application` | Review server application and owner | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/owner` | Transfer server ownership or return to site | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/game/server/{id}/review` | Approve, reject, restore or suspend server | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-080"></a>

### modules/server-users

Per-server roster, typed data, scoped metadata, bans, own data and player portal. Bans remain enforced when server-users is disabled.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/portal/servers` | Player server directory and own whitelist state | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/portal/servers/{id}/roles` | Bind a role from server portal | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/portal/servers/{id}/whitelist-apply` | Apply for server whitelist | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/portal/servers/{id}/whitelist-status` | Own whitelist status for one visible server | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/server-data` | Own visible server data and active bans | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/servers/{id}/roster` | Page server player roster | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`tag`; query:`status`; query:`roleStatus`; query:`minLevel`; query:`seenDays`; header:`X-Lang` |
| GET | `/api/account/servers/{id}/roster/bans` | Page ban history | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`active`; query:`keywords`; header:`X-Lang` |
| GET | `/api/account/servers/{id}/roster/export` | Download filtered player roster CSV | `bearerAuth` | path:`id`*; query:`sort`; query:`order`; query:`keywords`; query:`tag`; query:`status`; query:`roleStatus`; query:`minLevel`; query:`seenDays`; header:`X-Lang` |
| GET | `/api/account/servers/{id}/roster/schema` | Player field schema, quotas and usage | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/roster/schema` | Replace player data field schema | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/roster/summary` | Roster counts, quotas and caller capabilities | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/servers/{id}/roster/tags` | Most used roster tags | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/servers/{id}/roster/{uid}` | Player detail and latest 50 bans | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang` |
| POST | `/api/account/servers/{id}/roster/{uid}/ban` | Place or replace server ban | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/servers/{id}/roster/{uid}/data` | Read typed player data | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang` |
| PUT | `/api/account/servers/{id}/roster/{uid}/data` | Replace typed player data | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| PUT | `/api/account/servers/{id}/roster/{uid}/meta` | Update player tags and note | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/servers/{id}/roster/{uid}/unban` | Lift active server ban | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/whitelist/applications` | Own binding and whitelist application history | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/roster` | Page server player roster | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`tag`; query:`status`; query:`roleStatus`; query:`minLevel`; query:`seenDays`; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/roster/bans` | Page ban history | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`sort`; query:`order`; query:`active`; query:`keywords`; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/roster/config` | Set player quotas, encryption and plugin ban switch | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/roster/export` | Download filtered player roster CSV | `bearerAuth` | path:`id`*; query:`sort`; query:`order`; query:`keywords`; query:`tag`; query:`status`; query:`roleStatus`; query:`minLevel`; query:`seenDays`; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/roster/schema` | Player field schema, quotas and usage | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/roster/schema` | Replace player data field schema | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/roster/summary` | Roster counts, quotas and caller capabilities | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/roster/tags` | Most used roster tags | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/game/server/{id}/roster/{uid}` | Player detail and latest 50 bans | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang` |
| POST | `/api/admin/game/server/{id}/roster/{uid}/ban` | Place or replace server ban | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/game/server/{id}/roster/{uid}/data` | Read typed player data | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang` |
| PUT | `/api/admin/game/server/{id}/roster/{uid}/data` | Replace typed player data | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| PUT | `/api/admin/game/server/{id}/roster/{uid}/meta` | Update player tags and note | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/game/server/{id}/roster/{uid}/unban` | Lift active server ban | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| GET | `/server/v1/bans` | Plugin active bans and role identifiers | `serverKey` | query:`key`; query:`uid`; query:`name`; query:`uuid`; header:`X-Lang` |
| POST | `/server/v1/bans` | Plugin place server ban | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |
| GET | `/server/v1/bans/kick` | Online players the plugin should disconnect | `serverKey` | query:`key`; header:`X-Lang` |
| POST | `/server/v1/bans/lift` | Plugin lift server ban | `serverKey` | query:`key`; header:`X-Lang`; body:application/json |
| GET | `/server/v1/users/schema` | Plugin player data schema | `serverKey` | query:`key`; header:`X-Lang` |
| GET | `/server/v1/users/{uid}/data` | Plugin read player data | `serverKey` | query:`key`; path:`uid`*; header:`X-Lang` |
| PUT | `/server/v1/users/{uid}/data` | Plugin replace player data | `serverKey` | query:`key`; path:`uid`*; header:`X-Lang`; body:application/json |
| PATCH | `/server/v1/users/{uid}/data` | Plugin patch player data | `serverKey` | query:`key`; path:`uid`*; header:`X-Lang`; body:application/json |

<a id="group-081"></a>

### modules/sites

Main-site tenant control plane, developer grants, site membership and site-scoped staff settings.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/sites` | List owned or joined sites for launcher | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/sites/grants` | List grants issued to the current developer | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/sites/grants/{id}/open` | Open a site using own grant | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/sites/{id}/join` | Join a shared site from main account | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/site-events` | Page site delegation and lifecycle audit | `bearerAuth` | query:`page`; query:`limit`; query:`siteId`; header:`X-Lang` |
| GET | `/api/admin/site-grants` | Page developer site grants | `bearerAuth` | query:`page`; query:`limit`; header:`X-Lang` |
| POST | `/api/admin/site-grants` | Delegate site creation to a developer | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/site-grants/{id}` | Revoke a developer site grant | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/site-meta` | Site wizard capabilities and allowed plans | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/site-plans` | List site plans | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/site-plans` | Create a plan | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/site-plans/{code}` | Replace plan configuration | `bearerAuth` | path:`code`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/site-plans/{code}` | Delete unused custom plan | `bearerAuth` | path:`code`*; header:`X-Lang` |
| GET | `/api/admin/site-staff-templates` | List usable sub-admin roles for staff | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/sites` | List sites and quota usage | `bearerAuth` | query:`page`; query:`limit`; query:`keyword`; query:`status`; query:`plan`; query:`ownerId`; query:`memberId`; header:`X-Lang` |
| POST | `/api/admin/sites` | Create a site within a permitted plan | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/sites/current/settings` | Read staff-managed current-site settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/sites/current/settings` | Update staff-managed current-site overrides | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/sites/{id}` | Read site with domains | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}` | Update site configuration | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/sites/{id}` | Permanently erase an archived site | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/sites/{id}/admin-entry` | Enter a subsite as a main super administrator | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/sites/{id}/domains` | List site domains and verification records | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/sites/{id}/domains` | Register a custom site hostname | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/sites/{id}/domains/{did}` | Remove site domain | `bearerAuth` | path:`id`*; path:`did`*; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/domains/{did}/primary` | Select verified primary domain | `bearerAuth` | path:`id`*; path:`did`*; header:`X-Lang` |
| POST | `/api/admin/sites/{id}/domains/{did}/verify` | Verify site DNS ownership | `bearerAuth` | path:`id`*; path:`did`*; header:`X-Lang` |
| GET | `/api/admin/sites/{id}/export` | Export site configuration and memberships | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/sites/{id}/identity-export` | Export a cursor page of site-owned identities | `bearerAuth` | path:`id`*; query:`afterUserId`; query:`snapshotMaxUserId`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/sites/{id}/lifecycle` | Read identity deletion/export counts | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/sites/{id}/links` | Read interconnection matrix and editability | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/links` | Save interconnection matrix | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/sites/{id}/links/preview` | Preview interconnection impact | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sites/{id}/members` | List site memberships | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`status`; query:`keyword`; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/members/{uid}/status` | Set membership status | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sites/{id}/settings` | Read site setting overrides | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/settings` | Atomically update site overrides | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/sites/{id}/settings/upload` | Upload a branding image to a specific site from the main console | `bearerAuth` | path:`id`*; header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/sites/{id}/signing-keys` | Read one site's signing-key metadata without creating keys | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/sites/{id}/signing-keys/rotate` | Rotate one site's signing key after fresh operator verification | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sites/{id}/staff` | List site staff assignments | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/sites/{id}/staff-candidates` | Find eligible site staff accounts | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`keyword`; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/staff/{uid}` | Assign a site-scoped staff role | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/sites/{id}/staff/{uid}` | Revoke site staff assignment | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/status` | Set site lifecycle status | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sites/{id}/super-admins` | Root: page main-super access to a subsite | `bearerAuth` | path:`id`*; query:`page`; query:`limit`; query:`keyword`; header:`X-Lang` |
| PUT | `/api/admin/sites/{id}/super-admins/{uid}` | Root: revoke or restore a super administrator's site access | `bearerAuth` | path:`id`*; path:`uid`*; header:`X-Lang`; body:application/json |
| POST | `/api/sites/current/admin-entry/redeem` | Exchange a one-use subsite administration ticket | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| POST | `/api/sites/current/join` | Consent to membership in the current site | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/sites/tls-ask` | Authorize Caddy on-demand TLS issuance | 公开；仍受站点、限流及业务条件约束 | query:`domain`; header:`X-Lang` |

<a id="group-082"></a>

### modules/telemetry

Visit audit with sensitive-field permissions, optional browser collection, cursor timelines and bounded offline/online IP location diagnostics.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/account/telemetry/events` | Acknowledge a bounded batch of optional browser interactions | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`DNT`; header:`Sec-GPC`; cookie:`nw_legal_consent`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/telemetry/leave` | Record session engagement on page leave | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`DNT`; header:`Sec-GPC`; cookie:`nw_legal_consent`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/telemetry/preferences` | Read optional telemetry choice | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| PUT | `/api/account/telemetry/preferences` | Set optional telemetry choice | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| POST | `/api/account/telemetry/report` | Merge browser environment into session visit | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`DNT`; header:`Sec-GPC`; cookie:`nw_legal_consent`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/telemetry/visits` | Page own nonsensitive visit history | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | query:`page`; query:`limit`; query:`sort`; query:`order`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/admin/telemetry/geo` | Read geolocation configuration and provider status | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/telemetry/geo/download` | Download or update offline GeoIP sources | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/telemetry/geo/reload` | Reload offline location databases | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/telemetry/geo/test` | Perform bounded fresh location diagnostic | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/telemetry/related` | Find accounts sharing recent traits | `bearerAuth` | query:`userId`*; header:`X-Lang` |
| GET | `/api/admin/telemetry/retention` | Read per-kind retention and purged-volume counts | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/telemetry/timeline` | Read cursor page of one user's merged history | `bearerAuth` | query:`userId`*; query:`kind`; query:`ip`; query:`device`; query:`from`; query:`to`; query:`cursor`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/telemetry/verify` | Verify retained visit chain | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/telemetry/visits` | Page scope-filtered visit audit records | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`userId`; query:`keywords`; query:`kind`; query:`ip`; query:`visitorId`; query:`fingerprint`; query:`sessionId`; query:`minRisk`; query:`from`; query:`to`; header:`X-Lang` |
| GET | `/api/admin/telemetry/visits/{id}` | Read minimized visit detail and integrity | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/telemetry/probe` | Record missing address family using signed probe URL | 公开；仍受站点、限流及业务条件约束 | query:`t`; header:`X-Lang` |

<a id="group-083"></a>

### modules/textoverrides

Revisioned site/locale wording overrides using the actual frontend/backend catalogs, with validated placeholders and explicit null reset.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/textoverrides/catalog` | Search compiled and effective wording entries | `bearerAuth` | query:`locale`; query:`domain`; query:`q`; query:`key`; query:`overridden`; query:`offset`; query:`limit`; header:`X-Lang` |
| GET | `/api/admin/textoverrides/export` | Export local wording overrides | `bearerAuth` | query:`locale`; header:`X-Lang` |
| GET | `/api/admin/textoverrides/history` | List recent wording revision metadata | `bearerAuth` | query:`locale`; header:`X-Lang` |
| POST | `/api/admin/textoverrides/import` | Import validated locale overrides atomically | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/textoverrides/import/preview` | Preview a wording import with current revisions | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/textoverrides/preview` | Preview validated wording changes | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/textoverrides/values` | Apply exact wording changes atomically | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/textoverrides/{locale}` | Read cacheable public frontend wording overrides | 可匿名 或 `bearerAuth` | path:`locale`*; header:`If-None-Match`; header:`X-Lang` |

<a id="group-084"></a>

### modules/theme-packages

Site-owned theme configuration bundles and immutable revisions. Optional CSS follows the documented restricted grammar; no assets or arbitrary executable content are imported.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/theme-packages` | List theme drafts and selection version | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/theme-packages` | Save a new immutable theme draft revision | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/admin/theme-packages/preview` | Validate and canonicalize a portable theme bundle | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/theme-packages/selection` | Select or reset current site's default theme | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/theme-packages/{id}` | Read a theme draft and immutable revision metadata | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/theme-packages/{id}` | Save a new immutable theme draft revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/theme-packages/{id}` | Delete a theme and reset any active selection | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/theme-packages/{id}/export` | Export a portable theme revision | `bearerAuth` | path:`id`*; query:`revision`; header:`X-Lang` |
| POST | `/api/admin/theme-packages/{id}/publish` | Publish or roll back to a saved theme revision | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/theme-packages/{id}/unpublish` | Withdraw theme publication | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/theme-packages` | List published theme bundles for this site | 可匿名 或 `bearerAuth` | header:`X-Lang` |

<a id="group-085"></a>

### modules/tickets

Support tickets (`tickets`): users open, reply to, close and reopen tickets with attachments; staff filter, assign, reply, add internal notes and resolve (`ticket:view`, `ticket:reply`, `ticket:manage`) within their user data scope.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/tickets` | Own tickets | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; header:`X-Lang` |
| POST | `/api/account/tickets` | Open a ticket | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/tickets/attachments` | Upload an attachment | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/account/tickets/attachments/{id}` | Download an attachment of an own ticket | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/account/tickets/attachments/{id}` | Delete an own pending upload | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/tickets/{id}` | One own ticket with its thread and timeline | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/tickets/{id}/close` | Close an own ticket | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/tickets/{id}/reopen` | Reopen an own ticket | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/tickets/{id}/reply` | Reply to an own ticket | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/tickets` | Page tickets | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; query:`priority`; query:`category`; query:`assignee`; query:`keyword`; query:`from`; query:`to`; header:`X-Lang` |
| GET | `/api/admin/tickets/assignees` | Staff members that can be assigned | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/tickets/attachments` | Upload a staff attachment | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/tickets/attachments/{id}` | Download a ticket attachment | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/admin/tickets/attachments/{id}` | Delete an own pending upload | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/tickets/settings` | Ticket settings | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/tickets/settings` | Save ticket settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/tickets/stats` | Counters of the ticket centre | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/tickets/{id}` | One ticket with its thread and timeline | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/admin/tickets/{id}` | Change status, priority or assignee | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/admin/tickets/{id}/reply` | Reply, add an internal note, or reply and resolve | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-086"></a>

### modules/user-audit

User audit (`user-audit`): the per-account dossier page for administrators (`useraudit:view`) inside their user data scope; this module serves the grouped sign-in address history, the other sections read the existing audit endpoints under their own permissions.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/user-audit/users/{id}/ips` | Sign-in addresses of one account (at most 200, last used first) | `bearerAuth` | path:`id`*; header:`X-Lang` |

<a id="group-087"></a>

### modules/user-bulk

Bulk user import (CSV dry run, then a background job) and CSV export (`user:import`, `user:export`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/user-bulk/export` | Export users as CSV | `bearerAuth` | query:`keywords`; query:`username`; query:`nickname`; query:`email`; query:`phone`; query:`uid`; query:`status`; query:`securityLevel`; query:`realNameStatus`; query:`roleId`; query:`createTimeStart`; query:`createTimeEnd`; query:`fields`; header:`X-Lang` |
| GET | `/api/admin/user-bulk/export/meta` | Export fields and role options | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/user-bulk/import` | Import a CSV in a background job | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |
| GET | `/api/admin/user-bulk/import/jobs` | Recent import jobs (latest 20) | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/user-bulk/import/jobs/{id}` | One import job with its per-row results | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/user-bulk/import/meta` | Import columns, grantable roles and limits | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/user-bulk/import/preview` | Dry run: validate a CSV without writing | `bearerAuth` | header:`X-Lang`; body:multipart/form-data |

<a id="group-088"></a>

### notification-alerts

Read-only login-alert review and explicit one-use recognize/revoke decisions.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/api/notification-alerts/confirm` | Recognize or revoke exactly the login named by a proof | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| GET | `/api/notification-alerts/review` | Review a login alert without consuming its proof | `NotifyR15AlertProof` 或 `bearerAuth` | query:`alert`; header:`X-Lang` |

<a id="group-089"></a>

### oidc

OpenID Provider endpoints at the issuer root plus the session-authenticated consent API.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/.well-known/jwks.json` | Read stored public signing keys for this issuer | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/.well-known/openid-configuration` | Discover current-site OpenID Provider capabilities | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/api/oauth/consent` | Read consent requirements and released claim names | `bearerAuth` 或 `ssoBrowser` | query:`client_id`*; query:`redirect_uri`; query:`response_type`*; query:`scope`; query:`state`; query:`nonce`; query:`code_challenge`; query:`code_challenge_method`; query:`prompt`; query:`max_age`; query:`response_mode`; query:`login_hint`; query:`ui_locales`; query:`idp_hint`; query:`id_token_hint`; query:`reauth`; header:`X-Lang` |
| POST | `/api/oauth/consent` | Approve or refuse the current OAuth authorization | `bearerAuth` 或 `ssoBrowser` | header:`X-Lang`; body:application/json |
| GET | `/jwks.json` | Read stored public signing keys for this issuer | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/oauth2/authorize` | Begin or silently complete an authorization-code flow | 公开；仍受站点、限流及业务条件约束 | query:`client_id`*; query:`redirect_uri`; query:`response_type`*; query:`scope`; query:`state`; query:`nonce`; query:`code_challenge`; query:`code_challenge_method`; query:`prompt`; query:`max_age`; query:`response_mode`; query:`login_hint`; query:`ui_locales`; query:`idp_hint`; query:`id_token_hint`; query:`reauth`; header:`X-Lang` |
| POST | `/oauth2/introspect` | Introspect an owned OAuth token | `clientBasic` 或 可匿名 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| POST | `/oauth2/revoke` | Revoke an owned OAuth token | `clientBasic` 或 可匿名 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| POST | `/oauth2/token` | Issue, exchange or rotate OAuth tokens | `clientBasic` 或 可匿名 | header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/oauth2/userinfo` | Return claims for the OAuth token's user | `oidcAccessToken` | header:`X-Lang` |
| POST | `/oauth2/userinfo` | Return claims for the OAuth token's user | `oidcAccessToken` 或 可匿名 | header:`X-Lang`; body:application/x-www-form-urlencoded |

<a id="group-090"></a>

### oidc/registration

Optional dynamic client registration and registration-token management; standard OAuth JSON/statuses.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/oauth2/register` | Dynamically register an OAuth client | 可匿名 或 `AuthR15DCRInitial` | header:`X-Lang`; body:application/json |
| GET | `/oauth2/register/{clientID}` | Read dynamic registration | `AuthR15DCRManagement` | path:`clientID`*; header:`X-Lang` |
| PUT | `/oauth2/register/{clientID}` | Replace dynamic registration metadata | `AuthR15DCRManagement` | path:`clientID`*; header:`X-Lang`; body:application/json |
| DELETE | `/oauth2/register/{clientID}` | Delete a dynamic registration | `AuthR15DCRManagement` | path:`clientID`*; header:`X-Lang` |

<a id="group-091"></a>

### payments

Payment checkout, wallet, receipts and merchant integrations

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/pay/config` | Enabled currencies and wallet purchase limits | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/pay/merchant/apps` | List scoped merchant applications and gateway URLs | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/pay/merchant/apps` | Create merchant application and reveal initial key | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/account/pay/merchant/apps/{id}` | Update owned merchant application and resubmit for review | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| POST | `/api/account/pay/merchant/apps/{id}/key` | Rotate owned merchant application key | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/account/pay/merchant/apps/{id}/protocols` | Read owned relay protocol credentials and public trust material | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/account/pay/merchant/apps/{id}/protocols` | Save owned relay protocol credentials | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/pay/merchant/apps/{id}/protocols/refunds` | List scoped protocol refund callback records | `bearerAuth` | path:`id`*; query:`before`; header:`X-Lang` |
| POST | `/api/account/pay/merchant/apps/{id}/protocols/refunds/{refundID}/retry` | Reschedule failed refund callback | `bearerAuth` | path:`id`*; path:`refundID`*; header:`X-Lang` |
| GET | `/api/account/pay/merchant/orders` | List merchant relay orders | `bearerAuth` | query:`appId`; query:`page`; header:`X-Lang` |
| GET | `/api/account/pay/merchant/withdrawals` | List merchant withdrawals | `bearerAuth` | query:`appId`; query:`page`; header:`X-Lang` |
| POST | `/api/account/pay/merchant/withdrawals` | Reserve wallet funds for merchant withdrawal | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/account/pay/orders` | Current payer's orders | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`status`; header:`X-Lang` |
| POST | `/api/account/pay/orders` | Create a product or wallet top-up order | `bearerAuth` | header:`Idempotency-Key`; header:`X-Lang`; body:application/json |
| GET | `/api/account/pay/orders/{orderNo}` | Order detail and payment status | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/account/pay/orders/{orderNo}/cancel` | Cancel an unpaid owned order | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/account/pay/orders/{orderNo}/pay` | Start payment or answer a balance challenge | `bearerAuth` | path:`orderNo`*; header:`X-Lang`; body:application/json |
| GET | `/api/account/pay/orders/{orderNo}/receipt` | Payment receipt and immutable invoice revisions | `bearerAuth` | path:`orderNo`*; header:`X-Lang` |
| GET | `/api/account/pay/products` | Products visible to the current payer | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/pay/receipts/export` | Export payment reconciliation CSV | `bearerAuth` | query:`from`*; query:`to`*; query:`currency`; header:`X-Lang` |
| GET | `/api/account/pay/wallet` | Wallet balances by currency | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/pay/wallet/domains` | Read active and retained owned wallet domains | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| GET | `/api/account/pay/wallet/domains/{domainId}/txns` | Read the owned wallet-domain ledger | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | path:`domainId`*; query:`page`; query:`limit`; query:`currency`; query:`type`; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang` |
| POST | `/api/account/pay/wallet/transfers` | Transfer own available balance between allowed wallet domains | `bearerAuth` 或 `nativeAccessToken` 或 `R15RestrictedBrowserCookie` + `R15RestrictedBrowserToken` | header:`Idempotency-Key`*; header:`X-Native-Timestamp`; header:`X-Native-Nonce`; header:`X-Native-Signature`; header:`X-Lang`; body:application/json |
| GET | `/api/account/pay/wallet/txns` | Current payer's wallet ledger | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`currency`; query:`type`; header:`X-Lang` |
| GET | `/api/pay/checkout/{orderNo}` | Guest checkout | 公开；仍受站点、限流及业务条件约束 | path:`orderNo`*; header:`X-Lang` |
| POST | `/api/pay/checkout/{orderNo}/pay` | Start guest payment | 公开；仍受站点、限流及业务条件约束 | path:`orderNo`*; header:`X-Lang`; body:application/json |
| GET | `/api/pay/return/{orderNo}` | Return browser to checkout or approved merchant return | 公开；仍受站点、限流及业务条件约束 | path:`orderNo`*; header:`X-Lang` |

<a id="group-092"></a>

### public

Unauthenticated SPA endpoints (site config, captcha, public content).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/announcements` | Active announcements | 可匿名 或 `bearerAuth` | query:`placement`; header:`X-Lang` |
| GET | `/api/captcha` | Image captcha | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/api/config` | Public site configuration | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/api/developer` | Read public provider integration information | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/api/game/servers` | Visible game servers (with the caller's roles when logged in) | 可匿名 或 `bearerAuth` | header:`X-Lang` |
| GET | `/api/pages` | Published content pages (no bodies) | 公开；仍受站点、限流及业务条件约束 | query:`kind`; header:`X-Lang` |
| GET | `/api/pages/{slug}` | One published content page | 公开；仍受站点、限流及业务条件约束 | path:`slug`*; header:`X-Lang` |
| GET | `/api/providers` | Enabled social login providers（已弃用） | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/api/realname/doc-types` | Enabled real-name document types | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| POST | `/api/realname/kyc/callback/{provider}` | KYC provider callback | 公开；仍受站点、限流及业务条件约束 | path:`provider`*; header:`X-Lang`; body:application/json |

<a id="group-093"></a>

### server-api

Game-server plugin API authenticated with `X-Server-Key` (or `?key=`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| POST | `/server/v1/heartbeat` | Report live status | `serverKey` | header:`X-Lang`; body:application/json |
| GET | `/server/v1/whitelist` | Full whitelist of the calling server | `serverKey` | header:`X-Lang` |
| GET | `/server/v1/whitelist/check` | Single join check (whitelist + maintenance gate) | `serverKey` | query:`name`; query:`uuid`; header:`X-Lang` |

<a id="group-094"></a>

### sso-contracts

Application launchpad, gateway authentication, CAS and OIDC logout

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/account/sso/apps` | Read launch apps allowed by current account policy | `bearerAuth` | header:`X-Lang` |
| POST | `/api/account/sso/cas/continue` | Complete a signed CAS continuation after account checks | `bearerAuth` | header:`X-Lang`; body:application/json |
| POST | `/api/account/sso/launch/{id}` | Launch an allowed SSO app under its session deadline | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/account/sso/session` | Establish an opaque browser SSO grant | `bearerAuth` | header:`X-Lang` |
| GET | `/api/account/sso/sessions` | List the caller's live browser SSO grants | `bearerAuth` | header:`X-Lang` |
| DELETE | `/api/account/sso/sessions/{id}` | Revoke an owned browser SSO grant | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/sso/apps` | Read current-site SSO app policies | `bearerAuth` | header:`X-Lang` |
| POST | `/api/admin/sso/apps` | Create an SSO application | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/admin/sso/apps/{id}` | Replace an SSO application | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| DELETE | `/api/admin/sso/apps/{id}` | Delete SSO application and outstanding tickets | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/sso/apps/{id}/check` | Evaluate an account against application policy | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sso/apps/{id}/logout-events` | Read an SSO ticket app's logout delivery history | `bearerAuth` | path:`id`*; header:`X-Lang` |
| POST | `/api/admin/sso/apps/{id}/logout-events/{event}/retry` | Requeue an exhausted ticket-app logout notification | `bearerAuth` | path:`id`*; path:`event`*; header:`X-Lang` |
| POST | `/api/admin/sso/apps/{id}/rotate` | Rotate SSO application secret | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/admin/sso/cas/settings` | CAS provider settings and server URL | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/sso/cas/settings` | Update CAS provider settings | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/admin/sso/logout-clients` | OIDC clients eligible for logout configuration | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/sso/logout-clients/{client}` | OIDC logout registration and recent deliveries | `bearerAuth` | path:`client`*; header:`X-Lang` |
| PUT | `/api/admin/sso/logout-clients/{client}` | Replace OIDC client logout registration | `bearerAuth` | path:`client`*; header:`X-Lang`; body:application/json |
| GET | `/api/admin/sso/logs` | Latest SSO access and configuration log | `bearerAuth` | header:`X-Lang` |
| GET | `/api/admin/sso/settings` | Read SSO global ticket, cookie and header policy | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/admin/sso/settings` | Update SSO ticket, cookie and default header policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/sso/forward-auth` | Authorize a gateway request and emit mapped identity headers | `bearerAuth` + `ssoGatewayKey` 或 `ssoBrowser` + `ssoGatewayKey` | query:`app_id`*; header:`X-SSO-Host`; header:`X-Forwarded-Host`; header:`X-SSO-Uri`; header:`X-Forwarded-Uri`; header:`X-Lang` |
| POST | `/api/sso/v1/validate` | Redeem a one-use ticket and record target-session participation | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang`; body:application/json |
| GET | `/cas/login` | Begin CAS login with safe mandatory-account completion | 可匿名 或 `ssoBrowser` | query:`service`*; query:`renew`; query:`gateway`; header:`X-Lang` |
| GET | `/cas/logout` | End CAS/browser session and optionally return to a registered service | 可匿名 或 `ssoBrowser` | query:`service`; header:`X-Lang` |
| GET | `/cas/p3/serviceValidate` | Validate a CAS 3 service ticket with current app policy | 公开；仍受站点、限流及业务条件约束 | query:`service`*; query:`ticket`*; query:`renew`; query:`format`; query:`pgtUrl`; header:`X-Lang` |
| GET | `/cas/serviceValidate` | Validate a CAS 2 service ticket with current app policy | 公开；仍受站点、限流及业务条件约束 | query:`service`*; query:`ticket`*; query:`renew`; query:`format`; query:`pgtUrl`; header:`X-Lang` |
| GET | `/oauth2/logout` | RP-initiated OIDC logout | 可匿名 或 `ssoBrowser` | query:`id_token_hint`; query:`client_id`; query:`post_logout_redirect_uri`; query:`state`; query:`redirect`; query:`done`; header:`X-Lang` |
| POST | `/oauth2/logout` | RP-initiated OIDC logout (POST) | 可匿名 或 `ssoBrowser` | query:`id_token_hint`; query:`client_id`; query:`post_logout_redirect_uri`; query:`state`; query:`redirect`; header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/oauth2/logout/complete` | Complete OIDC logout notifications | 公开；仍受站点、限流及业务条件约束 | query:`flow`*; header:`X-Lang` |
| POST | `/oauth2/logout/confirm` | Confirm browser-bound OIDC logout | 公开；仍受站点、限流及业务条件约束 | cookie:`nowork_logout_confirm`*; header:`X-Lang`; body:application/x-www-form-urlencoded |
| GET | `/oauth2/logout/return` | Return after OIDC logout | 公开；仍受站点、限流及业务条件约束 | query:`flow`*; header:`X-Lang` |

<a id="group-095"></a>

### system

Health and metrics.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/healthz` | Liveness / readiness probe | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |
| GET | `/metrics` | Prometheus text exposition | 公开；仍受站点、限流及业务条件约束 | header:`X-Lang` |

<a id="group-096"></a>

### system-module

Roles, menus, dictionaries and audit records (`role:*`, `menu:*`, `dict:*`, `login-record:list`, `operation-record:list`).

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/system/dictionary` | List dictionaries | `bearerAuth` | query:`dictCode`; query:`dictName`; header:`X-Lang` |
| POST | `/api/system/dictionary` | Create a dictionary | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/system/dictionary` | Update a dictionary | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/system/dictionary-data` | List dictionary entries (any logged-in user) | `bearerAuth` | query:`dictCode`; query:`dictId`; header:`X-Lang` |
| POST | `/api/system/dictionary-data` | Create a dictionary entry | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/system/dictionary-data` | Update a dictionary entry | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/system/dictionary-data/batch` | Delete several entries | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/system/dictionary-data/page` | Page dictionary entries | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`keywords`; query:`dictDataName`; query:`dictDataCode`; query:`dictCode`; query:`dictId`; header:`X-Lang` |
| DELETE | `/api/system/dictionary-data/{id}` | Delete an entry | `bearerAuth` | path:`id`*; header:`X-Lang` |
| DELETE | `/api/system/dictionary/{id}` | Delete a dictionary | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/system/login-record` | List login records (export) | `bearerAuth` | query:`username`; query:`nickname`; query:`loginType`; header:`X-Lang` |
| GET | `/api/system/login-record/page` | Page login records | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`username`; query:`nickname`; query:`loginType`; query:`createTimeStart`; query:`createTimeEnd`; header:`X-Lang` |
| GET | `/api/system/menu` | List menus | `bearerAuth` | query:`title`; query:`path`; query:`authority`; query:`parentId`; header:`X-Lang` |
| POST | `/api/system/menu` | Create a menu | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/system/menu` | Update a menu | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/system/menu/{id}` | Delete a menu | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/system/operation-record` | List operation records (export) | `bearerAuth` | query:`username`; query:`module`; query:`status`; header:`X-Lang` |
| GET | `/api/system/operation-record/page` | Page operation records | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`username`; query:`nickname`; query:`module`; query:`description`; query:`status`; query:`createTimeStart`; query:`createTimeEnd`; header:`X-Lang` |
| GET | `/api/system/role` | List roles | `bearerAuth` | query:`roleName`; query:`roleCode`; header:`X-Lang` |
| POST | `/api/system/role` | Create a role | `bearerAuth` | header:`X-Lang`; body:application/json |
| PUT | `/api/system/role` | Update a role | `bearerAuth` | header:`X-Lang`; body:application/json |
| DELETE | `/api/system/role/batch` | Delete several roles | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/system/role/page` | Page roles | `bearerAuth` | query:`page`; query:`limit`; query:`sort`; query:`order`; query:`roleName`; query:`roleCode`; query:`comments`; header:`X-Lang` |
| DELETE | `/api/system/role/{id}` | Delete a role | `bearerAuth` | path:`id`*; header:`X-Lang` |
| GET | `/api/system/role/{id}/menu` | Menus with the role's checked state | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/system/role/{id}/menu` | Replace the role's menus | `bearerAuth` | path:`id`*; header:`X-Lang`; body:application/json |

<a id="group-097"></a>

### system/permissions

Main-super configuration of global grantable-role sets and delegation policy.

| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |
| --- | --- | --- | --- | --- |
| GET | `/api/system/permissions/me/scope` | Read the operator's own delegation ceiling | `bearerAuth` | header:`X-Lang` |
| GET | `/api/system/permissions/options` | Read permission catalogue, grantable set and delegation policy | `bearerAuth` | header:`X-Lang` |
| PUT | `/api/system/permissions/policy` | Update global delegation approval and lifetime policy | `bearerAuth` | header:`X-Lang`; body:application/json |
| GET | `/api/system/permissions/roles/{id}` | Read a role's held and separately grantable permissions | `bearerAuth` | path:`id`*; header:`X-Lang` |
| PUT | `/api/system/permissions/roles/{id}` | Replace a role's separately grantable permission set | `bearerAuth` | path:`id`*; header:`X-App-Approval-ID`; header:`X-App-Approval-Proof`; header:`X-Lang`; body:application/json |

## 更新和校验

在 SDK 根目录执行以下命令；第一条需要同级 Backend checkout：

```powershell
python ../Backend/scripts/gen_openapi.py --check
python scripts/generate_integration_reference.py
python scripts/generate_integration_reference.py --check
python scripts/check_docs.py
```

仅有 SDK checkout 时，`--check` 会检查已有快照、索引与内部引用的一致性，并明确说明没有与实时后端源契约比较；生成更新仍需要通过 `--source` 指定经过核对的后端 JSON。
