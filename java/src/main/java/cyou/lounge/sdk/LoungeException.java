package cyou.lounge.sdk;

/** Errors raised by the SDK. */
public class LoungeException extends Exception {
    public LoungeException(String message) {
        super(message);
    }

    public LoungeException(String message, Throwable cause) {
        super(message, cause);
    }

    /** Network / decoding failure. Callers should decide fail-open vs fail-closed. */
    public static class Transport extends LoungeException {
        public Transport(Throwable cause) {
            super("lounge: transport error: " + cause.getMessage(), cause);
        }

        public Transport(String message) {
            super("lounge: transport error: " + message);
        }
    }

    /**
     * A Server API reply was encrypted and could not be used (no decryption key configured). Not a
     * Transport error on purpose: the data was never read, so never fail open on it (an unreadable
     * whitelist is not a disabled whitelist).
     */
    public static class Encryption extends LoungeException {
        public final String route;
        public final String reason;

        public Encryption(String route, String reason) {
            super("lounge: encrypted response on " + route + ": " + reason);
            this.route = route;
            this.reason = reason;
        }
    }

    /** Business error (code != 0) from the user center. */
    public static class Api extends LoungeException {
        public final int code;

        public Api(int code, String message) {
            super("lounge: api error " + code + ": " + message);
            this.code = code;
        }
    }

    /** RFC 6749 error from an OAuth endpoint. */
    public static class OAuth extends LoungeException {
        public final String error;
        public final String description;

        public OAuth(String error, String description) {
            super("lounge: oauth error " + error + ": " + description);
            this.error = error;
            this.description = description;
        }
    }
}
