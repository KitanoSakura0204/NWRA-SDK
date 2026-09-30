# 玩家数据与服务器封禁（server-users）

> English version below.

模块 `server-users` 为每台服务器提供：玩家名单（绑定过角色、或被进服校验 / 心跳看到的账号）、按服务器生效的封禁、以及服主自定义字段的玩家数据。服主在用户中心的服务器控制台、管理员在后台的「玩家数据」中管理（字段定义、模块配置 `pluginBans` / 加密存储 / 容量上限也在那里设置）；游戏服务器插件通过下列 Server API（`X-Server-Key`，统一响应见 [PROTOCOL.md](PROTOCOL.md) §1）读写。

| 接口 | 需要模块开启 | 说明 |
| --- | --- | --- |
| `GET /server/v1/users/schema` | 是 | 字段定义 |
| `GET /server/v1/users/{uid}/data` | 是 | 读取一名玩家的数据 |
| `PUT /server/v1/users/{uid}/data` | 是 | 整体替换 |
| `PATCH /server/v1/users/{uid}/data` | 是 | 合并更新，`null` 删除字段 |
| `GET /server/v1/bans` | 否 | 生效中的封禁 |
| `GET /server/v1/bans/kick` | 否 | 需要踢出的在线玩家 |
| `POST /server/v1/bans` | 是，且配置 `pluginBans` 为真 | 封禁 |
| `POST /server/v1/bans/lift` | 是，且配置 `pluginBans` 为真 | 解除封禁 |

模块关闭后封禁仍然生效，所以查询封禁与踢出列表不要求模块开启。模块未开启时其他接口返回业务错误 `code = 1`。

## 1. 进服校验与白名单

- `GET /server/v1/whitelist/check`：账号在本服务器有生效中的封禁时返回 `allowed: false, reason: "banned"`，封禁原因在 `message` 字段（可能为空），请显示给玩家。按名称或 UUID 匹配到的任一账号被封禁都会拒绝，玩家无法换一个绑定角色绕过。封禁检查出错时返回 `reason: "check unavailable"` 并拒绝（fail closed）。
- `GET /server/v1/whitelist`：被封禁账号的条目不出现在全量白名单中（仅供参考，以进服校验为准）。
- 其他 `reason`：`not bound`、`pending review`、`account locked`、`whitelist disabled`、`maintenance`（见 PROTOCOL §2.2）。

## 2. 玩家数据

### 字段定义 `GET /server/v1/users/schema`

```json
{ "fields": [
    { "key": "coins", "label": "金币", "type": "number", "visible": true, "integer": true, "min": 0, "default": 0 },
    { "key": "rank", "label": "段位", "type": "select", "visible": true, "options": [ { "value": "gold", "label": "黄金" } ] },
    { "key": "stats", "label": "统计", "type": "json", "visible": false, "maxLength": 4096 }
  ],
  "maxUserBytes": 8192 }
```

- `type`：`text`（`maxLength` 为字符数，默认 255，最大 20000，不得含控制字符）、`number`（`min` / `max` / `integer`）、`bool`、`select`（值必须在 `options` 中）、`json`（任意 JSON，`maxLength` 为字节数，默认 4096）。
- `key` 以字母开头，只含字母、数字和下划线，最长 48 个字符。
- `visible` 为真的字段会在玩家自己的「我的服务器数据」页面只读显示。
- `maxUserBytes`：单个玩家存储值（JSON）的上限（256 B - 32 KiB，默认 8 KiB）；服务器另有总量上限。

### 读取 `GET /server/v1/users/{uid}/data`

`uid` 为账号 UID（进服校验的 `player.uid` 或白名单条目的 `uid`），账号必须在本服务器的玩家名单中，否则返回 `code = 404`。

```json
{ "uid": 100001, "data": { "coins": 12, "rank": null, "stats": null },
  "version": 3, "updatedAt": "2026-09-29 10:00:00", "updatedBy": "plugin", "size": 11 }
```

`data` 包含全部字段：已存的值，否则字段默认值，否则 `null`。`version` 为乐观锁版本（0 表示尚未存过数据），`updatedAt` 为服务器本地时间，首次写入前为 `null`。

### 写入 `PUT` / `PATCH /server/v1/users/{uid}/data`

```json
{ "data": { "coins": 20 }, "version": 3 }
```

