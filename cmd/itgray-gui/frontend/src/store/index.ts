import { create } from "zustand";
import type {
  Snapshot,
  ServerView,
  SubView,
  SettingsView,
  ChainStatus,
  SpeedSample,
} from "../api/client";

export interface ChainErrorEvent {
  kind: string;
  message: string;
}

export type StoreState = Snapshot & {
  // Last chain:error event payload — null until the first error arrives.
  // Cleared when status returns to idle/connecting via setSnapshot or applyVPNStatus.
  lastError: ChainErrorEvent | null;
  setSnapshot: (s: Snapshot) => void;
  applyVPNStatus: (status: ChainStatus) => void;
  applySpeed: (s: Partial<SpeedSample>) => void;
  applySubSync: (e: { id: string; status: string; at: string; importedCount: number; message?: string }) => void;
  applyProbeResult: (e: { results: Array<{ id: string; latencyMs: number; error?: string }> }) => void;
  applyHelperState: (state: string) => void;
  applyChainError: (e: ChainErrorEvent) => void;
};

const initial: Snapshot & { lastError: ChainErrorEvent | null } = {
  status: "idle",
  currentServer: null,
  mode: "auto",
  speeds: { upBps: 0, downBps: 0, at: "0001-01-01T00:00:00Z" },
  helperState: "missing",
  servers: [],
  subs: [],
  settings: {} as SettingsView,
  onboarded: false,
  version: "",
  lastError: null,
};

export const useStore = create<StoreState>((set) => ({
  ...initial,
  setSnapshot: (s) => set(() => ({ ...s, lastError: null })),
  applyVPNStatus: (status) =>
    set((cur) => ({
      status,
      lastError: status === "error" ? cur.lastError : null,
    })),
  applySpeed: (s) =>
    set((cur) => ({
      speeds: { ...cur.speeds, ...s, at: new Date().toISOString() },
    })),
  applySubSync: (e) =>
    set((cur) => ({
      subs: cur.subs.map((sub: SubView) =>
        sub.id === e.id
          ? {
              ...sub,
              lastSyncAt: e.at,
              lastSyncStatus: e.status,
              lastSyncMessage: e.message ?? "",
              serverCount: e.importedCount,
            }
          : sub,
      ),
    })),
  applyProbeResult: (e) =>
    set((cur) => ({
      servers: cur.servers.map((srv: ServerView) => {
        const r = e.results.find((rr) => rr.id === srv.id);
        if (!r) return srv;
        // Transient probe failure must not wipe the previously-known good RTT
        // (otherwise the badge flickers to em-dash on every error). Only
        // overwrite latencyMs on a successful probe.
        if (r.error) return srv;
        return { ...srv, latencyMs: r.latencyMs };
      }),
    })),
  applyHelperState: (state) => set({ helperState: state }),
  applyChainError: (e) =>
    set({ status: "error", lastError: { kind: e.kind, message: e.message } }),
}));
