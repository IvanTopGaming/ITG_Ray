// cmd/itgray-electron/src/main/window-state.ts
import { promises as fs } from "node:fs";
import type { BrowserWindow } from "electron";

export type WindowState = {
  x: number | null;
  y: number | null;
  width: number;
  height: number;
  maximised: boolean;
};

export const DEFAULT_STATE: WindowState = {
  x: null,
  y: null,
  width: 1024,
  height: 720,
  maximised: false,
};

const MIN_WIDTH = 400;
const MIN_HEIGHT = 300;

function isValid(s: unknown): s is WindowState {
  if (s === null || typeof s !== "object") return false;
  const o = s as Record<string, unknown>;
  const w = o.width;
  const h = o.height;
  if (typeof w !== "number" || !Number.isFinite(w) || w < MIN_WIDTH) return false;
  if (typeof h !== "number" || !Number.isFinite(h) || h < MIN_HEIGHT) return false;
  if (o.x !== null && (typeof o.x !== "number" || !Number.isFinite(o.x))) return false;
  if (o.y !== null && (typeof o.y !== "number" || !Number.isFinite(o.y))) return false;
  if (typeof o.maximised !== "boolean") return false;
  return true;
}

export async function loadState(filepath: string): Promise<WindowState> {
  let raw: string;
  try {
    raw = await fs.readFile(filepath, "utf8");
  } catch {
    return DEFAULT_STATE;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return DEFAULT_STATE;
  }
  return isValid(parsed) ? parsed : DEFAULT_STATE;
}

export async function saveState(filepath: string, state: WindowState): Promise<void> {
  await fs.writeFile(filepath, JSON.stringify(state), "utf8");
}

/**
 * Attaches resize/move/maximize/close handlers to win that debounce
 * (250 ms) into a single saveState write. Returns a cleanup function
 * that removes the handlers.
 */
export function attachStatePersister(win: BrowserWindow, filepath: string): () => void {
  let timer: NodeJS.Timeout | null = null;
  const flush = (): void => {
    timer = null;
    if (win.isDestroyed()) return;
    const bounds = win.getBounds();
    const state: WindowState = {
      x: bounds.x,
      y: bounds.y,
      width: bounds.width,
      height: bounds.height,
      maximised: win.isMaximized(),
    };
    void saveState(filepath, state);
  };
  const schedule = (): void => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(flush, 250);
  };
  win.on("move", schedule);
  win.on("resize", schedule);
  win.on("maximize", schedule);
  win.on("unmaximize", schedule);
  win.on("close", () => {
    if (timer) clearTimeout(timer);
    flush();
  });
  return () => {
    if (timer) clearTimeout(timer);
    win.off("move", schedule);
    win.off("resize", schedule);
    win.off("maximize", schedule);
    win.off("unmaximize", schedule);
  };
}
