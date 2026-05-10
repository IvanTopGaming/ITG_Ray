// cmd/itgray-electron/src/main/tray.ts
import { Tray, Menu, app, nativeImage, BrowserWindow } from "electron";
import path from "node:path";
import { isDevMode } from "./paths";

type Status = "idle" | "connecting" | "connected" | "error";

const STATUS_TO_ICON: Record<Status, string> = {
  idle: "tray-idle.png",
  connecting: "tray-connecting.png",
  connected: "tray-connected.png",
  error: "tray-error.png",
};

function resourcePath(file: string): string {
  if (isDevMode()) {
    // From cmd/itgray-electron/dist-main/main/tray.js, two levels up to
    // cmd/itgray-electron/, then into resources/.
    return path.join(__dirname, "..", "..", "resources", file);
  }
  return path.join(process.resourcesPath, "resources", file);
}

/**
 * Creates the system tray, sets the initial idle icon, wires the menu
 * and click handler. Returns the Tray reference (the caller MUST keep it
 * alive — losing the reference triggers GC and the icon disappears on
 * Linux) plus a setStatus(s) updater.
 */
export function createTray(getWindow: () => BrowserWindow | null): {
  tray: Tray;
  setStatus: (s: Status) => void;
} {
  const initial = nativeImage.createFromPath(resourcePath(STATUS_TO_ICON.idle));
  const tray = new Tray(initial);
  tray.setToolTip("ITG Ray");

  const showOrHide = () => {
    const win = getWindow();
    if (!win) return;
    if (win.isVisible()) win.hide();
    else {
      win.show();
      win.focus();
    }
  };

  const menu = Menu.buildFromTemplate([
    { label: "Show / Hide", click: showOrHide },
    { type: "separator" },
    { label: "Quit ITG Ray", click: () => app.quit() },
  ]);
  tray.setContextMenu(menu);
  tray.on("click", showOrHide);

  return {
    tray,
    setStatus(s: Status) {
      const file = STATUS_TO_ICON[s] ?? STATUS_TO_ICON.idle;
      tray.setImage(nativeImage.createFromPath(resourcePath(file)));
    },
  };
}
