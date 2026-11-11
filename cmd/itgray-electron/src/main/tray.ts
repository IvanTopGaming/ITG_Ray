// cmd/itgray-electron/src/main/tray.ts
import { Tray, Menu, nativeImage, BrowserWindow, type MenuItemConstructorOptions } from "electron";
import path from "node:path";
import { isDevMode } from "./paths";
import type { TrayCommand, TrayItem, TrayStatus } from "./tray-menu";

const STATUS_TO_ICON: Record<TrayStatus, string> = {
  idle: "tray-idle.png",
  connecting: "tray-connecting.png",
  disconnecting: "tray-connecting.png",
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

export function toMenuTemplate(items: TrayItem[], onCommand: (cmd: TrayCommand) => void): MenuItemConstructorOptions[] {
  return items.map((item) => {
    if (item.type === "separator") return { type: "separator" };
    const opts: MenuItemConstructorOptions = {
      label: item.label,
      enabled: item.enabled ?? true,
    };
    if (item.submenu) opts.submenu = toMenuTemplate(item.submenu, onCommand);
    if (item.type === "radio") {
      opts.type = "radio";
      opts.checked = item.checked ?? false;
    }
    const cmd = item.command;
    if (cmd) opts.click = () => onCommand(cmd);
    return opts;
  });
}

/**
 * Creates the system tray, sets the initial idle icon and the click handler.
 * Returns the Tray reference (the caller MUST keep it alive — losing the
 * reference triggers GC and the icon disappears on Linux) plus setters the
 * tray controller drives.
 */
export function createTray(
  getWindow: () => BrowserWindow | null,
  summonWindow: () => void,
): {
  tray: Tray;
  setIcon: (s: TrayStatus) => void;
  setTooltip: (text: string) => void;
  setMenu: (items: TrayItem[], onCommand: (cmd: TrayCommand) => void) => void;
} {
  const initial = nativeImage.createFromPath(resourcePath(STATUS_TO_ICON.idle));
  const tray = new Tray(initial);
  tray.setToolTip("ITG Ray");

  const showOrHide = () => {
    const win = getWindow();
    if (!win) {
      // Window was closed (X button) but app stayed alive in tray.
      // Re-create the window so the user can bring the UI back.
      summonWindow();
      return;
    }
    if (win.isVisible()) win.hide();
    else {
      win.show();
      win.focus();
    }
  };

  tray.on("click", showOrHide);

  let lastTooltip = "";
  return {
    tray,
    setIcon(s: TrayStatus) {
      const file = STATUS_TO_ICON[s] ?? STATUS_TO_ICON.idle;
      tray.setImage(nativeImage.createFromPath(resourcePath(file)));
    },
    setTooltip(text: string) {
      if (text === lastTooltip) return;
      lastTooltip = text;
      tray.setToolTip(text);
    },
    setMenu(items: TrayItem[], onCommand: (cmd: TrayCommand) => void) {
      tray.setContextMenu(Menu.buildFromTemplate(toMenuTemplate(items, onCommand)));
    },
  };
}
