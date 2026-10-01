import { test } from "node:test";
import assert from "node:assert/strict";
import { BotApiError } from "@lounge/bot-client";
import { chatAllowed, doBind, doMe, doPassthrough, doUnbind } from "../src/logic.js";

/** A fake BotClient: same method shapes as @lounge/bot-client's BotClient, no network. */
class FakeClient {
  calls: Record<string, unknown>[] = [];
  bindResult: unknown = { userId: 1, uid: 100001, nickname: "Ada", level: 2 };
  unbindResult: unknown = { unbound: true };
  lookupResult: unknown = { bound: true, nickname: "Ada", level: 2, levelName: "见习" };
  commandResult: unknown = { handled: false, reply: [] };

  async bind(platform: string, userId: string, code: string, name?: string) {
    this.calls.push({ op: "bind", platform, userId, code, name });
    if (this.bindResult instanceof Error) throw this.bindResult;
    return this.bindResult as never;
  }
  async unbind(platform: string, userId: string) {
    this.calls.push({ op: "unbind", platform, userId });
    if (this.unbindResult instanceof Error) throw this.unbindResult;
    return this.unbindResult as never;
  }
  async lookup(platform: string, userId: string) {
    this.calls.push({ op: "lookup", platform, userId });
    if (this.lookupResult instanceof Error) throw this.lookupResult;
    return this.lookupResult as never;
  }
  async command(input: Record<string, unknown>) {
    this.calls.push({ op: "command", ...input });
    if (this.commandResult instanceof Error) throw this.commandResult;
    return this.commandResult as never;
  }
}

test("doBind without a code asks for usage and makes no call", async () => {
  const client = new FakeClient();
  const reply = await doBind(client as never, "qq", "1", "Ada", "");
  assert.match(reply, /用法/);
  assert.equal(client.calls.length, 0);
});

test("doBind success includes the nickname", async () => {
  const client = new FakeClient();
  const reply = await doBind(client as never, "qq", "1", "Ada", "ABC123");
  assert.match(reply, /Ada/);
  assert.deepEqual(client.calls[0], { op: "bind", platform: "qq", userId: "1", code: "ABC123", name: "Ada" });
});

test("doBind surfaces a BotApiError message", async () => {
  const client = new FakeClient();
  client.bindResult = new BotApiError(400, "绑定码无效");
  const reply = await doBind(client as never, "qq", "1", "Ada", "BAD");
  assert.match(reply, /绑定码无效/);
});

test("doUnbind reports not-found", async () => {
  const client = new FakeClient();
  client.unbindResult = { unbound: false };
  const reply = await doUnbind(client as never, "qq", "1");
  assert.match(reply, /未找到/);
});

test("doMe reports unbound", async () => {
  const client = new FakeClient();
  client.lookupResult = { bound: false };
  const reply = await doMe(client as never, "qq", "1");
  assert.match(reply, /尚未绑定/);
});

test("doMe includes realname status when present", async () => {
  const client = new FakeClient();
  client.lookupResult = { bound: true, nickname: "Ada", level: 3, levelName: "资深", realnameVerified: true };
  const reply = await doMe(client as never, "qq", "1");
  assert.match(reply, /Ada/);
  assert.match(reply, /资深/);
  assert.match(reply, /已通过/);
});

test("doPassthrough returns null when not handled", async () => {
  const client = new FakeClient();
  const reply = await doPassthrough(client as never, "qq", "1", "Ada", "group", "g1", "随便说点什么");
  assert.equal(reply, null);
});

test("doPassthrough renders the reply when handled", async () => {
  const client = new FakeClient();
  client.commandResult = { handled: true, reply: [{ type: "text", data: { text: "ok" } }] };
  const reply = await doPassthrough(client as never, "qq", "1", "Ada", "group", "g1", "/help");
  assert.equal(reply, "ok");
});

test("doPassthrough swallows a BotApiError as null", async () => {
  const client = new FakeClient();
  client.commandResult = new BotApiError(500, "boom");
  const reply = await doPassthrough(client as never, "qq", "1", "Ada", "group", "g1", "/help");
  assert.equal(reply, null);
});

test("chatAllowed", () => {
  assert.equal(chatAllowed([], "anything"), true);
  assert.equal(chatAllowed(["g1"], "g1"), true);
  assert.equal(chatAllowed(["g1"], "g2"), false);
});
