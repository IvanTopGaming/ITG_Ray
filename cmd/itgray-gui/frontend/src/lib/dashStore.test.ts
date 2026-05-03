import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { eventHandlers, getSnapshotMock, runConnectMock, runDisconnectMock, testLatencyMock } = vi.hoisted(() => ({
  eventHandlers: {} as Record<string, (...args: any[]) => void>,
  getSnapshotMock: vi.fn(),
  runConnectMock: vi.fn(),
  runDisconnectMock: vi.fn(),
  testLatencyMock: vi.fn(),
}));

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, cb: (...args: any[]) => void) => {
    eventHandlers[name] = cb;
    return () => { delete eventHandlers[name]; };
  },
}));

vi.mock("../../wailsjs/go/bindings/AppService", () => ({
  GetSnapshot: () => getSnapshotMock(),
}));

vi.mock("../../wailsjs/go/bindings/RunService", () => ({
  Connect: (id: string, mode: string) => runConnectMock(id, mode),
  Disconnect: () => runDisconnectMock(),
}));

vi.mock("../../wailsjs/go/bindings/ServersService", () => ({
  TestLatency: (id: string) => testLatencyMock(id),
}));

import {
  __resetForTest,
  __bootstrapForTest,
  effectiveStatus,
  getDashState,
  dashConnect,
  dashDisconnect,
  dashSwitchMode,
  dashReconnect,
  clearLastError,
  type ChainStatus,
} from "./dashStore";

function fireEvent(name: string, payload: any) {
  const cb = eventHandlers[name];
  if (!cb) throw new Error(`no handler registered for ${name}`);
  cb(payload);
}

const baseSnapshot = {
  status: "idle",
  currentServer: null,
  mode: "tun",
  speeds: { upBps: 0, downBps: 0, at: new Date().toISOString() },
  helperState: "running",
  servers: [],
  subs: [],
  settings: {},
  onboarded: true,
  version: "test",
};

beforeEach(() => {
  for (const k of Object.keys(eventHandlers)) delete eventHandlers[k];
  getSnapshotMock.mockReset();
  runConnectMock.mockReset();
  runDisconnectMock.mockReset();
  testLatencyMock.mockReset();
  testLatencyMock.mockResolvedValue(undefined);
  __resetForTest();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("dashStore — bootstrap", () => {
  it("seeds state from GetSnapshot on first read", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      status: "idle",
      mode: "sysproxy",
      servers: [{ id: "a", name: "A", favorite: true, latencyMs: 10 }],
    });
    await __bootstrapForTest();
    const state = getDashState();
    expect(state.bootstrapped).toBe(true);
    expect(state.status).toBe("idle");
    expect(state.mode).toBe("sysproxy");
    expect(state.allServers).toHaveLength(1);
  });

  it("sets lastError when snapshot fetch fails", async () => {
    getSnapshotMock.mockRejectedValue(new Error("disk read failed"));
    await __bootstrapForTest();  // bootstrap catches the error internally; this resolves
    const state = getDashState();
    expect(state.bootstrapped).toBe(false);
    expect(state.lastError?.message).toContain("disk read failed");
  });
});

describe("dashStore — vpn:status", () => {
  it("connecting transition updates status only", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connecting" });
    expect(getDashState().status).toBe("connecting");
    expect(getDashState().currentServer).toBeNull();
  });

  it("connected transition resolves server from cache and sets connectedAt", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      servers: [{ id: "s1", name: "DE", favorite: false, latencyMs: 30 }],
    });
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "s1", mode: "tun" });
    const st = getDashState();
    expect(st.status).toBe("connected");
    expect(st.currentServer?.name).toBe("DE");
    expect(st.connectedAt).not.toBeNull();
  });

  it("connected with cache miss falls back to {id, name=''}", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "missing", mode: "tun" });
    const st = getDashState();
    expect(st.currentServer?.id).toBe("missing");
    expect(st.currentServer?.name).toBe("");
  });

  it("idle transition clears history/totals/connectedAt", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      servers: [{ id: "s1", name: "DE", favorite: false, latencyMs: 0 }],
    });
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "s1" });
    fireEvent("vpn:speed", { downBps: 1000, upBps: 500 });
    expect(getDashState().history).toHaveLength(1);
    fireEvent("vpn:status", { status: "idle" });
    const st = getDashState();
    expect(st.history).toEqual([]);
    expect(st.totals).toEqual({ down: 0, up: 0 });
    expect(st.connectedAt).toBeNull();
  });
});

