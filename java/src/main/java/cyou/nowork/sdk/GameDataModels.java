package cyou.nowork.sdk;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;
import java.lang.reflect.Constructor;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.RecordComponent;
import java.lang.reflect.Type;
import java.math.BigDecimal;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Typed NoWork game API responses. Component names match the wire protocol, including
 * camelCase in legacy binding responses. Nullable/omitted metadata stays null; status,
 * source and policy values are never inferred. Timestamps retain the server's original
 * string (normalized snapshots use RFC 3339; legacy bindings use the site's format).
 * Collections decoded by the SDK are immutable. Catalog entities do not imply ownership.
 */
public final class GameDataModels {
    private GameDataModels() {}

    @Retention(RetentionPolicy.RUNTIME) @Target(ElementType.RECORD_COMPONENT)
    @interface OptionalWire {}
    @Retention(RetentionPolicy.RUNTIME) @Target(ElementType.RECORD_COMPONENT)
    @interface Wire { String value(); }

    public record Policy(String visibility, boolean exportable) {}
    public record Provenance(String source_id, String kind, String name, @OptionalWire String url,
            String auth_scope, String fetched_at, @OptionalWire String updated_at,
            @OptionalWire String license, @OptionalWire String attribution) {}
    public record Icon(String url, @OptionalWire Map<String, String> alt, @OptionalWire String source_id,
            @OptionalWire Integer width, @OptionalWire Integer height) {}
    public record Capability(String id, Map<String, String> name, boolean supported, String auth_scope,
            String status, @OptionalWire String reason, @OptionalWire Provenance source) {}
    public record Game(String id, String provider, Map<String, String> name, @OptionalWire Icon icon,
            @OptionalWire List<String> regions, List<Capability> capabilities) {}
    public record BoundRole(String id, String game_id, String uid, String region,
            @OptionalWire String server, @OptionalWire String nickname, @OptionalWire Integer level,
            @OptionalWire Icon icon, Policy policy) {}

    /** Missing, JSON null, zero and false are distinct, even on stale/unavailable fields. */
    public sealed interface FieldValue permits MissingValue, NullValue, TextValue, NumberValue,
            BooleanValue, TextListValue {}
    public enum MissingValue implements FieldValue { INSTANCE }
    public enum NullValue implements FieldValue { INSTANCE }
    public record TextValue(String value) implements FieldValue {}
    /** Exact decimal representation, including integers larger than signed 64-bit. */
    public record NumberValue(BigDecimal value) implements FieldValue {}
    public record BooleanValue(boolean value) implements FieldValue {}
    public record TextListValue(List<String> value) implements FieldValue {
        public TextListValue { value = List.copyOf(value); }
    }
    public record Field(String key, Map<String, String> name, String kind, FieldValue value,
            @OptionalWire String unit, String status, Policy policy, @OptionalWire Provenance source,
            @OptionalWire String updated_at) {
        public boolean hasValue() { return value != MissingValue.INSTANCE; }
    }
    public record Entity(String id, String kind, Map<String, String> name, @OptionalWire Icon icon,
            List<Field> fields, Policy policy, @OptionalWire Provenance source) {}
    public record Section(String id, Map<String, String> name, String status, @OptionalWire String reason,
            List<Field> fields, List<Entity> entities, Policy policy, @OptionalWire Provenance source,
            @OptionalWire String updated_at, @OptionalWire String next_cursor) {}
    public record Snapshot(String game_id, @OptionalWire BoundRole role, List<Section> sections,
            String fetched_at, @OptionalWire String expires_at, @OptionalWire String next_cursor) {}
    public record GameList(List<Game> games) {}
    public record Discovery(String status, @OptionalWire String updatedAt, @OptionalWire String reason) {}
    public record AccountGames(List<Game> games, List<BoundRole> roles, Discovery discovery, String selectedRoleId) {}

    public record MihoyoRegion(String id, String name, boolean qr, boolean cookie) {}
    public record Recovery(String action, @OptionalWire String url) {}
    public record MihoyoProfile(String uid, String nickname, String avatar, String introduce) {}
    public record MihoyoGameOption(String gameBiz, String name) {}
    /** id is the database binding role ID accepted by roleId, not gameUid. */
    public record MihoyoRole(long id, String gameBiz, String gameId, String gameName, String region,
            String regionName, String gameUid, String nickname, int level, boolean isChosen,
            @OptionalWire String syncTime) {}
    public record MihoyoStatus(@OptionalWire Long accountId, String region, List<MihoyoRegion> regions,
            Recovery recovery, List<Game> gameCatalog, boolean enabled, boolean allowQr, boolean allowCookie,
            boolean recordEnabled, boolean bound, @OptionalWire MihoyoProfile profile, List<MihoyoRole> roles,
            @OptionalWire String lastSync, @OptionalWire String nextSyncAt, List<MihoyoGameOption> games,
            @OptionalWire String method, @OptionalWire String state, @OptionalWire String stateMessage) {}

    public record HypergryphStatus(boolean enabled, boolean allowSms, boolean allowToken, boolean serverApi,
            String tokenHelpUrl, @OptionalWire HypergryphAccount account) {}
    public record HypergryphAccount(long id, long userId, String sklandUid, String hgUid, String nickname,
            String avatar, String method, boolean share, String status, String lastError,
            @OptionalWire String lastSync, String selectedRoleId, String discoveryStatus,
            @OptionalWire String discoveryAt, String createTime, String updateTime, List<HypergryphRole> roles) {}
    public record HypergryphRole(long id, long accountId, long userId, String uid, String nickname,
            String channelName, String channelId, boolean isOfficial, boolean isDefault, int level,
            String syncError, @OptionalWire String lastSync, String createTime, String updateTime,
            @OptionalWire ArknightsSummary summary, int apNow) {}

    public record AssetEndpoint(String locale, String version, String url) {}
    public record AssetManifest(String id, String game_id, Map<String, String> name, Provenance source,
            List<AssetEndpoint> endpoints, List<String> media_hosts, int ttl_seconds, long max_bytes, int max_entries) {}
    public record AssetCache(String provider, String locale, String version, boolean disabled, String source_url,
            @OptionalWire String content_hash, int entry_count, @OptionalWire String fetched_at,
            @OptionalWire String checked_at, @OptionalWire String expires_at, @OptionalWire String attempted_at,
            @OptionalWire String last_error_code) {}
    public record AssetProvider(AssetManifest manifest, List<AssetCache> caches) {}
    public record AssetPage(AssetManifest provider, AssetCache cache, String status, List<Entity> list,
            int count, int page, int limit) {}

    // Archive uint64 IDs/revisions use BigInteger: no signed overflow or floating-point rounding.
    public record StoredPolicy(BigInteger id, String game_id, String path, String visibility, boolean exportable,
            BigInteger revision, String updated_at) {}
    public record SnapshotMetadata(String game_id, String role_id, int version, BigInteger revision,
            String status, boolean stale, int bytes, String fetched_at, String expires_at) {}
    public record ArchivedSnapshot(String provider, String role_id, int version, BigInteger revision,
            String status, boolean stale, String expires_at, Snapshot snapshot) {}
    public record History(BigInteger id, String game_id, String role_id, BigInteger revision,
            String status, int bytes, String created_at) {}
    /** previous/current are JSON-encoded policies; previous is "null" for initial creation. */
    public record PolicyAudit(BigInteger id, String game_id, String path, BigInteger revision,
            String previous, String current, String created_at) {}
    /** Scope is present only on the explicitly authorized archive export response. */
    public record ExportScope(BigInteger site_id, BigInteger user_id, BigInteger account_id) {}
    public record ArchiveExport(ExportScope scope, String provider, String exported_at,
            List<ArchivedSnapshot> snapshots, List<History> history, List<StoredPolicy> policies,
            List<PolicyAudit> policy_audit) {}

    /** Existing consent-checked server projection, not a normalized multi-game inventory API. */
    public record SharedRoles(long uid, String status, @OptionalWire String lastSync,
            List<SharedRole> roles, long serverTime) {}
    public record SharedRole(String uid, String nickname, String channelName, boolean isOfficial,
            boolean isDefault, int level, int apNow, @OptionalWire ArknightsSummary summary,
            @OptionalWire String lastSync) {}
    /** Legacy scalar zero can mean unknown upstream data; normalized fields above are explicit. */
    public record ArknightsSummary(String uid, String name, int level, String avatarUrl, String avatarId,
            String resume, long registerTs, long lastOnlineTs, String mainStage, String secretary,
            int apCurrent, int apMax, long apLastAddTs, long apCompleteTs, int charCount, int skinCount,
            int furnitureCount, int medals, @OptionalWire Progress daily, @OptionalWire Progress weekly,
            @OptionalWire Campaign campaign, @OptionalWire Tower tower, @OptionalWire List<Rogue> rogue,
            @OptionalWire List<Recruit> recruit, @OptionalWire Building building,
            @OptionalWire List<Operator> assist, @OptionalWire List<Operator> roster, boolean rosterTrimmed) {}
    public record Operator(String id, String name, int rarity, @Wire("class") String profession,
            int elite, int level, int potential, int skillLevel, List<Skill> skills, List<Equip> equips,
            int favor, long gainTs) {}
    public record Skill(String id, int spec) {}
    public record Equip(String id, String name, String type, int level, @Wire("default") boolean equipped) {}
    public record Progress(int current, int total) {}
    public record ModeRecord(String id, String name, int value, String picture) {}
    public record Campaign(Progress reward, List<ModeRecord> records) {}
    public record Tower(Progress higher, Progress lower, long ts, List<ModeRecord> records) {}
    public record Rogue(String id, String name, String picture, int relics, int bank, int bankRecord) {}
    public record Recruit(int state, long startTs, long finishTs) {}
    public record Room(String kind, int level, int chars, int value, int limit) {}
    public record Building(int controlLevel, int labor, int laborMax, long laborRemainSec, long laborTs,
            int tired, int hireRefresh, List<Room> rooms) {}

    private static final Object MISSING = new Object();
    private record Shape(Constructor<?> constructor, RecordComponent[] components) {}
    private static final ClassValue<Shape> SHAPES = new ClassValue<>() {
        @Override protected Shape computeValue(Class<?> type) {
            RecordComponent[] components = type.getRecordComponents();
            Class<?>[] types = new Class<?>[components.length];
            for (int i = 0; i < types.length; i++) types[i] = components[i].getType();
            try { return new Shape(type.getDeclaredConstructor(types), components); }
            catch (ReflectiveOperationException e) { throw new IllegalStateException("invalid game DTO schema", e); }
        }
    };

    static <T> T decode(Object raw, Class<T> type) { return type.cast(convert(raw, type, false)); }

    static <T> List<T> decodeList(Object raw, Class<T> type) {
        if (!(raw instanceof List<?> values)) throw malformed();
        List<T> result = new ArrayList<>(values.size());
        for (Object value : values) result.add(decode(value, type));
        return Collections.unmodifiableList(result);
    }

    private static Object convert(Object raw, Type type, boolean optional) {
        if (type == FieldValue.class) return fieldValue(raw);
        if (optional && (raw == MISSING || raw == null)) return null;
        if (raw == MISSING) throw malformed();
        if (type instanceof ParameterizedType generic) {
            if (generic.getRawType() == List.class) {
                if (raw == null) return List.of(); // Go nil slices serialize as null.
                if (!(raw instanceof List<?> values)) throw malformed();
                List<Object> result = new ArrayList<>(values.size());
                for (Object value : values) result.add(convert(value, generic.getActualTypeArguments()[0], false));
                return Collections.unmodifiableList(result);
            }
            if (generic.getRawType() == Map.class) {
                if (raw == null) return Map.of();
                Map<String, Object> values = object(raw);
                Map<String, String> result = new LinkedHashMap<>();
                for (var entry : values.entrySet()) result.put(entry.getKey(), (String) convert(entry.getValue(), String.class, false));
                return Collections.unmodifiableMap(result);
            }
            throw malformed();
        }
        if (!(type instanceof Class<?> target) || raw == null) throw malformed();
        if (target == String.class && raw instanceof String) return raw;
        if ((target == boolean.class || target == Boolean.class) && raw instanceof Boolean) return raw;
        if (raw instanceof BigInteger number) {
            try {
                if (target == long.class || target == Long.class) return number.longValueExact();
                if (target == int.class || target == Integer.class) return number.intValueExact();
                if (target == BigInteger.class && number.signum() >= 0 && number.bitLength() <= 64) return number;
            } catch (ArithmeticException e) { throw malformed(); }
        }
        if (!target.isRecord() || target.getEnclosingClass() != GameDataModels.class) throw malformed();
        Map<String, Object> values = object(raw);
        Shape shape = SHAPES.get(target);
        Object[] args = new Object[shape.components().length];
        for (int i = 0; i < args.length; i++) {
            RecordComponent component = shape.components()[i];
            Wire wire = component.getAnnotation(Wire.class);
            String name = wire == null ? component.getName() : wire.value();
            args[i] = convert(values.containsKey(name) ? values.get(name) : MISSING,
                    component.getGenericType(), component.isAnnotationPresent(OptionalWire.class));
        }
        try { return shape.constructor().newInstance(args); }
        catch (ReflectiveOperationException e) { throw malformed(); }
    }

    private static FieldValue fieldValue(Object raw) {
        if (raw == MISSING) return MissingValue.INSTANCE;
        if (raw == null) return NullValue.INSTANCE;
        if (raw instanceof String value) return new TextValue(value);
        if (raw instanceof Boolean value) return new BooleanValue(value);
        if (raw instanceof BigInteger value) return new NumberValue(new BigDecimal(value));
        if (raw instanceof BigDecimal value) return new NumberValue(value);
        if (raw instanceof List<?> values) {
            List<String> result = new ArrayList<>(values.size());
            for (Object value : values) {
                if (!(value instanceof String text)) throw malformed();
                result.add(text);
            }
            return new TextListValue(result);
        }
        throw malformed();
    }

    @SuppressWarnings("unchecked")
    static Map<String, Object> object(Object raw) {
        if (!(raw instanceof Map<?, ?> values)) throw malformed();
        for (Object key : values.keySet()) if (!(key instanceof String)) throw malformed();
        return (Map<String, Object>) values;
    }

    private static IllegalArgumentException malformed() { return new IllegalArgumentException("invalid game API payload"); }

    /** Strict bounded reader: the older shared Json reader rounds uint64 and accepts trailing JSON. */
    static Object parse(String text) {
        Reader reader = new Reader(text);
        Object value = reader.value(0);
        reader.ws();
        if (reader.offset != text.length()) throw malformed();
        return value;
    }

    private static final class Reader {
        final String text;
        int offset;
        int nodes;
        Reader(String text) { this.text = text; }
        void ws() { while (offset < text.length() && " \t\r\n".indexOf(text.charAt(offset)) >= 0) offset++; }
        boolean take(char expected) {
            if (offset < text.length() && text.charAt(offset) == expected) { offset++; return true; }
            return false;
        }
        Object value(int depth) {
            if (depth > 64 || ++nodes > 1_000_000) throw malformed();
            ws();
            if (offset >= text.length()) throw malformed();
            char next = text.charAt(offset);
            if (next == '"') return string();
            if (take('{')) {
                Map<String, Object> result = new LinkedHashMap<>();
                ws();
                if (take('}')) return Collections.unmodifiableMap(result);
                do {
                    ws();
                    String key = string();
                    ws();
                    if (!take(':') || result.containsKey(key)) throw malformed();
                    result.put(key, value(depth + 1));
                    ws();
                    if (take('}')) return Collections.unmodifiableMap(result);
                } while (take(','));
                throw malformed();
            }
            if (take('[')) {
                List<Object> result = new ArrayList<>();
                ws();
                if (take(']')) return Collections.unmodifiableList(result);
                do {
                    result.add(value(depth + 1));
                    ws();
                    if (take(']')) return Collections.unmodifiableList(result);
                } while (take(','));
                throw malformed();
            }
            if (text.startsWith("true", offset)) { offset += 4; return true; }
            if (text.startsWith("false", offset)) { offset += 5; return false; }
            if (text.startsWith("null", offset)) { offset += 4; return null; }
            return number();
        }
        String string() {
            if (!take('"')) throw malformed();
            StringBuilder result = new StringBuilder();
            while (offset < text.length()) {
                char c = text.charAt(offset++);
                if (c == '"') return result.toString();
                if (c < 0x20) throw malformed();
                if (c == '\\') {
                    if (offset >= text.length()) throw malformed();
                    char escape = text.charAt(offset++);
                    c = switch (escape) {
                        case '"', '\\', '/' -> escape;
                        case 'b' -> '\b'; case 'f' -> '\f'; case 'n' -> '\n'; case 'r' -> '\r'; case 't' -> '\t';
                        case 'u' -> unicode();
                        default -> throw malformed();
                    };
                }
                result.append(c);
            }
            throw malformed();
        }
        char unicode() {
            if (offset + 4 > text.length()) throw malformed();
            int value = 0;
            for (int n = 0; n < 4; n++) {
                int digit = Character.digit(text.charAt(offset++), 16);
                if (digit < 0) throw malformed();
                value = (value << 4) | digit;
            }
            return (char) value;
        }
        Object number() {
            int start = offset;
            take('-');
            if (!take('0')) {
                if (offset >= text.length() || text.charAt(offset) < '1' || text.charAt(offset) > '9') throw malformed();
                digits();
            }
            boolean decimal = false;
            if (take('.')) { decimal = true; requiredDigits(); }
            if (take('e') || take('E')) {
                decimal = true;
                if (!take('+')) take('-');
                requiredDigits();
            }
            // Named game fields are scalars; a giant numeric token is never a legitimate metadata ID.
            if (offset - start > 1024) throw malformed();
            String token = text.substring(start, offset);
            try { return decimal ? new BigDecimal(token) : new BigInteger(token); }
            catch (NumberFormatException e) { throw malformed(); }
        }
        void digits() { while (offset < text.length() && text.charAt(offset) >= '0' && text.charAt(offset) <= '9') offset++; }
        void requiredDigits() { int start = offset; digits(); if (start == offset) throw malformed(); }
    }
}
