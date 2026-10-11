import { test } from "node:test";
import assert from "node:assert/strict";
import { createHmac } from "node:crypto";
import { verifyPush, BotPushError } from "../src/index.js";

function fixture(siteId?: number) {
  const body = JSON.stringify({ type: "message", id: 7, botId: 2, siteId, target: { chatType: "private", userId: "x" }, message: [], time: 1000 });
  const headers: Record<string, string> = {
    "X-Nowork-Timestamp": "1000", "X-Nowork-Delivery": "7", "X-Nowork-Event": "bot.message",
    "X-Nowork-Signature": "sha256=" + createHmac("sha256", "secret").update("1000." + body).digest("hex"),
  };
  return { body, headers };
}

test("verifies signed push bytes and mixed-case headers", () => {
  const { body, headers } = fixture();
  assert.equal(verifyPush("secret", new Headers(headers), body, { nowSeconds: 1001 }).id, 7);
});

test("rejects altered body, key, timestamp, delivery and unsigned pushes", () => {
  const { body, headers } = fixture();
  assert.throws(() => verifyPush("secret", headers, body + " ", { nowSeconds: 1001 }), BotPushError);
  assert.throws(() => verifyPush("bad", headers, body, { nowSeconds: 1001 }), BotPushError);
  assert.throws(() => verifyPush("secret", headers, body, { nowSeconds: 2000 }), BotPushError);
  assert.throws(() => verifyPush("secret", { ...headers, "X-Nowork-Delivery": "8" }, body, { nowSeconds: 1001 }), BotPushError);
  assert.throws(() => verifyPush("secret", { ...headers, "X-Nowork-Signature": "" }, body, { nowSeconds: 1001 }), BotPushError);
});

test("signed optional site metadata is preserved and cannot be altered", () => {
  for (const siteId of [undefined, 0, 42]) {
    const { body, headers } = fixture(siteId);
    assert.equal(verifyPush("secret", headers, body, { nowSeconds: 1001 }).siteId, siteId);
    const tampered = JSON.stringify({ ...JSON.parse(body), siteId: (siteId ?? 0) + 1 });
    assert.throws(() => verifyPush("secret", headers, tampered, { nowSeconds: 1001 }), BotPushError);
  }
});
