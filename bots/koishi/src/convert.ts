import type { Message } from "@nowork/bot-client";

/** Renders a NoWork Message (from /command or a pushed notification) down to plain text.
 * Non-text segments degrade to a bracketed placeholder rather than being dropped silently. */
export function noworkMessageToText(message: Message | undefined | null): string {
  if (!message || message.length === 0) return "";
  const parts: string[] = [];
  for (const seg of message) {
    const data = seg.data ?? {};
    switch (seg.type) {
      case "text":
        parts.push(data.text ?? "");
        break;
      case "mention":
        parts.push(`@${data.userId ?? data.name ?? ""}`);
        break;
      case "mention_all":
        parts.push("@all");
        break;
      case "link": {
        const label = data.label || data.url || "";
        parts.push(label !== data.url ? `${label} (${data.url ?? ""})` : data.url ?? "");
        break;
      }
      case "image":
        parts.push(`[image: ${data.url ?? ""}]`);
        break;
      default:
        parts.push(`[${seg.type}]`);
    }
  }
  return parts.join("");
}

/** Maps a Koishi platform name to the Bot API platform string. Koishi's own platform
 * names (set by each adapter) already match the gateway's codec kinds for the common
 * ones; anything unmapped is passed through as-is so new codecs need no change here. */
const PLATFORM_ALIASES: Record<string, string> = {
  onebot: "onebot11",
  qq: "qq_bot",
  wechat: "wechat_mp",
  "wechat-official": "wechat_mp",
  lark: "feishu",
};

export function platformOf(koishiPlatform: string, override?: string): string {
  if (override) return override;
  return PLATFORM_ALIASES[koishiPlatform] ?? koishiPlatform;
}
