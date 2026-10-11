import { test } from "node:test";
import assert from "node:assert/strict";
import type { PluginEvent, Target } from "@nowork/bot-client";
import { notificationHandlers } from "../src/index.js";

function event(target: Target): PluginEvent {
  return { type: "message", id: 42, botId: 1, target, message: [{ type: "text", data: { text: "security notice" } }], time: 0 };
}

test("group delivery resolves the exact room ID and completes only after its public say promise", async () => {
  const calls: unknown[] = [];
  let started!: () => void;
  let finish!: () => void;
  const sending = new Promise<void>((resolve) => { started = resolve; });
  const delivered = new Promise<void>((resolve) => { finish = resolve; });
  const handlers = notificationHandlers({
    Room: { async find(query) {
      calls.push(["room.find", query]);
      return { async say(text) { calls.push(["room.say", text]); started(); await delivered; } };
    } },
    Contact: { async find(query) { calls.push(["contact.find", query]); return undefined; } },
    emitError(error) { throw error; },
  });
  let completed = false;
  const result = handlers.onEvent(event({ chatType: "group", chatId: "room-id", userId: "sender-id" }))
    .then(() => { completed = true; });
  await sending;
  assert.equal(completed, false);
  assert.deepEqual(calls, [["room.find", { id: "room-id" }], ["room.say", "security notice"]]);
  // Wechaty allows successful say() to return void; a Message object is optional.
  finish();
  await result;
  assert.equal(completed, true);
});

test("private delivery prefers userId and supports the documented contact chatId fallback", async () => {
  const calls: unknown[] = [];
  const handlers = notificationHandlers({
    Room: { async find(query) { calls.push(["room.find", query]); return undefined; } },
    Contact: { async find(query) {
      calls.push(["contact.find", query]);
      return { async say(text) { calls.push(["contact.say", text]); } };
    } },
    emitError(error) { throw error; },
  });
  await handlers.onEvent(event({ chatType: "private", chatId: "different-chat", userId: "contact-id" }));
  await handlers.onEvent(event({ chatType: "private", chatId: "fallback-id" }));
  assert.deepEqual(calls, [
    ["contact.find", { id: "contact-id" }], ["contact.say", "security notice"],
    ["contact.find", { id: "fallback-id" }], ["contact.say", "security notice"],
  ]);
});

test("missing destinations reject before lookup and never reinterpret a group sender as a room", async () => {
  let lookedUp = false;
  const handlers = notificationHandlers({
    Room: { async find() { lookedUp = true; return undefined; } },
    Contact: { async find() { lookedUp = true; return undefined; } },
    emitError(error) { throw error; },
  });
  await assert.rejects(handlers.onEvent(event({ chatType: "private" })), /destination is missing/);
  await assert.rejects(handlers.onEvent(event({ chatType: "group", userId: "sender-only" })), /requires chatId/);
  assert.equal(lookedUp, false);
});

test("unavailable public find results reject the event callback so the shared client cannot acknowledge them", async () => {
  for (const chatType of ["group", "private"]) {
    const handlers = notificationHandlers({
      Room: { async find() { return undefined; } },
      Contact: { async find() { return undefined; } },
      emitError(error) { throw error; },
    });
    await assert.rejects(handlers.onEvent(event({ chatType, chatId: "unavailable-id" })), /unavailable/);
  }
});

test("lookup and send failures propagate unchanged and use public emitError for reporting", async () => {
  for (const chatType of ["group", "private"]) {
    for (const step of ["find", "say"]) {
      const failure = new Error(`${chatType} ${step} failed`);
      const errors: unknown[] = [];
      const finder = {
        async find() {
          if (step === "find") throw failure;
          return { async say() { throw failure; } };
        },
      };
      const handlers = notificationHandlers({ Room: finder, Contact: finder, emitError(error) { errors.push(error); } });
      await assert.rejects(handlers.onEvent(event({ chatType, chatId: "target-id" })), (error) => error === failure);
      handlers.onError(failure);
      assert.deepEqual(errors, [failure]);
    }
  }
});
