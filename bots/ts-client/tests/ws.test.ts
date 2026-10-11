import { test } from "node:test";
import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { connectEvents } from "../src/ws.js";
import type { PluginEvent } from "../src/types.js";

/** A minimal fake WebSocket: enough of the DOM WebSocket surface for ws.ts. */
class FakeSocket extends EventEmitter {
  sent: string[] = [];
  sendError: Error | null = null;
  static last: FakeSocket | null = null;
  constructor(public url: string) {
    super();
    FakeSocket.last = this;
  }
  addEventListener(name: string, fn: (...a: unknown[]) => void) {
    this.on(name, fn);
  }
  removeEventListener(name: string, fn: (...a: unknown[]) => void) {
    this.off(name, fn);
  }
  send(data: string) {
    if (this.sendError) throw this.sendError;
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

test("acks a delivered message and answers an application ping", async (t) => {
  const received: PluginEvent[] = [];
  const close = connectEvents({
    baseUrl: "https://nowork.example.com",
    token: "tok",
    onEvent: (ev) => {
      received.push(ev);
    },
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
  });
  t.after(close);
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

test("reconnects with backoff after a close", async (t) => {
  const close = connectEvents({
    baseUrl: "https://nowork.example.com",
    token: "tok",
    onEvent: () => {},
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
    backoffMs: [5, 5],
  });
  t.after(close);
  const first = FakeSocket.last!;
  first.open();
  first.close();
  await new Promise((r) => setTimeout(r, 30));
  assert.notEqual(FakeSocket.last, first);
  close();
});

test("failed adapter delivery is reported and never acknowledged", async (t) => {
  const errors: unknown[] = [];
  const stop = connectEvents({ baseUrl: "https://account.example", token: "t",
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
    onEvent: async () => { throw new Error("adapter offline"); }, onError: (err) => errors.push(err) });
  t.after(stop);
  const socket = FakeSocket.last!;
  socket.frame({ type: "message", id: 7 });
  await new Promise((resolve) => setTimeout(resolve, 10));
  assert.equal(errors.length, 1);
  assert.deepEqual(socket.sent, []);
  stop();
});

test("WebSocket preserves old events and optional owning-site metadata", async (t) => {
  const received: PluginEvent[] = [];
  const stop = connectEvents({ baseUrl: "https://account.example", token: "site-bot-token",
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
    onEvent: (event) => { received.push(event); } });
  t.after(stop);
  const socket = FakeSocket.last!;
  for (const [index, siteId] of [undefined, 0, 42].entries()) {
    socket.frame({ type: "message", id: index + 1, botId: 2, siteId,
      target: { chatType: "private", userId: "member" }, message: [], time: 0 });
  }
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(received.map((event) => event.siteId), [undefined, 0, 42]);
  assert.deepEqual(socket.sent.map((frame) => JSON.parse(frame)), [
    { type: "ack", ids: [1] }, { type: "ack", ids: [2] }, { type: "ack", ids: [3] },
  ]);
  stop();
});

test("idle stream sends negotiated JSON heartbeats and repeated hello replaces its timer", (t) => {
  t.mock.timers.enable({ apis: ["setInterval", "setTimeout"] });
  const stop = connectEvents({ baseUrl: "https://account.example", token: "t",
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket, onEvent: () => {} });
  t.after(stop);
  const socket = FakeSocket.last!;
  socket.open();
  socket.frame({ type: "hello", heartbeat: 10 });
  t.mock.timers.tick(9_999);
  assert.deepEqual(socket.sent, []);
  t.mock.timers.tick(1);
  assert.deepEqual(socket.sent.map((frame) => JSON.parse(frame)), [{ type: "ping" }]);
  socket.frame({ type: "hello", heartbeat: 5 });
  socket.frame({ type: "hello", heartbeat: 5 });
  t.mock.timers.tick(5_000);
  assert.deepEqual(socket.sent.map((frame) => JSON.parse(frame)), [{ type: "ping" }, { type: "ping" }]);
  stop();
  t.mock.timers.tick(120_000);
  assert.equal(socket.sent.length, 2);
  assert.equal(FakeSocket.last, socket);
  assert.equal(socket.eventNames().length, 0);
});

test("heartbeat negotiation bounds short/large values and falls back for invalid or missing values", (t) => {
  t.mock.timers.enable({ apis: ["setInterval", "setTimeout"] });
  for (const [heartbeat, delay] of [[1, 5_000], [20, 20_000], [1e12, 60_000],
    [0, 60_000], [-1, 60_000], [true, 60_000], ["1", 60_000], [null, 60_000], [undefined, 60_000]] as const) {
    const stop = connectEvents({ baseUrl: "https://account.example", token: "t",
      WebSocketImpl: FakeSocket as unknown as typeof WebSocket, onEvent: () => {} });
    try {
      const socket = FakeSocket.last!;
      socket.open();
      socket.frame({ type: "hello", heartbeat });
      t.mock.timers.tick(delay - 1);
      assert.equal(socket.sent.length, 0);
      t.mock.timers.tick(1);
      assert.deepEqual(socket.sent.map((frame) => JSON.parse(frame)), [{ type: "ping" }]);
    } finally { stop(); }
  }
});

test("reconnect replaces the old heartbeat and removes old connection listeners", (t) => {
  t.mock.timers.enable({ apis: ["setInterval", "setTimeout"] });
  const stop = connectEvents({ baseUrl: "https://account.example", token: "t", backoffMs: [1_000],
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket, onEvent: () => {} });
  t.after(stop);
  const first = FakeSocket.last!;
  first.open();
  first.frame({ type: "hello", heartbeat: 5 });
  first.close();
  first.close();
  t.mock.timers.tick(1_000);
  const second = FakeSocket.last!;
  assert.notEqual(second, first);
  assert.equal(first.eventNames().length, 0);
  second.open();
  second.frame({ type: "hello", heartbeat: 5 });
  first.frame({ type: "message", id: 8 });
  t.mock.timers.tick(5_000);
  assert.deepEqual(first.sent, []);
  assert.deepEqual(second.sent.map((frame) => JSON.parse(frame)), [{ type: "ping" }]);
  assert.equal(FakeSocket.last, second);
  stop();
  t.mock.timers.tick(120_000);
  assert.equal(second.sent.length, 1);
});

test("heartbeat send failure closes the connection and reconnects once", (t) => {
  t.mock.timers.enable({ apis: ["setInterval", "setTimeout"] });
  const errors: unknown[] = [];
  const stop = connectEvents({ baseUrl: "https://account.example", token: "t", backoffMs: [1_000],
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket, onEvent: () => {}, onError: (error) => errors.push(error) });
  t.after(stop);
  const first = FakeSocket.last!;
  first.open();
  first.frame({ type: "hello", heartbeat: 5 });
  first.sendError = new Error("closed transport");
  t.mock.timers.tick(5_000);
  assert.deepEqual(errors, [first.sendError]);
  assert.equal(first.eventNames().length, 0);
  t.mock.timers.tick(1_000);
  const second = FakeSocket.last!;
  assert.notEqual(second, first);
  second.open();
  second.frame({ type: "hello", heartbeat: 5 });
  t.mock.timers.tick(5_000);
  assert.deepEqual(second.sent.map((frame) => JSON.parse(frame)), [{ type: "ping" }]);
  assert.equal(errors.length, 1);
});

test("an in-flight delivery never sends its acknowledgement after disconnect or onto the replacement socket", async (t) => {
  t.mock.timers.enable({ apis: ["setInterval", "setTimeout"] });
  let finish!: () => void;
  const delivered = new Promise<void>((resolve) => { finish = resolve; });
  const stop = connectEvents({ baseUrl: "https://account.example", token: "t", backoffMs: [1_000],
    WebSocketImpl: FakeSocket as unknown as typeof WebSocket, onEvent: () => delivered });
  t.after(stop);
  const first = FakeSocket.last!;
  first.open();
  first.frame({ type: "message", id: 42 });
  first.close();
  t.mock.timers.tick(1_000);
  const second = FakeSocket.last!;
  second.open();
  finish();
  await delivered;
  await Promise.resolve();
  assert.deepEqual(first.sent, []);
  assert.deepEqual(second.sent, []);
});
