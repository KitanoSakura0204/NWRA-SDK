export { BotClient, textMessage } from "./client.js";
export type { BotClientOptions } from "./client.js";
export { BotApiError } from "./errors.js";
export { connectEvents } from "./ws.js";
export type { EventStreamOptions } from "./ws.js";
export type {
  BindResult,
  CommandInfo,
  CommandResult,
  CommandsResult,
  Envelope,
  EventsPollResult,
  LookupResult,
  Message,
  PluginEvent,
  Segment,
  Target,
  UnbindResult,
} from "./types.js";
