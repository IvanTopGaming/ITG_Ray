import { describe, it, expect, vi, beforeEach } from "vitest";

const dash = {
  status: "idle" as string,
  currentServer: null as { id: string } | null,
  allServers: [] as Array<{ id: string; favorite: boolean }>,
};
let lastServer: string | null = null;
const mockConnect = vi.fn(async () => {});
const mockDisconnect = vi.fn(async () => {});
const mockSwitchMode = vi.fn(async () => {});

vi.mock("@/lib/dashStore", () => ({
  dashConnect: (...a: unknown[]) => mockConnect(...(a as [])),
  dashDisconnect: () => mockDisconnect(),
  dashSwitchMode: (...a: unknown[]) => mockSwitchMode(...(a as [])),
  getDashState: () => dash,
  loadLastServerId: () => lastServer,
  useDash: () => dash,
}));
vi.mock("@/lib/subsStore", () => ({ useSubs: vi.fn() }));

import { runTrayCommand } from "./trayBridge";

function deps() {
  return {
    navigate: vi.fn(),
    subs: { syncAll: vi.fn(async () => {}), cancelAll: vi.fn(async () => {}) },
  };
}

describe("runTrayCommand", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    dash.status = "idle";
    dash.currentServer = null;
    dash.allServers = [{ id: "a", favorite: false }, { id: "b", favorite: true }];
    lastServer = null;
  });

  it("connect prefers the current server, then the remembered one, then a favorite", async () => {
    await runTrayCommand({ kind: "connect" }, deps());
    expect(mockConnect).toHaveBeenLastCalledWith("b");
    lastServer = "a";
    await runTrayCommand({ kind: "connect" }, deps());
    expect(mockConnect).toHaveBeenLastCalledWith("a");
    dash.currentServer = { id: "b" };
    await runTrayCommand({ kind: "connect" }, deps());
    expect(mockConnect).toHaveBeenLastCalledWith("b");
  });

  it("connect does nothing without any server", async () => {
    dash.allServers = [];
    await runTrayCommand({ kind: "connect" }, deps());
    expect(mockConnect).not.toHaveBeenCalled();
  });

  it("selecting the connected server is a no-op, another one switches", async () => {
    dash.status = "connected";
    dash.currentServer = { id: "a" };
    await runTrayCommand({ kind: "selectServer", id: "a" }, deps());
    expect(mockConnect).not.toHaveBeenCalled();
    await runTrayCommand({ kind: "selectServer", id: "b" }, deps());
    expect(mockConnect).toHaveBeenCalledWith("b");
  });

  it("routes disconnect, mode, sync and navigation to the stores", async () => {
    const d = deps();
    await runTrayCommand({ kind: "disconnect" }, d);
    await runTrayCommand({ kind: "setMode", mode: "sysproxy" }, d);
    await runTrayCommand({ kind: "syncAll" }, d);
    await runTrayCommand({ kind: "cancelSync" }, d);
    await runTrayCommand({ kind: "navigate", path: "/logs" }, d);
    expect(mockDisconnect).toHaveBeenCalledTimes(1);
    expect(mockSwitchMode).toHaveBeenCalledWith("sysproxy");
    expect(d.subs.syncAll).toHaveBeenCalledTimes(1);
    expect(d.subs.cancelAll).toHaveBeenCalledTimes(1);
    expect(d.navigate).toHaveBeenCalledWith("/logs");
  });
});
