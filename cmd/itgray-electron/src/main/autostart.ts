// cmd/itgray-electron/src/main/autostart.ts
import AutoLaunch from "auto-launch";
import { app } from "electron";

export type Autostart = {
  get: () => Promise<boolean>;
  set: (enabled: boolean) => Promise<void>;
  reconcile: (desired: boolean) => Promise<void>;
};

/**
 * Factory taking an AutoLaunch constructor (or a fake for tests).
 * In production callers use `defaultAutostart()` which wires the real
 * AutoLaunch with the Electron app's name and exec path.
 */
export function makeAutostart(makeInstance: () => AutoLaunch): Autostart {
  let cached: AutoLaunch | null = null;
  const inst = (): AutoLaunch => (cached ??= makeInstance());

  return {
    async get() {
      return inst().isEnabled();
    },
    async set(enabled: boolean) {
      if (enabled) await inst().enable();
      else await inst().disable();
    },
    async reconcile(desired: boolean) {
      const current = await inst().isEnabled();
      if (current === desired) return;
      if (desired) await inst().enable();
      else await inst().disable();
    },
  };
}

/**
 * Resolves the executable path to register for autostart. Under an AppImage,
 * app.getPath("exe") points at the electron binary inside the ephemeral FUSE
 * mount (/tmp/.mount_*), which no longer exists after a reboot — so the
 * generated ~/.config/autostart entry would be dead. The AppImage runtime
 * exports APPIMAGE with the real, stable path to the .AppImage file; prefer it.
 */
export function resolveAutostartPath(
  appImagePath: string | undefined,
  exePath: string,
): string {
  return appImagePath && appImagePath.length > 0 ? appImagePath : exePath;
}

let prod: Autostart | null = null;

/** Production singleton — wires AutoLaunch with the running Electron app. */
export function defaultAutostart(): Autostart {
  return (prod ??= makeAutostart(
    () =>
      new AutoLaunch({
        name: app.getName(),
        path: resolveAutostartPath(process.env.APPIMAGE, app.getPath("exe")),
      }),
  ));
}
