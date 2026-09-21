// cmd/itgray-electron/src/main/ipc.ts
import { app, ipcMain, dialog, BrowserWindow } from "electron";
import { writeFile } from "node:fs/promises";
import { BRIDGE_TOPICS, type BridgeSupervisor } from "./bridge";
import type { RpcMethod } from "../shared/protocol";
import { defaultAutostart } from "./autostart";
import { createAutoConnectClaim } from "./autoconnect";

// This launch's single auto-connect. Lives in main because the renderer is
// re-created whenever the window is closed to the tray and re-opened, which
// resets any guard held there. See autoconnect.ts.
const claimAutoConnect = createAutoConnectClaim();

/**
 * Registers the renderer ↔ bridge IPC handlers. The renderer calls
 *   ipcRenderer.invoke('rpc', method, params)
 * and gets back the bridge response. Bridge → renderer notifications are
 * forwarded by topic on channel `event:<topic>`.
 */
export function wireIPC(
  supervisor: BridgeSupervisor,
  getWindow: () => BrowserWindow | null,
  trayStatus?: (s: "idle" | "connecting" | "connected" | "error") => void,
): void {
  ipcMain.handle("rpc", async (_event, method: RpcMethod, params: unknown) => {
    return supervisor.rpc().call(method, params as never);
  });

  // app.quit — Electron-native (does NOT go through the bridge).
  ipcMain.handle("app.quit", () => {
    app.quit();
  });

  // Returns true to the first caller of this app launch and false to every
  // one after it, so auto-connect fires once per launch no matter how often
  // the renderer is re-created.
  ipcMain.handle("app.claimAutoConnect", () => claimAutoConnect());

  // Window controls — drive the custom frameless title bar.
  ipcMain.handle("window.minimise", () => {
    const win = getWindow();
    if (win) win.minimize();
  });
  ipcMain.handle("window.toggleMaximise", () => {
    const win = getWindow();
    if (!win) return;
    if (win.isMaximized()) win.unmaximize();
    else win.maximize();
  });
  ipcMain.handle("window.isMaximised", () => {
    const win = getWindow();
    return win ? win.isMaximized() : false;
  });
  ipcMain.handle("window.close", () => {
    const win = getWindow();
    if (win) win.close();
  });

  // Autostart — read or write OS-level autostart entry directly. The
  // renderer also calls settings.update to persist user intent in
  // config.json so the reconciler can re-apply on next launch.
  ipcMain.handle("app.getAutostart", async () => {
    return defaultAutostart().get();
  });
  ipcMain.handle("app.setAutostart", async (_e, enabled: boolean) => {
    await defaultAutostart().set(enabled);
    return true;
  });

  // logs.save — Electron-native. Prompts for a destination and writes the
  // combined log text (fetched by the renderer via the logs.export RPC).
  ipcMain.handle("logs.save", async (_e, text: string) => {
    const opts = {
      defaultPath: `itgray-logs-${new Date().toISOString().slice(0, 10)}.txt`,
    };
    const win = getWindow();
    const { canceled, filePath } = win
      ? await dialog.showSaveDialog(win, opts)
      : await dialog.showSaveDialog(opts);
    if (canceled || !filePath) return null;
    await writeFile(filePath, text, "utf8");
    return filePath;
  });

  // Supervisor lifecycle → renderer (only path for bridge.state).
  supervisor.on("state", (payload) => {
    const win = getWindow();
    if (win) win.webContents.send("event:bridge.state", payload);
  });

  for (const topic of BRIDGE_TOPICS) {
    supervisor.on(topic, (payload) => {
      const win = getWindow();
      if (win) win.webContents.send(`event:${topic}`, payload);
      if (topic === "vpn.status" && trayStatus) {
        const status = (payload as { status?: string } | null)?.status;
        if (status === "idle" || status === "connecting" || status === "connected" || status === "error") {
          trayStatus(status);
        }
      }
    });
  }
}
