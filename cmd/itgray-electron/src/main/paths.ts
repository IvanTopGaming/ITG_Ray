// cmd/itgray-electron/src/main/paths.ts
import path from "node:path";
import { app } from "electron";

const BUNDLE_LAYOUT: Record<string, string> = {
  "itgray-bridge": "bridge",
  "itgray-helper": "helper",
  "itgray-cli": "cli",
  "sing-box": "cores",
  "xray": "cores",
  "wintun.dll": ".",
};

const isDev = process.env.ELECTRON_DEV === "1";

/** Absolute path to a bundled binary. In dev, points to the repo's dist/. */
export function bundledBinary(name: string): string {
  if (isDev) {
    // From cmd/itgray-electron/dist-main/main/paths.js, two levels up
    // to cmd/itgray-electron/, then into dist-bridge/. Co-locates the
    // bridge binary with the Electron build outputs (dist-main, dist-preload).
    return path.join(__dirname, "..", "..", "dist-bridge", name);
  }
  const stem = name.replace(/\.(exe|dll)$/i, "");
  const subdir = BUNDLE_LAYOUT[stem] ?? BUNDLE_LAYOUT[name];
  if (!subdir) throw new Error(`unknown bundled binary: ${name}`);
  const root = process.resourcesPath;
  return path.join(root, subdir === "." ? "" : subdir, name);
}

/** True when running from `npm run dev`. */
export function isDevMode(): boolean {
  return isDev;
}

/** Per-user data dir mirroring Wails' os.UserConfigDir() + "ITG Ray". */
export function dataDir(): string {
  return path.join(app.getPath("userData"));
}
