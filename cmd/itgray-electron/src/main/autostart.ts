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

let prod: Autostart | null = null;

/** Production singleton — wires AutoLaunch with the running Electron app. */
export function defaultAutostart(): Autostart {
  return (prod ??= makeAutostart(
    () =>
      new AutoLaunch({
        name: app.getName(),
        path: app.getPath("exe"),
      }),
  ));
}
