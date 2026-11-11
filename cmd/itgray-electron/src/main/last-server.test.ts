import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { loadLastServer, saveLastServer } from "./last-server";

test("round-trips the last server id", () => {
  const file = path.join(mkdtempSync(path.join(tmpdir(), "itg-last-")), "nested", "tray-state.json");
  assert.equal(loadLastServer(file), null);
  saveLastServer(file, "srv-1");
  assert.equal(loadLastServer(file), "srv-1");
});

test("corrupt or foreign content reads as no server", () => {
  const file = path.join(mkdtempSync(path.join(tmpdir(), "itg-last-")), "tray-state.json");
  writeFileSync(file, "{nope");
  assert.equal(loadLastServer(file), null);
  writeFileSync(file, JSON.stringify({ lastServerId: 42 }));
  assert.equal(loadLastServer(file), null);
});
