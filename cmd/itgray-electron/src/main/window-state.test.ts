// cmd/itgray-electron/src/main/window-state.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "node:fs";
import path from "node:path";
import os from "node:os";
import { loadState, saveState, DEFAULT_STATE } from "./window-state";

async function tmpFile(): Promise<string> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "winstate-"));
  return path.join(dir, "window-state.json");
}

test("loadState returns defaults when file missing", async () => {
  const fp = await tmpFile();
  const got = await loadState(fp);
  assert.deepEqual(got, DEFAULT_STATE);
});

test("loadState returns defaults when JSON malformed", async () => {
  const fp = await tmpFile();
  await fs.writeFile(fp, "not json");
  const got = await loadState(fp);
  assert.deepEqual(got, DEFAULT_STATE);
});

test("saveState then loadState roundtrips", async () => {
  const fp = await tmpFile();
  await saveState(fp, { x: 100, y: 200, width: 1200, height: 800, maximised: false });
  const got = await loadState(fp);
  assert.equal(got.x, 100);
  assert.equal(got.y, 200);
  assert.equal(got.width, 1200);
  assert.equal(got.height, 800);
  assert.equal(got.maximised, false);
});

test("saveState clamps absurd dimensions to defaults", async () => {
  const fp = await tmpFile();
  await saveState(fp, { x: 0, y: 0, width: 50, height: 50, maximised: false });
  const got = await loadState(fp);
  // width=50 is below the 400 minimum; loadState should treat it as invalid.
  assert.deepEqual(got, DEFAULT_STATE);
});

test("loadState rejects state with non-finite numbers", async () => {
  const fp = await tmpFile();
  await fs.writeFile(fp, JSON.stringify({ x: 100, y: 0, width: NaN, height: 720, maximised: false }));
  const got = await loadState(fp);
  // JSON.stringify writes NaN as null; null doesn't satisfy width's `number` check.
  assert.deepEqual(got, DEFAULT_STATE);
});
