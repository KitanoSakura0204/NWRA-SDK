package cyou.nowork.sdk;

/** Errors raised by the SDK. */
public class NoWorkException extends Exception {
    public NoWorkException(String message) {
        super(message);
    }

    public NoWorkException(String message, Throwable cause) {
        super(message, cause);
    }

    /** Network / decoding failure. Callers should decide fail-open vs fail-closed. */
    public static class Transport extends NoWorkException {
        public Transport(Throwable cause) {
            super("nowork: transport error: " + cause.getMessage(), cause);
        }

        public Transport(String message) {
            super("nowork: transport error: " + message);
        }
    }

    /**
     * A Server API reply could not be opened or its protected context did not match, or a whitelist
     * reply arrived plain with decryption keys configured. Not a Transport error: never use outage
     * admission for an unreadable or downgraded whitelist.
     */
    public static class Encryption extends NoWorkException {
        public final String route;
        public final String reason;

        public Encryption(String route, String reason) {
            this(route, reason, null);
        }

        public Encryption(String route, String reason, Throwable cause) {
            super("nowork: encrypted response on " + route + ": " + reason, cause);
            this.route = route;
            this.reason = reason;
        }
    }

    /** Business error (code != 0) from the user center. */
    public static class Api extends NoWorkException {
        public final int code;

        public Api(int code, String message) {
            super("nowork: api error " + code + ": " + message);
            this.code = code;
        }
    }

    /** RFC 6749 error from an OAuth endpoint. */
    public static class OAuth extends NoWorkException {
        public final String error;
        public final String description;

        public OAuth(String error, String description) {
            super("nowork: oauth error " + error + ": " + description);
            this.error = error;
            this.description = description;
        }
    }
}
