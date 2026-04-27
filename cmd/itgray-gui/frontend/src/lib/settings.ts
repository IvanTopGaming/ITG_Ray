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
