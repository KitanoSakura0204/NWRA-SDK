import { BotClient, connectEvents, type PluginEvent } from "@nowork/bot-client";
import type { Message, Wechaty } from "wechaty";
import { chatAllowed, doBind, doMe, doPassthrough, parseCommand, doUnbind } from "./logic.js";
import { noworkMessageToText } from "./convert.js";

export interface NoWorkPluginConfig {
  /** NoWork site base URL, e.g. "https://nowork.example.com". */
  baseUrl: string;
  /** This bot's token (issued in the admin: 开放平台 -> 机器人). */
  botToken: string;
  /** Platform name sent to the Bot API (default "wechat": framework personal WeChat). */
  platform?: string;
  /** Command prefixes this plugin answers to (default ["/"]) . */
  prefixes?: string[];
  /** Chat ids (room id or contact id) allowed to use /bind; empty = every chat. */
  allowedChats?: string[];
  /** Accept-Language sent to the Bot API. */
  lang?: string;
}

/** Only the public Wechaty methods used by the notification callbacks. */
interface NotificationTarget {
  say(text: string): Promise<unknown>;
}

interface NotificationBot {
  Room: { find(query: { id: string }): Promise<NotificationTarget | undefined> };
  Contact: { find(query: { id: string }): Promise<NotificationTarget | undefined> };
  emitError(error: unknown): void;
}

export function notificationHandlers(bot: NotificationBot): {
  onEvent(event: PluginEvent): Promise<void>;
  onError(error: unknown): void;
} {
  return {
    async onEvent(ev) {
      const text = noworkMessageToText(ev.message);
      if (!text) return;
      // Public find({id}) resolves the exact opaque ID and readies the entity.
      // It can return undefined after lookup failure; that must remain unacked.
      if (ev.target.chatType === "private") {
        const contactId = ev.target.userId ?? ev.target.chatId;
        if (!contactId) throw new Error("nowork: notification destination is missing");
        const contact = await bot.Contact.find({ id: contactId });
        if (!contact) throw new Error("nowork: notification contact is unavailable");
        await contact.say(text);
        return;
      }
      const roomId = ev.target.chatId;
      if (!roomId) throw new Error("nowork: group notification requires chatId");
      const room = await bot.Room.find({ id: roomId });
      if (!room) throw new Error("nowork: notification room is unavailable");
      await room.say(text);
    },
    onError(error) { bot.emitError(error); },
  };
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
  const platform = config.platform ?? "wechat";
  const prefixes = config.prefixes ?? ["/"];
  const allowedChats = config.allowedChats ?? [];
  const client = new BotClient({ baseUrl: config.baseUrl, token: config.botToken, lang: config.lang });

  return function install(bot: Wechaty) {
    let stopEvents: (() => void) | undefined;

    bot.on("start", () => {
      stopEvents = connectEvents({
        baseUrl: config.baseUrl,
        token: config.botToken,
        ...notificationHandlers(bot),
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
