import { beforeEach, describe, expect, it, vi } from "vitest";

const { eventHandlers, getSnapshotMock, setRoutingModeMock, getRoutingModeMock } = vi.hoisted(() => ({
  eventHandlers: {} as Record<string, (...args: any[]) => void>,
  getSnapshotMock: vi.fn(),
  setRoutingModeMock: vi.fn(),
  getRoutingModeMock: vi.fn(),
}));

vi.mock("@/lib/itg/runtime", () => ({
  EventsOn: (name: string, cb: (...args: any[]) => void) => {
    eventHandlers[name] = cb;
    return () => { delete eventHandlers[name]; };
  },
}));
vi.mock("@/lib/itg/AppService", () => ({
  GetSnapshot: () => getSnapshotMock(),
  ClaimAutoConnect: () => Promise.resolve(false),
}));
vi.mock("@/lib/itg/RunService", () => ({
  Connect: vi.fn(),
  Disconnect: vi.fn(),
  SetRoutingMode: (m: string) => setRoutingModeMock(m),
  GetRoutingMode: () => getRoutingModeMock(),
}));
vi.mock("@/lib/itg/ServersService", () => ({ TestLatency: vi.fn(() => Promise.resolve()) }));

import { __resetForTest, __bootstrapForTest, getDashState, dashSetRoutingMode } from "./dashStore";

const snap = (status: string) => ({
  status, currentServer: status === "connected" ? { id: "a" } : null, mode: "tun",
  speeds: { upBps: 0, downBps: 0, at: "" }, helperState: "running", servers: [], subs: [], settings: {}, onboarded: true, version: "t",
});

function fire(name: string, payload: any) {
  const cb = eventHandlers[name];
  if (!cb) throw new Error(`no handler for ${name}`);
  cb(payload);
}

beforeEach(() => {
  for (const k of Object.keys(eventHandlers)) delete eventHandlers[k];
  getSnapshotMock.mockReset();
  setRoutingModeMock.mockReset();
  getRoutingModeMock.mockReset();
  __resetForTest();
});

describe("dashStore routing mode", () => {
  it("starts in rule and follows routing:mode events", async () => {
    getSnapshotMock.mockResolvedValue(snap("idle"));
    await __bootstrapForTest();
    expect(getDashState().routingMode).toBe("rule");
    fire("routing:mode", { mode: "global" });
    expect(getDashState().routingMode).toBe("global");
    fire("routing:mode", { mode: "bogus" });
    expect(getDashState().routingMode).toBe("global");
  });

  it("reads the live mode on bootstrap when already connected (adopted chain)", async () => {
    getSnapshotMock.mockResolvedValue(snap("connected"));
    getRoutingModeMock.mockResolvedValue({ mode: "direct" });
    await __bootstrapForTest();
    await vi.waitFor(() => expect(getDashState().routingMode).toBe("direct"));
  });

  it("resets to rule when the tunnel goes idle", async () => {
    getSnapshotMock.mockResolvedValue(snap("connected"));
    getRoutingModeMock.mockResolvedValue({ mode: "global" });
    await __bootstrapForTest();
    await vi.waitFor(() => expect(getDashState().routingMode).toBe("global"));
    fire("vpn:status", { status: "idle" });
    expect(getDashState().routingMode).toBe("rule");
  });

  it("sets optimistically and reverts with an error when the call fails", async () => {
    getSnapshotMock.mockResolvedValue(snap("connected"));
    getRoutingModeMock.mockResolvedValue({ mode: "rule" });
    await __bootstrapForTest();
    setRoutingModeMock.mockResolvedValueOnce(undefined);
    await dashSetRoutingMode("direct");
    expect(setRoutingModeMock).toHaveBeenCalledWith("direct");
    expect(getDashState().routingMode).toBe("direct");
    setRoutingModeMock.mockRejectedValueOnce(new Error("boom"));
    await expect(dashSetRoutingMode("global")).rejects.toThrow(/boom/);
    expect(getDashState().routingMode).toBe("direct");
    expect(getDashState().lastError?.kind).toBe("routing_mode_failed");
  });

  it("is a no-op while not connected", async () => {
    getSnapshotMock.mockResolvedValue(snap("idle"));
    await __bootstrapForTest();
    await dashSetRoutingMode("global");
    expect(setRoutingModeMock).not.toHaveBeenCalled();
    expect(getDashState().routingMode).toBe("rule");
  });
});
