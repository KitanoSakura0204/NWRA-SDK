# 从本地源码接入 SDK 1.3.0

这次交付是源码版本，不代表六语言包已发布。以下六个独立消费工程直接使用同一份 SDK checkout；无需账号、密钥或外部 OIDC 服务。远程安装命令仅在确认包仓库已经发布所需版本后使用，不能保证未指定版本的命令获得本次新增 API。

以下命令使用 PowerShell。从 SDK 根目录开始，先设置绝对路径；每节可单独执行。示例产物放在本地忽略目录中，实际应用可以换成自己的工程路径。

```powershell
$sdkRoot = (Get-Location).Path
$consumerRoot = Join-Path $sdkRoot 'artifacts/sdk-1.3.0/local-consumers'
New-Item -ItemType Directory -Force $consumerRoot | Out-Null
```

工具下限保持 Go 1.21、Python 3.9、Node 18、Java 17、.NET 6、PHP 8.1。PHP 启用 curl/json；签名及加密的可选扩展见语言 README。TypeScript 需要本地开发依赖中的 `tsc`；.NET 需要已安装 net6.0 targeting pack 和 runtime。这里的离线命令复用已安装的工具与依赖，不会下载缺少的编译器或运行时。

烟雾示例调用 1.3.0 的 JWKS 方法，故意配置带查询串的无效 issuer，确认本地导入与拒绝行为；这不是登录示例，不会请求 provider。真实接入改用无凭据、无查询串和片段的 HTTP(S) issuer，并按 [OIDC 指南](OIDC_INTEGRATION.md) 校验 discovery、签名、state/nonce 和会话。

## Go：本地 require / replace

```powershell
New-Item -ItemType Directory -Force "$consumerRoot/go" | Out-Null
Set-Location "$consumerRoot/go"
go mod init example.local/sdk-consumer
go mod edit '-go=1.21'
go mod edit '-require=github.com/wangework/nowork-sdk-go@v1.3.0'
go mod edit "-replace=github.com/wangework/nowork-sdk-go=$sdkRoot/go"
```

将以下内容保存为该目录的 `main.go`。`replace` 使 `v1.3.0` 从本地目录解析，不需要远程 tag。

```go
package main

import (
    "context"
    "errors"
    "fmt"
    nowork "github.com/wangework/nowork-sdk-go"
)

func main() {
    if nowork.Version != "1.3.0" { panic("unexpected SDK version") }
    oidc := nowork.NewOIDC("https://account.example?smoke=1", "local", "", "")
    _, err := oidc.JWKS(context.Background())
    var transport *nowork.TransportError
    if !errors.As(err, &transport) { panic("expected invalid issuer rejection") }
    fmt.Println("Go local SDK 1.3.0 OK")
}
```

```powershell
go run .
Set-Location $sdkRoot
```

## Python：直接使用本地包路径

创建 `$consumerRoot/python`，将以下内容保存为该目录的 `consumer.py`：

```python
from nowork_sdk import NoworkOIDC, TransportError, __version__

assert __version__ == "1.3.0"
try:
    NoworkOIDC("https://account.example?smoke=1", "local").jwks()
except TransportError:
    print("Python local SDK 1.3.0 OK")
else:
    raise AssertionError("expected invalid issuer rejection")
```

```powershell
Set-Location "$consumerRoot/python"
$previousSdkPythonPath = $env:PYTHONPATH
try {
    $env:PYTHONPATH = "$sdkRoot/python"
    python consumer.py
} finally {
    $env:PYTHONPATH = $previousSdkPythonPath
    Set-Location $sdkRoot
}
```

这条路径无需 pip 或构建依赖。希望使用 editable install 时，可在自己的虚拟环境中执行 `python -m pip install --no-deps --no-build-isolation -e "$sdkRoot/python"`；这条可选路线需该环境已有 pip/setuptools，没有它们时使用上面的已验证路径即可。discovery/JWKS 不需要可选的 cryptography。

## TypeScript：构建本地包，再按包名导入

```powershell
Set-Location "$sdkRoot/typescript"
npm run build
New-Item -ItemType Directory -Force "$consumerRoot/typescript" | Out-Null
Set-Location "$consumerRoot/typescript"
npm init -y
npm pkg set type=module
npm install --offline --ignore-scripts --no-audit --no-fund "$sdkRoot/typescript"
```

将以下内容保存为该目录的 `consumer.ts`。从 `@nowork/sdk` 导入会同时验证包入口与导出的类型声明。

```ts
import { NoworkOIDC, SDK_VERSION, TransportError } from '@nowork/sdk';

async function main(): Promise<void> {
  if (SDK_VERSION !== '1.3.0') throw new Error('unexpected SDK version');
  try {
    await new NoworkOIDC('https://account.example?smoke=1', 'local').jwks();
    throw new Error('expected invalid issuer rejection');
  } catch (error) {
    if (!(error instanceof TransportError)) throw error;
  }
  console.log('TypeScript local SDK 1.3.0 OK');
}
void main().catch(error => { throw error; });
```

```powershell
& "$sdkRoot/typescript/node_modules/.bin/tsc.cmd" --strict --target ES2020 --module NodeNext --moduleResolution NodeNext --outDir out consumer.ts
node --experimental-global-webcrypto out/consumer.js
Set-Location $sdkRoot
```

