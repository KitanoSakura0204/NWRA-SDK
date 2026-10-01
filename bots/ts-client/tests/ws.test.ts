import { test } from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { connectEvents } from "../src/ws.js";
import type { PluginEvent } from "../src/types.js";

/** A minimal fake WebSocket: enough of the DOM WebSocket surface for ws.ts. */
class FakeSocket extends EventEmitter {
  sent: string[] = [];
  static last: FakeSocket | null = null;
  constructor(public url: string) {
    super();
    FakeSocket.last = this;
  }
  addEventListener(name: string, fn: (...a: unknown[]) => void) {
    this.on(name, fn);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    this.emit("close");
  }
  open() {
    this.emit("open");
  }
  frame(obj: unknown) {
    this.emit("message", { data: JSON.stringify(obj) });
  }
}

test("acks a delivered message and answers an application ping", async () => {
  const received: PluginEvent[] = [];
  const close = connectEvents({
    baseUrl: "https://lounge.example.com",
    token: "tok",
    onEvent: (ev) => {
      received.push(ev);
    },
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
  });
  const sock = FakeSocket.last!;
  assert.ok(sock.url.includes("token=tok"));
  sock.open();
  sock.frame({ type: "hello", botId: 1, heartbeat: 60 });
  sock.frame({ type: "message", id: 42, botId: 1, target: { chatType: "private", userId: "1" }, message: [], time: 0 });
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(received.length, 1);
  assert.equal(received[0].id, 42);
  assert.deepEqual(JSON.parse(sock.sent[0]), { type: "ack", ids: [42] });

  sock.frame({ type: "ping" });
  await new Promise((r) => setTimeout(r, 10));
  assert.deepEqual(JSON.parse(sock.sent[1]), { type: "pong" });

  close();
});

test("reconnects with backoff after a close", async () => {
  const close = connectEvents({
    baseUrl: "https://lounge.example.com",
    token: "tok",
    onEvent: () => {},
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
    backoffMs: [5, 5],
  });
  const first = FakeSocket.last!;
  first.open();
  first.close();
  await new Promise((r) => setTimeout(r, 30));
  assert.notEqual(FakeSocket.last, first);
  close();
});
