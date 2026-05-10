// cmd/itgray-electron/src/main/ipc.ts
import { ipcMain, BrowserWindow } from "electron";
import type { BridgeSupervisor } from "./bridge";
import type { RpcMethod } from "../shared/protocol";

/**
 * Registers the renderer ↔ bridge IPC handlers. The renderer calls
 *   ipcRenderer.invoke('rpc', method, params)
 * and gets back the bridge response. Bridge → renderer notifications are
 * forwarded by topic on channel `event:<topic>`.
 */
export function wireIPC(supervisor: BridgeSupervisor, getWindow: () => BrowserWindow | null): void {
  ipcMain.handle("rpc", async (_event, method: RpcMethod, params: unknown) => {
    return supervisor.rpc().call(method, params as never);
  });

  // Forward every bridge notification to the renderer. For Phase 0 we
  // forward all topics blindly; later phases coalesce vpn.speed at 100ms.
  const knownTopics = ["bridge.state"]; // Phase 0 only — bus emits this from BridgeSupervisor itself.
  supervisor.on("state", (payload) => {
    const win = getWindow();
    if (win) win.webContents.send("event:bridge.state", payload);
  });

  // Generic forwarding for any other topic from the bridge:
  const rpc = supervisor.rpc();
  // Phase 0 has no topics from the bridge yet — wiring is a stub.
  void rpc; void knownTopics;
}