Node 18 的加密调用需要上面的 Web Crypto 开关。本地包是目录引用，修改 SDK 源码后须重新 `npm run build`。若需要准备 TypeScript 开发依赖，在 SDK 的 `typescript` 目录按 `package.json` 安装；离线环境需事先缓存依赖。本节不把 `npm install @nowork/sdk` 当作本地 1.3.0 的来源。

## Java：javac / jar，无需 Maven

```powershell
Set-Location $sdkRoot
New-Item -ItemType Directory -Force "$consumerRoot/java/sdk-classes" | Out-Null
$sdkJavaSources = @(Get-ChildItem "$sdkRoot/java/src/main/java/cyou/nowork/sdk" -Filter '*.java' | ForEach-Object FullName)
javac --release 17 -encoding UTF-8 -d "$consumerRoot/java/sdk-classes" $sdkJavaSources
jar --create --file "$consumerRoot/java/nowork-sdk-1.3.0.jar" -C "$consumerRoot/java/sdk-classes" .
Set-Location "$consumerRoot/java"
```

将以下内容保存为该目录的 `Consumer.java`：

```java
import cyou.nowork.sdk.*;

public class Consumer {
    public static void main(String[] args) throws Exception {
        if (!"1.3.0".equals(NoworkClient.VERSION)) throw new AssertionError("unexpected SDK version");
        try {
            new NoworkOidc("https://account.example?smoke=1", "local", "", "").jwks();
            throw new AssertionError("expected invalid issuer rejection");
        } catch (NoWorkException.Transport expected) {
            System.out.println("Java local SDK 1.3.0 OK");
        }
    }
}
```

```powershell
javac --release 17 -encoding UTF-8 -cp nowork-sdk-1.3.0.jar -d classes Consumer.java
java -cp 'classes;nowork-sdk-1.3.0.jar' Consumer
Set-Location $sdkRoot
```

这里的分号是 Windows classpath 分隔符；其他系统使用冒号。jar 路线只需要 JDK，不依赖 Maven 或远程仓库。Java README 中的 Maven 坐标只有在目标仓库已有该版本时才可直接解析。

## C#：独立工程 ProjectReference

创建 `$consumerRoot/csharp`，将以下内容保存为该目录的 `Consumer.csproj`：

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net6.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
  </PropertyGroup>
</Project>
```

在同一目录保存 `Program.cs`：

```csharp
using NoWork.Sdk;

if (typeof(NoworkOidc).Assembly.GetName().Version?.ToString() != "1.3.0.0")
    throw new Exception("unexpected SDK version");
try
{
    await new NoworkOidc("https://account.example?smoke=1", "local").JwksAsync();
    throw new Exception("expected invalid issuer rejection");
}
catch (ArgumentException)
{
    Console.WriteLine("C# local SDK 1.3.0 OK");
}
```

```powershell
Set-Location "$consumerRoot/csharp"
dotnet add Consumer.csproj reference "$sdkRoot/csharp/Nowork.Sdk/Nowork.Sdk.csproj"
# 空源用于确认没有依赖远程 NuGet；需要预装 net6.0 targeting pack/runtime。
New-Item -ItemType Directory -Force empty-nuget-source | Out-Null
dotnet restore Consumer.csproj --source "$consumerRoot/csharp/empty-nuget-source" -p:NuGetAudit=false
dotnet run --project Consumer.csproj -c Release --no-restore
Set-Location $sdkRoot
```

这会实际构建被引用的 SDK。当前构建的 XML 文档 CS1591 及 .NET 6 NETSDK1138 警告仍存在，见 [验证记录](VERIFICATION.md)；不把它们隐藏或计为零警告。

## PHP：本地 PSR-4 autoload，无需 Composer

创建 `$consumerRoot/php`，将以下内容保存为该目录的 `consumer.php`：

```php
<?php
declare(strict_types=1);

$sdkSource = $argv[1];
spl_autoload_register(static function (string $class) use ($sdkSource): void {
    $prefix = 'NoWork\\Sdk\\';
    if (str_starts_with($class, $prefix)) {
        $file = $sdkSource . '/' . str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
        if (is_file($file)) require $file;
    }
});
if (NoWork\Sdk\NoworkClient::VERSION !== '1.3.0') throw new RuntimeException('unexpected SDK version');
try {
    (new NoWork\Sdk\NoworkOidc('https://account.example?smoke=1', 'local'))->jwks();
    throw new RuntimeException('expected invalid issuer rejection');
} catch (NoWork\Sdk\Exceptions\TransportError $expected) {
    echo "PHP local SDK 1.3.0 OK\n";
}
```

```powershell
Set-Location "$consumerRoot/php"
php consumer.php "$sdkRoot/php/src"
Set-Location $sdkRoot
```

autoload 映射与 SDK `composer.json` 中的 `NoWork\Sdk\` → `src/` 一致。此路线无需 Composer，也不会引用远程同名旧包；已有 Composer 的项目可以自行配置本地 path repository。请使用自己安装且扩展齐全的 PHP；本次验证使用 SDK 内便携 PHP，实际运行范围见验证记录。
