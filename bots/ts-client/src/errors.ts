/** A non-zero `code` from the Bot API envelope, or a transport failure. */
export class BotApiError extends Error {
  code: number;
  constructor(code: number, message: string) {
    super(message);
    this.name = "BotApiError";
    this.code = code;
  }
}
