import { createHmac, timingSafeEqual } from "node:crypto";
import type { PluginEvent } from "./types.js";

export class BotPushError extends Error {
  constructor(message: string) { super(message); this.name = "BotPushError"; }
}

/** Verify the exact raw bytes before parsing. Persist event.id for deduplication;
 * acknowledge with HTTP 2xx only after successful delivery. A bot push is a PluginEvent. */
export function verifyPush(secret: string, headers: Headers | Record<string, string | undefined>,
  body: Uint8Array | string, opts: { toleranceSeconds?: number; nowSeconds?: number } = {}): PluginEvent {
  const h: Record<string, string | undefined> = {};
  if (headers instanceof Headers) headers.forEach((v, k) => { h[k.toLowerCase()] = v; });
  else for (const [k, v] of Object.entries(headers)) h[k.toLowerCase()] = v;
  const stamp = h["x-nowork-timestamp"] ?? "";
  const sig = h["x-nowork-signature"] ?? "";
  if (!secret || !/^[0-9]{1,12}$/.test(stamp) || !/^sha256=[0-9a-f]{64}$/.test(sig)) {
    throw new BotPushError("missing or invalid push authentication");
  }
  const now = opts.nowSeconds ?? Date.now() / 1000;
  const tolerance = opts.toleranceSeconds ?? 300;
  if (!Number.isFinite(now) || !Number.isFinite(tolerance) || tolerance <= 0 || Math.abs(now - Number(stamp)) > tolerance) {
    throw new BotPushError("push timestamp outside allowed window");
  }
  const bytes = typeof body === "string" ? Buffer.from(body, "utf8") : Buffer.from(body);
  const expected = createHmac("sha256", secret).update(`${stamp}.`).update(bytes).digest();
  if (!timingSafeEqual(expected, Buffer.from(sig.slice(7), "hex"))) throw new BotPushError("push signature mismatch");
  let event: PluginEvent;
  try { event = JSON.parse(bytes.toString("utf8")) as PluginEvent; }
  catch { throw new BotPushError("invalid push JSON"); }
  if (!event || event.type !== "message" || !Number.isSafeInteger(event.id) || event.id <= 0 ||
      h["x-nowork-delivery"] !== String(event.id) || h["x-nowork-event"] !== "bot.message" ||
      !event.target || typeof event.target !== "object" || Array.isArray(event.target) || !Array.isArray(event.message)) {
    throw new BotPushError("invalid push event or delivery id");
  }
  return event;
}
