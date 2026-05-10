// cmd/itgray-electron/frontend/wails-shim/models.ts
//
// Wails-generated models.ts re-exported from the bridge protocol types.
// The hub.* namespace keeps the names the existing React code expects.
//
// TypeScript's "namespace + class merge" pattern: each name like SubView is
// declared both as a TYPE (for `hub.SubView` annotations) and as a VALUE
// (for `hub.SubView.createFrom(...)` runtime calls). This bypasses Wails'
// generated class-based unmarshalling — Phase 3 will replace this with
// codegen-driven typed bindings.

import type * as protocol from "../../src/shared/protocol";

class ServerViewClass {
  static createFrom(source: any = {}): protocol.ServerView {
    return source as protocol.ServerView;
  }
}
class SubViewClass {
  static createFrom(source: any = {}): protocol.SubView {
    return source as protocol.SubView;
  }
}
class SnapshotClass {
  static createFrom(source: any = {}): protocol.Snapshot {
    return source as protocol.Snapshot;
  }
}
class SettingsViewClass {
  static createFrom(source: any = {}): protocol.SettingsView {
    return source as protocol.SettingsView;
  }
}

export namespace hub {
  // Domain views
  export type ServerView = protocol.ServerView;
  export type SubView = protocol.SubView;
  export type Snapshot = protocol.Snapshot;
  export type SnapshotView = protocol.Snapshot; // some callers used the older name
  export type SettingsView = protocol.SettingsView;
  export type SpeedSample = protocol.SpeedSample;

  // Settings sub-types referenced by Settings.tsx and tests
  export type AboutSettings = protocol.AboutSettings;
  export type GeneralSettings = protocol.GeneralSettings;
  export type NetworkSettings = protocol.NetworkSettings;
  export type NotificationSettings = protocol.NotificationSettings;
  export type DebugSettings = protocol.DebugSettings;
  export type KillSwitchSettings = protocol.KillSwitchSettings;
  export type SubscriptionSettings = protocol.SubscriptionSettings;
  export type SecuritySettings = protocol.SecuritySettings;
  export type DNSSettings = protocol.DNSSettings;

  // Runtime values for callers that use createFrom() — bypasses Wails'
  // generated class-based unmarshalling. The matching type declarations
  // above let `hub.SubView` work as both an annotation and a value.
  export const ServerView = ServerViewClass;
  export const SubView = SubViewClass;
  export const Snapshot = SnapshotClass;
  export const SnapshotView = SnapshotClass;
  export const SettingsView = SettingsViewClass;
}
