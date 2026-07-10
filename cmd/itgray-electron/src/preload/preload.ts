// cmd/itgray-electron/src/preload/preload.ts
import { contextBridge, ipcRenderer } from "electron";
import type {
  RpcMethod,
  RpcParams,
  RpcResult,
  RulesReplaceAllParams,
  RulesRuleAddParams,
  RulesRuleEditParams,
} from "../shared/protocol";

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
    getPublicIP: () => rpc("app.getPublicIP"),
    quit: () => ipcRenderer.invoke("app.quit"),
    getAutostart: () => ipcRenderer.invoke("app.getAutostart") as Promise<boolean>,
    setAutostart: (enabled: boolean) => ipcRenderer.invoke("app.setAutostart", enabled) as Promise<boolean>,
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
    installLinux: () => rpc("helper.installLinux"),
    uninstallLinux: () => rpc("helper.uninstallLinux"),
  },
  servers: {
    list: () => rpc("servers.list"),
    add: (params: { uri: string; name: string }) => rpc("servers.add", params),
    edit: (params: { id: string; uri: string; name: string }) =>
      rpc("servers.edit", params),
    remove: (params: { id: string }) => rpc("servers.remove", params),
    toggleFavorite: (params: { id: string }) =>
      rpc("servers.toggleFavorite", params),
    testLatency: (params: { id: string }) => rpc("servers.testLatency", params),
  },
  subs: {
    list: () => rpc("subs.list"),
    add: (params: { url: string; name: string }) => rpc("subs.add", params),
    edit: (params: { id: string; url: string; name: string }) =>
      rpc("subs.edit", params),
    remove: (params: { id: string }) => rpc("subs.remove", params),
    syncOne: (params: { id: string }) => rpc("subs.syncOne", params),
    syncAll: () => rpc("subs.syncAll"),
  },
  rules: {
    list: () => rpc("rules.list"),
    replaceAll: (params: RulesReplaceAllParams) => rpc("rules.replaceAll", params),
    groupAdd: (params: { name: string }) => rpc("rules.groupAdd", params),
    groupEdit: (params: { id: string; name: string; enabled: boolean }) =>
      rpc("rules.groupEdit", params),
    groupRemove: (params: { id: string }) => rpc("rules.groupRemove", params),
    ruleAdd: (params: RulesRuleAddParams) =>
      rpc("rules.ruleAdd", params),
    ruleEdit: (params: RulesRuleEditParams) => rpc("rules.ruleEdit", params),
    ruleRemove: (params: { id: string }) => rpc("rules.ruleRemove", params),
    ruleToggle: (params: { id: string }) => rpc("rules.ruleToggle", params),
    ruleMove: (params: { id: string; toGroupId: string }) =>
      rpc("rules.ruleMove", params),
  },
  window: {
    minimise: () => ipcRenderer.invoke("window.minimise"),
    toggleMaximise: () => ipcRenderer.invoke("window.toggleMaximise"),
    isMaximised: () => ipcRenderer.invoke("window.isMaximised") as Promise<boolean>,
    close: () => ipcRenderer.invoke("window.close"),
  },
  run: {
    connect: (params: { serverId: string; mode: string }) =>
      rpc("run.connect", params),
    disconnect: () => rpc("run.disconnect"),
  },
  on,
});
