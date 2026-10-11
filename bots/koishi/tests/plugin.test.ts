import { test } from "node:test";
import assert from "node:assert/strict";
import { BotClient } from "@nowork/bot-client";
import { Universal } from "@koishijs/core";
import type { Context, Session } from "koishi";
import { apply, selectNotificationBot } from "../src/index.js";

type SessionInput = Partial<Pick<Session, "userId" | "channelId" | "platform" | "username" | "isDirect" | "guildId" | "content">>;
type Action = (argv: { session?: SessionInput }, code?: string) => Promise<unknown>;
type Middleware = (session: SessionInput, next: () => Promise<string>) => Promise<unknown>;

/** Registers the real plugin's handlers without emitting ready or opening a socket. */
function plugin(allowedChats: string[] = []) {
  const actions = new Map<string, Action>();
  const hooks = new Map<string, () => void>();
  let middleware!: Middleware;
  const ctx = {
    bots: [],
    command(name: string) {
      return { action(handler: Action) { actions.set(name.split(" ")[0], handler); } };
    },
    middleware(handler: Middleware) { middleware = handler; },
    on(name: string, handler: () => void) { hooks.set(name, handler); },
    logger() { return { warn() {}, debug() {} }; },
  };
  apply(ctx as unknown as Context, { baseUrl: "https://account.example", botToken: "offline-fixture", allowedChats });
  return { actions, middleware, hooks };
}

test("real command handlers never call the Bot API without a supplied user identity", async (t) => {
  const calls: string[] = [];
  t.mock.method(BotClient.prototype, "bind", async () => { calls.push("bind"); return { nickname: "Ada", level: 1 }; });
  t.mock.method(BotClient.prototype, "unbind", async () => { calls.push("unbind"); return { unbound: true }; });
  t.mock.method(BotClient.prototype, "lookup", async () => { calls.push("lookup"); return { bound: false }; });
  t.mock.method(BotClient.prototype, "command", async () => { calls.push("command"); return { handled: false, reply: [] }; });
  const p = plugin();
  let nextCalls = 0;
  for (const session of [undefined,
    { channelId: "room", platform: "onebot", content: "/help" },
    { userId: "", channelId: "room", platform: "onebot", content: "/help" },
  ]) {
    for (const command of ["bind", "unbind", "me"]) {
      assert.equal(await p.actions.get(command)!({ session }, "ABC123"), undefined);
    }
    if (session) {
      assert.equal(await p.middleware(session, async () => { nextCalls++; return "next-handler"; }), "next-handler");
    }
  }
  assert.deepEqual(calls, []);
  assert.equal(nextCalls, 2);
});

test("a real user identity supplies the existing private-chat fallback and is forwarded unchanged", async (t) => {
  const calls: unknown[] = [];
  t.mock.method(BotClient.prototype, "bind", async (platform, userId, code, name) => {
    calls.push({ platform, userId, code, name });
    return { userId: 1, uid: 1, nickname: "Ada", level: 1 };
  });
  t.mock.method(BotClient.prototype, "command", async (input) => {
    calls.push(input);
    return { handled: true, reply: [{ type: "text", data: { text: "handled" } }] };
  });
  const p = plugin(["member-id"]);
  const session = { userId: "member-id", platform: "onebot", username: "Ada", isDirect: true, content: "/help" };
  assert.match(String(await p.actions.get("bind")!({ session }, "ABC123")), /Ada/);
  assert.equal(await p.middleware(session, async () => "next-handler"), "handled");
  assert.deepEqual(calls, [
    { platform: "onebot11", userId: "member-id", code: "ABC123", name: "Ada" },
    { platform: "onebot11", platformUserId: "member-id", platformName: "Ada", chatType: "private", chatId: "member-id", text: "/help" },
  ]);
});

test("notification selection uses Koishi's real ONLINE enum and excludes connecting or offline bots", () => {
  const online = { selfId: "ready", status: Universal.Status.ONLINE };
  const bots = [
    { selfId: "offline", status: Universal.Status.OFFLINE },
    { selfId: "connecting", status: Universal.Status.CONNECT },
    { selfId: "reconnecting", status: Universal.Status.RECONNECT },
    { selfId: "disconnecting", status: Universal.Status.DISCONNECT },
    online,
  ];
  assert.equal(selectNotificationBot(bots), online);
  for (const bot of bots.slice(0, -1)) {
    assert.throws(() => selectNotificationBot([bot]), /notificationBotId/);
  }
});

test("an explicitly selected offline bot never falls back to a different online bot", () => {
  const offline = { selfId: "wanted", status: Universal.Status.OFFLINE };
  const online = { selfId: "other", status: Universal.Status.ONLINE };
  assert.throws(() => selectNotificationBot([offline, online], "wanted"), /notificationBotId/);
  assert.throws(() => selectNotificationBot([online], "missing"), /notificationBotId/);
});

test("multiple online bots require selection and return exactly that bot", () => {
  const first = { selfId: "one", status: Universal.Status.ONLINE };
  const second = { selfId: "two", status: Universal.Status.ONLINE };
  assert.throws(() => selectNotificationBot([first, second]), /notificationBotId/);
  assert.equal(selectNotificationBot([first, second], "two"), second);
});
