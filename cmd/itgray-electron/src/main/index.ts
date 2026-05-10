// cmd/itgray-electron/src/main/index.ts
import { app, BrowserWindow } from "electron";
import path from "node:path";
import { BridgeSupervisor } from "./bridge";
import { wireIPC } from "./ipc";
import { isDevMode } from "./paths";
import { createTray } from "./tray";
import { loadState, attachStatePersister } from "./window-state";
import { defaultAutostart } from "./autostart";

let mainWindow: BrowserWindow | null = null;
let supervisor: BridgeSupervisor | null = null;
let tray: ReturnType<typeof createTray> | null = null;
let quitting = false;

app.setName("ITG Ray");
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
      void createWindow();
    },
  );
  wireIPC(supervisor, () => mainWindow, (s) => tray?.setStatus(s));

  // Reconcile autostart with persisted user setting. Failure is
  // non-fatal — the renderer can still toggle via Settings.
  void (async () => {
    try {
      const snap = await supervisor!.rpc().call("app.getSnapshot", undefined);
      const desired = (snap as { settings?: { general?: { autostart?: boolean } } }).settings
        ?.general?.autostart;
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
