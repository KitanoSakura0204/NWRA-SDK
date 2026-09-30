# Lounge Account SDK

游戏服务器 / 第三方应用接入「休息室账号」用户中心的官方 SDK，覆盖两类能力：

| 能力 | 说明 | 鉴权 |
| --- | --- | --- |
| **Server API** | 白名单全量拉取、玩家进服校验、心跳上报 | `X-Server-Key`（后台“服务器管理”生成） |
| **OIDC** | 授权码登录、换取令牌、UserInfo、令牌内省 | `client_id` / `client_secret`（后台“接入应用”生成） |

| 语言 | 目录 | 最低版本 | 依赖 |
| --- | --- | --- | --- |
| Go | [`go/`](go/) | Go 1.21 | 无（标准库） |
| Java | [`java/`](java/) | Java 17 | 无（`java.net.http` + 内置 JSON 解析） |
| Python | [`python/`](python/) | Python 3.9 | 无（`urllib`） |
| TypeScript / Node.js | [`typescript/`](typescript/) | Node 18 | 无（全局 `fetch`） |
| C# / .NET | [`csharp/`](csharp/) | .NET 6 | 无（`System.Net.Http` + `System.Text.Json`） |
| PHP | [`php/`](php/) | PHP 8.1 | 无（`ext-curl` + `ext-json`） |

所有 SDK 行为一致：方法名、返回结构、错误类型都按同一份 [协议文档](docs/PROTOCOL.md) 实现，便于多语言服务端并存。

## 快速开始（以 Go 为例）

```go
client := lounge.New("https://account.example.com", lounge.WithServerKey("lsk_xxx"))

// 玩家进服校验
res, err := client.CheckWhitelist(ctx, "Steve", "069a79f4-44e9-4726-a5be-fca90e38aaf5")
if err == nil && res.Allowed { /* 放行 */ }

// 定期心跳
client.Heartbeat(ctx, lounge.Heartbeat{Online: 12, Max: 50, Version: "1.21.1"})

// OIDC（网站 / 启动器登录）
oidc := lounge.NewOIDC("https://account.example.com", "lc_xxx", "secret", "https://yourapp.com/callback")
url, _ := oidc.AuthorizeURL(state, nonce, "openid profile email game_roles")
tok, _ := oidc.Exchange(ctx, code)
user, _ := oidc.UserInfo(ctx, tok.AccessToken)
```

## 目录

```
SDK/
├── docs/PROTOCOL.md      协议与字段说明（所有语言共同遵循）
├── go/                   Go module: github.com/wangework/lounge-sdk-go
├── java/                 Maven: cyou.lounge:lounge-sdk
├── python/               PyPI: lounge-sdk
├── typescript/           npm: @lounge/sdk
├── csharp/               NuGet: Lounge.Sdk
├── php/                  Composer: lounge/sdk
└── examples/             示例：游戏服务器、Paper 插件、Webhook 接收端、Discord 身份组同步、登录页模板（见 examples/README.md）
```

## 版本

SDK 遵循语义化版本，与后端 Server API `v1` 对应；破坏性变更会在 [CHANGELOG](CHANGELOG.md) 中说明。
