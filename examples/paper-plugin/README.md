# Paper 插件骨架（Java）

> English version below.

在 Paper 1.21 服务器上接入休息室账号的最小插件，直接编译 Java SDK 源码（`../../java/src/main/java`，无第三方依赖）：

- `AsyncPlayerPreLoginEvent`（异步线程）实时校验白名单；未通过时以 `KICK_WHITELIST` 拒绝并显示原因。
- 用户中心不可达时按 `outage-policy` 降级：`cache` 放行 24 小时内实时校验通过的玩家，`deny` 全部拒绝，`allow` 全部放行。业务错误（密钥无效、服务器停用）与无法打开的加密响应一律拒绝。
- 心跳：主线程读取在线玩家、最大人数、版本与 1 分钟平均 TPS，异步上报（PROTOCOL §2.3）。
- GM 面板：启用 `server-gm` 模块时发布 `kick` / `broadcast` / `time` 三个类型化操作与世界列表 `worlds`，异步拉取命令、主线程执行、异步回执（PROTOCOL §2.4）。

Java SDK 目前不解密 Server API 信封：服务器开启加密（DELIVERY.md §5）后，SDK 抛出 `LoungeException.Encryption`，插件按拒绝处理。需要加密时请关闭该服务器的加密，或改用 Go / TypeScript / Python / C# SDK。

## 构建

需要 JDK 21 与 Gradle 8.5 以上：

```bash
gradle build        # 生成 build/libs/lounge-paper-plugin-1.0.0.jar，放入服务器 plugins/
```

首次启动生成 `plugins/LoungeAccount/config.yml`，填写 `base` 与 `server-key`（也可用环境变量 `LOUNGE_BASE` / `LOUNGE_SERVER_KEY`）后重启。本仓库未构建该示例（本机没有 Gradle 与 Paper API）；Java SDK 本身用 JDK 17 编译并由 `java/src/test/java/cyou/lounge/sdk/SelfCheck.java` 自检。

---

# Paper plugin skeleton (Java)

A minimal Paper 1.21 plugin that compiles the dependency-free Java SDK sources in: join checks on `AsyncPlayerPreLoginEvent` with an outage policy (`cache` admits players a live check admitted in the last 24 hours, `deny`, `allow`; business errors and encrypted replies always refuse), heartbeat with the player list and the 1-minute TPS (read on the main thread, sent asynchronously), and the GM channel (`kick`, `broadcast`, `time` with the `worlds` option list; commands run on the main thread, results are reported asynchronously).

The Java SDK does not open encrypted Server API replies yet: with server encryption on it throws `LoungeException.Encryption` and the plugin refuses the join. Build with JDK 21 and Gradle 8.5+ (`gradle build`), drop the jar into `plugins/`, then set `base` and `server-key` in `plugins/LoungeAccount/config.yml` or the `LOUNGE_BASE` / `LOUNGE_SERVER_KEY` environment variables. This example was not built here (no Gradle or Paper API on the machine).
