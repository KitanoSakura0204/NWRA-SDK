package cyou.lounge.sdk;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Models of the player-data and per-server ban routes (module "server-users",
 * SDK/docs/SERVER_USERS.md); the calls are on {@link LoungeClient}.
 */
public final class ServerUsers {
    private ServerUsers() {
    }

    /** Business codes of these routes ({@link LoungeException.Api#code}); other refusals use code 1. */
    public static final int CODE_NOT_ON_ROSTER = 404;
    /** The data changed since the version you read: read again, then write. */
    public static final int CODE_VERSION_CONFLICT = 409;
    /** A ban an administrator placed: only an administrator can change it. */
    public static final int CODE_ADMIN_BAN = 403;

    /** Whether a data write was refused because of a stale version. */
    public static boolean isVersionConflict(Throwable e) {
        return e instanceof LoungeException.Api && ((LoungeException.Api) e).code == CODE_VERSION_CONFLICT;
    }

    /** One entry of the data schema the server owner defined. type: text | number | bool | select | json. */
    public static class UserField {
        public String key;
        public String label;
        public String type;
        public String description;
        /** Shown read-only to the player in the user centre. */
        public boolean visible;
        /** Decoded JSON (String, Number, Boolean, Map, List) or null. */
        public Object defaultValue;
        /** Characters (text) or bytes (json); 0 = not set. */
        public int maxLength;
        public Double min, max;
        public boolean integer;
        /** Each with "value" and optional "label". */
        public List<Map<String, String>> options = new ArrayList<>();

        static UserField from(Map<?, ?> m) {
            UserField f = new UserField();
            f.key = Json.str(m.get("key"));
            f.label = Json.str(m.get("label"));
            f.type = Json.str(m.get("type"));
            f.description = Json.str(m.get("description"));
            f.visible = Boolean.TRUE.equals(m.get("visible"));
            f.defaultValue = m.get("default");
            f.maxLength = Json.num(m.get("maxLength")).intValue();
            f.min = m.get("min") instanceof Number ? ((Number) m.get("min")).doubleValue() : null;
            f.max = m.get("max") instanceof Number ? ((Number) m.get("max")).doubleValue() : null;
            f.integer = Boolean.TRUE.equals(m.get("integer"));
            if (m.get("options") instanceof List<?>) {
                for (Object o : (List<?>) m.get("options")) {
                    if (!(o instanceof Map<?, ?>)) continue;
                    Map<String, String> opt = new LinkedHashMap<>();
                    opt.put("value", Json.str(((Map<?, ?>) o).get("value")));
                    if (((Map<?, ?>) o).get("label") != null) opt.put("label", Json.str(((Map<?, ?>) o).get("label")));
                    f.options.add(opt);
                }
            }
            return f;
        }
    }

    public static class UserSchema {
        public List<UserField> fields = new ArrayList<>();
        /** Cap on one account's stored values (JSON bytes). */
        public int maxUserBytes;

        static UserSchema from(Map<String, Object> d) {
            UserSchema s = new UserSchema();
            s.maxUserBytes = Json.num(d.get("maxUserBytes")).intValue();
            if (d.get("fields") instanceof List<?>) {
                for (Object o : (List<?>) d.get("fields")) if (o instanceof Map<?, ?>) s.fields.add(UserField.from((Map<?, ?>) o));
            }
            return s;
        }
    }

    /**
     * One account's data: every schema field with its stored value, else the default, else null.
     * version is the optimistic lock (0 = nothing stored yet).
     */
    public static class UserData {
        public long uid;
        /** Decoded JSON values by field key. */
        public Map<String, Object> data = new LinkedHashMap<>();
        public long version;
        /** "2006-01-02 15:04:05" server time; null before the first write. */
        public String updatedAt;
        public String updatedBy;
        public int size;

        @SuppressWarnings("unchecked")
        static UserData from(Map<String, Object> d) {
            UserData u = new UserData();
            u.uid = Json.num(d.get("uid")).longValue();
            if (d.get("data") instanceof Map<?, ?>) u.data.putAll((Map<String, Object>) d.get("data"));
            u.version = Json.num(d.get("version")).longValue();
            u.updatedAt = d.get("updatedAt") == null ? null : Json.str(d.get("updatedAt"));
            u.updatedBy = Json.str(d.get("updatedBy"));
            u.size = Json.num(d.get("size")).intValue();
            return u;
        }
    }

    /** An active ban on this server with the account's bindings there. */
    public static class ServerBan {
        public long uid;
        public List<String> names = new ArrayList<>();
        public List<String> uuids = new ArrayList<>();
        public String reason;
        /** null = permanent. */
        public String expireTime;
        public String createTime;
        /** admin | owner | plugin */
        public String source;

        static ServerBan from(Map<?, ?> m) {
            ServerBan b = new ServerBan();
            b.uid = Json.num(m.get("uid")).longValue();
            if (m.get("names") instanceof List<?>) for (Object o : (List<?>) m.get("names")) b.names.add(Json.str(o));
            if (m.get("uuids") instanceof List<?>) for (Object o : (List<?>) m.get("uuids")) b.uuids.add(Json.str(o));
            b.reason = Json.str(m.get("reason"));
            b.expireTime = m.get("expireTime") == null ? null : Json.str(m.get("expireTime"));
            b.createTime = Json.str(m.get("createTime"));
            b.source = Json.str(m.get("source"));
            return b;
        }
    }

    /** An online player (from the last heartbeat) whose account is banned here. */
    public static class KickEntry {
        public String name;
        public String uuid;
        public long uid;
        public String reason;
        public String expireTime;

        static KickEntry from(Map<?, ?> m) {
            KickEntry k = new KickEntry();
            k.name = Json.str(m.get("name"));
            k.uuid = Json.str(m.get("uuid"));
            k.uid = Json.num(m.get("uid")).longValue();
            k.reason = Json.str(m.get("reason"));
            k.expireTime = m.get("expireTime") == null ? null : Json.str(m.get("expireTime"));
            return k;
        }
    }

    /** Names an account: its uid, or the name / uuid of one of its bindings on this server. */
    public static final class Target {
        final long uid;
        final String name;
        final String uuid;

        private Target(long uid, String name, String uuid) {
            this.uid = uid;
            this.name = name;
            this.uuid = uuid;
        }

        public static Target uid(long uid) {
            return new Target(uid, null, null);
        }

        public static Target name(String name) {
            return new Target(0, name, null);
        }

        public static Target uuid(String uuid) {
            return new Target(0, null, uuid);
        }

        Map<String, Object> toMap() {
            Map<String, Object> m = new LinkedHashMap<>();
            if (uid > 0) m.put("uid", uid);
            if (name != null && !name.isEmpty()) m.put("name", name);
            if (uuid != null && !uuid.isEmpty()) m.put("uuid", uuid);
            return m;
        }
    }
}
