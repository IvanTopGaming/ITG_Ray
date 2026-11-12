// cmd/itgray-electron/src/main/index.ts
import { app, BrowserWindow, Notification, ipcMain } from "electron";
import path from "node:path";
import { BridgeSupervisor } from "./bridge";
import { wireIPC } from "./ipc";
import { isDevMode } from "./paths";
import { createTray } from "./tray";
import { createTrayController, type RendererReport, type TrayController } from "./tray-controller";
import { loadLastServer, saveLastServer } from "./last-server";
import type { RpcMethod } from "../shared/protocol";
import { loadState, attachStatePersister } from "./window-state";
import { defaultAutostart } from "./autostart";
import { makeNotifier } from "./notifications";
import { resolveStartMinimized, type StartupSnapshot } from "./startup";
import { extractDeeplink } from "./deeplink";
import { checkForUpdate, registerUpdateIPC } from "./updates";
import { createAutoUpdate, loadUpdatePrefs, saveUpdatePrefs } from "./auto-update";

const AUTO_UPDATE_DELAY_MS = 10_000;

let mainWindow: BrowserWindow | null = null;
let supervisor: BridgeSupervisor | null = null;
let tray: ReturnType<typeof createTray> | null = null;
let trayController: TrayController | null = null;
let pendingNavigate: string | null = null;
let quitting = false;
let pendingDeeplink: string | null = null;

function deliverDeeplink(url: string): void {
  pendingDeeplink = url;
  if (mainWindow && !mainWindow.webContents.isLoading()) {
    mainWindow.webContents.send("event:deeplink", url);
    pendingDeeplink = null;
  }
}

app.setName("ITG Ray");
// Windows: attribute OS notifications to the installed app (the NSIS
// shortcut's AppUserModelID = electron-builder appId). Without this,
// toasts show the default "electron.app.Electron" identifier instead of
// "ITG Ray". No-op on other platforms.
app.setAppUserModelId("com.itgray.app");

app.setPath("userData", path.join(app.getPath("appData"), "ITG Ray"));
app.setAsDefaultProtocolClient("itgray");
pendingDeeplink = extractDeeplink(process.argv);

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  // Another Electron instance owns the lock — exit immediately so the
  // primary instance's "second-instance" handler can refocus its window.
  app.quit();
} else {
  app.on("second-instance", (_event, argv) => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      // The window may be hidden (start-minimized launch builds it with
      // show:false), in which case it is not "minimized" — show() is what
      // actually reveals it. restore() above still handles the minimized case.
      mainWindow.show();
      mainWindow.focus();
    }
    const url = extractDeeplink(argv);
    if (url) deliverDeeplink(url);
  });
}

