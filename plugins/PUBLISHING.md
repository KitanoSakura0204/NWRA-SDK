# 发布插件（插件标准 1.1）

[English](#publishing-plugins-plugin-standard-11)

本文说明如何把插件发布到插件市场：自建静态市场（`noworkplugin index`），或提交到站点自带的插件市场（后台模块 pluginmarket，默认关闭）。字段定义见 [MANIFEST.md](MANIFEST.md)，JSON Schema 见 [schema/](schema/)。

## 1. 标识

- `id` 是小写反向域名，2–5 段、最长 64：`com.example.hello`。用你能证明拥有的域名；没有域名时用 `io.github.<用户名>.<插件>`。
- 同一个 `id` 永远属于第一个发布它的开发者；站点市场拒绝别人用同一个 `id` 提交。
- 发布后不要改 `id`：宿主按 `id` 识别更新，改名等于一个新插件，已安装的站点收不到更新。

## 2. 版本

- `version` 是语义化版本 `主.次.修订`，可带预发布后缀 `-beta.1`。宿主按语义化版本比较，预发布排在同号正式版之前。
- 不兼容的改动（删除设置项、改变字段含义、申请新的高风险能力）升主版本；新功能升次版本；只修问题升修订号。
- 同一个 `id` + `version` 只能发布一次，内容不同必须换版本号。
- 预发布版本在市场里标为“预发布”，站点市场的列表按最高正式版显示（没有正式版时才显示预发布）。

## 3. 宿主兼容 `engines`

```json
"engines": { "nowork": ">=1.1.0 <2.0.0", "abi": 1 }
```

- `nowork`：宿主插件接口版本的范围（当前 `1.1.0`）。支持 `>=` `>` `<=` `<` `=`、`^1.1`（同主版本）、`~1.1.2`（同次版本）、`*`，空格表示“且”，`||` 表示“或”；不支持 `1.x` 写法；预发布标记只能跟在完整版本后（`>=1.1.0-beta`，不能写 `>=1.1-beta`）。
- `abi`：wasm 宿主 ABI 版本（当前 `1`，见 [ABI.md](ABI.md)）；`0` 与不写相同，即 ABI 1。
- 不写 `engines` 等于“任何 1.x 宿主、ABI 1”。不兼容的包在上传和从市场安装时都会被拒绝，并说明需要的版本。

## 4. 审核检查什么

站点市场的审核方式（后台“审核方式”页，类型“插件市场提交”）可以是人工、自动或 Webhook。自动只做静态检查，就是下面的全部；人工审核在此之外还要看说明与实际行为是否一致。

1. 包结构：zip 限制（10 MB、解压 32 MB、单文件 16 MB、500 个文件）、路径安全、`plugin.json` 严格解析，与安装时完全相同。
2. 清单：宿主的校验器（扩展点、能力与段落的配对、处理函数）加 1.1 列表字段（图标、截图、分类、关键词、更新说明、`engines`）。
3. 文件：图标 png / webp ≤ 256 KB，截图 png / webp / jpeg ≤ 1 MB、最多 8 张，按文件内容而不是扩展名判断类型。
4. 签名：提交的包可以不签名；包里的 `signature.json` 会被丢弃，通过后由市场用自己的发布者密钥重新签名（签名覆盖的内容不变）。
5. 版本：比该插件已发布的最高版本高，且 `id` 属于提交者。
6. 高风险项：申请 `user.contact`、`user.security`、`http`、`gm.actions`、`token.claims`、`server.whitelist` 时，提交表单里的“权限说明”必填（自动审核只检查是否填写，人工审核判断是否合理）。

`noworkplugin lint -dir .` 在本地跑 1–3 项与 6 项之外的全部检查。

## 5. 权限与能力的说明

每项数据权限和能力都要在提交说明里写一句“为什么需要”，例如“`http`：调用 api.example.com 查询天气”。只申请真正用到的：多余的高风险项是退回的最常见原因。管理员安装时会看到同样的清单（附风险等级），任何一项都需要他们同意。

## 6. 密钥

```bash
noworkplugin keygen -out publisher      # publisher.key（私钥，保密）+ publisher.pub（公钥）
noworkplugin pack -dir . -key publisher.key -out dist
```

- 私钥不要放进仓库（SDK 的 `.gitignore` 已排除 `*.key`），也不要放进插件包。
- 自建市场：把 `publisher.pub` 发给站点管理员，加入后台“插件 → 受信任的发布者”。
- 站点市场：安装方信任的是**市场**的发布者密钥（后台“插件市场 → 发布者密钥”显示 keyId 与公钥）；提交者身份由登录账号和开发者权限保证，不需要自己的密钥。

### 轮换与吊销

- 轮换：生成新密钥，先让站点把新公钥加入受信任列表，再用新密钥发布新版本；旧公钥保留到旧版本不再需要为止。站点市场由超级管理员在后台轮换，旧 keyId 在吊销前继续有效。
- 吊销：私钥泄露时立即吊销。索引的 `revokedKeys` 列出被吊销的 keyId（附原因、时间），宿主不再安装这些密钥签名的版本；管理员也应从“受信任的发布者”中删除对应公钥。
- `noworkplugin index -revoked revoked.json` 把吊销列表写进自建市场的索引。

## 7. 撤回（yank）与下架

- 撤回一个版本：索引里该版本带 `"yanked": true` 与 `yankedReason`，宿主显示但不能安装；已经安装的站点不受影响，但会看到原因。用于严重问题，修好后发布新版本。
- 下架整个插件：从市场列表消失，不能再安装。站点市场的开发者可以自己撤回版本、下架插件；管理员也可以。
- 自建市场：`noworkplugin index -yanked yanked.json`，文件格式 `{"com.example.hello@1.0.1": "登录时崩溃"}`。

---

# Publishing plugins (plugin standard 1.1)

How to publish a plugin: to a static market you host (`noworkplugin index`) or to a site's own plugin market (the pluginmarket module, off by default). Fields: [MANIFEST.md](MANIFEST.md); JSON Schemas: [schema/](schema/).

## 1. Identifier

- `id` is a lower-case reverse domain, 2-5 labels, at most 64 characters: `com.example.hello`. Use a domain you can show you own, or `io.github.<user>.<plugin>`.
- An `id` belongs to the developer who first published it; a site market refuses the same `id` from anyone else.
- Never change the `id` after publishing: hosts match updates by `id`.

## 2. Versions

- `version` is semver `major.minor.patch` with an optional pre-release suffix (`-beta.1`), which sorts before its release.
- Breaking changes (a removed setting, a new high-risk capability) bump major; features bump minor; fixes bump patch.
- An `id` + `version` is published once; different content needs a new version.

## 3. Host compatibility `engines`

`{"nowork": ">=1.1.0 <2.0.0", "abi": 1}`: `nowork` is a range over the host plugin API version (1.1.0 today; `>=`, `<`, `^`, `~`, `||`), `abi` the wasm host ABI (1; `0` is the same as leaving it out). A pre-release tag needs a full version (`>=1.1.0-beta`, not `>=1.1-beta`). Without `engines`: any 1.x host, ABI 1. Incompatible packages are refused on upload and market install, with the version they need.

## 4. What review checks

The review mode (admin "Review modes", kind "Plugin market submission") is manual, automatic or webhook. Automatic runs the static checks only: package limits and path safety, the host's manifest validator plus the 1.1 listing fields, icon (png / webp, 256 KB) and screenshots (png / webp / jpeg, 1 MB, at most 8) by content, (the package's own signature.json is dropped; the market re-signs with its publisher key), a version above the published ones and an `id` owned by the submitter, and a filled-in justification for high-risk items (`user.contact`, `user.security`, `http`, `gm.actions`, `token.claims`, `server.whitelist`). A human reviewer also checks that the description matches the behaviour. `noworkplugin lint` runs the package checks locally.

## 5. Justify permissions and capabilities

One sentence per item, for example "`http`: calls api.example.com for the weather". Ask only for what you use; extra high-risk items are the usual reason for a rejection.

## 6. Keys

`noworkplugin keygen -out publisher` writes `publisher.key` (secret; never commit or package it) and `publisher.pub`. For a static market, send `publisher.pub` to the site administrators (admin "Plugins → Trusted publishers"). On a site market installers trust the market's own publisher key; your account and developer access identify you, no key needed.

Rotation: create a new key, get it trusted, publish new versions with it, keep the old key trusted until the old versions are no longer needed. A site market's key is rotated by the super administrator; old key ids keep verifying until revoked. Revocation: the index's `revokedKeys` lists revoked key ids with a reason and a time; hosts refuse versions signed by them (`noworkplugin index -revoked revoked.json`).

## 7. Yanking and delisting

A yanked version (`"yanked": true`, `yankedReason`) stays visible but cannot be installed; installed copies keep running and show the reason. Delisting removes the whole plugin from the market. Static markets: `noworkplugin index -yanked yanked.json` with `{"com.example.hello@1.0.1": "crashes on sign-in"}`.
