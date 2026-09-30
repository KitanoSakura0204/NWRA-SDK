# plugin.json 参考

插件包根目录的 `plugin.json` 描述插件的一切。解析是**严格**的：未知字段直接拒绝（拼错 `capabilities` 不会被悄悄忽略）。所有校验在上传 / 下载时进行，错误信息按管理员的语言显示。

## 1. 基本字段

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `id` | 是 | 小写反向域名，2–5 段，最长 64：`com.example.hello` |
| `name` | 是 | 1–64 字符 |
| `version` | 是 | 语义化版本 `1.2.0` / `1.2.0-beta.1`；更新时比较大小 |
| `author` | 是 | 1–64 字符 |
| `description` |  | 最多 512 字符 |
| `homepage` / `license` |  | 主页必须是 https |
| `kind` | 是 | `declarative` / `wasm` / `external` |
| `scope` |  | `global`（默认，启用即全站生效）/ `server`（管理员按游戏服务器逐台开启） |
| `locales` |  | `{"zh_TW": {"name": "...", "description": "..."}}`，语言限 `zh_CN` `zh_TW` `en` `ja` |
| `permissions` |  | 数据权限，见 §2 |
| `capabilities` |  | 能力，见 §2 |

`Text` 类型（标题、标签）是按语言的对象：`{"en": "Greet", "zh_CN": "问候"}`，至少一种语言非空，每项最多 128 字符；显示时依次回退到当前语言、`zh_CN`、`en`。

## 2. 数据权限与能力

**数据权限**决定插件能看到用户的哪些字段（事件载荷、`user.get`、页面上下文都按它过滤）；`userId`、`uid`、`status` 始终可见。

| 权限 | 风险 | 可见字段 |
| --- | --- | --- |
| `user.profile` | 低 | username、nickname、avatar、sex、birthday、introduction、region、createTime |
| `user.contact` | 高 | email、phone 及验证状态 |
| `user.security` | 高 | 信任等级、实名状态、最近登录时间 / IP / 地区；事件里的 ip、region、device、os、browser |
| `server.info` | 低 | 游戏服务器事件（心跳、白名单检查）所需 |

**能力**决定插件能调用哪些宿主函数、使用哪些清单段落。

| 能力 | 风险 | 允许 |
| --- | --- | --- |
| `log` | 低 | 写插件日志（审计页可见） |
| `kv` | 低 | 插件自己的键值存储 `kv.*` |
| `ui` | 低 | `ui` 界面插槽与 `links` 菜单链接 |
| `jobs` | 低 | `jobs` 定时任务 |
| `server.read` | 低 | `server.get` / `servers.list` |
| `user.read` | 中 | `user.get`（按数据权限过滤） |
| `fields` | 中 | `fields` 自定义用户字段与 `fields.*` |
| `notify` | 中 | `notify.send` 给用户发站内通知 |
| `http` | 高 | `http.fetch`，只能访问 `http.allowHosts`，并经过 SSRF 防护 |
| `gm.actions` | 高 | `gmActions` 为服务器增加 GM 操作 |
| `token.claims` | 高 | 订阅 `token.claims`，往 ID 令牌 / userinfo 里加字段 |
| `server.whitelist` | 高 | 订阅 `server.whitelist_check`（只能把允许改成拒绝） |

声明式插件只能申请 `ui` 与 `fields`，且不能包含 `jobs` / `gmActions` / `claims` / `http`。

## 3. 订阅扩展点 `points`

```json
"points": [
  { "point": "user.login", "handler": "on_login" },            // wasm: 导出函数名
  { "point": "user.login", "url": "https://hooks.example.com" } // declarative: 推送地址
]
```

external 插件只写 `point`（可选 `handler` 作为标签），所有调用都发往 `external.endpoint`。每个扩展点只能订阅一次，最多 64 个。

| 扩展点 | 类型 | 需要 | 说明 |
| --- | --- | --- | --- |
| `user.register` | 事件 | — | 自助注册 / 第三方首次登录建号后 |
| `user.login` | 事件 | — | 登录成功（含第三方登录） |
| `user.update` | 事件 | — | 资料、状态、等级变化（同一用户 2 秒内合并） |
| `user.delete` | 事件 | — | 账号删除 / 自助注销（事务提交后） |
| `server.heartbeat` | 事件 | `server.info` | 服务器心跳采样（每台每分钟最多一次） |
| `token.claims` | 过滤 | `token.claims` | 签发令牌时追加字段，见 §6 |
| `server.whitelist_check` | 过滤 | `server.whitelist` + `server.info` | 进服检查，返回 `{"deny":true,"message":"..."}` 拒绝 |

事件是异步的，永远不阻塞触发它的请求。过滤是同步的，所有插件并行执行，总时长不超过 `plugins.sync_budget_ms`（默认 300 ms）；超时、出错或输出不是 JSON 对象的插件被跳过，登录照常完成。其他模块可以注册自己的扩展点，后台「扩展点」页列出当前版本的全部扩展点与接入状态。

## 4. 其他段落

**`settings`**：设置结构（JSON Schema 子集）。一层 `type: "object"`，1–50 个属性，属性类型 `string` / `number` / `integer` / `boolean`，支持 `title`、`x-title`（按语言）、`description`、`x-description`、`default`、`enum`、`minimum`、`maximum`、`minLength`、`maxLength`、`pattern`、`format`（`secret` / `url` / `email` / `textarea`）与顶层 `required`。不支持的关键字直接拒绝。`secret` 字段加密存储、后台只显示“已设置”。插件在每次调用的 `settings` 里收到当前值。缺少必填设置时插件不能启用。

