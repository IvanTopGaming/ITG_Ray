// cmd/itgray-electron/src/main/bridge.test.ts
import { test } from "node:test";
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
