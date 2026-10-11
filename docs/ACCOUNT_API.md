# 用户门户客户端（2026-10-08）

Go `NewAccount(base, token, nil)`、Python `AccountClient(base, token)`、TypeScript `new AccountClient(base, { token })` 提供相同的用户门户接口。`token` 是站内登录 JWT；服务器密钥、机器人令牌和 OAuth access token 不可代替它。客户端只发送 `Authorization: Bearer …`，不发送 `X-Server-Key`。默认 10 秒超时，可配置；拒绝重定向，防止身份凭据被转发到意外地址。

| 操作 | 路径 | Go / Python / TypeScript |
| --- | --- | --- |
| 可游玩服务器 | GET `/api/account/portal/servers` | `Servers` / `servers` / `servers` |
| 本人白名单状态 | GET `/api/account/portal/servers/{id}/whitelist-status` | `WhitelistStatus` / `whitelist_status` / `whitelistStatus` |
| 本人申请/角色列表 | GET `/api/account/whitelist/applications` | `WhitelistApplications` / `whitelist_applications` / `whitelistApplications` |
| 申请白名单 | POST `/api/account/portal/servers/{id}/whitelist-apply` | `ApplyWhitelist` / `apply_whitelist` / `applyWhitelist` |
| 绑定角色（也支持开放服） | POST `/api/account/portal/servers/{id}/roles` | `BindRole` / `bind_role` / `bindRole` |
| 以角色发起 GM 登录 | POST `/api/account/gm/servers/{id}/launch` | `LaunchGM` / `launch_gm` / `launchGM` |

角色写入体为 `{roleName,roleUuid?,remark?}`；GM 发起体为 `{roleId}`。Go 使用 `PortalRoleInput`，TypeScript 使用 `PortalRoleInput`，Python 方法直接接收角色名、UUID 和备注。`whitelist-apply` 会拒绝白名单关闭的服务器，这类服务器应使用 `roles`。两种操作都执行同一套身份、数量、等级、实名和封禁校验。

服务器 DTO 包含公共名称/地址/版本/状态、`whitelistMode`、`requiresWhitelist`、`userHasApplied`、`userIsWhitelisted`、`canBind`、`bindTip`、本人 `roles` 和 `ban`、`serverGmLoginEnabled`、`playerDataEnabled` 及角色字段要求。只按返回字段展示是否可申请、进入或登录，不能仅从白名单模式推断权限。`/api/account/servers` 保留给服主控制台，不能用作玩家服务器列表。

```ts
import { AccountClient } from '@nowork/sdk';
const account = new AccountClient('https://account.example', { token: siteLoginJwt });
const servers = await account.servers();
const launch = await account.launchGM(selectedServer.id, selectedRole.id);
// Open launch.startUrl immediately in the same browser that holds the site's login.
```

Java/C#/PHP 当前提供全部游戏服务器接口（含角色 GM 兑换和聊天中继），未另建站内 JWT 门户客户端；嵌入式前端可按上述路径使用自己的站内会话 HTTP 封装。门户客户端不会获取、刷新或保存用户登录 JWT。

## English

The Go, Python and TypeScript account clients expose the same portal, whitelist, role-binding and GM launch operations. They require a site login JWT and keep account authentication separate from server keys. Use `/portal/servers/{id}/roles` to bind on open servers; the whitelist application endpoint is only for whitelist-enabled servers. Preserve the user's returned eligibility/ban/status fields, and open GM launch URLs immediately in the authenticated browser.
