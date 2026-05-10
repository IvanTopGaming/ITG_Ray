// cmd/itgray-electron/src/main/index.ts
import { app, BrowserWindow } from "electron";
import path from "node:path";
import { BridgeSupervisor } from "./bridge";
import { wireIPC } from "./ipc";
import { isDevMode } from "./paths";

let mainWindow: BrowserWindow | null = null;
let supervisor: BridgeSupervisor | null = null;

app.setName("ITG Ray");
app.setPath("userData", path.join(app.getPath("appData"), "ITG Ray"));

function createWindow(): void {
  mainWindow = new BrowserWindow({
    width: 1024,
    height: 720,
    title: "ITG Ray",
    webPreferences: {
      preload: path.join(__dirname, "..", "..", "dist-preload", "preload", "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  });

  if (isDevMode()) {
    mainWindow.loadURL("http://localhost:5173");
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
  wireIPC(supervisor, () => mainWindow);
  createWindow();
});

app.on("window-all-closed", async () => {
  if (supervisor) await supervisor.stop();
  app.quit();
});
