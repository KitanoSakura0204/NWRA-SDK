import { test } from "node:test";
import assert from "node:assert/strict";
import { loungeMessageToText } from "../src/convert.js";

test("loungeMessageToText renders segments", () => {
  const text = loungeMessageToText([
    { type: "text", data: { text: "hello " } },
    { type: "mention", data: { userId: "42" } },
    { type: "link", data: { label: "docs", url: "https://example.com" } },
    { type: "image", data: { url: "https://example.com/a.png" } },
  ]);
  assert.equal(text, "hello @42docs (https://example.com)[image: https://example.com/a.png]");
});

test("loungeMessageToText handles empty input", () => {
  assert.equal(loungeMessageToText(undefined), "");
  assert.equal(loungeMessageToText([]), "");
});
