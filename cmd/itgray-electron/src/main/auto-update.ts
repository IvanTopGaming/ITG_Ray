import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import path from "node:path";
import type { UpdateCheckResult } from "./updates";

export interface UpdatePrefs {
  autoCheck: boolean;
  skipped: string;
}

export interface AvailableUpdate {
  latest: string;
  htmlUrl?: string;
}

const DEFAULT_PREFS: UpdatePrefs = { autoCheck: true, skipped: "" };

export function loadUpdatePrefs(file: string): UpdatePrefs {
  try {
    const data = JSON.parse(readFileSync(file, "utf8")) as Partial<Record<keyof UpdatePrefs, unknown>>;
    return {
      autoCheck: typeof data.autoCheck === "boolean" ? data.autoCheck : DEFAULT_PREFS.autoCheck,
      skipped: typeof data.skipped === "string" ? data.skipped : DEFAULT_PREFS.skipped,
    };
  } catch {
    return { ...DEFAULT_PREFS };
  }
}

export function saveUpdatePrefs(file: string, prefs: UpdatePrefs): void {
  try {
    mkdirSync(path.dirname(file), { recursive: true });
    writeFileSync(file, JSON.stringify(prefs), "utf8");
  } catch (err) {
    console.warn("updates: could not persist prefs", err);
  }
}

export interface AutoUpdateDeps {
  getVersion: () => Promise<string>;
  check: (currentVersion: string) => Promise<UpdateCheckResult>;
  loadPrefs: () => UpdatePrefs;
  savePrefs: (prefs: UpdatePrefs) => void;
  publish: (update: AvailableUpdate) => void;
}

export interface AutoUpdate {
  run: () => Promise<void>;
  pending: () => AvailableUpdate | null;
  skip: (version: string) => void;
  autoCheck: () => boolean;
  setAutoCheck: (enabled: boolean) => void;
}

export function createAutoUpdate(deps: AutoUpdateDeps): AutoUpdate {
  let pending: AvailableUpdate | null = null;
  return {
    run: async () => {
      const prefs = deps.loadPrefs();
      if (!prefs.autoCheck) return;
      let result: UpdateCheckResult;
      try {
        result = await deps.check(await deps.getVersion());
      } catch {
        return;
      }
      if (result.status !== "available" || !result.latest) return;
      if (result.latest === prefs.skipped) return;
      pending = { latest: result.latest, htmlUrl: result.htmlUrl };
      deps.publish(pending);
    },
    pending: () => pending,
    skip: (version) => {
      deps.savePrefs({ ...deps.loadPrefs(), skipped: version });
      if (pending?.latest === version) pending = null;
    },
    autoCheck: () => deps.loadPrefs().autoCheck,
    setAutoCheck: (enabled) => {
      deps.savePrefs({ ...deps.loadPrefs(), autoCheck: enabled });
    },
  };
}
