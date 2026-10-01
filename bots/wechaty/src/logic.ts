import { BotApiError, BotClient } from "@lounge/bot-client";
import { loungeMessageToText } from "./convert.js";

/** The actual bind/unbind/me/passthrough behaviour, kept free of Wechaty's Message/Contact
 * so it is testable with a fake BotClient and no running Wechaty puppet. */

export async function doBind(client: BotClient, platform: string, userId: string, name: string, code: string): Promise<string> {
  if (!code) return "用法: /bind <绑定码>（在「个人中心 -> 机器人」页面获取）";
  try {
    const result = await client.bind(platform, userId, code, name);
    return `绑定成功，欢迎 ${result.nickname}（等级 ${result.level}）`;
  } catch (err) {
    if (err instanceof BotApiError) return `绑定失败：${err.message}`;
    throw err;
  }
}

export async function doUnbind(client: BotClient, platform: string, userId: string): Promise<string> {
  try {
    const result = await client.unbind(platform, userId);
    return result.unbound ? "已解绑" : "未找到绑定记录";
  } catch (err) {
    if (err instanceof BotApiError) return `解绑失败：${err.message}`;
    throw err;
  }
}

export async function doMe(client: BotClient, platform: string, userId: string): Promise<string> {
  try {
    const info = await client.lookup(platform, userId);
    if (!info.bound) return "尚未绑定，请使用 /bind <绑定码>";
    const lines = [`昵称：${info.nickname}`, `等级：${info.levelName ?? info.level}`];
    if (info.realnameVerified !== undefined) {
      lines.push(info.realnameVerified ? "实名认证：已通过" : "实名认证：未通过");
    }
    return lines.join("\n");
  } catch (err) {
    if (err instanceof BotApiError) return `查询失败：${err.message}`;
    throw err;
  }
}

/** Returns the reply text, or null when the server did not handle the text. */
export async function doPassthrough(
  client: BotClient,
  platform: string,
  userId: string,
  name: string,
  chatType: string,
  chatId: string,
  text: string,
): Promise<string | null> {
  try {
    const result = await client.command({ platform, platformUserId: userId, platformName: name, chatType, chatId, text });
    if (!result.handled) return null;
    return loungeMessageToText(result.reply) || null;
  } catch (err) {
    if (err instanceof BotApiError) return null;
    throw err;
  }
}

export function chatAllowed(allowedChats: string[], chatId: string): boolean {
  return allowedChats.length === 0 || allowedChats.includes(chatId);
}

/** "/bind ABC123" (any configured prefix) -> {name:"bind", args:["ABC123"]}; null for
 * ordinary chat text. Wechaty gives no command router, unlike NoneBot2 / Koishi. */
export function parseCommand(text: string, prefixes: string[]): { name: string; args: string[] } | null {
  const trimmed = text.trim();
  for (const prefix of prefixes) {
    if (trimmed.startsWith(prefix)) {
      const rest = trimmed.slice(prefix.length).trim();
      const fields = rest.split(/\s+/).filter(Boolean);
      if (fields.length === 0) return null;
      return { name: fields[0].toLowerCase(), args: fields.slice(1) };
    }
  }
  return null;
}
