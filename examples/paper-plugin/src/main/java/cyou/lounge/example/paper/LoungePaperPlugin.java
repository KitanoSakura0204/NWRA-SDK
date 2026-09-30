package cyou.lounge.example.paper;

import cyou.lounge.sdk.LoungeClient;
import cyou.lounge.sdk.LoungeClient.GMAction;
import cyou.lounge.sdk.LoungeClient.GMActionParam;
import cyou.lounge.sdk.LoungeClient.GMOption;
import cyou.lounge.sdk.LoungeClient.OnlinePlayer;
import cyou.lounge.sdk.LoungeException;
import net.kyori.adventure.text.Component;
import org.bukkit.Bukkit;
import org.bukkit.World;
import org.bukkit.entity.Player;
import org.bukkit.event.EventHandler;
import org.bukkit.event.Listener;
import org.bukkit.event.player.AsyncPlayerPreLoginEvent;
import org.bukkit.plugin.java.JavaPlugin;

import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;
import java.util.logging.Level;

/**
 * Lounge Account on a Paper server: join check on AsyncPlayerPreLoginEvent (off the main thread),
 * heartbeat with TPS and the player list, and the GM channel (typed catalogue, polling, results).
 * Bukkit state is read and changed on the main thread only; HTTP runs on async tasks.
 */
public final class LoungePaperPlugin extends JavaPlugin implements Listener {
    private static final long ADMIT_CACHE_MILLIS = Duration.ofHours(24).toMillis();

    private LoungeClient lounge;
    private String outagePolicy;
    private volatile boolean gmEnabled;
    /** Players a live check admitted, with the time: the "cache" outage policy lets them in again. */
    private final Map<UUID, Long> admitted = new ConcurrentHashMap<>();

    @Override
    public void onEnable() {
        saveDefaultConfig();
        String base = env("LOUNGE_BASE", getConfig().getString("base", ""));
        String key = env("LOUNGE_SERVER_KEY", getConfig().getString("server-key", ""));
        if (base.isEmpty() || key.isEmpty()) {
            getLogger().severe("set base and server-key in config.yml (or LOUNGE_BASE / LOUNGE_SERVER_KEY); plugin disabled");
            getServer().getPluginManager().disablePlugin(this);
            return;
        }
        outagePolicy = getConfig().getString("outage-policy", "cache");
        lounge = new LoungeClient(base, key, Duration.ofSeconds(5));
        getServer().getPluginManager().registerEvents(this, this);

        long beat = 20L * Math.max(15, getConfig().getInt("heartbeat-seconds", 60));
        long poll = 20L * Math.max(5, getConfig().getInt("gm-poll-seconds", 15));
        // snapshot on the main thread, send off it
        Bukkit.getScheduler().runTaskTimer(this, this::heartbeat, 20L, beat);
        Bukkit.getScheduler().runTaskAsynchronously(this, this::setupGM);
        Bukkit.getScheduler().runTaskTimerAsynchronously(this, this::pollGM, poll, poll);
    }

    // ---- join check ----

    @EventHandler
    public void onPreLogin(AsyncPlayerPreLoginEvent e) {
        UUID id = e.getUniqueId();
        try {
            LoungeClient.CheckResult r = lounge.checkWhitelist(e.getName(), id.toString());
            if (r.allowed) {
                admitted.put(id, System.currentTimeMillis());
                return;
            }
            admitted.remove(id);
            refuse(e, "未通过白名单 / Not whitelisted: " + (r.reason == null || r.reason.isEmpty() ? "not bound" : r.reason));
        } catch (LoungeException.Encryption ex) {
            // encrypted reply this plugin cannot open: never an outage, always refuse (fail closed)
            getLogger().severe("join check: " + ex.getMessage() + " (turn server encryption off or use the Go / TypeScript / Python / C# SDK)");
            refuse(e, "账号服务配置错误 / Account service misconfigured");
        } catch (LoungeException.Api ex) {
            // 401 wrong key, 403 server disabled or not approved: a configuration problem, refuse
            getLogger().severe("join check refused by the user center: " + ex.getMessage());
            refuse(e, "账号服务拒绝了校验 / Account service refused the check");
        } catch (LoungeException ex) {
            onOutage(e, ex);
        }
    }

    private void onOutage(AsyncPlayerPreLoginEvent e, LoungeException ex) {
        getLogger().log(Level.WARNING, "join check for " + e.getName() + " failed, outage policy " + outagePolicy, ex);
        switch (outagePolicy) {
            case "allow":
                return;
            case "cache": {
                Long at = admitted.get(e.getUniqueId());
                if (at != null && System.currentTimeMillis() - at < ADMIT_CACHE_MILLIS) return;
                break;
            }
            default:
                break;
        }
        refuse(e, "账号服务暂不可用，请稍后再试 / Account service unavailable, try again later");
    }

    private static void refuse(AsyncPlayerPreLoginEvent e, String message) {
        e.disallow(AsyncPlayerPreLoginEvent.Result.KICK_WHITELIST, Component.text(message));
    }

    // ---- heartbeat (PROTOCOL 2.3) ----

