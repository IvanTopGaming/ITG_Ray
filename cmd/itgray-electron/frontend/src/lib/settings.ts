import { useSyncExternalStore, useCallback } from 'react';
import { Get as GetSettings, Update as UpdateSettings } from '@/lib/itg/SettingsService';
import { EventsOn, EventsOff } from '@/lib/itg/runtime';
import { backendToFrontend, frontendToBackend } from './settingsAdapter';

const HUB_EVENT_SETTINGS = 'settings:changed'; // mirrors hub.EventSettings in cmd/itgray-gui/hub/events.go

// ──────────────────────────────────────────────────────────────────────
//  Type aliases (preserve existing exports)
// ──────────────────────────────────────────────────────────────────────

export type Language = 'en' | 'ru';
export type NetworkMode = 'tun' | 'sysproxy';
export type DnsMode = 'auto' | 'custom';
export type LogLevel = 'error' | 'info' | 'debug' | 'trace';
export type Ipv6Mode = 'prefer-v4' | 'prefer-v6' | 'disabled';
export type GeoPreset = 'runetfreedom' | 'sagernet' | 'custom';

// NetworkSnapshot captures the network-section values the runtime ACTUALLY
// used at the last successful Connect (sourced from the vpn:status connected
// event payload, NOT the live in-memory store — avoids the edit-during-
// connect race where the user changes a field between chainctl reading
// config.Network and emitting the event).
export type NetworkSnapshot = {
  serverId: string;
  mode: NetworkMode;
  network: {
    tunCidr: string;
    tunMtu: number;
    socksPort: number;
    httpPort: number;
    allowLan: boolean;
    ipv6Mode: Ipv6Mode;
    dnsMode: DnsMode;
    dnsCustom: string;
  };
};

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
  geoPreset: GeoPreset;
  geoCustomURL: string;
  dnsMode: DnsMode;
  dnsCustom: string;
  tunCidr: string;
  tunMtu: number;
  // killswitch
  killSwitchEnabled: boolean;
  killSwitchAlwaysOn: boolean;
  // subscriptions
  defaultUpdateInterval: number;
  userAgent: string;
  hwidEnabled: boolean;
  sendDeviceOS: boolean;
  sendOSVersion: boolean;
  sendDeviceModel: boolean;
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
  geoPreset: 'runetfreedom',
  geoCustomURL: '',
  dnsMode: 'auto',
  dnsCustom: '',
  tunCidr: '198.18.0.1/15',
  tunMtu: 1500,
  killSwitchEnabled: true,
  killSwitchAlwaysOn: false,
  defaultUpdateInterval: 3600,
  userAgent: '',
  hwidEnabled: true,
  sendDeviceOS: true,
  sendOSVersion: true,
  sendDeviceModel: true,
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
// Counter (not bool) so overlapping flushes don't clear the in-flight
// gate prematurely. Incremented in flushSettings(); decremented in
// flushNow's finally so an early-return or thrown exception can't
// leave it stuck above zero.
let inFlightUpdates = 0;
let lastConnectSnapshot: NetworkSnapshot | null = null;
let activeServerEdited = false;
// networkDiffDismissed: user clicked Dismiss on a network-diff toast and
// has not edited any field since. Cleared on any settings update, on
// snapshot rebuild (re-Connect), and on snapshot clear (idle/error) so
// a fresh diff re-arms the toast. Active-edit signal is independent.
let networkDiffDismissed = false;
// rulesDirtyAfterConnect: a rules mutation happened while chain was
// connected. Set by markRulesDirty() (called from rulesStore on
// successful mutation), cleared on the next vpn:status=connected
// snapshot rebuild and on idle/error. Mirrors activeServerEdited
// shape so useReconnectNeeded ORs all three signals together.
let rulesDirtyAfterConnect = false;
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
  if (typeof window === 'undefined' || (!(window as any).go && !(window as any).itg)) return;
  let p;
  try {
    p = GetSettings();
  } catch (err) {
    console.warn('SettingsService.Get unavailable', err);
    backendFetchStarted = false; // allow retry on next subscribe
    return;
  }
  p.then((view) => {
    const diskPatch = backendToFrontend(view);
    // Disk truth wins over DEFAULTS, but in-flight optimistic edits win over disk.
    currentState = { ...currentState, ...diskPatch, ...(pendingPatch ?? {}) };
    notifyListeners();
  }).catch((err) => {
    console.warn('SettingsService.Get failed', err);
    // Clear the one-shot guard so a future remount/subscribe retries.
    // Without this, a transient RPC failure on first paint leaves the
    // UI permanently at DEFAULTS.
    backendFetchStarted = false;
  });
}

