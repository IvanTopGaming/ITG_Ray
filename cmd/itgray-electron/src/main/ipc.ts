// cmd/itgray-electron/src/main/ipc.ts
import { app, ipcMain, BrowserWindow } from "electron";
import type { BridgeSupervisor } from "./bridge";
import type { RpcMethod, EventTopic } from "../shared/protocol";

// Topics emitted by the bridge subprocess (not the supervisor itself).
// `bridge.state` is supervisor-driven and forwarded separately below.
const BRIDGE_TOPICS: Exclude<EventTopic, "bridge.state">[] = [
  "chain.error",
  "helper.state",
  "probe.result",
  "servers.changed",
  "sub.synced",
  "vpn.speed",
  "vpn.status",
];

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

  // Supervisor lifecycle → renderer (only path for bridge.state).
  supervisor.on("state", (payload) => {
    const win = getWindow();
    if (win) win.webContents.send("event:bridge.state", payload);
  });

  // Generic forwarder for every bridge-emitted topic. Subscribed once via
  // the live RpcClient. The supervisor.rpc() throw guard means this runs
  // after supervisor.start() — wireIPC is called from index.ts after start().
  // A future Phase 3+ optimisation can coalesce vpn.speed at ~100ms here.
  const rpc = supervisor.rpc();
  for (const topic of BRIDGE_TOPICS) {
    rpc.on(topic, (payload) => {
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
