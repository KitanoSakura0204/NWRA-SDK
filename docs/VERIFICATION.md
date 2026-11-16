# SDK 1.3.0 验证与交付记录

核对日期：2026-11-16。交付目录为 `D:\Project\Admin\SDK`，本地分支为 `codex/sdk-integration-docs`，起点为 `974deb3f5db1bf5154fdd16c134a8a78921f6260`。以下记录区分实际执行、源码兼容检查与部署外部验证。

## 交付内容与证据对应

| 要求 | 交付 | 证据 |
| --- | --- | --- |
| 六语言 SDK 更新 | 1.3.0 discovery/JWKS 和 nonce-bound 验证，保留旧签名 | 六语言测试、逐任务独立评审、版本一致性检查 |
| 完整游戏服对接 | [游戏服务器指南](GAME_SERVER_INTEGRATION.md)及更新后的 Go 游戏示例 | 指南 Go 程序编译、示例 test/vet、源码契约核对 |
| 完整 OIDC 对接 | [OIDC 指南](OIDC_INTEGRATION.md)及[可运行 RP](../examples/oidc-rp-go/README.md) | 模拟 OP 的完整登录、拒绝、重放、并发和本地会话测试 |
| 完整项目对接 | [项目指南](PROJECT_INTEGRATION.md)、[接口索引](API_REFERENCE.md)、[离线契约](reference/openapi.json) | 后端生成器检查、SDK 快照哈希和内部引用检查 |
| 时间与时区交接 | [时间协议](TIME_HANDLING.md) | 记录此前 Backend/Web 修复及验证边界；本次 SDK 不平移时间戳 |

## 实际测试结果

测试环境是 Windows amd64。SDK 版本声明更新至 1.3.0 后已运行六语言完整现有套件；下表记录最后成功结果。只有命令支持的范围被计为通过。

| 语言 | 实测环境 | 结果 | 最低版本与限制 |
| --- | --- | --- | --- |
| Go | Go 1.26.8 | SDK `go test ./...`、`go vet ./...`；两个 Go 示例同样通过 | 保持 go 1.21；1.21.13 下载遇到 EOF，未在 1.21 实跑 |
| Python | Python 3.14.8、cryptography 50.0.2、pytest 9.1.1 | pytest 146 tests＋242 subtests；修复阶段 unittest 63 项，无跳过 | Python 3.9 grammar 检查通过，未运行 3.9 解释器 |
| TypeScript | Node 24.21.0；另实跑 Node 18.20.8 | TypeScript build、88/88 测试；导出声明消费检查通过 | Node 18 使用 `--experimental-global-webcrypto`；X25519 有实验功能提示 |
| Java | JDK 25.0.4.1，`--release 17`；修复后另实跑 JDK 17.0.20.1 | SelfCheck 通过；EncryptedTransport 109、GameData 27、OIDC 152 项通过 | 实际 JDK 17 已执行；未运行 Maven、Paper 或 Velocity 构建 |
| C# | .NET SDK 10.0.402，实际 runtime 6.0.36 | net6.0 Release 构建；OIDC 8 组、GameData 12 组通过 | 未隐藏警告：构建有 439 个 CS1591 XML 文档警告，console 项目有 NETSDK1138 提示；0 构建错误 |
| PHP | PHP 8.4.27，curl/openssl/sodium 已启用 | 基础 36 项、GameData 12 组、OIDC 14 组通过；30 文件 lint 通过 | PHP 8.1 语法兼容已审查，未在 8.1 实跑；便携运行时未全局安装 |

新增测试覆盖固定 issuer 路径、无认证公共请求、issuer 与文档结构、错误状态、严格 JSON/UTF-8、1 MiB 边界、nonce 错误及 JWE 内层验签。各语言的 transport 注入、流式读取和取消能力不同，见对应 README；自定义适配器不能依赖 SDK 替它取消已经发生的外部副作用或限制已经物化的下载。

## 重现命令

以下是已执行检查的可重现形式。从 SDK 根目录打开 PowerShell；先准备上表中的工具及语言 README 列明的开发依赖。命令需要逐条检查退出码。Python 测试需要 pytest 和可选 cryptography，TypeScript 构建需要本地 TypeScript 编译器。

```powershell
Push-Location go
go test ./...
go vet ./...
Pop-Location

Push-Location examples/game-server-go
go test ./...
go vet ./...
Pop-Location

Push-Location examples/oidc-rp-go
go test ./...
go vet ./...
Pop-Location

Push-Location python
$previousSdkPythonPath = $env:PYTHONPATH
try {
    $env:PYTHONPATH = (Get-Location).Path
    python -m unittest discover -s tests -q
    python -m pytest tests -q
} finally {
    $env:PYTHONPATH = $previousSdkPythonPath
    Pop-Location
}

Push-Location typescript
npm run build
$sdkTestFiles = @(Get-ChildItem tests -Filter '*.test.mjs' | ForEach-Object FullName)
node --experimental-global-webcrypto --test $sdkTestFiles
Pop-Location
```

Java 在 SDK 根目录用 JDK 17 或更高版本执行，以下输出目录属于忽略的构建产物：