// ──────────────────────────────────────────────────────────────────────
//  Cross-process refresh: hub.EventSettings → re-fetch + merge
// ──────────────────────────────────────────────────────────────────────

function onSettingsEvent(): void {
  if (typeof window === 'undefined' || (!(window as any).go && !(window as any).itg)) return;
  // Short-circuit if a local optimistic patch is mid-debounce, or any
  // Update RPC is still in flight. flushNow itself issues a post-flush
  // GetSettings when the counter drops to zero, so dropping the echo
  // here is safe: cross-process drift is picked up on completion.
  if (pendingPatch !== null || inFlightUpdates > 0) return;
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
  if (typeof window === 'undefined' || (!(window as any).go && !(window as any).itg)) return;
  EventsOn(HUB_EVENT_SETTINGS, onSettingsEvent);
  EventsOn('vpn:status', (payload: unknown) => {
    if (!payload || typeof payload !== 'object') return;
    const p = payload as { status?: string };
    if (p.status === 'connected') {
      snapshotFromConnectedPayload(payload as Parameters<typeof snapshotFromConnectedPayload>[0]);
      clearRulesDirty();
    } else if (p.status === 'idle' || p.status === 'error') {
      clearConnectSnapshot();
      clearActiveServerEdited();
      clearRulesDirty();
    }
  });
  eventsRegistered = true;
}

// ──────────────────────────────────────────────────────────────────────
//  Reconnect snapshot (vpn:status connected payload)
// ──────────────────────────────────────────────────────────────────────

export function snapshotFromConnectedPayload(payload: {
  serverId?: string;
  mode?: string;
  network?: {
    tunCidr?: string;
    tunMtu?: number;
    socksPort?: number;
    httpPort?: number;
    allowLan?: boolean;
    ipv6Mode?: string;
    dns?: { mode?: string; servers?: string[] };
  };
}): void {
  if (!payload?.network) return;
  const n = payload.network;
  const mode: NetworkMode = payload.mode === 'sysproxy' ? 'sysproxy' : 'tun';
  const ipv6Mode: Ipv6Mode =
    n.ipv6Mode === 'prefer-v6' || n.ipv6Mode === 'disabled' ? n.ipv6Mode : 'prefer-v4';
  const dnsMode: DnsMode = n.dns?.mode === 'custom' ? 'custom' : 'auto';
  lastConnectSnapshot = {
    serverId: payload.serverId ?? '',
    mode,
    network: {
      tunCidr: n.tunCidr ?? '',
      tunMtu: n.tunMtu ?? 0,
      socksPort: n.socksPort ?? 0,
      httpPort: n.httpPort ?? 0,
      allowLan: n.allowLan ?? false,
      ipv6Mode,
      dnsMode,
      dnsCustom: (n.dns?.servers ?? []).join(', '),
    },
  };
  // Fresh snapshot — any prior dismiss is no longer relevant.
  networkDiffDismissed = false;
  notifyListeners();
}

export function clearConnectSnapshot(): void {
  if (lastConnectSnapshot === null && !networkDiffDismissed) return;
  lastConnectSnapshot = null;
  networkDiffDismissed = false;
  notifyListeners();
}

export function getConnectSnapshot(): NetworkSnapshot | null {
  return lastConnectSnapshot;
}

function networkDiffersFromSnapshot(): boolean {
  if (lastConnectSnapshot === null) return false;
  const s = currentState;
  const snap = lastConnectSnapshot.network;
  return (
    s.tunCidr !== snap.tunCidr ||
    s.tunMtu !== snap.tunMtu ||
    s.socksPort !== snap.socksPort ||
    s.httpPort !== snap.httpPort ||
    s.allowLan !== snap.allowLan ||
    s.ipv6Mode !== snap.ipv6Mode ||
    s.dnsMode !== snap.dnsMode ||
    s.dnsCustom !== snap.dnsCustom
  );
}

