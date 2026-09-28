# 自定义 SDK 登录模板（LOGIN_TEMPLATES）

SDK 登录页（`/sdk/login/{serverCode}`，协议见 [PROTOCOL.md §6](PROTOCOL.md)）内置一套**标准模板**：PC 端是品牌背景上的居中卡片，手机端是整屏面板。管理员可以按服务器、按设备（PC / 手机）改用**自定义模板**。自定义模板由 HTML、CSS 与可选的 JS 组成，只能通过本文的 `window.LoungeLogin` 接口与用户中心交互。

后台入口：「游戏服务 → SDK 登录页」→ 选择服务器 →「登录模板」。保存、发布、回滚、切换需要按钮权限 `sdklogin:template`（默认只有超级管理员有），每次改动都写入操作日志。

## 1. 工作方式

```
游戏客户端 ──打开──▶ /sdk/login/{serverCode}?…&device=pc|mobile   （用户中心页面，宿主）
                         │  会话、人机验证、通行密钥、授权确认、结果页、交回游戏都在这里
                         │  MessageChannel（LoungeLogin 桥，协议 v1）
                         ▼
               GET /api/sdk-login/{serverCode}/template?device=…   （沙箱文档：你的模板）
                         不透明源、不能联网、不能读写本站存储
```

- 模板文档带 `Content-Security-Policy: … sandbox allow-scripts allow-forms`，即使被直接打开也是不透明源：拿不到用户中心的 Cookie、localStorage 与令牌。
- 模板**只负责登录与「以 X 继续」两步**。登录成功后，授权确认、登录码 / 成功 / 取消的结果页与交回游戏都由用户中心显示，模板拿不到授权码、令牌、回调地址或两步验证的 challenge。
- 模板不能选择回调地址、不能跳过 PKCE 或授权确认：这些参数来自游戏客户端的原始 URL，由宿主原样转交后端校验（PROTOCOL §6.1）。
- 出错即回退：模板文档没有加载、8 秒内没有握手、握手后 10 秒内没有调用 `ready()`、`ready()` 之前抛出未捕获的错误、框架被导航到别的文档、或模板调用 `LoungeLogin.fallback()`，页面都会换回标准模板（已经登录的玩家看到「以 X 继续」）。

## 2. 模板的组成

| 部分 | 说明 | 上限 |
| --- | --- | --- |
| HTML | `<body>` 里的片段，不含 `<html>` `<head>` `<body>` | 64 KB |
| CSS | 放进 `<head>` 的 `<style>` | 64 KB |
| JS | 片段之后以 nonce 运行的经典脚本（`LoungeLogin` 已就绪） | 128 KB |
| 脚本来源 | 允许 `LoungeLogin.loadScript()` 加载的 https 源 | 10 个 |
| 资源来源 | 允许加载图片 / 字体 / 样式 / 媒体的 https 源 | 10 个 |

三部分合计不超过 192 KB。用户中心生成的文档大致是：

```html
<!doctype html>
<html lang="zh-CN" data-device="pc" data-theme="light">
<head> …charset / viewport / referrer… <style>你的 CSS</style> <script nonce>LoungeLogin 运行时</script> </head>
<body> 你的 HTML <script nonce>你的 JS</script> </body>
</html>
```

`data-device`（`pc` / `mobile`）与 `data-theme`（`light` / `dark`）可直接用于 CSS 选择器；`lang` 是玩家的语言。

## 3. 设备选择

每台服务器的 PC 与手机各自选择标准或自定义模板，互不影响。页面按以下顺序决定设备：

1. 登录 URL 上的 `device=pc|mobile`（推荐：客户端最清楚自己是什么设备）；
2. 请求头 `Sec-CH-UA-Mobile`（`?1` 手机，`?0` PC）；
3. User-Agent：手机为 mobile，平板与桌面为 pc。

## 4. 生命周期

