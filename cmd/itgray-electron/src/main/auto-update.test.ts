import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { createAutoUpdate, loadUpdatePrefs, saveUpdatePrefs, type UpdatePrefs } from "./auto-update";
import type { UpdateCheckResult } from "./updates";

function tmpFile(): string {
  return path.join(mkdtempSync(path.join(tmpdir(), "itg-upd-")), "nested", "update-prefs.json");
}

test("prefs default to auto-check on with nothing skipped", () => {
  assert.deepEqual(loadUpdatePrefs(tmpFile()), { autoCheck: true, skipped: "" });
});

test("prefs round-trip and ignore corrupt content", () => {
  const file = tmpFile();
  saveUpdatePrefs(file, { autoCheck: false, skipped: "v0.1.4" });
  assert.deepEqual(loadUpdatePrefs(file), { autoCheck: false, skipped: "v0.1.4" });
  writeFileSync(file, "{nope");
  assert.deepEqual(loadUpdatePrefs(file), { autoCheck: true, skipped: "" });
  writeFileSync(file, JSON.stringify({ autoCheck: "yes", skipped: 4 }));
  assert.deepEqual(loadUpdatePrefs(file), { autoCheck: true, skipped: "" });
});

function harness(prefs: UpdatePrefs, result: UpdateCheckResult) {
  const state = { prefs: { ...prefs }, published: [] as unknown[], checked: [] as string[] };
  const auto = createAutoUpdate({
    getVersion: async () => "v0.1.3-beta.1",
    check: async (v) => {
      state.checked.push(v);
      return result;
    },
    loadPrefs: () => state.prefs,
    savePrefs: (p) => {
      state.prefs = p;
    },
    publish: (u) => state.published.push(u),
  });
  return { auto, state };
}

test("publishes an available update and keeps it pending", async () => {
  const { auto, state } = harness(
    { autoCheck: true, skipped: "" },
    { status: "available", latest: "0.1.4", htmlUrl: "https://github.com/x/y/releases/tag/v0.1.4" },
  );
  await auto.run();
  const want = { latest: "0.1.4", htmlUrl: "https://github.com/x/y/releases/tag/v0.1.4" };
  assert.deepEqual(state.checked, ["v0.1.3-beta.1"]);
  assert.deepEqual(state.published, [want]);
  assert.deepEqual(auto.pending(), want);
});

test("does nothing when auto-check is off", async () => {
  const { auto, state } = harness({ autoCheck: false, skipped: "" }, { status: "available", latest: "0.1.4" });
  await auto.run();
  assert.deepEqual(state.checked, []);
  assert.equal(auto.pending(), null);
});

test("stays quiet for a skipped version", async () => {
  const { auto, state } = harness({ autoCheck: true, skipped: "0.1.4" }, { status: "available", latest: "0.1.4" });
  await auto.run();
  assert.deepEqual(state.published, []);
  assert.equal(auto.pending(), null);
});

test("stays quiet when up to date or the check fails", async () => {
  for (const result of [{ status: "uptodate" }, { status: "error" }] as UpdateCheckResult[]) {
    const { auto, state } = harness({ autoCheck: true, skipped: "" }, result);
    await auto.run();
    assert.deepEqual(state.published, []);
    assert.equal(auto.pending(), null);
  }
});

test("a version lookup failure is swallowed", async () => {
  const published: unknown[] = [];
  const auto = createAutoUpdate({
    getVersion: async () => {
      throw new Error("bridge down");
    },
    check: async () => ({ status: "available", latest: "0.1.4" }),
    loadPrefs: () => ({ autoCheck: true, skipped: "" }),
    savePrefs: () => {},
    publish: (u) => published.push(u),
  });
  await auto.run();
  assert.deepEqual(published, []);
});

test("skip persists the version and clears the pending update", async () => {
  const { auto, state } = harness({ autoCheck: true, skipped: "" }, { status: "available", latest: "0.1.4" });
  await auto.run();
  auto.skip("0.1.4");
  assert.deepEqual(state.prefs, { autoCheck: true, skipped: "0.1.4" });
  assert.equal(auto.pending(), null);
});

test("setAutoCheck persists without touching the skipped version", () => {
  const { auto, state } = harness({ autoCheck: true, skipped: "0.1.4" }, { status: "uptodate" });
  auto.setAutoCheck(false);
  assert.deepEqual(state.prefs, { autoCheck: false, skipped: "0.1.4" });
  assert.equal(auto.autoCheck(), false);
});
