import WebSocket from "ws";
import type { Message, PluginEvent, Target } from "./types.js";

export interface EventStreamOptions {
  /** Site base URL, e.g. "https://nowork.example.com". */
  baseUrl: string;
  token: string;
  /** Called for each queued notification; return (or resolve) to ack it. */
  onEvent: (ev: PluginEvent) => void | Promise<void>;
  /** Called on connect / disconnect, for logging. */
  onStatus?: (status: "connecting" | "open" | "closed", detail?: string) => void;
  /** Delivery errors leave the event unacknowledged for the server's next retry. */
  onError?: (error: unknown) => void;
  /** Override the WebSocket implementation (tests inject a fake server). */
  WebSocketImpl?: typeof WebSocket;
  /** Backoff schedule in ms for reconnects (default: 1s, 2s, 5s, 10s, then 30s). */
  backoffMs?: number[];
}

const DEFAULT_BACKOFF = [1000, 2000, 5000, 10000, 30000];
const DEFAULT_HEARTBEAT_SECONDS = 60;

function heartbeatDelay(value: unknown): number {
  const seconds = typeof value === "number" && Number.isFinite(value) && value > 0
    ? Math.min(DEFAULT_HEARTBEAT_SECONDS, Math.max(5, value))
    : DEFAULT_HEARTBEAT_SECONDS;
  return seconds * 1000;
}

function wsUrl(baseUrl: string, token: string): string {
  const u = new URL(baseUrl.replace(/\/+$/, "") + "/api/bot/v1/events");
  u.protocol = u.protocol === "https:" ? "wss:" : "ws:";
  u.searchParams.set("token", token);
  return u.toString();
}

/**
 * Connects to GET /api/bot/v1/events as a WebSocket, acking each delivered event, and
 * reconnects with backoff on drop. Returns a function that closes the stream for good.
 *
 * Protocol (Backend/internal/modules/bots/ws.go, hub.go): the server sends
 * {"type":"hello","botId","heartbeat"} then {"type":"message","id",...} per queued
 * notification; the client answers {"type":"ack","ids":[id]} (also accepts {"id":n}), and
 * the client sends {"type":"ping"} at the hello heartbeat interval (seconds). Control
 * ping frames alone do not refresh the server's application-message read deadline.
 */
export function connectEvents(opts: EventStreamOptions): () => void {
  const backoff = opts.backoffMs ?? DEFAULT_BACKOFF;
  if (!backoff.length || backoff.some((delay) => !Number.isFinite(delay) || delay <= 0)) throw new RangeError("backoffMs must contain positive delays");
  const Impl = opts.WebSocketImpl ?? WebSocket;
  let closed = false;
  let attempt = 0;
  let socket: WebSocket | null = null;
  let stopCurrent: (() => void) | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;

  function scheduleReconnect() {
    if (closed || retryTimer !== null) return;
    const delay = backoff[Math.min(attempt, backoff.length - 1)];
    attempt++;
    retryTimer = setTimeout(connect, delay);
  }

  function connect() {
    retryTimer = null;
    if (closed) return;
    opts.onStatus?.("connecting");
    const ws = new Impl(wsUrl(opts.baseUrl, opts.token));
    socket = ws;
    let live = true;
    let finished = false;
    let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
    const active = () => live && !closed && socket === ws;

    function clearHeartbeat() {
      if (heartbeatTimer !== null) clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }

    function stopWork() {
      live = false;
      clearHeartbeat();
      ws.removeEventListener("open", onOpen);
      ws.removeEventListener("message", onMessage);
    }

    function closeSocket() {
      stopWork();
      // Keep the error/close handlers until close completes, including an abort
      // during the opening handshake (ws emits error before close in that case).
      ws.close();
    }

    function startHeartbeat(value: unknown) {
      clearHeartbeat();
      if (!active()) return;
      heartbeatTimer = setInterval(() => {
        if (!active()) { clearHeartbeat(); return; }
        try {
          ws.send(JSON.stringify({ type: "ping" }));
        } catch (error) {
          closeSocket();
          opts.onError?.(error);
        }
      }, heartbeatDelay(value));
    }

    function onOpen() {
      if (!active()) return;
      attempt = 0;
      startHeartbeat(undefined);
      opts.onStatus?.("open");
    }

    function onMessage(ev: { data: unknown }) {
      if (!active()) return;
      void handleFrame(ws, String(ev.data), opts, startHeartbeat, active)
        .catch((error: unknown) => opts.onError?.(error));
    }

    function onClose() {
      if (finished) return;
      finished = true;
      stopWork();
      ws.removeEventListener("close", onClose);
      ws.removeEventListener("error", onError);
      if (socket === ws) { socket = null; stopCurrent = null; }
      opts.onStatus?.("closed");
      scheduleReconnect();
    }

    function onError() {
      // "close" always follows; nothing to do here beyond letting it fire.
    }

    stopCurrent = closeSocket;
    ws.addEventListener("open", onOpen);
    ws.addEventListener("message", onMessage);
    ws.addEventListener("close", onClose);
    ws.addEventListener("error", onError);
  }

  connect();

  return () => {
    if (closed) return;
    closed = true;
    if (retryTimer !== null) clearTimeout(retryTimer);
    retryTimer = null;
    stopCurrent?.();
  };
}

async function handleFrame(ws: WebSocket, raw: string, opts: EventStreamOptions,
  onHello: (heartbeat: unknown) => void, active: () => boolean): Promise<void> {
  let frame: { type?: string; heartbeat?: unknown; id?: number; botId?: number; siteId?: number; kind?: string; target?: Target; message?: Message; time?: number };
  try {
    frame = JSON.parse(raw);
  } catch {
    return;
  }
  if (!frame || typeof frame !== "object") return;
  switch (frame.type) {
    case "hello":
      onHello(frame.heartbeat);
      return;
    case "ping":
      ws.send(JSON.stringify({ type: "pong" }));
      return;
    case "message": {
      if (!Number.isSafeInteger(frame.id) || (frame.id ?? 0) <= 0) return;
      const ev = frame as PluginEvent;
      await opts.onEvent(ev);
      // A delivery that completed after disconnect stays pending for redelivery;
      // never acknowledge it through a different or already closed connection.
      if (active()) ws.send(JSON.stringify({ type: "ack", ids: [ev.id] }));
      return;
    }
    default:
      return;
  }
}