1. 运行时（在你的脚本之前）向宿主发送 `hello`，宿主回复 `init` 并移交 `MessagePort`。
2. 你的脚本调用 `await LoungeLogin.ready()`，得到配置对象（§6）与文案。**其他方法都必须在 `ready()` 之后调用**，否则返回 `not_ready`。
3. 按 `config.viewer` 显示「以 X 继续」或登录表单；调用登录方法（§5），按返回的 `status` 更新界面。
4. 某次调用返回 `{ status: 'done' }`：宿主接管（授权确认、结果页、交回游戏），模板保持「处理中」即可。之后桥只接受 `openLink`，直到宿主发出 `reset` 事件。
5. 宿主发出 `reset`（会话过期、授权被拒、换账号后）：回到登录或「以 X 继续」，`data.viewer` 是当前登录的玩家（或 `null`），`data.message` 是要显示的原因（可能为空）。

## 5. LoungeLogin 接口（协议 v1）

`window.LoungeLogin` 是冻结的只读对象，`LoungeLogin.version === 1`。除 `t` / `on` / `fallback` / `loadScript` 外，方法都返回 Promise。

| 方法 | 参数 | 说明 |
| --- | --- | --- |
| `ready()` | — | 握手并返回配置（§6）；可重复调用，返回同一个 Promise |
| `t(key, params?)` | 文案键、占位参数 | 用户中心的文案（§7），如 `t('cont.as', { name })` |
| `on(name, fn)` | 事件名、回调 | 订阅宿主事件，返回取消订阅的函数；目前只有 `reset` |
| `signInWithPassword(p)` | `{ username, password, remember }` | 密码登录；站点开启了人机验证时，宿主会先弹出验证 |
| `sendCode(p)` | `{ target }` | 向邮箱 / 手机发送登录验证码 |
| `signInWithCode(p)` | `{ target, code, remember }` | 验证码登录 |
| `signInWithPasskey(p)` | `{ remember }` | 通行密钥登录（由宿主调用 WebAuthn） |
| `verifyTwoFactor(p)` | `{ code, rememberDevice }` | 提交两步验证的动态码或恢复码 |
| `twoFactorWithPasskey(p)` | `{ rememberDevice }` | 用通行密钥完成两步验证 |
| `signInWithProvider(p)` | `{ provider }` | 第三方登录：整页跳转，回来后宿主自动继续 |
| `continueAsViewer()` | — | 以当前登录的玩家继续 |
| `switchAccount()` | — | 退出当前玩家，回到登录（返回 `signin`） |
| `cancel()` | — | 取消登录，按回传方式交回 `access_denied`（仅当 `config.cancellable`） |
| `openLink(p)` | `{ name: 'register' \| 'forget' }` | 在新标签页打开注册 / 找回密码 |
| `loadScript(src)` | URL | 加载「脚本来源」里允许的外部脚本，其他来源会被 CSP 拦下 |
| `fallback(reason?)` | 原因 | 放弃自定义模板，换回标准模板 |

**返回值**（正常的业务结果一律 resolve，不会 reject）：

| `status` | 含义 | 附带字段 |
| --- | --- | --- |
| `done` | 已交给用户中心（授权确认 / 结果页 / 第三方跳转） | — |
| `two_factor` | 需要两步验证 | `methods`（如 `['totp','passkey']`）、`trustedDeviceDays`（可勾选「此设备 N 天内不再询问」，0 或缺省表示不提供） |
| `sent` | 验证码已发送 | `cooldown`（秒） |
| `signin` | 回到登录表单 | — |
| `error` | 失败，可重试 | `message`（可直接显示给玩家的文案） |

**拒绝**：只有违反接口约定时才 reject，`Error.code` 为以下之一：`not_ready`（未先调用 `ready()`）、`unknown_method`、`method_disabled`（该服务器 / 站点 / 浏览器不提供这种登录方式）、`provider_not_allowed`、`bad_state`（例如没有待完成的两步验证、没有已登录的玩家、流程已结束）、`busy`（上一次调用还没结束：同一时间只处理一个调用）、`bad_params`、`failed`。

## 6. 配置对象（`ready()` 的结果）