export function useNetworkChangedSinceConnect(): boolean {
  return useSyncExternalStore(subscribe, networkDiffersFromSnapshot, networkDiffersFromSnapshot);
}

export function markActiveServerEdited(): void {
  if (activeServerEdited) return;
  activeServerEdited = true;
  notifyListeners();
}

export function clearActiveServerEdited(): void {
  if (!activeServerEdited) return;
  activeServerEdited = false;
  notifyListeners();
}

// markRulesDirty: called by rulesStore on successful mutation when chain
// is currently connected. Arms the reconnect signal; the next successful
// Connect (vpn:status=connected) — or an idle/error transition — clears
// it via clearRulesDirty().
export function markRulesDirty(): void {
  if (rulesDirtyAfterConnect) return;
  rulesDirtyAfterConnect = true;
  notifyListeners();
}

export function clearRulesDirty(): void {
  if (!rulesDirtyAfterConnect) return;
  rulesDirtyAfterConnect = false;
  notifyListeners();
}

// dismissNetworkDiff hides the network-diff signal until the next
// settings edit (which clears the flag in useSettings.update) or a
// snapshot rebuild. Active-edit signal is unaffected — that path has
// its own clearActiveServerEdited().
export function dismissNetworkDiff(): void {
  if (networkDiffDismissed) return;
  networkDiffDismissed = true;
  notifyListeners();
}

function reconnectNeeded(): boolean {
  return (
    (networkDiffersFromSnapshot() && !networkDiffDismissed) ||
    activeServerEdited ||
    rulesDirtyAfterConnect
  );
}

export function useReconnectNeeded(): boolean {
  return useSyncExternalStore(subscribe, reconnectNeeded, reconnectNeeded);
}

// ──────────────────────────────────────────────────────────────────────
//  Write path: optimistic merge -> debounced section dispatch
// ──────────────────────────────────────────────────────────────────────

function scheduleFlush(): void {
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(flushSettings, DEBOUNCE_MS);
}

async function flushNow(patch: Partial<Settings>, snap: Settings): Promise<void> {
  let didDispatch = false;
  try {
    const sections = frontendToBackend(patch);
    if (sections.size === 0) return;
    didDispatch = true;

    const calls = Array.from(sections, ([section, p]) =>
      UpdateSettings(section, p as Record<string, any>),
    );
    const results = await Promise.allSettled(calls);

    for (const r of results) {
      if (r.status === 'rejected') {
        console.warn('SettingsService.Update failed', r.reason);
      }
    }
    void snap; // pre-mutation snapshot; reserved for future toast/rollback hook.
  } finally {
    inFlightUpdates = Math.max(0, inFlightUpdates - 1);
  }

  // Only the last in-flight flush re-syncs from disk. Backend
  // EventSettings echoes that fired while inFlightUpdates > 0 were
  // short-circuited; this Get picks up any cross-process changes (CLI
  // editing config.json, partial-failure rollback) and also implicitly
  // rolls back optimistic state on rejected writes.
  if (didDispatch && inFlightUpdates === 0) {
    try {
      const view = await GetSettings();
      const diskPatch = backendToFrontend(view);
      // Any new optimistic patch queued during our await wins over disk.
      currentState = { ...currentState, ...diskPatch, ...(pendingPatch ?? {}) };
      notifyListeners();
    } catch (err) {
      console.warn('post-flush settings refresh failed', err);
    }
  }
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
  inFlightUpdates += 1;
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
  inFlightUpdates = 0;
  lastConnectSnapshot = null;
  activeServerEdited = false;
  networkDiffDismissed = false;
  rulesDirtyAfterConnect = false;
  if (eventsRegistered) {
    EventsOff(HUB_EVENT_SETTINGS);
    EventsOff('vpn:status');
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
    // Any settings edit cancels the dismissed state so a fresh diff
    // (re-)arms the toast.
    if (networkDiffDismissed) networkDiffDismissed = false;
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