describe("dashStore — vpn:speed", () => {
  beforeEach(async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "s1" });
  });

  it("ignores samples when not connected", () => {
    fireEvent("vpn:status", { status: "idle" });
    fireEvent("vpn:speed", { downBps: 999, upBps: 999 });
    expect(getDashState().history).toEqual([]);
  });

  it("caps the ring buffer at 60", () => {
    for (let i = 0; i < 65; i++) {
      fireEvent("vpn:speed", { downBps: i, upBps: i });
    }
    expect(getDashState().history).toHaveLength(60);
    expect(getDashState().history[0].downBps).toBe(5);
    expect(getDashState().history[59].downBps).toBe(64);
  });

  it("accumulates totals", () => {
    fireEvent("vpn:speed", { downBps: 100, upBps: 50 });
    fireEvent("vpn:speed", { downBps: 200, upBps: 100 });
    expect(getDashState().totals).toEqual({ down: 300, up: 150 });
  });
});

describe("dashStore — chain:error", () => {
  it("sets lastError without changing status", async () => {
    getSnapshotMock.mockResolvedValue({ ...baseSnapshot, status: "connecting" });
    await __bootstrapForTest();
    fireEvent("chain:error", { kind: "bringup_failed", message: "tunnel up failed" });
    const st = getDashState();
    expect(st.status).toBe("connecting");
    expect(st.lastError?.kind).toBe("bringup_failed");
    expect(st.lastError?.message).toBe("tunnel up failed");
  });
});

describe("dashStore — helper:state", () => {
  it("updates helperState", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("helper:state", { state: "stopped" });
    expect(getDashState().helperState).toBe("stopped");
  });
});

describe("dashStore — sub:synced", () => {
  it("refreshes allServers from a fresh snapshot", async () => {
    getSnapshotMock.mockResolvedValueOnce({ ...baseSnapshot, servers: [] });
    await __bootstrapForTest();
    expect(getDashState().allServers).toHaveLength(0);

    getSnapshotMock.mockResolvedValueOnce({
      ...baseSnapshot,
      servers: [{ id: "new1", name: "New", favorite: false, latencyMs: 0 }],
    });
    fireEvent("sub:synced", {});
    await Promise.resolve();
    await Promise.resolve();
    expect(getDashState().allServers).toHaveLength(1);
    expect(getDashState().allServers[0].id).toBe("new1");
  });
});

describe("dashStore — servers:changed", () => {
  it("refreshes allServers from a fresh snapshot", async () => {
    getSnapshotMock.mockResolvedValueOnce({ ...baseSnapshot, servers: [] });
    await __bootstrapForTest();
    expect(getDashState().allServers).toHaveLength(0);

    getSnapshotMock.mockResolvedValueOnce({
      ...baseSnapshot,
      servers: [{ id: "m1", name: "Manual", favorite: false, latencyMs: 0 }],
    });
    fireEvent("servers:changed", {});
    await Promise.resolve();
    await Promise.resolve();
    expect(getDashState().allServers).toHaveLength(1);
    expect(getDashState().allServers[0].id).toBe("m1");
  });
});

describe("dashStore — probe:result", () => {
  it("auto-fires TestLatency on bootstrap when servers have latencyMs=0", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      servers: [
        { id: "a", name: "A", favorite: false, latencyMs: 0 },
        { id: "b", name: "B", favorite: false, latencyMs: 50 },
      ],
    });
    await __bootstrapForTest();
    expect(testLatencyMock).toHaveBeenCalledWith("");
  });

  it("does NOT fire TestLatency when every server is already probed", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      servers: [
        { id: "a", name: "A", favorite: false, latencyMs: 25 },
        { id: "b", name: "B", favorite: false, latencyMs: 50 },
      ],
    });
    await __bootstrapForTest();
    expect(testLatencyMock).not.toHaveBeenCalled();
  });

  it("patches latencyMs in allServers from probe:result payload", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      servers: [
        { id: "a", name: "A", favorite: false, latencyMs: 0 },
        { id: "b", name: "B", favorite: false, latencyMs: 0 },
      ],
    });
    await __bootstrapForTest();
    fireEvent("probe:result", {
      results: [
        { id: "a", latencyMs: 17 },
        { id: "b", latencyMs: 0, error: "timeout" },
      ],
    });
    const servers = getDashState().allServers;
    expect(servers.find((s) => s.id === "a")?.latencyMs).toBe(17);
    // server "b" had an error → latencyMs stays at 0.
    expect(servers.find((s) => s.id === "b")?.latencyMs).toBe(0);
  });
});

