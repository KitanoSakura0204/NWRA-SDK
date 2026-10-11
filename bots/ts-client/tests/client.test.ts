import { test } from "node:test";
import assert from "node:assert/strict";
import { BotClient, BotApiError, BotTransportError } from "../src/index.js";

interface FakeCall {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: unknown;
}

test("malformed and HTTP-failed envelopes are transport failures", async () => {
  for (const [body, status] of [[[], 200], [{ code: false, data: {} }, 200], [{ code: 0 }, 200], [{ code: 0, data: {} }, 503]] as const) {
    const client = new BotClient({ baseUrl: "https://account.example", token: "t",
      fetch: async () => new Response(JSON.stringify(body), { status }) });
    await assert.rejects(client.commands(), BotTransportError);
  }
});

test("deadline includes reading the response body", async () => {
  const client = new BotClient({ baseUrl: "https://account.example", token: "t", timeoutMs: 5,
    fetch: (async (_url, init) => ({ status: 200, ok: true,
      json: () => new Promise((_resolve, reject) => init!.signal!.addEventListener("abort", () => reject(new Error("aborted")), { once: true })),
    })) as typeof fetch });
  await assert.rejects(client.commands(), BotTransportError);
});

function fakeFetch(responses: Record<string, { code: number; message?: string; data?: unknown }>, calls: FakeCall[]) {
  return (async (input: string | URL, init?: RequestInit) => {
    const url = String(input);
    const path = new URL(url).pathname;
    calls.push({
      url,
      method: init?.method ?? "GET",
      headers: (init?.headers as Record<string, string>) ?? {},
      body: init?.body ? JSON.parse(String(init.body)) : undefined,
    });
    const env = responses[path] ?? { code: 1, message: "not stubbed" };
    return new Response(JSON.stringify(env), { status: 200 });
  }) as typeof fetch;
}

test("bind sends the token header and parses data", async () => {
  const calls: FakeCall[] = [];
  const fetchImpl = fakeFetch(
    { "/api/bot/v1/bind": { code: 0, data: { userId: 1, uid: 100001, nickname: "Ada", level: 2 } } },
    calls,
  );
  const client = new BotClient({ baseUrl: "https://nowork.example.com", token: "tok123", fetch: fetchImpl });
  const result = await client.bind("qq", "10001", "ABC123", "Ada");
  assert.equal(result.nickname, "Ada");
  assert.equal(calls[0].headers.Authorization, "Bot tok123");
  assert.deepEqual(calls[0].body, { platform: "qq", platformUserId: "10001", platformName: "Ada", code: "ABC123" });
});

test("lookup sends query params and GET", async () => {
  const calls: FakeCall[] = [];
  const fetchImpl = fakeFetch({ "/api/bot/v1/lookup": { code: 0, data: { bound: true, nickname: "Ada", level: 2 } } }, calls);
  const client = new BotClient({ baseUrl: "https://nowork.example.com/", token: "t", fetch: fetchImpl });
  const result = await client.lookup("qq", "10001");
  assert.equal(result.bound, true);
  assert.equal(calls[0].method, "GET");
  assert.ok(calls[0].url.includes("platform=qq"));
  assert.ok(calls[0].url.includes("platformUserId=10001"));
});

test("a non-zero code throws BotApiError", async () => {
  const calls: FakeCall[] = [];
  const fetchImpl = fakeFetch({ "/api/bot/v1/unbind": { code: 404, message: "not bound" } }, calls);
  const client = new BotClient({ baseUrl: "https://nowork.example.com", token: "t", fetch: fetchImpl });
  await assert.rejects(() => client.unbind("qq", "x"), (err: unknown) => {
    assert.ok(err instanceof BotApiError);
    assert.equal(err.code, 404);
    assert.equal(err.message, "not bound");
    return true;
  });
});

test("command posts the chat context and returns reply + handled", async () => {
  const calls: FakeCall[] = [];
  const fetchImpl = fakeFetch(
    { "/api/bot/v1/command": { code: 0, data: { handled: true, reply: [{ type: "text", data: { text: "ok" } }] } } },
    calls,
  );
  const client = new BotClient({ baseUrl: "https://nowork.example.com", token: "t", fetch: fetchImpl });
  const result = await client.command({ platform: "qq", platformUserId: "1", chatType: "group", chatId: "g1", text: "/me" });
  assert.equal(result.handled, true);
  assert.equal(result.reply[0].data?.text, "ok");
});

test("pollEvents and ackEvents hit the right paths", async () => {
  const calls: FakeCall[] = [];
  const fetchImpl = fakeFetch(
    {
      "/api/bot/v1/events": { code: 0, data: { events: [{ type: "message", id: 7, botId: 1, target: { chatType: "private", userId: "1" }, message: [], time: 0 }] } },
      "/api/bot/v1/events/ack": { code: 0, data: { acked: 1 } },
    },
    calls,
  );
  const client = new BotClient({ baseUrl: "https://nowork.example.com", token: "t", fetch: fetchImpl });
  const polled = await client.pollEvents();
  assert.equal(polled.events[0].id, 7);
  const acked = await client.ackEvents([7]);
  assert.equal(acked.acked, 1);
  assert.deepEqual(calls[1].body, { ids: [7] });
});

test("polling preserves optional owning-site metadata without changing acknowledgements", async () => {
  for (const siteId of [undefined, 0, 42]) {
    const calls: FakeCall[] = [];
    const client = new BotClient({ baseUrl: "https://account.example", token: "site-bot-token",
      fetch: fakeFetch({
        "/api/bot/v1/events": { code: 0, data: { events: [
          { type: "message", id: 7, botId: 2, siteId, target: { chatType: "private", userId: "member" }, message: [], time: 0 },
        ] } },
        "/api/bot/v1/events/ack": { code: 0, data: { acked: 1 } },
      }, calls) });
    const events = await client.pollEvents();
    assert.equal(events.events[0].siteId, siteId);
    await client.ackEvents([events.events[0].id]);
    assert.equal(calls[0].headers.Authorization, "Bot site-bot-token");
    assert.deepEqual(calls[1].body, { ids: [7] });
  }
});