```js
{
  v: 1,
  device: 'pc' | 'mobile',
  locale: 'zh_CN' | 'zh_TW' | 'en' | 'ja',
  theme: 'light' | 'dark',
  siteName: '休息室账号',
  server: { code, name, icon, status, maintenanceMessage? },   // status 2 = 维护中
  client: { name, logoUrl },                                    // 接入应用（游戏）
  branding: { title, subtitle, logo, background, accent },     // 已按玩家语言取好; background 为 #RRGGBB 或图片地址
  methods: ['password', 'code', 'passkey', 'social'],           // 此处真正可用的方式（已扣除站点关闭、浏览器不支持、内嵌时的第三方）
  providers: [{ code, name, icon, color }],                     // 可用的第三方身份源
  viewer: { uid, nickname, avatar } | null,                     // 已登录的玩家; avatar 仅在是本站图片时提供
  redirectHost: 'mygame://',                                    // 显示用: 登录后返回哪里
  handoff: 'redirect' | 'postMessage' | 'code-display',
  cancellable: true,
  registration: true,                                           // 站点是否开放注册
  texts: { … }                                                  // t() 使用的文案
}
```

配置里没有任何密钥、令牌、授权码或回调地址。

## 7. 文案

`LoungeLogin.t(key, params)` 返回用户中心在玩家语言下的文案，`{name}` 形式的占位由 `params` 填入。可用的键：

`loading` `secured`（`{site}`）`returnsTo`（`{host}`）`cancel` `maintenance` `working`、
`signin.title` `signin.password` `signin.code` `signin.username` `signin.passwordPh` `signin.target` `signin.codePh` `signin.remember` `signin.forget` `signin.register` `signin.submit` `signin.passkey` `signin.others` `signin.noMethods` `signin.required`（`{name}`）、
`challenge.title` `challenge.subtitle` `challenge.code` `challenge.remember`（`{days}`）`challenge.verify` `challenge.passkey` `challenge.back`、
`cont.title` `cont.as`（`{name}`）`cont.switch`、`code.send` `code.resend`（`{s}`）。

未知的键原样返回。模板也可以自带文案，按 `config.locale` 选择。

## 8. 安全规则

**运行时的 CSP**（响应头，每次请求生成新的 nonce）：

```
default-src 'none';
script-src 'nonce-…' <脚本来源>;                  没有 'unsafe-inline' / 'unsafe-eval' / 'unsafe-hashes'
style-src 'self' <站点> <资源来源> 'unsafe-inline';
img-src 'self' <站点> <资源来源> <品牌图片的源> data: blob:;
font-src … data:;  media-src …;
connect-src 'none'; form-action 'none'; base-uri 'none'; object-src 'none';
frame-src 'none'; worker-src 'none'; manifest-src 'none';
frame-ancestors 'self' <用户中心的源> [postMessage 模式下游戏页面的源];
sandbox allow-scripts allow-forms
```

由此可知：

- 模板**不能联网**（fetch / XHR / WebSocket / EventSource / sendBeacon 都被拦下），也不能提交表单：表单要监听 `submit` 并 `preventDefault()`，再调用 `LoungeLogin`。
- **内联事件不会执行**（`onclick="…"`、`javascript:` 链接），请用 `addEventListener`。
- 图片、字体、样式只能来自本站、服务器的品牌图片与「资源来源」；外部脚本只能来自「脚本来源」且要用 `loadScript()` 加载。
- 沙箱里没有 `localStorage` / `sessionStorage` / Cookie（访问会抛异常），也不能弹窗、不能导航顶层页面；框架一旦导航到别的文档，页面立即回退到标准模板。

**保存时的检查**（预览、保存、发布与每次线上读取都执行，不合规则的版本不会上线）：

- HTML 不能包含 `<script>` `<iframe>` `<frame>` `<object>` `<embed>` `<applet>` `<base>` `<meta>` `<html>` `<head>` `<body>` `<portal>` `<noscript>` `<plaintext>` 等标签，不能有 `on*=` 内联事件属性，HTML 与 CSS 里不能出现 `javascript:` / `vbscript:`。
- CSS 不能包含 `</style`；JS 不能包含 `<script`、`</script` 或 `<!--`（需要时拆成字符串拼接）。
- 来源只能是 `https://域名[:端口]`（本机开发可用 `http://127.0.0.1:端口` / `http://localhost:端口`），不能带路径、通配符、引号或分号。
- 必须是合法 UTF-8，不能含 NUL；大小见 §2。

## 9. 后台：版本、发布与回滚