async function createWindow(): Promise<void> {
  const stateFile = path.join(app.getPath("userData"), "window-state.json");
  const state = await loadState(stateFile);

  mainWindow = new BrowserWindow({
    x: state.x ?? undefined,
    y: state.y ?? undefined,
    width: state.width,
    height: state.height,
    title: "ITG Ray",
    frame: false,
    show: false,
    webPreferences: {
      preload: path.join(__dirname, "..", "..", "dist-preload", "preload", "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  });

  if (state.maximised) mainWindow.maximize();
  attachStatePersister(mainWindow, stateFile);

  if (isDevMode()) {
    mainWindow.loadURL("http://localhost:34115");
    mainWindow.webContents.openDevTools({ mode: "detach" });
  } else {
    mainWindow.loadFile(path.join(__dirname, "..", "..", "dist-frontend", "index.html"));
  }

  mainWindow.webContents.on("did-finish-load", () => {
    if (pendingDeeplink && mainWindow) {
      mainWindow.webContents.send("event:deeplink", pendingDeeplink);
      pendingDeeplink = null;
    }
  });

  mainWindow.on("closed", () => {
    mainWindow = null;
    trayController?.onRendererGone();
  });
}

app.whenReady().then(async () => {
  supervisor = new BridgeSupervisor();
  supervisor.start();
  await createWindow();
  // Tray "summon" is an explicit request to see the window. createWindow
  // builds it hidden (show:false) so the launch path can honor
  // start-minimized; this path must show it after (re)creating.
  const openWindow = async (): Promise<void> => {
    if (!mainWindow) await createWindow();
    if (mainWindow?.isMinimized()) mainWindow.restore();
    mainWindow?.show();
    mainWindow?.focus();
  };
  tray = createTray(() => mainWindow, () => void openWindow());
  const trayStateFile = path.join(app.getPath("userData"), "tray-state.json");
  const trayIcon = tray;
  const controller = createTrayController({
    rpc: (method, params) => supervisor!.rpc().call(method as RpcMethod, params as never),
    hasRenderer: () =>
      !!mainWindow && !mainWindow.isDestroyed() && !mainWindow.webContents.isLoading(),
    sendToRenderer: (cmd) => mainWindow?.webContents.send("event:tray.command", cmd),
    showWindow: openWindow,
    queueNavigate: (p) => {
      pendingNavigate = p;
    },
    quit: () => app.quit(),
    setMenu: (items) => trayIcon.setMenu(items, (cmd) => void controller.dispatch(cmd)),
    setTooltip: (text) => trayIcon.setTooltip(text),
    setIcon: (s) => trayIcon.setIcon(s),
    locale: () => app.getLocale(),
    loadLastServer: () => loadLastServer(trayStateFile),
    saveLastServer: (id) => saveLastServer(trayStateFile, id),
  });
  trayController = controller;
  supervisor.on("vpn.status", (p) => controller.onVpnStatus(p));
  supervisor.on("vpn.speed", (p) => controller.onVpnSpeed(p));
  supervisor.on("routing.mode", (p) => controller.onRoutingMode(p));
  supervisor.on("servers.changed", () => void controller.refresh());
  supervisor.on("sub.synced", () => void controller.refresh());
  supervisor.on("state", (p: { state?: string }) => {
    if (p?.state === "running") void controller.refresh();
  });
  ipcMain.on("tray.report", (_e, report: RendererReport) => {
    if (report && typeof report === "object") controller.onRendererReport(report);
  });
  ipcMain.handle("tray.takePendingNavigate", () => {
    const p = pendingNavigate;
    pendingNavigate = null;
    return p;
  });
  wireIPC(supervisor, () => mainWindow);
  registerUpdateIPC();

  const updatePrefsFile = path.join(app.getPath("userData"), "update-prefs.json");
  const autoUpdate = createAutoUpdate({
    getVersion: async () => {
      const ping = (await supervisor!.rpc().call("app.ping", undefined)) as { version?: string };
      return ping?.version ?? "";
    },
    check: (v) => checkForUpdate(v),
    loadPrefs: () => loadUpdatePrefs(updatePrefsFile),
    savePrefs: (p) => saveUpdatePrefs(updatePrefsFile, p),
    publish: (u) => mainWindow?.webContents.send("event:update.available", u),
  });
  ipcMain.handle("update.pending", () => autoUpdate.pending());
  ipcMain.handle("update.skip", (_e, version: unknown) => {
    if (typeof version === "string" && version) autoUpdate.skip(version);
  });
  ipcMain.handle("update.getAutoCheck", () => autoUpdate.autoCheck());
  ipcMain.handle("update.setAutoCheck", (_e, enabled: unknown) => {
    autoUpdate.setAutoCheck(enabled === true);
    return autoUpdate.autoCheck();
  });
  setTimeout(() => void autoUpdate.run(), AUTO_UPDATE_DELAY_MS);

  ipcMain.handle("deeplink.getPending", () => {
    const url = pendingDeeplink;
    pendingDeeplink = null;
    return url ?? null;
  });

  // OS notifications on connect / disconnect / sub-synced. Prefs are read
  // fresh per event via the bridge snapshot; sound maps to !silent.
  const notifier = makeNotifier({
    notify: (title, body, opts) => {
      if (!Notification.isSupported()) return;
      new Notification({ title, body, silent: opts.silent }).show();
    },
    getSettings: async () => {
      const snap = (await supervisor!.rpc().call("app.getSnapshot", undefined)) as {
        settings?: { notifications?: Partial<import("./notifications").NotifPrefs> };
      };
      const n = snap.settings?.notifications;
      return {
        onConnected: n?.onConnected ?? false,
        onDisconnected: n?.onDisconnected ?? false,
        onSubSynced: n?.onSubSynced ?? false,
        sound: n?.sound ?? true,
      };
    },
  });
  supervisor.on("vpn.status", (p) => void notifier.onVpnStatus(p));
  supervisor.on("sub.synced", (p) => void notifier.onSubSynced(p));

  // One snapshot read drives autostart reconcile AND start-minimized.
  void (async () => {
    let snap: StartupSnapshot | undefined;
    try {
      // Bound the read: the window is created hidden, so its visibility now
      // hinges on this resolving. RpcClient.call has no timeout — a live but
      // unresponsive bridge would otherwise leave the window invisible
      // forever. On timeout snap stays undefined → window shows (fail-safe).
      snap = (await Promise.race([
        supervisor!.rpc().call("app.getSnapshot", undefined),
        new Promise<undefined>((resolve) => setTimeout(() => resolve(undefined), 3000)),
      ])) as StartupSnapshot | undefined;
    } catch (err) {
      console.warn("startup snapshot read failed:", err);
    }
    // Tray adopt: the bridge's Reconcile() announces a helper-held tunnel
    // before the tray subscribes, so seed it from a pull instead of waiting
    // for a vpn.status event that already came and went.
    void trayController?.refresh();
    // Window visibility: show unless the user opted into tray-only launch.
    if (mainWindow && !resolveStartMinimized(snap)) {
      mainWindow.show();
      mainWindow.focus();
    }
    // Autostart reconcile (non-fatal).
    try {
      const desired = snap?.settings?.general?.autostart;
      if (typeof desired === "boolean") {
        await defaultAutostart().reconcile(desired);
      }
    } catch (err) {
      console.warn("autostart reconcile skipped:", err);
    }
  })();
});

app.on("window-all-closed", () => {
  // Tray-only mode — do NOT quit when last window closes. Window can
  // be brought back via the tray. app.quit() is the only path that
  // triggers actual shutdown (via the before-quit hook below).
});

app.on("before-quit", (event) => {
  if (quitting) return;
  // First invocation: defer the actual exit so we can await supervisor
  // teardown — Electron does not await async before-quit listeners.
  event.preventDefault();
  quitting = true;
  void (async () => {
    try {
      if (supervisor) await supervisor.stop();
    } catch (err) {
      console.warn("supervisor.stop failed during shutdown:", err);
    } finally {
      app.exit(0);
    }
  })();
});
