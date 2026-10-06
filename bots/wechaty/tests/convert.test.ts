import { test } from "node:test";
import assert from "node:assert/strict";
import { noworkMessageToText } from "../src/convert.js";

test("noworkMessageToText renders segments", () => {
  const text = noworkMessageToText([
    { type: "text", data: { text: "hello " } },
    { type: "mention", data: { userId: "42" } },
    { type: "link", data: { label: "docs", url: "https://example.com" } },
    { type: "image", data: { url: "https://example.com/a.png" } },
  ]);
  assert.equal(text, "hello @42docs (https://example.com)[image: https://example.com/a.png]");
});

test("noworkMessageToText handles empty input", () => {
  assert.equal(noworkMessageToText(undefined), "");
  assert.equal(noworkMessageToText([]), "");
});
