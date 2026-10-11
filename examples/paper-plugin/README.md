# Paper 插件骨架（Java）

> English version below.

在 Paper 1.21 服务器上接入休息室账号的最小插件，直接编译 Java SDK 源码（`../../java/src/main/java`，无第三方依赖）：

- `AsyncPlayerPreLoginEvent`（异步线程）实时校验白名单；未通过时以 `KICK_WHITELIST` 拒绝并显示原因。
- 用户中心不可达时按 `outage-policy` 降级：`cache` 放行 24 小时内实时校验通过的玩家，`deny` 全部拒绝，`allow` 全部放行。业务错误（密钥无效、服务器停用）与无法打开的加密响应一律拒绝。
- 心跳：主线程读取在线玩家、最大人数、版本与 1 分钟平均 TPS，异步上报（PROTOCOL §2.3）。
- GM 面板：启用 `server-gm` 模块时发布 `kick` / `broadcast` / `time` 三个类型化操作与世界列表 `worlds`，异步拉取命令、主线程执行、异步回执（PROTOCOL §2.4）。

Java SDK 已支持通过显式配置的私钥自动解密 Server API 信封（[Java README](../../java/README.md)、DELIVERY.md §5）。这个最小示例默认使用不配置私钥的构造器，因此服务器开启加密后会抛 `NoWorkException.Encryption`，插件拒绝加入。需要加密时，在 `onEnable` 的客户端构造处使用 `new NoworkClient(base, key, timeout, serverCode, currentKey, previousKey)`；传后台的服务器代码和本地读取的私有 JWK，单把钥匙时省略 `previousKey`。示例的 `config.yml` 没有私钥字段，不会自动读取或上传私钥。

配置密钥后，SDK 核对受保护的服务器代码和请求路由，并拒绝白名单明文降级；正常心跳、模块与 GM 回复仍可为明文。错误密钥、上下文不匹配或损坏的加密回复都必须拒绝，不能进入 `outage-policy`。

## 构建

需要 JDK 21 与 Gradle 8.5 以上：

```bash
gradle build        # 生成 build/libs/nowork-paper-plugin-1.0.0.jar，放入服务器 plugins/
```

构建声明为 `io.papermc.paper:paper-api:1.21.1-R0.1-SNAPSHOT`（`compileOnly`），仓库为 Maven Central 与 PaperMC。仓库不附 Gradle wrapper 或 Paper API JAR；首次解析依赖需要取得这些构建依赖，完整缓存后可用 `gradle --offline build`。插件 JAR 包含本仓库 Java SDK 源码编译产物，Paper/Adventure 等运行时 API 由 Paper 提供。

首次启动生成 `plugins/NoWorkAccount/config.yml`，填写 `base` 与 `server-key`（也可用环境变量 `NOWORK_BASE` / `NOWORK_SERVER_KEY`）后重启。插件入口为 `cyou.nowork.example.paper.NoworkPaperPlugin`。心跳和世界列表在主线程读取，HTTP 与 GM 拉取/回执在异步线程，GM 游戏操作回到主线程执行。

该示例尚未获得本轮 JDK 21 / Gradle / Paper 构建及服务器运行验收；现有 JDK 17 只能验证 SDK 本身。SDK 的 `SelfCheck`、`EncryptedTransportTest`、`GameDataClientTest` 是显式运行的离线 `main` 测试，不能当作本插件的构建证明；本示例没有 Paper 行为测试，`gradle test` 的空任务也不能证明运行正确。

---

# Paper plugin skeleton (Java)

A minimal Paper 1.21 plugin that compiles the dependency-free Java SDK sources in: join checks on `AsyncPlayerPreLoginEvent` with an outage policy (`cache` admits players a live check admitted in the last 24 hours, `deny`, `allow`; business errors and encrypted replies always refuse), heartbeat with the player list and the 1-minute TPS (read on the main thread, sent asynchronously), and the GM channel (`kick`, `broadcast`, `time` with the `worlds` option list; commands run on the main thread, results are reported asynchronously).

The Java SDK supports encrypted Server API replies through explicitly configured private JWKs. This minimal example still uses the unkeyed constructor, so encrypted replies raise `NoWorkException.Encryption` and refuse admission. To adapt it for encryption, use `new NoworkClient(base, key, timeout, serverCode, currentKey, previousKey)` at the client construction in `onEnable`; use the actual server code and locally loaded keys, omitting the previous key outside rotation. See the [Java README](../../java/README.md). The example does not expose private keys in `config.yml` or upload them. The SDK checks exact server/route context and refuses plaintext whitelist downgrades while permitting normal plaintext heartbeat/GM replies.

Build with JDK 21 and Gradle 8.5+ (`gradle build`). The declared compile-only dependency is `io.papermc.paper:paper-api:1.21.1-R0.1-SNAPSHOT`, resolved through Maven Central/PaperMC; neither a Gradle wrapper nor the Paper API JAR is bundled. Once all dependencies are cached, `gradle --offline build` can build without remote repositories. The JAR contains the Java SDK classes; Paper supplies its own runtime APIs. Drop the JAR into `plugins/`, then set `base` and `server-key` in `plugins/NoWorkAccount/config.yml` or the matching environment variables. The entry class is `cyou.nowork.example.paper.NoworkPaperPlugin`.

This round has not yet verified a JDK 21/Gradle/Paper build or a running Paper server. Java SDK executable checks run separately and do not prove this plugin builds or behaves correctly; the example has no Paper behavior tests, so an empty Gradle `test` task is not runtime validation.
