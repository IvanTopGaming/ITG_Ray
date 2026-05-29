import { test } from "node:test";
import assert from "node:assert/strict";
import { makeNotifier, type NotifPrefs } from "./notifications";

function fixture(prefs: Partial<NotifPrefs>) {
  const calls: Array<{ title: string; body: string; silent: boolean }> = [];
  const notifier = makeNotifier({
    notify: (title, body, opts) => calls.push({ title, body, silent: opts.silent }),
    getSettings: async () => ({
      onConnected: false,
      onDisconnected: false,
      onSubSynced: false,
      sound: true,
      ...prefs,
    }),
  });
  return { calls, notifier };
}

test("notifies on connect when onConnected is true", async () => {
  const { calls, notifier } = fixture({ onConnected: true });
  await notifier.onVpnStatus({ status: "connecting" });
  await notifier.onVpnStatus({ status: "connected" });
  assert.equal(calls.length, 1);
  assert.match(calls[0].title, /connect/i);
});

test("does not notify on connect when onConnected is false", async () => {
  const { calls, notifier } = fixture({ onConnected: false });
  await notifier.onVpnStatus({ status: "connected" });
  assert.equal(calls.length, 0);
});

test("fires connect only once per transition (no duplicate on repeat events)", async () => {
  const { calls, notifier } = fixture({ onConnected: true });
  await notifier.onVpnStatus({ status: "connected" });
  await notifier.onVpnStatus({ status: "connected" });
  assert.equal(calls.length, 1);
});

test("notifies on disconnect only when transitioning from connected", async () => {
  const { calls, notifier } = fixture({ onDisconnected: true });
  await notifier.onVpnStatus({ status: "connected" });
  await notifier.onVpnStatus({ status: "idle" });
  assert.equal(calls.length, 1);
  assert.match(calls[0].title, /disconnect/i);
});

test("does not fire disconnect from a never-connected state", async () => {
  const { calls, notifier } = fixture({ onDisconnected: true });
  await notifier.onVpnStatus({ status: "connecting" });
  await notifier.onVpnStatus({ status: "idle" });
  assert.equal(calls.length, 0);
});

test("sound=false marks the notification silent", async () => {
  const { calls, notifier } = fixture({ onConnected: true, sound: false });
  await notifier.onVpnStatus({ status: "connected" });
  assert.equal(calls[0].silent, true);
});

test("fires disconnect when transitioning connected -> error", async () => {
  const { calls, notifier } = fixture({ onDisconnected: true });
  await notifier.onVpnStatus({ status: "connected" });
  await notifier.onVpnStatus({ status: "error" });
  assert.equal(calls.length, 1);
  assert.match(calls[0].title, /disconnect/i);
});

test("notifies on sub-synced success with imported count in body", async () => {
  const { calls, notifier } = fixture({ onSubSynced: true });
  await notifier.onSubSynced({ status: "ok", importedCount: 5 });
  assert.equal(calls.length, 1);
  assert.match(calls[0].body, /5 servers/);
});

test("sub-synced success without a count uses a generic body", async () => {
  const { calls, notifier } = fixture({ onSubSynced: true });
  await notifier.onSubSynced({ status: "ok" });
  assert.equal(calls.length, 1);
  assert.match(calls[0].body, /subscription/i);
});

test("does NOT notify on a failed sub-sync (status error)", async () => {
  const { calls, notifier } = fixture({ onSubSynced: true });
  await notifier.onSubSynced({ status: "error", importedCount: 0 });
  assert.equal(calls.length, 0);
});
