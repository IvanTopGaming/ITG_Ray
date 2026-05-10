// cmd/itgray-electron/src/preload/preload.ts
import { contextBridge, ipcRenderer } from "electron";
import type { RpcMethod, RpcParams, RpcResult } from "../shared/protocol";

function rpc<M extends RpcMethod>(method: M, params?: RpcParams<M>): Promise<RpcResult<M>> {
  return ipcRenderer.invoke("rpc", method, params ?? null);
}

function on(topic: string, cb: (payload: unknown) => void): () => void {
  const channel = `event:${topic}`;
  const handler = (_e: Electron.IpcRendererEvent, payload: unknown) => cb(payload);
  ipcRenderer.on(channel, handler);
  return () => ipcRenderer.off(channel, handler);
}

contextBridge.exposeInMainWorld("itg", {
  app: {
    ping: () => rpc("app.ping"),
    getSnapshot: () => rpc("app.getSnapshot"),
    quit: () => ipcRenderer.invoke("app.quit"),
  },
  onboarding: {
    getState: () => rpc("onboarding.getState"),
    complete: () => rpc("onboarding.complete"),
    skip: () => rpc("onboarding.skip"),
  },
  settings: {
    get: () => rpc("settings.get"),
    update: (section: string, patch: Record<string, unknown>) =>
      rpc("settings.update", { section, patch }),
  },
  helper: {
    status: () => rpc("helper.status"),
    install: () => rpc("helper.install"),
    start: () => rpc("helper.start"),
    stop: () => rpc("helper.stop"),
    restart: () => rpc("helper.restart"),
    reinstall: () => rpc("helper.reinstall"),
  },
  on,
});
