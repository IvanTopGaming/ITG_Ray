import { test } from "node:test";
import assert from "node:assert/strict";
import { resolveStartMinimized, resolveTrayStatus } from "./startup";

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

test("tray status adopts a tunnel the helper kept alive across a restart", () => {
  assert.equal(resolveTrayStatus({ status: "connected" }), "connected");
});

test("tray status passes through every icon-backed chain status", () => {
  assert.equal(resolveTrayStatus({ status: "idle" }), "idle");
  assert.equal(resolveTrayStatus({ status: "connecting" }), "connecting");
  assert.equal(resolveTrayStatus({ status: "error" }), "error");
});

test("tray status maps disconnecting onto the connecting icon", () => {
  assert.equal(resolveTrayStatus({ status: "disconnecting" }), "connecting");
});

test("tray status is null when the snapshot says nothing usable", () => {
  assert.equal(resolveTrayStatus(undefined), null);
  assert.equal(resolveTrayStatus({}), null);
  assert.equal(resolveTrayStatus({ status: "" }), null);
  assert.equal(resolveTrayStatus({ status: "bogus" }), null);
});