```powershell
$sdkJavaSources = @(Get-ChildItem java/src/main/java/cyou/nowork/sdk -Filter '*.java' | ForEach-Object FullName) + @(Get-ChildItem java/src/test/java/cyou/nowork/sdk -Filter '*.java' | ForEach-Object FullName)
New-Item -ItemType Directory -Force java/target/verification | Out-Null
javac --release 17 -encoding UTF-8 -d java/target/verification $sdkJavaSources
java -cp java/target/verification cyou.nowork.sdk.SelfCheck
java -cp java/target/verification cyou.nowork.sdk.EncryptedTransportTest
java -cp java/target/verification cyou.nowork.sdk.GameDataClientTest
java -cp java/target/verification cyou.nowork.sdk.OidcTest

dotnet build csharp/Nowork.Sdk/Nowork.Sdk.csproj -c Release
dotnet run --project csharp/Nowork.Sdk.Oidc.Tests/Nowork.Sdk.Oidc.Tests.csproj -c Release
dotnet run --project csharp/Nowork.Sdk.GameData.Tests/Nowork.Sdk.GameData.Tests.csproj -c Release

php php/tests/run.php
php php/tests/game_data.php
php php/tests/oidc_metadata_nonce.php
Get-ChildItem php/src,php/tests -Recurse -Filter '*.php' | ForEach-Object {
    php -l $_.FullName
    if ($LASTEXITCODE -ne 0) { throw 'PHP lint failed' }
}
```

本机 PHP 不在 PATH，实际使用经过 SHA-256 校验的便携 `php.exe`。上面的 `php` 命令适用于已配置 PATH 的环境；没有依赖运行时下载链接才能阅读或使用本交付。

## 示例验证边界

Go RP 测试使用 loopback 模拟 OP 和临时 RSA 密钥，包含成功登录、PKCE、Cookie/state/nonce/issuer/audience/signature/sub 错误、到期、重复回调、八个并发回调只交换一次、会话容量与 TTL、HTML 转义、CSRF 和退出。

已实际执行 RP 的 `go run .`，其未登录首页返回 HTTP 200 和 no-store；检查结束后进程已停止。游戏示例的 `go run . login` 对故意不可达的 loopback issuer 正确返回 discovery 失败，成功流程由模拟 OP 测试覆盖。没有用真实外部账号完成登录。

RP 仅实现 RS256 签名 ID Token 和普通 JSON UserInfo；query/GET 回调、单进程内存会话、固定 loopback 监听。刷新管理、远程单点退出、JWE 配置和多实例共享存储不在示例实现内，不能把指南中的全协议范围等同于示例已实现的能力。

游戏指南中的完整 Go 程序已提取到独立临时 module，通过 test/vet 编译检查。没有把带环境占位符的 HTTP 示例当作真实服务器成功响应。`CGO_ENABLED=0`，未运行 Go race detector。

## 文档与接口校验

后端来源是 `Backend/internal/modules/devdocs/openapi_full.json`。当前快照包含 1,629 个操作、1,335 个路径、1,368 个 schema；1,623 个操作有当前源码注册位置，另外 6 个是保留文档的弃用接口。

SHA-256：`e8acd4d0d99814103840dc2497d997cfcc092b41df647b006944d1bbd2c6e9ea`。机器可读记录见 [manifest](reference/manifest.json)。索引引用完整快照；阅读无需父工程或网络。

```powershell
python ../Backend/scripts/gen_openapi.py --check
python scripts/generate_integration_reference.py --check
python scripts/check_docs.py
python scripts/check_release.py
git diff --check
```

后端检查通过，并输出既有未引用 schema 提示；SDK 检查验证快照与源文件同字节、索引、JSON 内部引用、鉴权方案及版本。文档脚本检查本地文件链接和 UTF-8 替换字符，不声称验证外站可用性或所有 Markdown fragment。独立 SDK checkout 缺少同级 Backend 时，生成器明确报告仅检查本地快照一致性。

本次还将生成器、索引和快照复制到没有同级 Backend 的独立目录，实际通过 standalone 检查。文档检查通过 25 份文件、178 个本地链接；版本检查通过 11 处核心声明、manifest 和 README 标题。

## 实现取舍

| 决定 | 原因 | 后续成本或边界 |
| --- | --- | --- |
| 直接使用 SDK 的独立本地分支 | 该仓库起始干净；父工程已有用户改动 | 未另外复制工作树，协作需保持文件所有权 |
| 文档使用现有编辑工具与源码核对 | 普通 Markdown 不依赖未安装的通用写作工具 | 未执行不存在的辅助工具，但已完成本地链接和契约检查 |
| 不进行推送、发布或生产认证 | 用户要求本地 SDK 更新和文档交付 | 仍需后续部署及真实站点联调 |
| Java/C# 加密 nonce API 使用独立 WithNonce 名称 | 避免旧私钥可变参数或 null 参数调用歧义 | 新调用者采用新名称，旧调用保留 |
| PHP 默认 cURL 增加有界读取，保留旧五参数适配器 | 兼顾实际下载上限与自定义 transport 兼容 | 已物化响应的自定义适配器自行限制下载 |
| 同时更新旧游戏登录示例的验签与 nonce | 防止文档继续指向只读取 UserInfo 的登录参考 | 示例要求签名 ID Token 与匹配的 UserInfo.sub |
| 新交接只放在 SDK 内，保留本地分支 | 遵循用户最后指定的交付目录 | 父工程旧交接不自动包含本次版本；后续发布另行执行 |

## 交付和后续使用

源码、三份主指南、专题协议、接口快照、示例和验证脚本都在 SDK 目录。本机详细日志和评审记录归档在忽略的 `artifacts/sdk-1.3.0/`，不作为运行依赖或发布包内容；可提交的本文件记录了结果和重现方式。

此次没有推送分支、发布 npm/PyPI/Maven/NuGet/Composer 包、部署或重启正式服务。真实站点的 TLS/反向代理、应用审核与回调登记、支付平台、消息渠道、游戏引擎、移动推送和生产时钟状态仍需在对应部署中验证。

既有时间修复的 Backend/Web 更新要求见 [时间协议](TIME_HANDLING.md)。SDK 的本地版本更新不修改父项目用户改动，也不把文档生成或模拟测试视为生产上线。
