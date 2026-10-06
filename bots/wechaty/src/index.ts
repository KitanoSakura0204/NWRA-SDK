import { BotClient, connectEvents, type PluginEvent } from "@nowork/bot-client";
import type { Message, Wechaty } from "wechaty";
import { chatAllowed, doBind, doMe, doPassthrough, parseCommand, doUnbind } from "./logic.js";
import { noworkMessageToText } from "./convert.js";

export interface NoWorkPluginConfig {
  /** NoWork site base URL, e.g. "https://nowork.example.com". */
  baseUrl: string;
  /** This bot's token (issued in the admin: 开放平台 -> 机器人). */
  botToken: string;
  /** Platform name sent to the Bot API (default "wechaty": a generic/bridge bot kind). */
  platform?: string;
  /** Command prefixes this plugin answers to (default ["/"]) . */
  prefixes?: string[];
  /** Chat ids (room id or contact id) allowed to use /bind; empty = every chat. */
  allowedChats?: string[];
  /** Accept-Language sent to the Bot API. */
  lang?: string;
}

/**
 * wechaty-plugin-nowork: a Wechaty plugin (see
 * https://wechaty.js.org/docs/tutorials/using-plugin-with-wechaty, read 2026-09-29: "A
 * Wechaty Plugin is a JavaScript function that returns a function which accepts a Wechaty
 * instance") for binding a WeChat account to a NoWork Account user, and bridging chat text
 * to the NoWork Bot API's command registry. Works with any Wechaty puppet.
 *
 * Protocol: see SDK/docs/BOTS_SDK.md and @nowork/bot-client (read from the landed backend,
 * Backend/internal/modules/bots, commit df7b675 / d52a936, 2026-09-29).
 */
export function NoWorkPlugin(config: NoWorkPluginConfig) {
  const platform = config.platform ?? "wechaty";
  const prefixes = config.prefixes ?? ["/"];
  const allowedChats = config.allowedChats ?? [];
  const client = new BotClient({ baseUrl: config.baseUrl, token: config.botToken, lang: config.lang });

  return function install(bot: Wechaty) {
    let stopEvents: (() => void) | undefined;

    bot.on("start", () => {
      stopEvents = connectEvents({
        baseUrl: config.baseUrl,
        token: config.botToken,
        onEvent: async (ev: PluginEvent) => {
          const text = noworkMessageToText(ev.message);
          if (!text) return;
          // Target.chatId is an opaque WeChat room/contact id (not a topic or display
          // name), so Room.find({topic}) / Contact.find({name}) do not apply here;
          // load(id) resolves an entity directly from a known id (bot.Contact.load:
          // https://wechaty.js.org/docs/api/contact, read 2026-09-29). The room id is
          // tried first since group notifications are the common case.
          const targetId = ev.target.chatId ?? ev.target.userId;
          if (!targetId) return;
          // load(id) is synchronous (returns the entity right away, from cache or a bare
          // id; https://wechaty.js.org/docs/api/contact shows it called without await) -
          // sync() afterwards fetches its data before say() is safe to call.
          try {
            if (ev.target.chatId) {
              // unverified: Room.load(id) mirrors Contact.load(id) by Wechaty's own
              // convention, but was not directly confirmed in the fetched docs.
              const room = bot.Room.load(ev.target.chatId);
              await room.sync();
              await room.say(text);
              return;
            }
          } catch (err) {
            bot.emit("error", err as Error); // fall through to the contact attempt below
          }
          try {
            const contact = bot.Contact.load(targetId);
            await contact.sync();
            await contact.say(text);
          } catch (err) {
            bot.emit("error", err as Error);
          }
        },
      });
    });

    bot.on("stop", () => stopEvents?.());

    bot.on("message", async (message: Message) => {
      if (message.self()) return;
      const text = message.text().trim();
      if (!text) return;
      const room = message.room();
      const talker = message.talker();
      const chatId = room ? room.id : talker.id;
      if (!chatAllowed(allowedChats, chatId)) return;
      const chatType = room ? "group" : "private";

      const cmd = parseCommand(text, prefixes);
      if (cmd) {
        const name = talker.name();
        if (cmd.name === "bind") {
          await message.say(await doBind(client, platform, talker.id, name, cmd.args[0] ?? ""));
          return;
        }
        if (cmd.name === "unbind") {
          await message.say(await doUnbind(client, platform, talker.id));
          return;
        }
        if (cmd.name === "me") {
          await message.say(await doMe(client, platform, talker.id));
          return;
        }
      }

      const reply = await doPassthrough(client, platform, talker.id, talker.name(), chatType, chatId, text);
      if (reply) await message.say(reply);
    });
  };
}
