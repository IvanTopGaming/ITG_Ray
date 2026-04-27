import { useSyncExternalStore } from 'react';

export type Language = 'en' | 'ru';
export type NetworkMode = 'tun' | 'system-proxy' | 'off';
export type DnsMode = 'auto' | 'custom';
export type LogLevel = 'error' | 'info' | 'debug' | 'trace';

export type Settings = {
  language: Language;
  launchOnStartup: boolean;
  startMinimized: boolean;
  networkMode: NetworkMode;
  dnsMode: DnsMode;
  dnsCustom: string;
  allowLan: boolean;
  notifyConnection: boolean;
  notifySound: boolean;
  notifySubFailure: boolean;
  logLevel: LogLevel;
};

export const DEFAULTS = {
  language: 'en',
  launchOnStartup: false,
  startMinimized: false,
  networkMode: 'tun',
  dnsMode: 'auto',
  dnsCustom: '',
  allowLan: true,
  notifyConnection: true,
  notifySound: false,
  notifySubFailure: true,
  logLevel: 'info',
} as const satisfies Settings;

export const STORAGE_KEY = 'itgray.settings.v1';

// Returned objects are always fresh shallow copies so callers can mutate freely
// without corrupting the singleton DEFAULTS reference (TS marks it readonly,
// but the runtime object is shared otherwise).
export function loadSettings(): Settings {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return { ...DEFAULTS };
  try {
    const parsed = JSON.parse(raw) as Partial<Settings>;
    return { ...DEFAULTS, ...parsed };
  } catch (err) {
    console.warn('[settings] corrupt JSON in localStorage, using defaults', err);
    return { ...DEFAULTS };
  }
}

let saveTimer: ReturnType<typeof setTimeout> | null = null;
let pendingValue: Settings | null = null;

export function saveSettings(s: Settings): void {
  pendingValue = s;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    if (pendingValue) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(pendingValue));
    }
    saveTimer = null;
    pendingValue = null;
  }, 200);
}

// Force any pending debounced write to flush immediately. Wire this to
// `beforeunload` once a consumer (e.g. useSettings in T3) is available so the
// last edit before tab-close isn't lost in the 200ms window.
export function flushSettings(): void {
  if (!saveTimer) return;
  clearTimeout(saveTimer);
  saveTimer = null;
  if (pendingValue) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(pendingValue));
    pendingValue = null;
  }
}

// ---------------------------------------------------------------------------
// External store for useSyncExternalStore — shared singleton state so all
// hook consumers re-render together on any patch or cross-tab storage event.
// ---------------------------------------------------------------------------

let currentState: Settings | null = null;
const listeners = new Set<() => void>();

function getSnapshot(): Settings {
  if (currentState === null) currentState = loadSettings();
  return currentState;
}

function handleStorage(event: StorageEvent): void {
  if (event.key === STORAGE_KEY) {
    currentState = loadSettings();
    notifyListeners();
  }
}

function handleBeforeUnload(): void {
  flushSettings();
}

function notifyListeners(): void {
  for (const cb of listeners) cb();
}

function subscribe(cb: () => void): () => void {
  const isFirst = listeners.size === 0;
  listeners.add(cb);
  if (isFirst && typeof window !== 'undefined') {
    window.addEventListener('storage', handleStorage);
    window.addEventListener('beforeunload', handleBeforeUnload);
  }
  return (): void => {
    listeners.delete(cb);
    if (listeners.size === 0) {
      // Reset cached state so the next subscriber gets a fresh read from
      // localStorage. This also means tests get a clean slate after unmount.
      currentState = null;
      if (typeof window !== 'undefined') {
        window.removeEventListener('storage', handleStorage);
        window.removeEventListener('beforeunload', handleBeforeUnload);
      }
    }
  };
}

export function useSettings(): [Settings, (patch: Partial<Settings>) => void] {
  const state = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  const update = (patch: Partial<Settings>): void => {
    currentState = { ...getSnapshot(), ...patch };
    saveSettings(currentState);
    notifyListeners();
  };
  return [state, update];
}
