// cmd/itgray-electron/src/main/index.ts
import { app, BrowserWindow, Notification } from "electron";
import path from "node:path";
import { BridgeSupervisor } from "./bridge";
import { wireIPC } from "./ipc";
import { isDevMode } from "./paths";
import { createTray } from "./tray";
import { loadState, attachStatePersister } from "./window-state";
import { defaultAutostart } from "./autostart";
import { makeNotifier } from "./notifications";
import { resolveStartMinimized, type StartupSnapshot } from "./startup";

let mainWindow: BrowserWindow | null = null;
let supervisor: BridgeSupervisor | null = null;
let tray: ReturnType<typeof createTray> | null = null;
let quitting = false;

app.setName("ITG Ray");
// Windows: attribute OS notifications to the installed app (the NSIS
// shortcut's AppUserModelID = electron-builder appId). Without this,
// toasts show the default "electron.app.Electron" identifier instead of
// "ITG Ray". No-op on other platforms.
app.setAppUserModelId("com.itgray.app");
app.setPath("userData", path.join(app.getPath("appData"), "ITG Ray"));

const gotLock = app.requestSingleInstanceLock();
if (!gotLock) {
  // Another Electron instance owns the lock — exit immediately so the
  // primary instance's "second-instance" handler can refocus its window.
  app.quit();
} else {
  app.on("second-instance", () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      // The window may be hidden (start-minimized launch builds it with
      // show:false), in which case it is not "minimized" — show() is what
      // actually reveals it. restore() above still handles the minimized case.
      mainWindow.show();
      mainWindow.focus();
    }
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

  mainWindow.on("closed", () => {
    mainWindow = null;
  });
}

app.whenReady().then(async () => {
  supervisor = new BridgeSupervisor();
  supervisor.start();
  await createWindow();
  tray = createTray(
    () => mainWindow,
    () => {
      // Tray "summon" is an explicit request to see the window. createWindow
      // now builds it hidden (show:false) so the launch path can honor
      // start-minimized; this path must show it after (re)creating.
      void createWindow().then(() => {
        mainWindow?.show();
        mainWindow?.focus();
      });
    },
  );
  wireIPC(supervisor, () => mainWindow, (s) => tray?.setStatus(s));

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
  const notifierRpc = supervisor!.rpc();
  notifierRpc.on("vpn.status", (p) => void notifier.onVpnStatus(p));
  notifierRpc.on("sub.synced", (p) => void notifier.onSubSynced(p));

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
