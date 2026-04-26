import { describe, expect, it } from "vitest";
import { useStore } from "./index";
import type { Snapshot, SettingsView } from "../api/client";

describe("store", () => {
  it("starts with empty snapshot defaults", () => {
    // reset to known defaults before observing initial-style state
    useStore.getState().setSnapshot(emptySnapshot());
    const s = useStore.getState();
    expect(s.status).toBe("idle");
    expect(s.servers).toEqual([]);
    expect(s.subs).toEqual([]);
  });

  it("setSnapshot replaces all top-level fields", () => {
    useStore.getState().setSnapshot({
      status: "connected",
      currentServer: null,
      mode: "tun",
      speeds: { upBps: 0, downBps: 0, at: new Date().toISOString() },
      helperState: "running",
      servers: [{
        id: "x", name: "DE", country: "DE", address: "h:443",
        transport: "tcp", security: "reality", latencyMs: 15,
        origin: "okins", favorite: false, tags: [],
      }],
      subs: [],
      settings: defaultSettings(),
      onboarded: true,
      version: "test",
    });
    expect(useStore.getState().status).toBe("connected");
    expect(useStore.getState().servers).toHaveLength(1);
  });

  it("applyVPNStatus updates only status", () => {
    useStore.getState().setSnapshot(emptySnapshot());
    useStore.getState().applyVPNStatus("connecting");
    expect(useStore.getState().status).toBe("connecting");
    expect(useStore.getState().servers).toEqual([]);
  });

  it("applySpeed merges into speeds", () => {
    useStore.getState().setSnapshot(emptySnapshot());
    useStore.getState().applySpeed({ upBps: 100, downBps: 200 });
    expect(useStore.getState().speeds.upBps).toBe(100);
    expect(useStore.getState().speeds.downBps).toBe(200);
  });

  it("applySubSync replaces matching sub fields", () => {
    useStore.getState().setSnapshot({
      ...emptySnapshot(),
      subs: [{
        id: "s1", name: "x", url: "u", updateInterval: 60,
        lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "",
        serverCount: 0,
      }],
    });
    useStore.getState().applySubSync({
      id: "s1", status: "OK", at: new Date().toISOString(), importedCount: 5,
    });
    const s = useStore.getState().subs[0];
    expect(s.lastSyncStatus).toBe("OK");
    expect(s.serverCount).toBe(5);
  });

  it("applyProbeResult updates server latency by id", () => {
    useStore.getState().setSnapshot({
      ...emptySnapshot(),
      servers: [
        { id: "a", name: "A", country: "", address: "", transport: "", security: "", latencyMs: 0, origin: "", favorite: false, tags: [] },
        { id: "b", name: "B", country: "", address: "", transport: "", security: "", latencyMs: 0, origin: "", favorite: false, tags: [] },
      ],
    });
    useStore.getState().applyProbeResult({
      results: [
        { id: "a", latencyMs: 42 },
        { id: "b", latencyMs: 999, error: "timeout" },
      ],
    });
    const servers = useStore.getState().servers;
    expect(servers[0].latencyMs).toBe(42);
    expect(servers[1].latencyMs).toBe(0);
  });

  it("applyChainError parks payload in lastError and flips status", () => {
    useStore.getState().setSnapshot(emptySnapshot());
    useStore.getState().applyChainError({ kind: "chain_crashed", message: "sing-box exited" });
    const s = useStore.getState();
    expect(s.status).toBe("error");
    expect(s.lastError).toEqual({ kind: "chain_crashed", message: "sing-box exited" });

    // Returning to a non-error status clears lastError.
    useStore.getState().applyVPNStatus("idle");
    expect(useStore.getState().lastError).toBeNull();
  });
});

function emptySnapshot(): Snapshot {
  return {
    status: "idle",
    currentServer: null,
    mode: "auto",
    speeds: { upBps: 0, downBps: 0, at: "0001-01-01T00:00:00Z" },
    helperState: "missing",
    servers: [],
    subs: [],
    settings: defaultSettings(),
    onboarded: false,
    version: "test",
  };
}

function defaultSettings(): SettingsView {
  return {
    general: { language: "auto", theme: "dark", autostart: false, closeToTray: true, startMinimized: false },
    network: { defaultMode: "auto", tunCidr: "198.18.0.1/15", tunName: "ITGRay-TUN", socksPort: 1080, xrayPort: 1081 },
    subscriptions: { defaultUpdateInterval: 3600, userAgent: "ITG-Ray/0.1" },
    notifications: { onConnected: true, onDisconnected: true, onError: true, onSubSynced: false },
    debug: { logLevel: "info" },
    about: { version: "test", gitRev: "", buildDate: "" },
    security: { method: "Unencrypted", available: false, warning: "" },
  };
}
