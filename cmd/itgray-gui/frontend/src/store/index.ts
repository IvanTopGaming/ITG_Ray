import { create } from "zustand";
import type {
  Snapshot,
  ServerView,
  SubView,
  SettingsView,
  ChainStatus,
  SpeedSample,
} from "../api/client";

export type StoreState = Snapshot & {
  setSnapshot: (s: Snapshot) => void;
  applyVPNStatus: (status: ChainStatus) => void;
  applySpeed: (s: Partial<SpeedSample>) => void;
  applySubSync: (e: { id: string; status: string; at: string; importedCount: number; message?: string }) => void;
  applyProbeResult: (e: { results: Array<{ id: string; latencyMs: number; error?: string }> }) => void;
  applyHelperState: (state: string) => void;
  applyChainError: (e: { kind: string; message: string }) => void;
};

const initial: Snapshot = {
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
};

export const useStore = create<StoreState>((set) => ({
  ...initial,
  setSnapshot: (s) => set(() => ({ ...s })),
  applyVPNStatus: (status) => set({ status }),
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
        return r ? { ...srv, latencyMs: r.error ? 0 : r.latencyMs } : srv;
      }),
    })),
  applyHelperState: (state) => set({ helperState: state }),
  applyChainError: () => set({ status: "error" }),
}));
