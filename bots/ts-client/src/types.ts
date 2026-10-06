// Wire types for the NoWork Bot API (/api/bot/v1). Mirrors
// Backend/internal/modules/bots/platform.go (Segment, Message, Target) and the handlers in
// Backend/internal/modules/bots/api.go / ws.go / hub.go. Field names and shapes come from
// reading the landed Go source (no public spec exists yet), 2026-09-29.

/** One piece of a message (text / mention / mention_all / image / link / reply / markdown). */
export interface Segment {
  type: string;
  data?: Record<string, string>;
}

/** A message is a list of segments. */
export type Message = Segment[];

/** Where a message goes, or where an inbound command came from. */
export interface Target {
  chatType: string;
  chatId?: string;
  userId?: string;
  reply?: Record<string, string>;
}

/** The envelope every /api/bot/v1 response uses: {code, message, data}. code 0 = success. */
export interface Envelope<T> {
  code: number;
  message?: string;
  data?: T;
}

export interface BindResult {
  userId: number;
  uid: number;
  nickname: string;
  level: number;
}

export interface UnbindResult {
  unbound: boolean;
}

export interface LookupResult {
  bound: boolean;
  uid?: number;
  nickname?: string;
  level?: number;
  levelName?: string;
  realnameVerified?: boolean;
}

export interface CommandResult {
  handled: boolean;
  reply: Message;
}

export interface CommandInfo {
  name: string;
  aliases: string[];
  help: string;
  needBind: boolean;
  minLevel: number;
}

export interface CommandsResult {
  prefixes: string[];
  commands: CommandInfo[];
}

/** One message offered to the plugin, over the WebSocket or the long-poll fallback. */
export interface PluginEvent {
  type: "message";
  id: number;
  botId: number;
  kind?: string;
  target: Target;
  message: Message;
  time: number;
}

export interface EventsPollResult {
  events: PluginEvent[];
}
