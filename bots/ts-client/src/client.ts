import { BotApiError } from "./errors.js";
import type {
  BindResult,
  CommandResult,
  CommandsResult,
  Envelope,
  EventsPollResult,
  LookupResult,
  Message,
  UnbindResult,
} from "./types.js";

export interface BotClientOptions {
  /** Site base URL, e.g. "https://nowork.example.com" (no trailing slash required). */
  baseUrl: string;
  /** The bot's token (Backend BotToken), sent as "Authorization: Bot <token>". */
  token: string;
  /** Override fetch (tests inject a fake). Defaults to the global fetch. */
  fetch?: typeof fetch;
  /** Accept-Language sent on every request; omit to let the server use its default. */
  lang?: string;
}

/**
 * Minimal client for the NoWork Bot API (/api/bot/v1). Framework plugins (Koishi, Wechaty)
 * use it for bind / unbind / lookup / command and for pulling queued notifications
 * (events + eventsPoll + ack); `connectEvents` in ws.ts drives the WebSocket.
 *
 * Contract read from the landed backend (Backend/internal/modules/bots), commits df7b675 /
 * d52a936, 2026-09-29. No public spec exists yet; see SDK/docs/BOTS_SDK.md.
 */
export class BotClient {
  readonly baseUrl: string;
  readonly token: string;
  private readonly fetchImpl: typeof fetch;
  private readonly lang?: string;

  constructor(opts: BotClientOptions) {
    this.baseUrl = opts.baseUrl.replace(/\/+$/, "");
    this.token = opts.token;
    this.fetchImpl = opts.fetch ?? fetch;
    this.lang = opts.lang;
  }

  private headers(extra?: Record<string, string>): Record<string, string> {
    const h: Record<string, string> = {
      Authorization: `Bot ${this.token}`,
      "Content-Type": "application/json",
      ...extra,
    };
    if (this.lang) h["Accept-Language"] = this.lang;
    return h;
  }

  private async call<T>(method: string, path: string, body?: unknown, query?: Record<string, string>): Promise<T> {
    let url = `${this.baseUrl}${path}`;
    if (query) {
      const qs = new URLSearchParams(query).toString();
      if (qs) url += `?${qs}`;
    }
    const res = await this.fetchImpl(url, {
      method,
      headers: this.headers(),
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    let env: Envelope<T>;
    try {
      env = (await res.json()) as Envelope<T>;
    } catch {
      throw new BotApiError(res.status, `bot api: non-JSON response (HTTP ${res.status})`);
    }
    if (env.code !== 0) {
      throw new BotApiError(env.code, env.message || `bot api error ${env.code}`);
    }
    return env.data as T;
  }

  /** POST /api/bot/v1/bind: the user sent "/bind CODE" in chat. */
  bind(platform: string, platformUserId: string, code: string, platformName?: string): Promise<BindResult> {
    return this.call("POST", "/api/bot/v1/bind", { platform, platformUserId, platformName, code });
  }

  /** POST /api/bot/v1/unbind */
  unbind(platform: string, platformUserId: string): Promise<UnbindResult> {
    return this.call("POST", "/api/bot/v1/unbind", { platform, platformUserId });
  }

  /** GET /api/bot/v1/lookup */
  lookup(platform: string, platformUserId: string): Promise<LookupResult> {
    return this.call("GET", "/api/bot/v1/lookup", undefined, { platform, platformUserId });
  }

  /** POST /api/bot/v1/command: pass a chat line through the server's command registry. */
  command(input: {
    platform: string;
    platformUserId: string;
    platformName?: string;
    chatType: string;
    chatId: string;
    text: string;
  }): Promise<CommandResult> {
    return this.call("POST", "/api/bot/v1/command", input);
  }

  /** GET /api/bot/v1/commands: for building the framework's own help / completion. */
  commands(): Promise<CommandsResult> {
    return this.call("GET", "/api/bot/v1/commands");
  }

  /**
   * GET /api/bot/v1/events without a WebSocket upgrade: the due pending notifications (at
   * most 50), each re-offered after a backoff until acked. Call `ackEvents` after handling.
   */
  pollEvents(): Promise<EventsPollResult> {
    return this.call("GET", "/api/bot/v1/events");
  }

  /** POST /api/bot/v1/events/ack */
  ackEvents(ids: number[]): Promise<{ acked: number }> {
    return this.call("POST", "/api/bot/v1/events/ack", { ids });
  }
}

export function textMessage(text: string): Message {
  return [{ type: "text", data: { text } }];
}
