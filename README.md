# NoWork Account SDK 1.3.0

游戏服务器、网站、启动器和外部系统接入 NoWork 用户中心的 SDK 与完整对接文档。六语言核心包含 Server API、OIDC、Webhook 和数据投递；门户、聊天、游戏资料、机器人与插件扩展按下表选用。

本次为源代码中的 1.3.0 更新，未宣称已经发布到 npm、PyPI、Maven、NuGet 或 Composer。先按[六语言本地接入](docs/LOCAL_SETUP.md)在独立工程引用这份源码；只有确认仓库或包仓库存在该版本后才按远程版本安装。

## 从这里开始

| 你要做什么 | 完整指南 |
| --- | --- |
| 游戏服务器接入：审核、白名单、心跳、GM、数据、封禁、推送 | [游戏服务器完整对接](docs/GAME_SERVER_INTEGRATION.md) |
| 网站／启动器登录：注册、PKCE、state/nonce、验签、会话、刷新、退出 | [OIDC 完整对接](docs/OIDC_INTEGRATION.md) |
| 整个项目集成：站点、凭据、账号、支付、App、机器人、插件、运维 | [项目对接总指南](docs/PROJECT_INTEGRATION.md) |
| 查询任意接口、参数、响应和数据结构 | [全接口索引](docs/API_REFERENCE.md)／[离线 OpenAPI](docs/reference/openapi.json) |
| 查看实现、示例和测试范围 | [文档目录](docs/README.md)／[示例目录](examples/README.md)／[验证记录](docs/VERIFICATION.md) |

## 语言与运行条件

| 语言 | 目录 | 最低运行版本 | 核心 HTTP／加密要求 |
| --- | --- | --- | --- |
| Go | [go](go/README.md) | Go 1.21 | 标准库 |
| Java | [java](java/README.md) | Java 17 | 标准库 HTTP／JSON／加密实现 |
| Python | [python](python/README.md) | Python 3.9 | HTTP 为标准库；签名／JWE 需要可选 cryptography |
| TypeScript | [typescript](typescript/README.md) | Node 18／具有相应 Web API 的环境 | fetch；加密使用 Web Crypto，Node 18 按 README 启用支持 |
| C# | [csharp](csharp/README.md) | .NET 6 | HttpClient、System.Text.Json、平台加密；不支持 X25519 |
| PHP | [php](php/README.md) | PHP 8.1 | curl、json；加密按算法需要 OpenSSL／sodium |

这些是 SDK 的运行条件；Backend 和 Web 工程的构建要求不同，见项目对接指南。

## 能力矩阵

| 能力 | SDK 封装范围 | 详细契约 |
| --- | --- | --- |
| 白名单、加入校验、心跳和模块配置 | 六语言 | [PROTOCOL](docs/PROTOCOL.md) |
| GM 动作／选项发布、指令拉取和回执 | 六语言 | [游戏服指南](docs/GAME_SERVER_INTEGRATION.md) |
| 玩家数据、乐观锁和服务器封禁 | 六语言 | [SERVER_USERS](docs/SERVER_USERS.md) |
| 独立角色 GM SSO code 兑换 | 六语言；不自动授予 GM 权限 | [GM_SSO](docs/GM_SSO.md) |
| 游戏聊天票据／中继／回执／上报 | 六语言 Server 客户端 | [CHAT](docs/CHAT.md) |
| OIDC 授权码、刷新、UserInfo、内省、撤销和退出 URL | 六语言；默认机密客户端使用 client_secret_post | [OIDC 指南](docs/OIDC_INTEGRATION.md) |
| OIDC discovery、JWKS、nonce-bound ID Token 验证 | 六语言，1.3.0 新增 | [OIDC 指南](docs/OIDC_INTEGRATION.md) |
| Webhook 签名和 JWE 投递 | 六语言，算法／依赖存在差异 | [DELIVERY](docs/DELIVERY.md) |
| GameDataClient（米游社／森空岛资料、档案和导出） | 六语言；角色／区域／权限以实际返回为准 | 各语言 README 与 [全接口参考](docs/API_REFERENCE.md) |
| 用户门户 AccountClient、聊天 REST Client | Go、Python、TypeScript；其余按 HTTP 契约接入 | [ACCOUNT_API](docs/ACCOUNT_API.md)、[CHAT](docs/CHAT.md) |
| SDK 品牌登录页 URL／回调便利助手 | Go、Python、TypeScript；另有浏览器 login-kit | [登录页协议](docs/PROTOCOL.md)、[login-kit](login-kit/README.md) |
| 应用权益读写便利助手 | Go、Python；其他语言可直接使用对应 HTTP 契约 | [DELIVERY](docs/DELIVERY.md) |
| 框架机器人 | 独立 Python／TypeScript 共享客户端及 NoneBot2、Koishi、Wechaty | [BOTS_SDK](docs/BOTS_SDK.md) |
| WASM／声明式／外部插件与发布工具 | 独立插件 SDK 和 ABI | [plugins](plugins/README.md) |

