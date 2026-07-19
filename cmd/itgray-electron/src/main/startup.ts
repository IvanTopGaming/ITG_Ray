// cmd/itgray-electron/src/main/startup.ts

/** Tray icon states, mirroring the Status union in tray.ts. */
export type TrayStatus = "idle" | "connecting" | "connected" | "error";

/** Narrow view of the app.getSnapshot result that startup logic reads. */
export interface StartupSnapshot {
  status?: string;
  settings?: { general?: { startMinimized?: boolean; autostart?: boolean } };
}

/**
 * resolveTrayStatus maps a startup snapshot's chain status onto a tray icon.
 *
 * The tray is otherwise fed only by live vpn.status notifications, but the
 * bridge publishes the adopted-chain status from Reconcile() during its own
 * boot — before the main process subscribes — and RpcClient drops
 * notifications that have no listener. Without this pull-based seed the tray
 * stays grey after an app restart that left the helper's tunnel up.
 *
 * Returns null when the snapshot carries no status we have an icon for, so
 * the caller can leave the tray at its creation-time idle icon.
 */
export function resolveTrayStatus(snap: unknown): TrayStatus | null {
  const status = (snap as StartupSnapshot | undefined)?.status;
  switch (status) {
    case "idle":
    case "connecting":
    case "connected":
    case "error":
      return status;
    // hub.StatusDisconnecting has no icon of its own; it is a transient
    // teardown state, so borrow the connecting (in-flight) icon.
    case "disconnecting":
      return "connecting";
    default:
      return null;
  }
}

/**
 * resolveStartMinimized decides whether the main window should stay hidden
 * (tray-only) on launch. Defaults to false (show) for any missing/malformed
 * snapshot so a snapshot read failure never hides the UI from the user.
 */
export function resolveStartMinimized(snap: unknown): boolean {
  const s = snap as StartupSnapshot | undefined;
  return s?.settings?.general?.startMinimized === true;
}
