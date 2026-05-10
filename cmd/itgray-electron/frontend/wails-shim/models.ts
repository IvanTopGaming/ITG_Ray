// cmd/itgray-electron/frontend/wails-shim/models.ts
//
// Wails-generated models.ts re-exported from the bridge protocol types.
// The hub.* namespace keeps the names the existing React code expects.

import type * as protocol from "../../src/shared/protocol";

export namespace hub {
  // Re-export each domain type. Names match what the existing React
  // code imports from the old wailsjs/go/models module.
  export type ServerView = protocol.ServerView;
  export type SubView = protocol.SubView;
  export type SnapshotView = protocol.Snapshot;  // codegen emits "Snapshot" not "SnapshotView"
  export type SettingsView = protocol.SettingsView;
  export type SpeedSample = protocol.SpeedSample;
  // Add others as the build catches missing names — the renderer may
  // import additional types like GeneralSettings, NetworkSettings, etc.
}
