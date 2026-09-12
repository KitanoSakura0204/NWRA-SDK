# lounge-sdk (Java)

零依赖（Java 17+，`java.net.http`），可直接放进 Paper / Spigot / Velocity / Fabric 服务端插件。

```xml
<dependency>
  <groupId>cyou.lounge</groupId>
  <artifactId>lounge-sdk</artifactId>
  <version>1.0.0</version>
</dependency>
```

## Paper 插件示例

```java
public final class LoungePlugin extends JavaPlugin implements Listener {
    private LoungeClient lounge;
    private final Map<UUID, Boolean> cache = new ConcurrentHashMap<>();

    @Override public void onEnable() {
        saveDefaultConfig();
        lounge = new LoungeClient(getConfig().getString("base"), getConfig().getString("server-key"));
        getServer().getPluginManager().registerEvents(this, this);
        // 心跳: 每 60 秒
        getServer().getScheduler().runTaskTimerAsynchronously(this, () -> {
            try { lounge.heartbeat(Bukkit.getOnlinePlayers().size(), Bukkit.getMaxPlayers(), Bukkit.getMinecraftVersion()); }
            catch (LoungeException e) { getLogger().warning("heartbeat failed: " + e.getMessage()); }
        }, 20L, 20L * 60);
    }

    @EventHandler
    public void onPreLogin(AsyncPlayerPreLoginEvent e) {
        try {
            LoungeClient.CheckResult r = lounge.checkWhitelist(e.getName(), e.getUniqueId().toString());
            cache.put(e.getUniqueId(), r.allowed);
            if (!r.allowed) e.disallow(AsyncPlayerPreLoginEvent.Result.KICK_WHITELIST,
                "§c请先在用户中心绑定角色\n§7原因: " + r.reason);
        } catch (LoungeException.Transport ex) {
            // 网络异常: 使用缓存, 没有缓存时按需放行/拒绝
            if (!cache.getOrDefault(e.getUniqueId(), false))
                e.disallow(AsyncPlayerPreLoginEvent.Result.KICK_OTHER, "§c账号服务暂不可用, 请稍后再试");
        } catch (LoungeException ex) {
            getLogger().warning("whitelist check failed: " + ex.getMessage());
        }
    }
}
```

## OIDC

```java
LoungeOidc oidc = new LoungeOidc(issuer, clientId, clientSecret, redirectUri);
String url = oidc.authorizeUrl(state, nonce, "openid profile email game_roles", null);
Map<String,Object> token = oidc.exchange(code, null);
Map<String,Object> me = oidc.userInfo((String) token.get("access_token"));
```
