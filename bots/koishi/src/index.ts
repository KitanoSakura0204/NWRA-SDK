import { BotClient, connectEvents, type PluginEvent } from "@lounge/bot-client";
import { Context, Schema } from "koishi";
import { chatAllowed, doBind, doMe, doPassthrough, doUnbind } from "./logic.js";
import { loungeMessageToText, platformOf } from "./convert.js";

export const name = "lounge";

export interface Config {
  baseUrl: string;
  botToken: string;
  platform?: string;
  allowedChats: string[];
  lang?: string;
}

export const Config: Schema<Config> = Schema.object({
  baseUrl: Schema.string().required().description("休息室账号站点地址，例如 https://lounge.example.com"),
  botToken: Schema.string().required().role("secret").description("机器人令牌（管理后台「开放平台 -> 机器人」创建并签发）"),
  platform: Schema.string().description("发给站点的 platform 字段；留空按 Koishi 平台名自动映射"),
  allowedChats: Schema.array(Schema.string()).default([]).description("允许使用 bind 指令的会话 id 白名单；留空表示不限制"),
  lang: Schema.string().description("发给站点的 Accept-Language（zh-CN / en / ja / zh-TW）"),
}).description("休息室账号（Lounge Account）");

/**
 * koishi-plugin-lounge: binds a chat account to a Lounge Account user, and bridges chat
 * text to the Lounge Bot API's command registry. Works on any Koishi adapter since it only
 * uses the generic Session surface (userId, channelId, platform, isDirect).
 *
 * Protocol: see SDK/docs/BOTS_SDK.md and @lounge/bot-client (read from the landed backend,
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
      if (!session) return;
      if (code && !chatAllowed(config.allowedChats, session.channelId ?? session.userId)) {
        return "此会话不允许使用绑定指令";
      }
      return doBind(client, platformFor(session), session.userId, session.username ?? "", code ?? "");
    });

  ctx.command("unbind", "解除休息室账号绑定").action(async ({ session }) => {
    if (!session) return;
    return doUnbind(client, platformFor(session), session.userId);
  });

  ctx.command("me", "查看我的休息室账号信息").action(async ({ session }) => {
    if (!session) return;
    return doMe(client, platformFor(session), session.userId);
  });

  // Lowest priority (large number = runs late), passes through unhandled text to the
  // site's command registry; next() lets other plugins still see the message.
  ctx.middleware(async (session, next) => {
    const text = session.content?.trim();
    if (!text) return next();
    const chatId = session.channelId ?? session.userId;
    if (!chatAllowed(config.allowedChats, chatId)) return next();
    // unverified: isDirect / guildId distinguish private vs. group vs. channel on every
    // adapter we checked (OneBot, Discord), but a chatType mismatch on an untested
    // platform would only affect which built-in command sees the message, not delivery.
    const chatType = session.isDirect ? "private" : session.guildId ? "channel" : "group";
    const reply = await doPassthrough(client, platformFor(session), session.userId, session.username ?? "", chatType, chatId, text);
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
        const text = loungeMessageToText(ev.message);
        if (!text) return;
        const bot = ctx.bots.find((b) => b.status === "online");
        if (!bot) {
          ctx.logger("lounge").warn("no connected bot could deliver a notification");
          return;
        }
        const target = ev.target.chatId ?? ev.target.userId;
        if (!target) return;
        await bot.sendMessage(target, text);
      },
      onStatus: (status, detail) => ctx.logger("lounge").debug(`events stream: ${status}${detail ? ` (${detail})` : ""}`),
    });
  });
  ctx.on("dispose", () => stopEvents?.());
}
