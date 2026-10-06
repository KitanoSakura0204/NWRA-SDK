package cyou.nowork.sdk;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Map;
import java.util.Optional;
import java.util.function.Function;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Verifies webhook deliveries from the user center (X-NoWork-Signature / X-NoWork-Timestamp). */
public final class NoWorkWebhook {

    /** A verified webhook envelope. */
    public static final class Event {
        public final String id;
        public final String event;
        public final long timestamp;
        public final Map<String, Object> data;

        Event(String id, String event, long timestamp, Map<String, Object> data) {
            this.id = id;
            this.event = event;
            this.timestamp = timestamp;
            this.data = data;
        }
    }

    /** Thrown when the signature or timestamp is invalid. */
    public static final class InvalidSignature extends NoWorkException {
        public InvalidSignature(String message) {
            super(message);
        }
    }

    private NoWorkWebhook() {}

    /** sha256=HMAC_SHA256(secret, "&lt;ts&gt;.&lt;body&gt;") */
    public static String signature(String secret, long ts, byte[] body) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            mac.update((ts + ".").getBytes(StandardCharsets.US_ASCII));
            mac.update(body);
            StringBuilder sb = new StringBuilder("sha256=");
            for (byte b : mac.doFinal()) sb.append(String.format("%02x", b));
            return sb.toString();
        } catch (Exception e) {
            throw new IllegalStateException(e);
        }
    }

    /**
     * Verify and parse an incoming request.
     *
     * @param secret     endpoint secret from the admin console
     * @param header     header lookup (case-insensitive), e.g. {@code request::getHeader}
     * @param body       raw request body
     * @param tolerance  replay window in seconds; 0 disables
     */
    @SuppressWarnings("unchecked")
    public static Event verify(String secret, Function<String, String> header, byte[] body, long tolerance) throws NoWorkException {
        long ts;
        try {
            ts = Long.parseLong(Optional.ofNullable(header.apply("X-NoWork-Timestamp")).orElse("").trim());
        } catch (NumberFormatException e) {
            throw new InvalidSignature("missing or invalid X-NoWork-Timestamp");
        }
        if (tolerance > 0 && Math.abs(System.currentTimeMillis() / 1000L - ts) > tolerance) {
            throw new InvalidSignature("webhook timestamp outside tolerance");
        }
        String expected = signature(secret, ts, body);
        String given = Optional.ofNullable(header.apply("X-NoWork-Signature")).orElse("").trim();
        if (!MessageDigest.isEqual(expected.getBytes(StandardCharsets.US_ASCII), given.getBytes(StandardCharsets.US_ASCII))) {
            throw new InvalidSignature("invalid webhook signature");
        }
        Object parsed = Json.parse(new String(body, StandardCharsets.UTF_8));
        if (!(parsed instanceof Map)) {
            throw new InvalidSignature("invalid webhook body");
        }
        Map<String, Object> map = (Map<String, Object>) parsed;
        Object data = map.get("data");
        long timestamp = map.get("timestamp") instanceof Number ? ((Number) map.get("timestamp")).longValue() : 0L;
        return new Event(String.valueOf(map.getOrDefault("id", "")), String.valueOf(map.getOrDefault("event", "")), timestamp,
                data instanceof Map ? (Map<String, Object>) data : Map.of());
    }
}
