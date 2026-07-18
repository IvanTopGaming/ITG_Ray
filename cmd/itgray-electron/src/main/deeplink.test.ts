import { test } from "node:test";
import assert from "node:assert/strict";
import { extractDeeplink } from "./deeplink";

test("finds an itgray:// arg", () => {
  assert.equal(
    extractDeeplink(["electron", ".", "itgray://rules/import/abc"]),
    "itgray://rules/import/abc",
  );
});

test("returns null when absent", () => {
  assert.equal(extractDeeplink(["electron", "."]), null);
});

test("ignores non-itgray args", () => {
  assert.equal(extractDeeplink(["itgrayx://nope", "https://itgray"]), null);
});
