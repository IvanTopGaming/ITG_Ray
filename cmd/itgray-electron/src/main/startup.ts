// cmd/itgray-electron/src/main/startup.ts

/** Narrow view of the app.getSnapshot result that startup logic reads. */
export interface StartupSnapshot {
  settings?: { general?: { startMinimized?: boolean; autostart?: boolean } };
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
