# nowork-sdk (Java)

零依赖（Java 17+，`java.net.http`），可直接放进 Paper / Spigot / Velocity / Fabric 服务端插件。

```xml
<dependency>
  <groupId>cyou.nowork</groupId>
  <artifactId>nowork-sdk</artifactId>
  <version>1.2.0</version>
</dependency>
```

## Server API 与加密响应

```java
NoworkClient plain = new NoworkClient(base, serverKey); // 默认超时 10 秒
NoworkClient timed = new NoworkClient(base, serverKey, Duration.ofSeconds(5));

NoWorkDelivery.PrivateJwk current = NoWorkDelivery.PrivateJwk.parse(
    Files.readString(Path.of("keys", "current-private.jwk")));
NoWorkDelivery.PrivateJwk previous = NoWorkDelivery.PrivateJwk.parse(
    Files.readString(Path.of("keys", "previous-private.jwk")));
NoworkClient encrypted = new NoworkClient(
    base, serverKey, Duration.ofSeconds(5), "survival", current, previous);
NoworkClient.CheckResult result = encrypted.checkWhitelist("Steve", null);
```

代码片段需导入 `cyou.nowork.sdk.*`、`java.time.Duration`、`java.nio.file.Files` 和 `java.nio.file.Path`。`survival` 是后台服务器管理中的**服务器代码**，不是数字 ID；单把钥匙时只传 `current`，轮换窗口同时保留新旧私钥。公钥登记在服务端的投递配置，私钥由调用方本地读取，不发送给账号服务。

新增构造器为 `NoworkClient(base, serverKey, timeout, serverCode, PrivateJwk... keys)`。它复用 `NoWorkDelivery`，支持 `RSA-OAEP-256`、`ECDH-ES+A256KW`（P-256 / X25519），内容加密为 `A256GCM`，Java 17+ 无额外依赖。配置私钥时必须同时提供非空服务器代码，密钥列表不能为空；客户端复制传入的密钥数组。

- 已有两个构造器保持原行为：普通明文可读；遇到加密响应抛 `NoWorkException.Encryption`。
- 配置密钥后，受保护的 `ctx` 必须精确等于 `server:<服务器代码>:<请求路由>`（不带查询串）。不匹配、没有匹配的轮换密钥、损坏的密文/信封/解密 JSON 均抛 `Encryption`。
- `whitelist` 与 `whitelist/check` 成功响应必须加密，明文降级也抛 `Encryption`；心跳、模块、GM 的正常明文响应仍可使用。任何路由实际返回的加密信封都先解密并校验上下文。
- `Encryption` 不属于 `Transport`，插件必须拒绝，不能进入网络中断时的缓存/放行策略。真正的网络失败仍为 `Transport`，服务端非零 `code` 仍为 `Api`。
- `raw(method, path, body)` 保留有效响应的原始信封和正文，即使配置密钥也不解密或阻止明文检查。它仍校验 HTTP/业务信封；标为加密但格式损坏的响应不会被当作普通网络中断。

手动处理原始响应时可调用 `NoWorkDelivery.openServerData(raw.body, "server:survival:whitelist", current, previous)`；必须传入实际期望的上下文。完整协议见 [DELIVERY.md](../docs/DELIVERY.md#5-server-api-加密信封)。

## Paper 插件示例

见完整的 [Paper 1.21 插件骨架](../examples/paper-plugin/)：异步加入校验、主线程采集的心跳/世界列表、异步 GM 通信与主线程执行。示例默认使用不配置私钥的构造器，收到加密回复时拒绝；需要加密时在客户端构造处使用上面的显式配置方式。

SDK 本身的目标是 Java 17；Paper 示例另需 JDK 21、Gradle 8.5+ 和其声明的 Paper API 依赖。SDK 离线测试不能替代 Paper 编译及服务器运行验收。

## Velocity 代理端示例

见 [`examples/velocity/README.md`](examples/velocity/README.md)：在代理层做一次校验并缓存，子服用 Paper 示例兜底。

## OIDC

```java
NoworkOidc oidc = new NoworkOidc(issuer, clientId, clientSecret, redirectUri);
String url = oidc.authorizeUrl(state, nonce, "openid profile email game_roles", null);
Map<String,Object> token = oidc.exchange(code, null);
Map<String,Object> me = oidc.userInfo((String) token.get("access_token"));
```

## 离线自检

`src/test/java/cyou/nowork/sdk/` 下的 `SelfCheck`、`EncryptedTransportTest`、`GameDataClientTest` 都是可执行 `main` 类，不依赖 JUnit，也不会由 Maven/Gradle 的空测试任务自动执行。编译 SDK 和这些测试类后分别运行；`EncryptedTransportTest` 可接收 `SDK/go/testdata/vectors.json` 的路径作为唯一参数（从 `SDK/java` 运行时默认 `../go/testdata/vectors.json`）。测试只读取仓库测试密钥并请求本机 HTTP fixture，不需要账号服务或 Paper。

---

## English

Java 17+, no third-party runtime dependencies. Existing `NoworkClient(base, serverKey[, timeout])` constructors accept ordinary plaintext and refuse encrypted replies with `NoWorkException.Encryption`. Opt in using `NoworkClient(base, serverKey, timeout, serverCode, PrivateJwk... keys)`: supply the actual server **code** and one or more locally held private JWKs; include old and new keys during rotation. This reuses `NoWorkDelivery` for RSA-OAEP-256 or ECDH-ES+A256KW (P-256/X25519), with A256GCM.

The authenticated context must exactly match `server:<configured-code>:<request-route>` without the query string. Only `whitelist` and `whitelist/check` require encrypted success when keys are configured; ordinary plaintext heartbeat, module and GM replies remain valid. Wrong context/key, malformed encrypted data and plaintext downgrades raise `Encryption`, which must never enter an outage admission policy. Genuine network errors remain `Transport`; nonzero API codes remain `Api`. `raw` keeps valid original response bodies undecoded, with normal HTTP/envelope validation.

The [Paper example](../examples/paper-plugin/) uses the unkeyed constructor by default. Configure the constructor as above when adapting it for an encrypted server. The example requires JDK 21, Gradle 8.5+ and the declared Paper API dependency; SDK checks alone do not verify a Paper build or runtime. `SelfCheck`, `EncryptedTransportTest` and `GameDataClientTest` are executable offline test classes, not JUnit tests; run them explicitly after compilation.
