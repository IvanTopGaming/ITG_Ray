import { test } from "node:test";
import assert from "node:assert/strict";
import { createAutoConnectClaim } from "./autoconnect";

test("grants the first claim of a launch", () => {
  const claim = createAutoConnectClaim();
  assert.equal(claim(), true);
});

// The renderer is re-created whenever the window is closed to the tray and
// re-opened, and it re-runs its auto-connect check each time. Only the first
// check of an app launch may win, otherwise the app reconnects every time the
// window comes back.
test("denies every claim after the first", () => {
  const claim = createAutoConnectClaim();
  claim();
  assert.equal(claim(), false);
  assert.equal(claim(), false);
});

test("gives each launch its own claim", () => {
  const first = createAutoConnectClaim();
  first();
  const second = createAutoConnectClaim();
  assert.equal(second(), true);
});
