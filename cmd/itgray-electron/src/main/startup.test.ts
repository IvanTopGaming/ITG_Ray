import { test } from "node:test";
import assert from "node:assert/strict";
import { resolveStartMinimized } from "./startup";

test("true when general.startMinimized is true", () => {
  assert.equal(resolveStartMinimized({ settings: { general: { startMinimized: true } } }), true);
});

test("false when general.startMinimized is false", () => {
  assert.equal(resolveStartMinimized({ settings: { general: { startMinimized: false } } }), false);
});

test("false when snapshot shape is missing fields", () => {
  assert.equal(resolveStartMinimized(undefined), false);
  assert.equal(resolveStartMinimized({}), false);
  assert.equal(resolveStartMinimized({ settings: {} }), false);
});
