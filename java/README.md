# nowork-sdk (Java)

零依赖（Java 17+，`java.net.http`），可直接放进 Paper / Spigot / Velocity / Fabric 服务端插件。

使用本地 1.3.0 源码见[独立工程的 javac / jar 接入](../docs/LOCAL_SETUP.md#javajavac--jar无需-maven)，无需安装 Maven。下面的 Maven 依赖仅在所用仓库已发布该版本时可直接解析：

```xml
<dependency>
  <groupId>cyou.nowork</groupId>
  <artifactId>nowork-sdk</artifactId>
  <version>1.3.0</version>
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
Map<String,Object> discovery = oidc.discover();
String jwks = oidc.jwks();
String url = oidc.authorizeUrl(state, nonce, "openid profile email game_roles", null);
Map<String,Object> token = oidc.exchange(code, null);
Map<String,Object> claims = NoWorkDelivery.verifyIdToken(
    (String) token.get("id_token"), jwks, issuer, clientId, nonce);
Map<String,Object> me = oidc.userInfo((String) token.get("access_token"));
```

`issuer` 应使用配置的规范值（无尾部 `/`），`state` 与非空 `nonce` 由应用生成并绑定到本次授权会话；回调先校验 `state`，然后把保存的 `nonce` 传给验证器。不要从收到的 ID token 中取 nonce 作为期望值。`verifyIdToken(token, jwks, issuer, audience, expectedNonce)` 先验证 RS256 签名、发行者、受众和有效期，再要求已签名的字符串 nonce 精确匹配。显式传入 `null`、空字符串、缺失/非字符串/不匹配的 nonce 均抛 `NoWorkDelivery.DecryptException`；四参数旧签名保留原来的行为。

`discover()` 返回完整的 `Map<String,Object>`，`jwks()` 返回通过结构校验的原始 JSON 文本，未知字段保留。它们分别对配置发行者的固定 `/.well-known/openid-configuration` 和 `/jwks.json` 发起无认证 GET，不发送客户端密钥、Bearer token 或服务器密钥，也不跟随 HTTP 重定向或元数据中公布的地址。发现文档必须包含非空字符串 `issuer`、`authorization_endpoint`、`token_endpoint`、`jwks_uri`，其中发行者必须精确等于构造器去掉尾部 `/` 后的配置值；JWKS 的 `keys` 必须是对象数组。非 2xx、超过 1 MiB 的实际响应正文、无效 UTF-8/JSON 或结构错误均抛 `NoWorkException.Transport`，不能静默采用其他发行者或未验证的文档。复用现有 HTTP 客户端，连接和请求使用原有 10 秒超时；正文消费也受 10 秒限制。

元数据请求只接受有效的 HTTP(S) 发行者地址，可包含发行者路径，但不能包含 URL 用户凭据、查询串、片段或 `.` / `..` 路径段；无效配置抛 `Transport`，不会发出请求。

加密 ID token 使用同样的 nonce 绑定验证，先解密再验证内部签名和声明：

```java
Map<String,Object> claims = NoWorkDelivery.decryptIdTokenWithNonce(
    encryptedIdToken, jwks, issuer, clientId, nonce, current, previous);
```

旧的 `decryptIdToken(token, jwks, issuer, audience, PrivateJwk... keys)` 签名仍可使用。OIDC 客户端凭据用于应用授权；`NoworkClient` 的服务器密钥用于游戏 Server API，账号/原生凭据属于独立流程。

## 离线自检

`src/test/java/cyou/nowork/sdk/` 下的 `SelfCheck`、`EncryptedTransportTest`、`GameDataClientTest`、`OidcTest` 都是可执行 `main` 类，不依赖 JUnit，也不会由 Maven/Gradle 的空测试任务自动执行。编译 SDK 和这些测试类后分别运行；`EncryptedTransportTest`、`OidcTest` 可接收 `SDK/go/testdata/vectors.json` 的路径作为唯一参数（从 `SDK/java` 运行时默认 `../go/testdata/vectors.json`）。测试只读取仓库测试密钥并请求本机 HTTP fixture，不需要账号服务或 Paper。`OidcTest` 检查固定发现/JWKS 路径、无认证请求、严格 JSON 和字节限制、正文超时/中断、nonce 失败，以及后端签名再加密的 ID token 向量；包括一个实际等待 10 秒的正文超时检查。

---

## English

Java 17+, no third-party runtime dependencies. Existing `NoworkClient(base, serverKey[, timeout])` constructors accept ordinary plaintext and refuse encrypted replies with `NoWorkException.Encryption`. Opt in using `NoworkClient(base, serverKey, timeout, serverCode, PrivateJwk... keys)`: supply the actual server **code** and one or more locally held private JWKs; include old and new keys during rotation. This reuses `NoWorkDelivery` for RSA-OAEP-256 or ECDH-ES+A256KW (P-256/X25519), with A256GCM.

The authenticated context must exactly match `server:<configured-code>:<request-route>` without the query string. Only `whitelist` and `whitelist/check` require encrypted success when keys are configured; ordinary plaintext heartbeat, module and GM replies remain valid. Wrong context/key, malformed encrypted data and plaintext downgrades raise `Encryption`, which must never enter an outage admission policy. Genuine network errors remain `Transport`; nonzero API codes remain `Api`. `raw` keeps valid original response bodies undecoded, with normal HTTP/envelope validation.

OIDC `discover()` returns the complete metadata map and `jwks()` returns validated original JSON. Both use unauthenticated GETs to the configured issuer's fixed discovery/JWKS paths, with no redirects or automatic use of advertised endpoints. Discovery requires four nonblank string fields (`issuer`, `authorization_endpoint`, `token_endpoint`, `jwks_uri`) and an exact configured issuer match after constructor normalization. JWKS requires an array of key objects. Non-2xx, invalid UTF-8/JSON/shape and actual bodies over 1 MiB raise `NoWorkException.Transport`; the existing ten-second timeout also covers body consumption. Unknown fields are preserved.

Pass the authorization session's saved nonempty nonce to `NoWorkDelivery.verifyIdToken(token, jwks, issuer, audience, expectedNonce)`, or to `decryptIdTokenWithNonce(token, jwks, issuer, audience, expectedNonce, PrivateJwk... keys)` for an encrypted ID token. These verify the inner RS256 signature, issuer, audience and time claims before requiring an exact signed string nonce match. Invalid expected nonce or missing/mismatched/non-string claims raise `DecryptException`. Existing signatures remain usable, including null entries preceding a valid decryption key. Validate callback state separately. OIDC application credentials, game Server API keys and account/native credentials serve separate flows.

The [Paper example](../examples/paper-plugin/) uses the unkeyed constructor by default. Configure the constructor as above when adapting it for an encrypted server. The example requires JDK 21, Gradle 8.5+ and the declared Paper API dependency; SDK checks alone do not verify a Paper build or runtime. `SelfCheck`, `EncryptedTransportTest`, `GameDataClientTest` and `OidcTest` are executable offline test classes, not JUnit tests; run them explicitly after compilation.
