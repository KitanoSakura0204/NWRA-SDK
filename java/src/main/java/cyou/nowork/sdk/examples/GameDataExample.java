package cyou.nowork.sdk.examples;

import cyou.nowork.sdk.GameDataClient;
import cyou.nowork.sdk.GameDataModels;
import cyou.nowork.sdk.NoWorkException;
import java.time.Duration;
import java.util.List;

/**
 * Environment-only credentials; never logs raw role IDs, field values or tokens.
 * NOWORK_BASE_URL is required. MODE=account (default), public, admin, or server.
 * Account/admin: NOWORK_SITE_JWT. Server: NOWORK_SERVER_KEY + NOWORK_PLAYER_UID.
 * Optional account snapshot: GAME_PROVIDER=mihoyo|hypergryph, GAME_ID, GAME_SECTION,
 * GAME_ROLE_ID (miHoYo's stored numeric role ID, or Hypergryph's discovered role key).
 * Discovery responses/capabilities determine availability; a catalog entry is not ownership.
 */
public final class GameDataExample {
    private GameDataExample() {}

    public static void main(String[] args) throws NoWorkException {
        String base = required("NOWORK_BASE_URL");
        String mode = System.getenv().getOrDefault("MODE", "account");
        GameDataClient.Options options = new GameDataClient.Options(Duration.ofSeconds(10), 24 * 1024 * 1024);
        switch (mode) {
            case "public" -> describe(GameDataClient.publicData(base, options).hypergryphGames().games());
            case "admin" -> {
                var admin = GameDataClient.admin(base, required("NOWORK_SITE_JWT"), options);
                describe(admin.mihoyoGames());
                describe(admin.hypergryphGames().games());
                System.out.println("Available asset providers: " + admin.assetProviders().size());
            }
            case "server" -> {
                var shared = GameDataClient.server(base, required("NOWORK_SERVER_KEY"), options)
                        .sharedHypergryphRoles(Long.parseLong(required("NOWORK_PLAYER_UID")));
                System.out.println("Shared Arknights roles: " + shared.roles().size() + "; binding status: " + shared.status());
            }
            case "account" -> {
                var account = GameDataClient.account(base, required("NOWORK_SITE_JWT"), options);
                String providerName = System.getenv().getOrDefault("GAME_PROVIDER", "hypergryph");
                GameDataClient.Provider provider = switch (providerName) {
                    case "mihoyo" -> GameDataClient.Provider.MIHOYO;
                    case "hypergryph" -> GameDataClient.Provider.HYPERGRYPH;
                    default -> throw new IllegalArgumentException("GAME_PROVIDER must be mihoyo or hypergryph");
                };
                if (provider == GameDataClient.Provider.MIHOYO) {
                    describe(account.mihoyoGames());
                    var binding = account.mihoyoStatus();
                    System.out.println("Binding present: " + binding.bound() + "; recovery action: " + binding.recovery().action());
                } else {
                    var discovery = account.hypergryphGames();
                    describe(discovery.games());
                    System.out.println("Discovery status: " + discovery.discovery().status() + "; roles: " + discovery.roles().size());
                }
                String game = System.getenv("GAME_ID");
                if (game != null && !game.isBlank()) {
                    var query = new GameDataClient.SnapshotQuery(System.getenv("GAME_ROLE_ID"),
                            List.of(required("GAME_SECTION")), null, 50, "en");
                    // Two pages maximum. PaginationException carries an explicit resume cursor.
                    account.walkSnapshots(provider, game, query, 2, page -> {
                        for (var section : page.sections()) {
                            System.out.println("Section " + section.id() + ": " + section.status()
                                    + "; entities on this page: " + section.entities().size());
                        }
                        return true;
                    });
                }
            }
            default -> throw new IllegalArgumentException("MODE must be account, public, admin or server");
        }
    }

    private static void describe(List<GameDataModels.Game> games) {
        for (var game : games) {
            System.out.println(game.provider() + "/" + game.id());
            for (var capability : game.capabilities()) {
                System.out.println("  " + capability.id() + ": " + capability.status() + "; access: " + capability.auth_scope());
            }
        }
    }

    private static String required(String name) {
        String value = System.getenv(name);
        if (value == null || value.isBlank()) throw new IllegalArgumentException("Set " + name);
        return value;
    }
}