- `PUT` 用 `data` 整体替换（未给出的字段被清除）；`PATCH` 合并，值为 `null` 的字段被删除。
- `version` 可选：给出时必须等于当前版本，否则返回 `code = 409`，请重新读取后再写；省略则无条件写入。
- 未定义的字段、类型不符、超出长度或范围、超过 `maxUserBytes` 或服务器总量时返回 `code = 1` 与原因。
- 成功时返回与读取相同的对象。服主开启「加密存储」时数据在后端加密保存，对接口透明。操作日志只记录字段名，不记录值。

## 3. 封禁

目标账号用 `uid`，或本服务器上某个绑定角色的 `name` / `uuid` 指定（`name` 不区分大小写）；一个名称对应多个账号时返回业务错误，请改用 `uid`。

### 查询 `GET /server/v1/bans?uid=&name=&uuid=`

不带参数返回全部生效中的封禁（最多 5000 条，新的在前），带参数时只返回匹配账号的封禁：

```json
{ "count": 1, "bans": [ { "uid": 100001, "names": ["Steve"], "uuids": ["069a79f4-44e9-4726-a5be-fca90e38aaf5"],
    "reason": "x-ray", "expireTime": null, "createTime": "2026-09-29 10:00:00", "source": "plugin" } ] }
```

`names` / `uuids` 为该账号在本服务器的绑定（已驳回的除外）；`expireTime` 为 `null` 表示永久；`source` 为 `admin`、`owner` 或 `plugin`。

### 踢出列表 `GET /server/v1/bans/kick`

按最近一次心跳上报的在线玩家（PROTOCOL §2.3 `players`）匹配生效中的封禁，返回应立即断开的玩家：

```json
{ "count": 1, "players": [ { "name": "Steve", "uuid": "069a...", "uid": 100001, "reason": "x-ray", "expireTime": null } ] }
```

建议每次心跳后调用一次。心跳不带 `players` 时列表始终为空。

### 封禁 `POST /server/v1/bans`

```json
{ "name": "Steve", "reason": "x-ray", "durationMinutes": 1440 }
```

- `expireTime`（`2006-01-02 15:04:05` 或 RFC 3339，须晚于现在且不超过 3650 天）与 `durationMinutes` 只能给一个，都不给为永久封禁；`reason` 最长 255 个字符。
- 已有封禁时替换为新的；管理员设置的封禁插件不能替换或解除（`code = 403`）。
- 返回 `{ "uid": 100001, "ban": { ...同查询 } }`。

### 解除 `POST /server/v1/bans/lift`

`{ "uid": 100001, "reason": "申诉通过" }` → `{ "uid": 100001, "lifted": true }`；没有生效中的封禁时返回业务错误。

## 4. 错误码

| `code` | 含义 |
| --- | --- |
| `404` | 该 uid 不在本服务器的玩家名单中 |
| `409` | 数据已被修改（`version` 过期） |
| `403` | 管理员设置的封禁，插件无权修改 |
| `1` | 其他业务错误：模块未开启、不允许插件封禁、字段校验失败、名称对应多个账号、没有生效中的封禁等，`message` 为原因 |

## 5. SDK

| 语言 | 玩家数据 | 封禁 | 其他 |
| --- | --- | --- | --- |
| Go | `UserSchema` / `UserData` / `PutUserData` / `PatchUserData`（`version` 传 -1 表示不校验） | `Bans` / `KickList` / `Ban` / `Unban` | `IsVersionConflict`，`CheckResult.Message` |
| TypeScript | `userSchema` / `userData` / `putUserData` / `patchUserData`（`{ version }`） | `bans` / `kickList` / `ban` / `unban` | `isVersionConflict`，`CheckResult.message` |
| Python | `user_schema` / `user_data` / `put_user_data` / `patch_user_data`（`version=`） | `bans` / `kick_list` / `ban` / `unban` | `is_version_conflict`，`CheckResult.message` |
| Java | `userSchema` / `userData` / `putUserData` / `patchUserData`（`version` 传 -1 表示不校验） | `bans` / `kickList` / `ban` / `unban`（`ServerUsers.Target`） | `ServerUsers.isVersionConflict`，`CheckResult.message` |
| C# | `UserSchemaAsync` / `UserDataAsync` / `PutUserDataAsync` / `PatchUserDataAsync`（`version: null` 不校验） | `BansAsync` / `KickListAsync` / `BanAsync` / `UnbanAsync`（`BanTarget`） | `ServerUsersCodes.IsVersionConflict`，`CheckResult.Message` |
| PHP | `userSchema` / `userData` / `putUserData` / `patchUserData`（`$version = null` 不校验） | `bans` / `kickList` / `ban` / `unban`（命名参数 `uid:` / `name:` / `uuid:`） | `UserData::isVersionConflict`，`CheckResult->message` |

