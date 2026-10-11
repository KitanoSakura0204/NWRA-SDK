# 角色 GM 登录接入示例

这是一个可运行的 GM 服务端接入示例：用户中心选择已绑定角色 → GM `/start` 生成浏览器会话、PKCE 和 state → 用户中心授权 → GM `/callback` 检查会话并用服务器密钥兑换 → `/me` 读取短期角色会话。不需要玩家再次输入密码。

在服务器的独立 GM 模块配置中登记同源地址：`https://gm.example/start` 和 `https://gm.example/callback`。设置环境变量 `NOWORK_BASE_URL`、`NOWORK_SERVER_ID`、`NOWORK_SERVER_KEY`、`GM_PUBLIC_URL=https://gm.example`，让 Python 能找到仓库中的 `SDK/python`（设置 `PYTHONPATH` 或在自己的环境安装该本地包），运行 `python server.py`。默认只监听 `127.0.0.1:8088`，由 HTTPS 反向代理转发。仅本地开发允许显式设置 `GM_ALLOW_LOOPBACK_HTTP=1` 并使用 loopback HTTP；中心后端也必须允许该开发配置。

密钥和 verifier 只留在服务端。一次性流程会话存活 180 秒，角色会话存活 15 分钟；Cookie 为 HttpOnly、SameSite=Lax，HTTPS 时附带 Secure。日志不记录含授权码的 URL，代理也应为 `/start` 与 `/callback` 关闭查询字符串日志。回调成功后跳到干净的 `/me`，失败需从用户中心重新发起，不能重试已消费的授权码。

`scope=character:login` 只证明角色身份，不提供 GM 管理权限。真实 GM 应按返回的 `subject` 与 `serverId` 建立自己的权限规则；本示例只展示身份。示例使用有限内存会话，重启即退出；多进程部署请换成共享、有到期时间并支持原子消费的会话存储。

## English

Runnable standard-library relying party for character SSO. Configure matching HTTPS `/start` and `/callback` URLs in the server GM module, set the four environment variables above, put `SDK/python` on `PYTHONPATH`, and run `python server.py` behind an HTTPS reverse proxy.

The flow binds state and PKCE to an HttpOnly browser cookie, exchanges the single-use code on the backend, then rotates into a 15-minute local session. It never sends the server key or verifier through the browser. Character login conveys identity only; implement your own authorization rules. Replace the bounded in-memory store with atomic shared storage for multiple workers. No production provider calls are needed for the unit tests.
