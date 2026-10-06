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
  /** Override the WebSocket implementation (tests inject a fake server). */
  WebSocketImpl?: typeof WebSocket;
  /** Backoff schedule in ms for reconnects (default: 1s, 2s, 5s, 10s, then 30s). */
  backoffMs?: number[];
}

const DEFAULT_BACKOFF = [1000, 2000, 5000, 10000, 30000];

function wsUrl(baseUrl: string, token: string): string {
  const u = new URL("/api/bot/v1/events", baseUrl.replace(/\/+$/, ""));
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
 * {"type":"ping"} is answered with {"type":"pong"} (we rely on the library's own ping/pong
 * instead and only reply if the server sends an application-level ping frame as text).
 */
export function connectEvents(opts: EventStreamOptions): () => void {
  const backoff = opts.backoffMs ?? DEFAULT_BACKOFF;
  const Impl = opts.WebSocketImpl ?? WebSocket;
  let closed = false;
  let attempt = 0;
  let socket: WebSocket | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;

  function scheduleReconnect() {
    if (closed) return;
    const delay = backoff[Math.min(attempt, backoff.length - 1)];
    attempt++;
    retryTimer = setTimeout(connect, delay);
  }

  function connect() {
    if (closed) return;
    opts.onStatus?.("connecting");
    const ws = new Impl(wsUrl(opts.baseUrl, opts.token));
    socket = ws;
    ws.addEventListener("open", () => {
      attempt = 0;
      opts.onStatus?.("open");
    });
    ws.addEventListener("message", (ev: { data: unknown }) => {
      void handleFrame(ws, String(ev.data), opts);
    });
    ws.addEventListener("close", () => {
      opts.onStatus?.("closed");
      if (!closed) scheduleReconnect();
    });
    ws.addEventListener("error", () => {
      // "close" always follows; nothing to do here beyond letting it fire.
    });
  }

  connect();

  return () => {
    closed = true;
    if (retryTimer) clearTimeout(retryTimer);
    socket?.close();
  };
}

async function handleFrame(ws: WebSocket, raw: string, opts: EventStreamOptions): Promise<void> {
  let frame: { type?: string; id?: number; botId?: number; kind?: string; target?: Target; message?: Message; time?: number };
  try {
    frame = JSON.parse(raw);
  } catch {
    return;
  }
  switch (frame.type) {
    case "hello":
      return;
    case "ping":
      ws.send(JSON.stringify({ type: "pong" }));
      return;
    case "message": {
      const ev = frame as PluginEvent;
      await opts.onEvent(ev);
      ws.send(JSON.stringify({ type: "ack", ids: [ev.id] }));
      return;
    }
    default:
      return;
  }
}