典型用法（Go）：进服校验通过后用 `player.uid` 读取数据；写入时带上读到的 `Version`，遇到 `IsVersionConflict` 重新读取、合并后再写；每次心跳（带 `Players`）后调用 `KickList` 断开被封禁的玩家。

---

# Player data and per-server bans (server-users)

The `server-users` module gives every game server a roster (accounts that bound a role or were seen by join checks and heartbeats), bans that apply on that server only, and typed player data whose fields the server owner defines. Owners manage it in their server console in the user centre, administrators under Player data in the admin console (schema, the `pluginBans` setting, encryption at rest and the size limits live there too); the game-server plugin uses the Server API routes in the table above (`X-Server-Key`, envelope as in PROTOCOL.md section 1). Bans stay enforced after the module is switched off, so listing bans and the kick list work either way; the other routes need the module on (otherwise `code = 1`), and placing or lifting a ban also needs the module setting `pluginBans`.

- Join check: an account banned on this server gets `allowed: false, reason: "banned"` with the ban reason in `message` (show it to the player). Any account bound to the joining name or uuid that is banned refuses the join, so another binding cannot get around it. A failing ban check answers `reason: "check unavailable"` and refuses (fail closed). The full whitelist leaves banned accounts out (advisory; the join check decides).
- Schema: `GET /server/v1/users/schema` returns `fields` (key, label, type `text` / `number` / `bool` / `select` / `json`, `visible` = shown read-only to the player, optional `default`, `maxLength` in characters for text or bytes for json, `min` / `max` / `integer`, `options`) and `maxUserBytes`, the cap on one account's stored JSON (256 B - 32 KiB, default 8 KiB).
- Read: `GET /server/v1/users/{uid}/data` (uid from the join check or the whitelist; not on the roster = `code 404`) returns every field (stored value, else the default, else null), the optimistic-lock `version` (0 = nothing stored), `updatedAt` (server time, null before the first write), `updatedBy` and `size`.
- Write: `PUT` replaces, `PATCH` merges (null deletes a field), body `{ "data": {...}, "version": 3 }`. With `version` the write succeeds only on the current version, otherwise `code 409`: read again and retry; without it the write is unconditional. Unknown fields, wrong types, limits and quotas answer `code 1` with the reason. The reply is the stored data. Encryption at rest, when the owner turns it on, is transparent; the operation log records key names only.
- Bans: name the account by `uid`, or by the `name` (case-insensitive) / `uuid` of one of its bindings here; a name bound by several accounts is refused (use the uid). `GET /server/v1/bans` lists active bans (all, at most 5000 newest first, or those of `?uid=` / `?name=` / `?uuid=`), each with the account's `names` / `uuids` here, `reason`, `expireTime` (null = permanent), `createTime` and `source` (`admin`, `owner`, `plugin`). `GET /server/v1/bans/kick` matches the last heartbeat's `players` against the active bans: disconnect those players (call it after each heartbeat; without `players` it is always empty). `POST /server/v1/bans` takes the target plus `reason` (at most 255 characters) and `expireTime` (`2006-01-02 15:04:05` or RFC 3339, in the future, at most 3650 days) or `durationMinutes` (neither = permanent), replaces an existing ban and answers `{ uid, ban }`; a ban an administrator placed cannot be replaced or lifted by the plugin (`code 403`). `POST /server/v1/bans/lift` takes the target and an optional `reason` and answers `{ uid, lifted: true }`.
- Codes: `404` not on the roster, `409` stale version, `403` administrator ban, `1` every other refusal with the reason in `message`.
- SDKs: see the table in section 5 (Go, TypeScript, Python, Java, C#, PHP; the join check's `message` is exposed as `CheckResult.Message` / `message`).
