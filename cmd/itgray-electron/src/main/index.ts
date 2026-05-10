// cmd/itgray-electron/src/main/index.ts
import { app, BrowserWindow } from "electron";
import path from "node:path";
import { BridgeSupervisor } from "./bridge";
import { wireIPC } from "./ipc";
import { isDevMode } from "./paths";
import { createTray } from "./tray";

let mainWindow: BrowserWindow | null = null;
let supervisor: BridgeSupervisor | null = null;
let tray: ReturnType<typeof createTray> | null = null;

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

function createWindow(): void {
  mainWindow = new BrowserWindow({
    width: 1024,
    height: 720,
    title: "ITG Ray",
    frame: false,
    webPreferences: {
      preload: path.join(__dirname, "..", "..", "dist-preload", "preload", "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  });

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

app.whenReady().then(() => {
  supervisor = new BridgeSupervisor();
  supervisor.start();
  createWindow();
  tray = createTray(() => mainWindow);
  wireIPC(supervisor, () => mainWindow, (s) => tray?.setStatus(s));
});

app.on("window-all-closed", () => {
  // Tray-only mode — do NOT quit when last window closes. Window can
  // be brought back via the tray. app.quit() is the only path that
  // triggers actual shutdown (via the before-quit hook below).
});

app.on("before-quit", async () => {
  if (supervisor) await supervisor.stop();
});
