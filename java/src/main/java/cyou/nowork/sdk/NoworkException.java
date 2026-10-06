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
     * A Server API reply was encrypted and could not be used (no decryption key configured). Not a
     * Transport error on purpose: the data was never read, so never fail open on it (an unreadable
     * whitelist is not a disabled whitelist).
     */
    public static class Encryption extends NoWorkException {
        public final String route;
        public final String reason;

        public Encryption(String route, String reason) {
            super("nowork: encrypted response on " + route + ": " + reason);
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
