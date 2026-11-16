# SDK 文档导航

## 完整接入指南

1. [游戏服务器完整对接](GAME_SERVER_INTEGRATION.md)：创建和审核、凭据、进服、心跳、GM、玩家数据、封禁、客户端登录、推送、重试和验收。
2. [OIDC 完整对接](OIDC_INTEGRATION.md)：应用注册、发现、公钥、PKCE/state/nonce、令牌校验、本地会话、刷新、退出和原生客户端。
3. [项目对接总指南](PROJECT_INTEGRATION.md)：部署与租户、全部凭据边界、账号、自动化、支付、App、联邦、机器人、插件、隐私和运维。
4. [全接口索引](API_REFERENCE.md)：所有操作及分组；[OpenAPI 快照](reference/openapi.json)包含全部字段／请求／响应／schema，[manifest](reference/manifest.json)记录来源和校验值。
5. [验证记录](VERIFICATION.md)：本次实际测试、环境版本、限制和交付信息。
6. [时间与浏览器时区](TIME_HANDLING.md)：请求级时区转换、时间戳规则、历史数据和更新要求。

## 专题协议

| 文档 | 内容 |
| --- | --- |
| [PROTOCOL](PROTOCOL.md) | Server API、OIDC、Webhook、SDK 登录页的底层约定 |
| [SERVER_USERS](SERVER_USERS.md) | 玩家 schema、读写、version 冲突、封禁与踢出 |
| [GM_SSO](GM_SSO.md) | 一次性角色 GM 登录；区别于 OIDC 与 GM 指令 |
| [DELIVERY](DELIVERY.md) | 字段映射、pairwise、JWE、密钥轮换、应用权益 |
| [ACCOUNT_API](ACCOUNT_API.md) | 用户会话下的玩家门户、申请和角色绑定 |
| [CHAT](CHAT.md) | Chat 会话、游戏中继、ACK、消息幂等与 WebSocket |
| [BOTS_SDK](BOTS_SDK.md) | Bot 认证、框架插件、通知推送和确认 |
| [LOGIN_TEMPLATES](LOGIN_TEMPLATES.md) | 自定义登录页模板、沙箱桥接、四语状态更新 |

## 工具与示例

- [SDK 语言／能力矩阵](../README.md)
- [可运行示例目录](../examples/README.md)
- [插件清单](../plugins/MANIFEST.md)、[ABI](../plugins/ABI.md)、[发布与撤回](../plugins/PUBLISHING.md)
- [浏览器 login-kit](../login-kit/README.md)

新项目先选一条主指南走通最小链路，再按实际能力打开专题协议；不要把未知接口、额外 scope 或不同类别的凭据混在一起试用。
