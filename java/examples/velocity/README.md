# Velocity 代理端示例

在 Velocity 代理层做白名单校验，可以让所有后端子服共享一次校验结果；子服再用 `Paper` 示例做二次兜底即可。

`build.gradle.kts` 依赖：

```kotlin
dependencies {
    compileOnly("com.velocitypowered:velocity-api:3.3.0-SNAPSHOT")
    annotationProcessor("com.velocitypowered:velocity-api:3.3.0-SNAPSHOT")
    implementation("cyou.lounge:lounge-sdk:1.0.0") // 或直接把 src/main/java/cyou/lounge/sdk 拷进插件
}
```

```java
package cyou.lounge.example.velocity;

import com.google.inject.Inject;
import com.velocitypowered.api.event.Subscribe;
import com.velocitypowered.api.event.connection.LoginEvent;
import com.velocitypowered.api.event.proxy.ProxyInitializeEvent;
import com.velocitypowered.api.plugin.Plugin;
import com.velocitypowered.api.proxy.ProxyServer;
import com.velocitypowered.api.event.ResultedEvent.ComponentResult;
import cyou.lounge.sdk.LoungeClient;
import cyou.lounge.sdk.LoungeException;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.format.NamedTextColor;
import org.slf4j.Logger;

import java.time.Duration;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

@Plugin(id = "lounge-whitelist", name = "Lounge Whitelist", version = "1.0.0")
public final class LoungeVelocityPlugin {

    private final ProxyServer proxy;
    private final Logger logger;
    private LoungeClient lounge;
    /** 最近一次校验结果缓存, 网络故障时兜底 */
    private final Map<UUID, Boolean> cache = new ConcurrentHashMap<>();

    @Inject
    public LoungeVelocityPlugin(ProxyServer proxy, Logger logger) {
        this.proxy = proxy;
        this.logger = logger;
    }

    @Subscribe
    public void onInit(ProxyInitializeEvent e) {
        // 建议从 config 读取; 服务器密钥在后台「服务器管理 → 密钥」生成, 只显示一次
        lounge = new LoungeClient(System.getenv("LOUNGE_BASE"), System.getenv("LOUNGE_SERVER_KEY"));
        proxy.getScheduler().buildTask(this, () -> {
            try {
                lounge.heartbeat(proxy.getPlayerCount(), proxy.getConfiguration().getShowMaxPlayers(), proxy.getVersion().getVersion());
            } catch (LoungeException ex) {
                logger.warn("heartbeat failed: {}", ex.getMessage());
            }
        }).repeat(Duration.ofSeconds(60)).schedule();
    }

    @Subscribe
    public void onLogin(LoginEvent e) {
        UUID uuid = e.getPlayer().getUniqueId();
        String name = e.getPlayer().getUsername();
        try {
            LoungeClient.CheckResult r = lounge.checkWhitelist(name, uuid.toString());
            cache.put(uuid, r.allowed);
            if (!r.allowed) {
                e.setResult(ComponentResult.denied(Component.text("请先在用户中心绑定角色", NamedTextColor.RED)
                        .append(Component.newline())
                        .append(Component.text("原因: " + r.reason, NamedTextColor.GRAY))));
            }
        } catch (LoungeException.Transport ex) {
            // 账号中心不可达: 有缓存按缓存, 无缓存拒绝 (fail-closed)
            if (!cache.getOrDefault(uuid, false)) {
                e.setResult(ComponentResult.denied(Component.text("账号服务暂不可用, 请稍后再试", NamedTextColor.RED)));
            }
        } catch (LoungeException ex) {
            logger.warn("whitelist check failed: {}", ex.getMessage());
        }
    }
}
```

要点：

- `LoginEvent` 在 Velocity 已经完成 Mojang 认证之后触发，`uuid` 可信；离线模式下只能按名字校验，此时建议后台把服务器设为「人工审核」。
- 每个代理只需要一把服务器密钥；如果各子服也接入了心跳，后台会显示多台服务器，按需选择。
- 心跳超过 5 分钟没有上报，后台自动把服务器标记为离线。