describe("effectiveStatus", () => {
  it("returns 'error' when idle + lastError", () => {
    const st = { status: "idle" as ChainStatus, lastError: { kind: "x", message: "y", at: 0 } } as any;
    expect(effectiveStatus(st)).toBe("error");
  });
  it("returns raw status otherwise", () => {
    const a = { status: "connecting", lastError: null } as any;
    expect(effectiveStatus(a)).toBe("connecting");
    const b = { status: "connected", lastError: { kind: "x", message: "y", at: 0 } } as any;
    expect(effectiveStatus(b)).toBe("connected");
  });
});

describe("clearLastError", () => {
  it("clears the error field", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("chain:error", { kind: "x", message: "y" });
    expect(getDashState().lastError).not.toBeNull();
    clearLastError();
    expect(getDashState().lastError).toBeNull();
  });
});

describe("dashConnect", () => {
  it("calls Connect with serverId and current mode when idle", async () => {
    getSnapshotMock.mockResolvedValue({ ...baseSnapshot, mode: "sysproxy" });
    await __bootstrapForTest();
    runConnectMock.mockResolvedValue(undefined);
    await dashConnect("server-id-1");
    expect(runConnectMock).toHaveBeenCalledWith("server-id-1", "sysproxy");
  });

  it("disconnects first when currently connected", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "old" });
    runDisconnectMock.mockImplementation(async () => {
      setTimeout(() => fireEvent("vpn:status", { status: "idle" }), 0);
    });
    runConnectMock.mockResolvedValue(undefined);
    await dashConnect("new");
    expect(runDisconnectMock).toHaveBeenCalled();
    expect(runConnectMock).toHaveBeenCalledWith("new", "tun");
  });

  it("sets lastError on Connect rejection", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    runConnectMock.mockRejectedValue(new Error("helper down"));
    await expect(dashConnect("s1")).rejects.toThrow("helper down");
    expect(getDashState().lastError?.kind).toBe("connect_failed");
  });
});

describe("dashDisconnect", () => {
  it("calls Disconnect", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    runDisconnectMock.mockResolvedValue(undefined);
    await dashDisconnect();
    expect(runDisconnectMock).toHaveBeenCalled();
  });
});

describe("dashSwitchMode", () => {
  it("just updates mode locally when idle", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    await dashSwitchMode("sysproxy");
    expect(getDashState().mode).toBe("sysproxy");
    expect(runDisconnectMock).not.toHaveBeenCalled();
    expect(runConnectMock).not.toHaveBeenCalled();
  });

  it("does Disconnect → Connect when connected", async () => {
    getSnapshotMock.mockResolvedValue({
      ...baseSnapshot,
      servers: [{ id: "s1", name: "DE", favorite: false, latencyMs: 0 }],
    });
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "s1" });
    runDisconnectMock.mockImplementation(async () => {
      setTimeout(() => fireEvent("vpn:status", { status: "idle" }), 0);
    });
    runConnectMock.mockResolvedValue(undefined);
    await dashSwitchMode("sysproxy");
    expect(runDisconnectMock).toHaveBeenCalled();
    expect(runConnectMock).toHaveBeenCalledWith("s1", "sysproxy");
    expect(getDashState().mode).toBe("sysproxy");
  });
});

describe("dashReconnect", () => {
  it("does Disconnect → Connect with explicit serverId+mode when connected", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "old" });
    runDisconnectMock.mockImplementation(async () => {
      setTimeout(() => fireEvent("vpn:status", { status: "idle" }), 0);
    });
    runConnectMock.mockResolvedValue(undefined);
    await dashReconnect("snap-server", "sysproxy");
    expect(runDisconnectMock).toHaveBeenCalled();
    expect(runConnectMock).toHaveBeenCalledWith("snap-server", "sysproxy");
    expect(getDashState().mode).toBe("sysproxy");
  });

  it("surfaces Connect failure via lastError after a successful Disconnect", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "old" });
    runDisconnectMock.mockImplementation(async () => {
      setTimeout(() => fireEvent("vpn:status", { status: "idle" }), 0);
    });
    runConnectMock.mockRejectedValue(new Error("helper down"));
    await expect(dashReconnect("s1", "tun")).rejects.toThrow("helper down");
    expect(getDashState().lastError?.kind).toBe("reconnect_failed");
    expect(getDashState().lastError?.message).toBe("helper down");
  });

  it("skips the Disconnect step when not currently connected", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    runConnectMock.mockResolvedValue(undefined);
    await dashReconnect("s1", "tun");
    expect(runDisconnectMock).not.toHaveBeenCalled();
    expect(runConnectMock).toHaveBeenCalledWith("s1", "tun");
  });
});

