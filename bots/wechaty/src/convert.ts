import type { Message } from "@lounge/bot-client";

/** Renders a Lounge Message (from /command or a pushed notification) down to plain text.
 * Non-text segments degrade to a bracketed placeholder rather than being dropped silently. */
export function loungeMessageToText(message: Message | undefined | null): string {
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
