import { BotClient, connectEvents, type PluginEvent } from "@nowork/bot-client";
import { Schema, Universal } from "@koishijs/core";
import type { Context, Bot } from "koishi";
import { chatAllowed, doBind, doMe, doPassthrough, doUnbind } from "./logic.js";
import { noworkMessageToText, platformOf } from "./convert.js";

export const name = "nowork";

export interface Config {
  baseUrl: string;
  botToken: string;
  platform?: string;
  allowedChats: string[];
  lang?: string;
  notificationBotId?: string;
}

export const Config: Schema<Config> = Schema.object({
  baseUrl: Schema.string().required().description("休息室账号站点地址，例如 https://nowork.example.com"),
  botToken: Schema.string().required().role("secret").description("机器人令牌（管理后台「开放平台 -> 机器人」创建并签发）"),
  platform: Schema.string().description("发给站点的 platform 字段；留空按 Koishi 平台名自动映射"),
  allowedChats: Schema.array(Schema.string()).default([]).description("允许使用 bind 指令的会话 id 白名单；留空表示不限制"),
  lang: Schema.string().description("发给站点的 Accept-Language（zh-CN / en / ja / zh-TW）"),
  notificationBotId: Schema.string().description("通知发送机器人 selfId；多个机器人在线时必须指定"),
}).description("休息室账号（NoWork Account）");

/** Koishi's status is a numeric protocol enum; connecting bots cannot deliver yet. */
export function selectNotificationBot<B extends Pick<Bot, "status" | "selfId">>(bots: readonly B[], botId?: string): B {
  const candidates = bots.filter((bot) => bot.status === Universal.Status.ONLINE && (!botId || bot.selfId === botId));
  if (candidates.length !== 1) throw new Error("nowork: configure one connected notificationBotId");
  return candidates[0];
}

/**
 * koishi-plugin-nowork: binds a chat account to a NoWork Account user, and bridges chat
 * text to the NoWork Bot API's command registry. Works on any Koishi adapter since it only
 * uses the generic Session surface (userId, channelId, platform, isDirect).
 *
 * Protocol: see SDK/docs/BOTS_SDK.md and @nowork/bot-client (read from the landed backend,
 * Backend/internal/modules/bots, commit df7b675 / d52a936, 2026-09-29).
 */
export function apply(ctx: Context, config: Config) {
  const client = new BotClient({ baseUrl: config.baseUrl, token: config.botToken, lang: config.lang });

  function platformFor(session: { platform?: string }): string {
    return platformOf(session.platform ?? "generic", config.platform);
  }

  ctx
    .command("bind <code:string>", "绑定休息室账号")
    .action(async ({ session }, code) => {
      const userId = session?.userId;
      if (!session || !userId) return;
      if (code && !chatAllowed(config.allowedChats, session.channelId ?? userId)) {
        return "此会话不允许使用绑定指令";
      }
      return doBind(client, platformFor(session), userId, session.username ?? "", code ?? "");
    });

  ctx.command("unbind", "解除休息室账号绑定").action(async ({ session }) => {
    const userId = session?.userId;
    if (!session || !userId) return;
    return doUnbind(client, platformFor(session), userId);
  });

  ctx.command("me", "查看我的休息室账号信息").action(async ({ session }) => {
    const userId = session?.userId;
    if (!session || !userId) return;
    return doMe(client, platformFor(session), userId);
  });

  // Lowest priority (large number = runs late), passes through unhandled text to the
  // site's command registry; next() lets other plugins still see the message.
  ctx.middleware(async (session, next) => {
    const text = session.content?.trim();
    const userId = session.userId;
    if (!text || !userId) return next();
    const chatId = session.channelId ?? userId;
    if (!chatAllowed(config.allowedChats, chatId)) return next();
    // unverified: isDirect / guildId distinguish private vs. group vs. channel on every
    // adapter we checked (OneBot, Discord), but a chatType mismatch on an untested
    // platform would only affect which built-in command sees the message, not delivery.
    const chatType = session.isDirect ? "private" : session.guildId ? "channel" : "group";
    const reply = await doPassthrough(client, platformFor(session), userId, session.username ?? "", chatType, chatId, text);
    if (reply) return reply;
    return next();
  }, true);

  // Notification bridge: the site pushes to this bot's bound chats over WebSocket.
  let stopEvents: (() => void) | undefined;
  ctx.on("ready", () => {
    stopEvents = connectEvents({
      baseUrl: config.baseUrl,
      token: config.botToken,
      onEvent: async (ev: PluginEvent) => {
        const text = noworkMessageToText(ev.message);
        if (!text) return;
        const bot = selectNotificationBot(ctx.bots, config.notificationBotId);
        const target = ev.target.chatId ?? ev.target.userId;
        if (!target) throw new Error("nowork: notification destination is missing");
        if (ev.target.chatType === "private") await bot.sendPrivateMessage(ev.target.userId ?? target, text);
        else await bot.sendMessage(target, text);
      },
      onError: (error) => ctx.logger("nowork").warn("notification left pending: %s", String(error)),
      onStatus: (status, detail) => ctx.logger("nowork").debug(`events stream: ${status}${detail ? ` (${detail})` : ""}`),
    });
  });
  ctx.on("dispose", () => stopEvents?.());
}
