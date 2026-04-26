// Re-exports Wails-generated bindings under stable names; this file is the
// only place the rest of the app touches `window.go.*`. Types are mirrored
// from `wailsjs/go/models.ts` (auto-generated) so that downstream code can
// rely on plain interfaces rather than the generated class shapes.

import { GetSnapshot as wailsGetSnapshot, GetVersion as wailsGetVersion } from "../../wailsjs/go/bindings/AppService";

export type ChainStatus = "idle" | "connecting" | "connected" | "disconnecting" | "error";

export interface SpeedSample {
  upBps: number;
  downBps: number;
  at: string;
}

export interface ServerView {
  id: string;
  name: string;
  country: string;
  address: string;
  transport: string;
  security: string;
  latencyMs: number;
  origin: string;
  favorite: boolean;
  tags: string[];
}

export interface SubView {
  id: string;
  name: string;
  url: string;
  updateInterval: number;
  lastSyncAt: string;
  lastSyncStatus: string;
  lastSyncMessage?: string;
  serverCount: number;
}

// SettingsView shape mirrors the Go DTO; section-by-section so tests can
// construct fixtures without leaning on generated classes.
export interface SettingsView {
  general: {
    language: string;
    theme: string;
    autostart: boolean;
    closeToTray: boolean;
    startMinimized: boolean;
  };
  network: {
    defaultMode: string;
    tunCidr: string;
    tunName: string;
    socksPort: number;
    xrayPort: number;
  };
  subscriptions: {
    defaultUpdateInterval: number;
    userAgent: string;
  };
  notifications: {
    onConnected: boolean;
    onDisconnected: boolean;
    onError: boolean;
    onSubSynced: boolean;
  };
  debug: {
    logLevel: string;
  };
  about: {
    version: string;
    gitRev: string;
    buildDate: string;
  };
  security: {
    method: string;
    available: boolean;
    warning?: string;
  };
}

export interface Snapshot {
  status: ChainStatus;
  currentServer: ServerView | null;
  mode: string;
  speeds: SpeedSample;
  helperState: string;
  servers: ServerView[];
  subs: SubView[];
  settings: SettingsView;
  onboarded: boolean;
  version: string;
}

// Wails v2 generates the TS signature with a `context.Context` arg even
// though the runtime injects it transparently — the JS shim simply forwards
// whatever JS passes (undefined is fine; the runtime fills it in). We cast
// through `unknown` to keep the call site clean.
const wailsGetSnapshotAny = wailsGetSnapshot as unknown as () => Promise<Snapshot>;

export const api = {
  getSnapshot: (): Promise<Snapshot> => wailsGetSnapshotAny(),
  getVersion: (): Promise<string> => wailsGetVersion(),
};
