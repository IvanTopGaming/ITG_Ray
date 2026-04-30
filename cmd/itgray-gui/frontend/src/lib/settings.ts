import { useSyncExternalStore, useCallback } from 'react';
import { Get as GetSettings, Update as UpdateSettings } from '../../wailsjs/go/bindings/SettingsService';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { backendToFrontend, frontendToBackend } from './settingsAdapter';

// ──────────────────────────────────────────────────────────────────────
//  Type aliases (preserve existing exports)
// ──────────────────────────────────────────────────────────────────────

export type Language = 'en' | 'ru';
export type NetworkMode = 'tun' | 'sysproxy';
export type DnsMode = 'auto' | 'custom';
export type LogLevel = 'error' | 'info' | 'debug' | 'trace';
export type Ipv6Mode = 'prefer-v4' | 'prefer-v6' | 'disabled';

export type Settings = {
  // general
  language: Language;
  autostart: boolean;
  startMinimized: boolean;
  // network
  defaultMode: NetworkMode;
  socksPort: number;
  httpPort: number;
  allowLan: boolean;
  ipv6Mode: Ipv6Mode;
  dnsMode: DnsMode;
  dnsCustom: string;
  tunCidr: string;
  tunMtu: number;
  // killswitch
  killSwitchEnabled: boolean;
  killSwitchAlwaysOn: boolean;
  // notifications
  onConnected: boolean;
  onDisconnected: boolean;
  onQuotaLow: boolean;
  onSubSynced: boolean;
  notifySound: boolean;
  // debug
  logLevel: LogLevel;
};

export const DEFAULTS = {
  language: 'en',
  autostart: false,
  startMinimized: false,
  defaultMode: 'tun',
  socksPort: 1080,
  httpPort: 8888,
  allowLan: false,
  ipv6Mode: 'prefer-v4',
  dnsMode: 'auto',
  dnsCustom: '',
  tunCidr: '198.18.0.1/15',
  tunMtu: 1500,
  killSwitchEnabled: true,
  killSwitchAlwaysOn: false,
  onConnected: true,
  onDisconnected: true,
  onQuotaLow: true,
  onSubSynced: true,
  notifySound: true,
  logLevel: 'info',
} as const satisfies Settings;

// ──────────────────────────────────────────────────────────────────────
//  Module state
// ──────────────────────────────────────────────────────────────────────

const DEBOUNCE_MS = 200;

let currentState: Settings = { ...DEFAULTS };
let pendingPatch: Partial<Settings> | null = null;
let prevSnapshot: Settings | null = null;
let saveTimer: ReturnType<typeof setTimeout> | null = null;
let backendFetchStarted = false;
let eventsRegistered = false;
const listeners = new Set<() => void>();

function notifyListeners(): void {
  listeners.forEach((cb) => cb());
}

// ──────────────────────────────────────────────────────────────────────
//  Backend bootstrap
// ──────────────────────────────────────────────────────────────────────

function loadFromBackend(): void {
  if (backendFetchStarted) return;
  backendFetchStarted = true;
  if (typeof window === 'undefined' || !(window as any).go) return;
  let p;
  try {
    p = GetSettings();
  } catch (err) {
    console.warn('SettingsService.Get unavailable', err);
    return;
  }
  p.then((view) => {
    const diskPatch = backendToFrontend(view);
    // Disk truth wins over DEFAULTS, but in-flight optimistic edits win over disk.
    currentState = { ...currentState, ...diskPatch, ...(pendingPatch ?? {}) };
    notifyListeners();
  }).catch((err) => {
    console.warn('SettingsService.Get failed', err);
  });
}

// ──────────────────────────────────────────────────────────────────────
//  Cross-process refresh: hub.EventSettings → re-fetch + merge
// ──────────────────────────────────────────────────────────────────────

function onSettingsEvent(): void {
  if (typeof window === 'undefined' || !(window as any).go) return;
  // Short-circuit if a local optimistic patch is mid-debounce. The next
  // flushSettings() will reconcile state with disk, and the resulting
  // backend EventSettings echo will trigger a fresh fetch then.
  if (pendingPatch !== null) return;
  GetSettings()
    .then((view) => {
      const patch = backendToFrontend(view);
      currentState = { ...currentState, ...patch };
      notifyListeners();
    })
    .catch((err) => {
      console.warn('settings refresh after event failed', err);
    });
}

function ensureEventsRegistered(): void {
  if (eventsRegistered) return;
  if (typeof window === 'undefined' || !(window as any).go) return;
  EventsOn('settings', onSettingsEvent);
  eventsRegistered = true;
}

// ──────────────────────────────────────────────────────────────────────
//  Write path: optimistic merge -> debounced section dispatch
// ──────────────────────────────────────────────────────────────────────

function scheduleFlush(): void {
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(flushSettings, DEBOUNCE_MS);
}

async function flushNow(patch: Partial<Settings>, snap: Settings): Promise<void> {
  const sections = frontendToBackend(patch);
  if (sections.size === 0) return;

  const calls = Array.from(sections, ([section, p]) =>
    UpdateSettings(section, p as Record<string, any>),
  );
  const results = await Promise.allSettled(calls);

  for (const r of results) {
    if (r.status === 'rejected') {
      console.warn('SettingsService.Update failed', r.reason);
      // TODO(tier2a-followup): rollback prev section state + emit toast.
      // Hook: `snap` carries the pre-mutation snapshot for future use.
    }
  }
  // Reference snap so TS does not flag it unused; remove once rollback wired.
  void snap;
}

export function flushSettings(): void {
  if (saveTimer) {
    clearTimeout(saveTimer);
    saveTimer = null;
  }
  if (!pendingPatch) return;
  const patch = pendingPatch;
  const snap = prevSnapshot ?? currentState;
  pendingPatch = null;
  prevSnapshot = null;
  void flushNow(patch, snap);
}

// ──────────────────────────────────────────────────────────────────────
//  Test seam
// ──────────────────────────────────────────────────────────────────────

export function __resetForTests(): void {
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = null;
  currentState = { ...DEFAULTS };
  pendingPatch = null;
  prevSnapshot = null;
  backendFetchStarted = false;
  if (eventsRegistered) {
    EventsOff('settings');
    eventsRegistered = false;
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener('beforeunload', flushSettings);
  }
  listeners.clear();
}

// ──────────────────────────────────────────────────────────────────────
//  React subscription (unchanged signature)
// ──────────────────────────────────────────────────────────────────────

function getSnapshot(): Settings {
  return currentState;
}

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  if (!backendFetchStarted) loadFromBackend();
  ensureEventsRegistered();
  return () => listeners.delete(cb);
}

export function useSettings(): [Settings, (patch: Partial<Settings>) => void] {
  const state = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  const update = useCallback((patch: Partial<Settings>): void => {
    if (prevSnapshot === null) prevSnapshot = currentState;
    currentState = { ...currentState, ...patch };
    pendingPatch = { ...(pendingPatch ?? {}), ...patch };
    notifyListeners();
    scheduleFlush();
  }, []);
  return [state, update];
}

// ──────────────────────────────────────────────────────────────────────
//  Browser lifecycle
// ──────────────────────────────────────────────────────────────────────

if (typeof window !== 'undefined') {
  window.removeEventListener('beforeunload', flushSettings);
  window.addEventListener('beforeunload', flushSettings);
}