describe("dashStore — concurrency guards", () => {
  it("dashConnect coalesces concurrent re-entry (single backend Connect)", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    let resolveConnect!: () => void;
    runConnectMock.mockImplementation(() => new Promise<void>((r) => { resolveConnect = () => r(); }));
    const p1 = dashConnect("s1");
    const p2 = dashConnect("s1");  // concurrent; should not issue a second Connect
    resolveConnect();
    await Promise.all([p1, p2]);
    expect(runConnectMock).toHaveBeenCalledTimes(1);
  });

  it("dashConnect 'superseded' rejection does not poison lastError", async () => {
    getSnapshotMock.mockResolvedValue(baseSnapshot);
    await __bootstrapForTest();
    fireEvent("vpn:status", { status: "connected", serverId: "old" });

    // First call's Disconnect resolves but waitForIdle is still pending.
    runDisconnectMock.mockResolvedValue(undefined);
    const p1 = dashConnect("new1");

    // Wait one tick for p1's await Disconnect to advance into waitForIdle.
    await Promise.resolve();
    await Promise.resolve();

    // Second call should be coalesced — the in-flight guard means it just
    // returns the existing promise; no superseded scenario actually fires
    // here because we now coalesce. But we still verify lastError stays null.
    const p2 = dashConnect("new2");

    // Resolve p1 by emitting idle.
    runConnectMock.mockResolvedValue(undefined);
    fireEvent("vpn:status", { status: "idle" });
    await Promise.all([p1, p2]);
    // Coalesced: only one Disconnect+Connect pair should have fired across
    // both calls. Critical regression guard: lastError must not be poisoned
    // by a "superseded" rejection from a stale waitForIdle.
    expect(runDisconnectMock).toHaveBeenCalledTimes(1);
    expect(runConnectMock).toHaveBeenCalledTimes(1);
    expect(getDashState().lastError).toBeNull();
  });

  it("bootstrap coalesces concurrent calls (single GetSnapshot)", async () => {
    let resolveSnap!: (v: any) => void;
    getSnapshotMock.mockImplementation(() => new Promise((r) => { resolveSnap = r; }));
    const p1 = __bootstrapForTest();
    const p2 = __bootstrapForTest();  // concurrent; should coalesce
    resolveSnap(baseSnapshot);
    await Promise.all([p1, p2]);
    expect(getSnapshotMock).toHaveBeenCalledTimes(1);
  });
});

describe("probeState", () => {
  beforeEach(() => {
    __resetForTest();
    getSnapshotMock.mockResolvedValue({
      status: "idle",
      mode: "tun",
      currentServer: null,
      helperState: "running",
      servers: [
        { id: "a", name: "A", country: "", address: "h:443", transport: "tcp",
          security: "none", latencyMs: 0, origin: "manual", favorite: false,
          tags: [], uri: "" },
        { id: "b", name: "B", country: "", address: "h:443", transport: "tcp",
          security: "none", latencyMs: 0, origin: "manual", favorite: false,
          tags: [], uri: "" },
      ],
    });
  });

  it("dashSetProbing marks given ids as probing", async () => {
    const { dashSetProbing } = await import("./dashStore");
    await __bootstrapForTest();
    dashSetProbing(["a", "b"]);
    const s = getDashState();
    expect(s.probeState.get("a")).toBe("probing");
    expect(s.probeState.get("b")).toBe("probing");
  });

  it("onProbeResult sets probeState to ok or error", async () => {
    await __bootstrapForTest();
    eventHandlers["probe:result"]({
      results: [
        { id: "a", latencyMs: 25 },
        { id: "b", error: "timeout" },
      ],
    });
    const s = getDashState();
    expect(s.probeState.get("a")).toBe("ok");
    expect(s.probeState.get("b")).toBe("error");
    expect(s.allServers.find((x) => x.id === "a")!.latencyMs).toBe(25);
    // 'b' had an error → latencyMs stays untouched (0).
    expect(s.allServers.find((x) => x.id === "b")!.latencyMs).toBe(0);
  });

  it("dashProbeOne sets error on TestLatency rejection", async () => {
    const { dashProbeOne } = await import("./dashStore");
    await __bootstrapForTest();
    testLatencyMock.mockRejectedValueOnce(new Error("nope"));
    await dashProbeOne("a");
    expect(getDashState().probeState.get("a")).toBe("error");
  });

  it("dashProbeAll sets error for every loaded id on rejection", async () => {
    const { dashProbeAll } = await import("./dashStore");
    await __bootstrapForTest();
    testLatencyMock.mockRejectedValueOnce(new Error("nope"));
    await dashProbeAll();
    const s = getDashState();
    expect(s.probeState.get("a")).toBe("error");
    expect(s.probeState.get("b")).toBe("error");
  });
});