**`fields`**（需要 `fields`）：自定义用户字段，最多 50 个。`{ key, type: string|number|boolean|select, label: Text, options?: [{value, label}], maxLength?: 1–2000（默认 200）, userVisible?, userEditable? }`。`userEditable` 蕴含 `userVisible`；用户在「应用与插件」里编辑，管理员在「用户字段」里可以改全部字段。账号删除时字段值一起删除。

**`links`**（需要 `ui`）：`{ area: home|admin, title: Text, url }`，`url` 是 https 地址或 `/home/…`（admin 区也可 `/admin/…`）站内路径。

**`ui`**（需要 `ui`）：界面插槽，最多 20 个。`{ slot, title?: Text, entry: "ui/index.html", height?: 0–2000, handler? }`。插槽：`admin.page`（插件详情里的页面）、`admin.dashboard`、`admin.user.detail`、`home.page`（用户中心「应用与插件」里的独立页面）、`home.overview`、`home.profile`；`*.page` 必须有标题。`handler` 接收页面的桥接 `invoke`；没有它页面只能展示。

**`jobs`**（需要 `jobs`）：`{ name, every: "5m".."168h", handler }`，最多 10 个。启用后过一个间隔首次运行；多实例部署时同一次运行只有一个实例执行。

**`gmActions`**（需要 `gm.actions`）：`{ key, title: Text, description?: Text, params?: [{ name, type: string|number|boolean|player, required?, label?: Text }], handler, danger?: normal|confirm|critical }`，最多 50 个，每个最多 20 个参数，文本参数最多 256 个字符。操作出现在服务器 GM 面板里（servergm 操作来源 `plugins`，目录键 `plugins:<插件 id>.<key>`，id 太长时为 `plugins:p<12 位十六进制>.<key>`，插件更新后不变），与游戏插件自己的操作一样受授权、信任等级和确认步骤约束，并记入命令记录。`danger` 决定确认步骤（`critical` 要求操作者输入操作键），服主的覆盖只能更严格。server 范围插件的 GM 操作只出现在它开启的服务器上；服务器还要启用「服务器 GM 面板」模块。

**`claims`**（订阅 `token.claims` 时必填）：`{ key, type: string|number|boolean|object, description?: Text }`，最多 30 个。只有声明过、类型匹配的字段会被发布；文本最长 1024，对象序列化后最长 4 KB。

**`http`**（与能力 `http` 同时出现）：`{ "allowHosts": ["api.example.com", "*.example.org"] }`，最多 20 项，不能写 IP。

**`wasm`**（wasm 插件）：`{ module: "plugin.wasm", memoryMb?: 0–512, timeoutMs?: 0–60000 }`，实际限制取清单请求与后台上限中较小者。

**`external`**（external 插件）：`{ endpoint: "https://…" }`，必须是 https（调试模式可用 http）且不能指向内网。

## 5. 包内文件

插件包是 zip：根目录 `plugin.json`，可选 `signature.json`，加上清单引用的文件（`wasm.module`、每个 `ui[].entry` 必须存在）。限制：zip 最大 10 MB、解压后 32 MB、单个文件 16 MB、最多 500 个文件；路径不能以 `/` 开头、不能包含 `..`、`\`、`:` 或以 `.` 开头的段；`__MACOSX/` 与 `.DS_Store` 自动跳过。浏览器只能取到界面文件类型（html、js、css、json、txt、svg、png、jpg、gif、webp、ico、woff、woff2），wasm 模块不会下发。

## 6. 令牌字段如何下发

`token.claims` 通过下发模块（`delivery`）的属性注册表接入：属性 `plugin_claims` = `{"<插件 id>": {"<key>": 值}}`，敏感度 personal。管理员在下发配置里为某个应用或服务器勾选它才会出现在 ID 令牌 / userinfo / 服务器载荷中；后台预览只显示声明的字段与类型，不调用插件。

## 7. 市场列表字段与宿主兼容（标准 1.1）

以下字段都可选，v1 清单不写它们仍然合法。机器可读定义：[schema/plugin.schema.json](schema/plugin.schema.json)（JSON Schema 2020-12，与宿主校验器共用 `schema/testdata/manifests.json` 测试向量）。

| 字段 | 说明 |
| --- | --- |
| `engines` | `{"lounge": ">=1.1.0 <2.0.0", "abi": 1}`。`lounge` 是宿主插件接口版本（当前 1.1.0）的范围：`>=` `>` `<=` `<` `=` `^` `~` `*`，空格为“且”，`\|\|` 为“或”，最长 128；`abi` 是 wasm 宿主 ABI（当前 1）。不写 = 任何 1.x 宿主、ABI 1。不兼容时上传与市场安装都会拒绝 |
| `icon` | 包内 png / webp，≤ 256 KB，建议 256×256 |
| `screenshots` | 最多 8 个 `{ "path": "shots/1.png", "caption"?: Text }`，png / webp / jpeg，每张 ≤ 1 MB |
| `categories` | 1–3 个，取自固定列表：`account` 账号、`security` 安全、`game` 游戏、`server` 服务器、`community` 社区、`notification` 通知、`integration` 集成、`analytics` 统计、`appearance` 外观、`developer` 开发、`other` 其他 |
| `keywords` | 最多 10 个，每个 1–32 字符，不含逗号、首尾无空白 |
| `changelog` | 本版本的更新说明，按语言 `{"zh_CN": "…", "en": "…"}`，每种最多 2000 字符 |
| `repository` | https 源码地址 |
| `support` | `{ "url"?: "https://…", "email"?: "…" }`，至少一项 |

图标与截图按文件内容（不是扩展名）判断类型。`loungeplugin lint -dir .` 在本地做同样的检查；发布流程见 [PUBLISHING.md](PUBLISHING.md)。
