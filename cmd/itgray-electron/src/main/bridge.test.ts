// cmd/itgray-electron/src/main/bridge.test.ts
import { test } from "node:test";
import childProcess from "node:child_process";
import { EventEmitter } from "node:events";
import { PassThrough } from "node:stream";
import * as paths from "./paths";
import assert from "node:assert/strict";
import { BridgeSupervisor } from "./bridge";

// When the bridge crashes 6+ times within the 60s window the supervisor
// transitions to "failed" and the restart loop stops. The pre-fix bug:
// this.child / this.client were not cleared in that branch, so a
// subsequent stop() would walk through child.stdin?.end() and then wait
// the full 5s exit timeout against a dead/zombie process before
// SIGKILLing. After the fix, stop() in failed state returns immediately
// because child is undefined.
test("handleExit clears child + client on transition to failed", () => {
  const bridge = new BridgeSupervisor();
  // Simulate "we already spawned and have a live process" without actually
  // forking anything — handleExit's first guard is `if (!this.child)`.
  const fakeChild = { kill() {}, stdin: { end() {} }, once() {} } as never;
  (bridge as unknown as { child: unknown }).child = fakeChild;
  (bridge as unknown as { client: unknown }).client = {};
  // Six crashes inside the 60s window force the failed branch on this tick.
  (bridge as unknown as { restartCount: number }).restartCount = 5;
  (bridge as unknown as { restartWindowStart: number }).restartWindowStart = Date.now();
  (bridge as unknown as { handleExit(c: number | null, s: NodeJS.Signals | null): void }).handleExit(
    1,
    null,
  );
  assert.equal(bridge.getState(), "failed");
  assert.equal(
    (bridge as unknown as { child: unknown }).child,
    undefined,
    "child must be cleared so stop() does not block on a dead process",
  );
  assert.equal(
    (bridge as unknown as { client: unknown }).client,
    undefined,
    "client must be cleared so callers do not RPC into a closed pipe",
  );
});

class FakeBridgeProcess extends EventEmitter {
  stdin = new PassThrough();
  stdout = new PassThrough();
  stderr = new PassThrough();

  constructor() {
    super();
    this.stdin.once("finish", () => this.emit("exit", 0, null));
  }

  publish(topic: string, payload: unknown): void {
    this.stdout.write(JSON.stringify({ jsonrpc: "2.0", method: `event:${topic}`, params: payload }) + "\n");
  }
}

test("bridge event consumers follow restarted clients and ignore the old client", async (t) => {
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const children: FakeBridgeProcess[] = [];
  t.mock.method(paths, "bundledBinary", () => "/synthetic/bridge");
  t.mock.method(childProcess, "spawn", () => {
    const child = new FakeBridgeProcess();
    children.push(child);
    return child;
  });
  const bridge = new BridgeSupervisor();
  t.after(() => bridge.stop());
  const rendererStatuses: string[] = [];
  const notificationStatuses: string[] = [];
  const synced: unknown[] = [];
  bridge.on("vpn.status", (payload) => rendererStatuses.push(payload.status));
  bridge.on("vpn.status", (payload) => notificationStatuses.push(payload.status));
  bridge.on("sub.synced", (payload) => synced.push(payload));
  bridge.start();
  children[0].publish("vpn.status", { status: "connected" });
  children[0].emit("exit", 1, null);
  children[0].publish("vpn.status", { status: "stale" });
  t.mock.timers.tick(1000);
  children[0].emit("exit", 1, null);
  assert.equal(bridge.getState(), "running");
  children[1].publish("vpn.status", { status: "idle" });
  children[1].publish("sub.synced", { id: "subscription-1" });
  children[1].emit("exit", 1, null);
  t.mock.timers.tick(5000);
  children[2].publish("vpn.status", { status: "connected" });
  assert.deepEqual(rendererStatuses, ["connected", "idle", "connected"]);
  assert.deepEqual(notificationStatuses, ["connected", "idle", "connected"]);
  assert.deepEqual(synced, [{ id: "subscription-1" }]);
});

test("stopping the bridge detaches its event forwarders", async (t) => {
  const child = new FakeBridgeProcess();
  t.mock.method(paths, "bundledBinary", () => "/synthetic/bridge");
  t.mock.method(childProcess, "spawn", () => child);
  const bridge = new BridgeSupervisor();
  const statuses: string[] = [];
  bridge.on("vpn.status", (payload) => statuses.push(payload.status));
  bridge.start();
  child.publish("vpn.status", { status: "connected" });
  await bridge.stop();
  child.publish("vpn.status", { status: "idle" });
  assert.deepEqual(statuses, ["connected"]);
});
