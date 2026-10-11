/** A non-zero `code` from the Bot API envelope, or a transport failure. */
export class BotApiError extends Error {
  code: number;
  constructor(code: number, message: string) {
    super(message);
    this.name = "BotApiError";
    this.code = code;
  }
}

/** Network, timeout, malformed envelope or HTTP failure without a business error.
 * Extends BotApiError for existing adapter catch blocks; code -1 is local, never a server code. */
export class BotTransportError extends BotApiError {
  constructor(message: string, public cause?: unknown) {
    super(-1, message);
    this.name = "BotTransportError";
  }
}