    /** Runs on the main thread: read the server state there, send it asynchronously. */
    private void heartbeat() {
        List<OnlinePlayer> players = new ArrayList<>();
        for (Player p : Bukkit.getOnlinePlayers()) players.add(new OnlinePlayer(p.getName(), p.getUniqueId().toString()));
        int online = players.size();
        int max = Bukkit.getMaxPlayers();
        String version = Bukkit.getMinecraftVersion();
        double tps = Math.min(20.0, Bukkit.getTPS()[0]); // Paper: 1-minute average
        Bukkit.getScheduler().runTaskAsynchronously(this, () -> {
            try {
                lounge.heartbeat(online, max, version, tps, players);
            } catch (LoungeException ex) {
                getLogger().warning("heartbeat: " + ex.getMessage());
            }
        });
    }

    // ---- GM channel (PROTOCOL 2.4) ----

    private void setupGM() {
        try {
            if (!lounge.modules().containsKey("server-gm")) {
                getLogger().info("server-gm module not enabled for this server: GM channel off");
                return;
            }
            GMActionParam ticks = new GMActionParam("ticks", "number").label("时间（tick）").range(0, 23999).defaultValue(1000);
            ticks.integer = true;
            GMActionParam message = new GMActionParam("message", "text").required().label("内容");
            message.maxLength = 500;
            GMAction kick = new GMAction("kick", "踢出玩家").category("players").danger("confirm")
                    .param(new GMActionParam("player", "player").required().label("玩家"))
                    .param(new GMActionParam("reason", "string").label("原因"));
            kick.labels.put("en", "Kick player");
            GMAction broadcast = new GMAction("broadcast", "全服广播").category("chat").param(message);
            broadcast.labels.put("en", "Broadcast");
            GMAction time = new GMAction("time", "设置时间").category("world")
                    .param(new GMActionParam("world", "select").required().label("世界").source("worlds"))
                    .param(ticks);
            time.labels.put("en", "Set time");
            LoungeClient.GMPublishResult r = lounge.publishGMCatalog(List.of(kick, broadcast, time), Map.of("worlds", worldOptions()));
            getLogger().info("GM catalogue published: " + r.count + " stored");
            for (Map<String, String> s : r.skipped) getLogger().warning("GM entry skipped: " + s);
            gmEnabled = true;
        } catch (LoungeException ex) {
            getLogger().warning("GM setup: " + ex.getMessage());
        }
    }

    private static List<GMOption> worldOptions() {
        List<GMOption> out = new ArrayList<>();
        for (World w : Bukkit.getWorlds()) out.add(new GMOption(w.getName()));
        return out;
    }

    /** Async: fetch the commands, run each on the main thread, report from an async task. */
    private void pollGM() {
        if (!gmEnabled) return;
        List<Map<String, Object>> cmds;
        try {
            cmds = lounge.pollGMCommands();
        } catch (LoungeException ex) {
            getLogger().warning("GM poll: " + ex.getMessage());
            return;
        }
        for (Map<String, Object> cmd : cmds) {
            long id = cmd.get("id") instanceof Number n ? n.longValue() : 0;
            String action = String.valueOf(cmd.get("action"));
            Map<?, ?> params = cmd.get("params") instanceof Map<?, ?> m ? m : Map.of();
            Bukkit.getScheduler().runTask(this, () -> {
                String result;
                boolean ok;
                try {
                    result = run(action, params);
                    ok = true;
                } catch (IllegalArgumentException ex) {
                    result = ex.getMessage();
                    ok = false;
                }
                getLogger().info("GM #" + id + " " + action + " by " + cmd.getOrDefault("operator", "-") + ": " + result);
                final String text = result;
                final boolean success = ok;
                Bukkit.getScheduler().runTaskAsynchronously(this, () -> {
                    try {
                        lounge.reportGMResult(id, success, text);
                    } catch (LoungeException ex) {
                        getLogger().warning("GM #" + id + " result: " + ex.getMessage());
                    }
                });
            });
        }
    }

    /** Main thread. Throws IllegalArgumentException with the text shown as the failed result. */
    private String run(String action, Map<?, ?> p) {
        switch (action) {
            case "kick": {
                Player target = Bukkit.getPlayerExact(str(p.get("player")));
                if (target == null) throw new IllegalArgumentException(str(p.get("player")) + " is not online");
                String reason = str(p.get("reason"));
                target.kick(Component.text(reason.isEmpty() ? "Kicked by an operator" : reason));
                return target.getName() + " kicked";
            }
            case "broadcast": {
                String msg = str(p.get("message")).trim();
                if (msg.isEmpty()) throw new IllegalArgumentException("empty message");
                Bukkit.broadcast(Component.text(msg));
                return "broadcast sent";
            }
            case "time": {
                World w = Bukkit.getWorld(str(p.get("world")));
                if (w == null) throw new IllegalArgumentException("no world " + str(p.get("world")));
                long ticks = p.get("ticks") instanceof Number n ? n.longValue() : 1000L;
                if (ticks < 0 || ticks > 23999) throw new IllegalArgumentException("ticks must be 0-23999");
                w.setTime(ticks);
                return w.getName() + " time set to " + ticks;
            }
            default:
                throw new IllegalArgumentException("unknown action " + action);
        }
    }

    private static String str(Object o) {
        return o == null ? "" : String.valueOf(o);
    }

    private static String env(String name, String fallback) {
        String v = System.getenv(name);
        return v != null && !v.isBlank() ? v.trim() : (fallback == null ? "" : fallback.trim());
    }
}