- 每次保存都是一个新的**不可变版本**（v1、v2…，每个设备最多保留 50 个，最早的未发布版本会被清理）。
- **发布**让某个版本上线并把该设备切到「自定义」；**回滚**就是发布一个旧版本。已发布的版本不能删除。
- 「标准 / 自定义」开关随时可切：切回标准不会丢掉已发布的版本，再切回自定义就恢复它。
- **预览**用与线上相同的规则渲染草稿（CSP 以 `<meta>` 形式放进沙箱 iframe），桥由后台模拟：用户名 `wrong` 返回密码错误，`2fa` 进入两步验证，其他直接成功；可切换「模拟已登录的玩家」。
- **载入标准模板**得到内置标准模板的 HTML / CSS / JS（接口 `GET /api/admin/sdk-login/template-starter`），可作为起点；**下载**得到一个 JSON 包（`{"format":"lounge-login-template","v":1,"device","html","css","js","scriptOrigins","assetOrigins"}`），**导入**接受这个包，或任意组合的 `.html` / `.css` / `.js` 文件。

后台接口（都在 `/api/admin/sdk-login` 下；读取需 `sdklogin:view`，改动需 `sdklogin:template` 并写操作日志）：

| 方法与路径 | 说明 |
| --- | --- |
| `GET servers/{id}/templates` | PC / 手机两个槽位：模式、已发布版本、是否在线、无法使用的原因、版本列表 |
| `GET servers/{id}/templates/{device}/versions/{vid}` | 一个版本的完整内容 |
| `POST servers/{id}/templates/{device}/preview` | 渲染草稿（不保存） |
| `POST servers/{id}/templates/{device}/versions` | 保存新版本；`"publish": true` 时同时发布 |
| `POST servers/{id}/templates/{device}/versions/{vid}/publish` | 发布 / 回滚 |
| `DELETE servers/{id}/templates/{device}/versions/{vid}` | 删除未发布的版本 |
| `PUT servers/{id}/templates/{device}/mode` | `{"mode":"standard"\|"custom"}` |

## 10. 最小示例

HTML：

```html
<main class="box">
  <h1 id="title"></h1>
  <form id="f" novalidate>
    <input name="username" autocomplete="username">
    <input name="password" type="password" autocomplete="current-password">
    <button type="submit" id="go"></button>
  </form>
  <p id="err" role="alert" hidden></p>
</main>
```

JS：

```js
(async () => {
  const L = window.LoungeLogin;
  const cfg = await L.ready();
  const $ = (id) => document.getElementById(id);
  $('title').textContent = cfg.branding.title || cfg.server.name;
  $('go').textContent = L.t('signin.submit');
  $('f').addEventListener('submit', async (e) => {
    e.preventDefault();
    const f = e.currentTarget.elements;
    const r = await L.signInWithPassword({ username: f.namedItem('username').value, password: f.namedItem('password').value, remember: true });
    $('err').hidden = r.status !== 'error';
    $('err').textContent = r.message || '';
    // two_factor: 显示动态码输入框并调用 L.verifyTwoFactor({ code, rememberDevice })
    // done: 用户中心接管, 不需要做任何事
  });
  L.on('reset', (d) => ($('err').textContent = (d && d.message) || ''));
})();
```

完整的示例（PC 分栏 + 手机底部面板、验证码、通行密钥、第三方、两步验证、「以 X 继续」）见 [`SDK/examples/login-templates/aurora/`](../examples/login-templates/aurora/)；内置标准模板本身可在后台「载入标准模板」后查看。

## 11. 发布前检查

- PC 与手机都要预览，亮色与暗色都要看（`html[data-theme]`）；窄屏（约 360px）与横屏各看一次。
- `config.methods` 里没有的方式不要显示；`config.providers` 为空时隐藏第三方区域；`config.cancellable` 为假时隐藏「取消」。
- 每个输入框都要有可见的标签或 `aria-label`；错误信息放在 `role="alert"` 的元素里；按钮在处理中时要禁用，避免重复提交（桥同一时间只处理一个调用）。
- 不要在模板里写死回调地址、授权码或令牌：它们既拿不到，也用不到。
- 发布后如果「登录模板」区显示「自定义模板无法使用」，玩家看到的是标准模板；按提示修正后重新发布即可。