协议能力与某种语言是否有便利方法是两件事。普通 OIDC 登录、品牌 SDK 登录页、角色 GM 登录、用户中心会话和游戏服务器密钥不能互换。

## 跑通本地示例

以下从 SDK 目录执行，不需要真实凭据即可运行模拟测试：

```powershell
Push-Location go
go test ./...
go vet ./...
Pop-Location

Push-Location examples/game-server-go
go test ./...
Pop-Location

Push-Location examples/oidc-rp-go
go test ./...
Pop-Location
```

连接真实站点时再按对应示例 README 注入自己的 issuer、服务器密钥或 OAuth 客户端配置。游戏服使用 [game-server-go](examples/game-server-go/README.md)；网站登录使用 [oidc-rp-go](examples/oidc-rp-go/README.md)，该示例包含 PKCE、浏览器绑定、nonce 验证、UserInfo subject 检查和本地会话。

### 调用时的几个关键约定

`CheckWhitelist` 必须同时处理 error 和 allowed；`allowed=true` 不保证 player 非空。Go 的 `AuthorizeURL` 返回单个 string，不是 `(string, error)`。登录回调使用 nonce-bound 验签并完成 UserInfo subject 检查后才能创建自己的会话。

时间戳和签名不按浏览器时区加减；密钥、授权码、verifier 和完整令牌不写日志。完整代码、错误处理和撤销规则见对应指南与示例。

## 目录

```text
SDK/
  docs/             三套主指南、完整参考快照、专题协议及验证记录
  go/               Go module: github.com/wangework/nowork-sdk-go
  java/             Maven coordinates: cyou.nowork:nowork-sdk
  python/           Python distribution: nowork-sdk
  typescript/       npm package: @nowork/sdk
  csharp/           NuGet package: NoWork.Sdk
  php/              Composer package: nowork/sdk
  examples/         游戏服、OIDC RP、Webhook、聊天与平台适配示例
  bots/             机器人专用客户端和框架插件
  plugins/          插件 ABI、示例、打包与签名工具
  login-kit/        浏览器 SDK 登录入口组件
  scripts/          接口快照生成和文档校验
```

## 更新与验证

核心六语言库采用语义化版本，新增 API 保留旧调用。机器人、login-kit 和插件协议各自维护版本，本次不机械同步它们的版本号。变化和兼容细节见 [CHANGELOG](CHANGELOG.md)。

```powershell
python scripts/generate_integration_reference.py --check
python scripts/check_docs.py
python scripts/check_release.py
```

完整 OpenAPI 快照包含管理契约，供工程人员离线阅读；发布 Web 静态站点时继续使用项目专门生成的 public subset。当前执行结果、运行时下限验证差异和真实部署未验证项见 [VERIFICATION](docs/VERIFICATION.md)。
