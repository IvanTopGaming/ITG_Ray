import { useSyncExternalStore } from "react";
import { GetSnapshot } from "@/lib/itg/AppService";
import { Connect, Disconnect } from "@/lib/itg/RunService";
import { Update as UpdateSettings } from "@/lib/itg/SettingsService";
import { seedConnectSnapshotFromSnapshot } from "@/lib/settings";
import { TestLatency } from "@/lib/itg/ServersService";
import { EventsOn } from "@/lib/itg/runtime";
import type { hub } from "@/lib/itg/models";

type ServerView = hub.ServerView;
type Snapshot = hub.Snapshot;

export type ChainStatus = "idle" | "connecting" | "connected" | "disconnecting";
export type Mode = "tun" | "sysproxy";

export type SpeedPoint = { t: number; downBps: number; upBps: number };

export type DashState = {
  status: ChainStatus;
  mode: Mode;
  currentServer: ServerView | null;
  helperState: "running" | "stopped" | "missing";
  allServers: ServerView[];
  speed: { downBps: number; upBps: number; at: number };
  history: SpeedPoint[];
  totals: { down: number; up: number };
  connectedAt: number | null;
  endpoint: { socksPort: number; httpPort: number } | null;
  lastError: { kind: string; message: string; at: number } | null;
  bootstrapped: boolean;
  probeState: Map<string, "probing" | "ok" | "error">;
};

const HISTORY_CAP = 60;
const RECONNECT_TIMEOUT_MS = 5000;
const LAST_ERROR_KEY = "itg.dashStore.lastError";
// The banner should persist across a GUI relaunch (user closes mid-failure
// and reopens later to inspect) but a chain.error from days ago is just
// noise on a fresh boot. 1h captures the relaunch-immediately use case
// without leaking stale state into the next session.
const LAST_ERROR_TTL_MS = 60 * 60 * 1000;

function loadPersistedError(): DashState["lastError"] {
  try {
    const raw = localStorage.getItem(LAST_ERROR_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as DashState["lastError"];
    if (!parsed || typeof parsed.at !== "number") return null;
    if (Date.now() - parsed.at > LAST_ERROR_TTL_MS) {
      localStorage.removeItem(LAST_ERROR_KEY);
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function persistError(err: DashState["lastError"]): void {
  try {
    if (err) localStorage.setItem(LAST_ERROR_KEY, JSON.stringify(err));
    else localStorage.removeItem(LAST_ERROR_KEY);
  } catch {
    // Storage quota / unavailable — non-fatal, keep in-memory state.
  }
}

// Last successfully-connected server id, persisted so "connect on start" has a
// target after a fresh launch (the backend snapshot only carries currentServer
// while a chain is live, so it is null on an idle boot).
const LAST_SERVER_KEY = "itg.dashStore.lastServerId";

function loadLastServerId(): string | null {
  try {
    return localStorage.getItem(LAST_SERVER_KEY);
  } catch {
    return null;
  }
}

function saveLastServerId(id: string): void {
  if (!id) return;
  try {
    localStorage.setItem(LAST_SERVER_KEY, id);
  } catch {
    // Storage unavailable — auto-connect just won't have a target next launch.
  }
}

// One-shot guard: doBootstrap also runs on mutation events, but auto-connect
// must fire at most once per app session.
let autoConnectDone = false;

const initialState = (): DashState => ({
  status: "idle",
  mode: "tun",
  currentServer: null,
  helperState: "missing",
  allServers: [],
  speed: { downBps: 0, upBps: 0, at: 0 },
  history: [],
  totals: { down: 0, up: 0 },
  connectedAt: null,
  endpoint: null,
  lastError: loadPersistedError(),
  bootstrapped: false,
  probeState: new Map(),
});

let state: DashState = initialState();
const listeners = new Set<() => void>();

function notify() { for (const l of listeners) l(); }
function setState(next: DashState) {
  if (next.lastError !== state.lastError) persistError(next.lastError);
  state = next;
  notify();
}
function subscribe(cb: () => void) { listeners.add(cb); return () => { listeners.delete(cb); }; }

let pendingIdleAck: { resolve: () => void; reject: (e: Error) => void; timer: ReturnType<typeof setTimeout> } | null = null;

function waitForIdle(): Promise<void> {
  if (state.status === "idle") return Promise.resolve();
  return new Promise((resolve, reject) => {
    if (pendingIdleAck) {
      clearTimeout(pendingIdleAck.timer);
      pendingIdleAck.reject(new Error("superseded"));
    }
    const timer = setTimeout(() => {
      pendingIdleAck = null;
      reject(new Error("reconnect_timeout"));
    }, RECONNECT_TIMEOUT_MS);
    pendingIdleAck = { resolve, reject, timer };
  });
}

function resolveIdleAck() {
  if (!pendingIdleAck) return;
  clearTimeout(pendingIdleAck.timer);
  pendingIdleAck.resolve();
  pendingIdleAck = null;
}

let bootstrapInFlight: Promise<void> | null = null;

async function bootstrap(): Promise<void> {
  if (bootstrapInFlight) return bootstrapInFlight;
  bootstrapInFlight = doBootstrap();
  try {
    await bootstrapInFlight;
  } finally {
    bootstrapInFlight = null;
  }
}

async function doBootstrap(): Promise<void> {
  try {
    const snap: Snapshot = await GetSnapshot();
    const servers = snap.servers ?? [];
    const nextStatus = (snap.status as ChainStatus) || "idle";
    // Adopt path: if the bridge reconciled a still-live chain at boot, the
    // vpn:status "connected" event that seeds the reconnect snapshot fired
    // before this renderer subscribed. Seed it from the pull so the Reconnect
    // toast works after an app reopen.
    seedConnectSnapshotFromSnapshot(snap);
    // Preserve optimistic currentServer (set by dashConnect) when the chain
    // is not actually connected — bootstrap is also triggered by mutation
    // events (servers:changed from probes / favourites), and overwriting
    // a user-selected card with backend's null on every probe-all batch
    // visibly clears the "selected" highlight.
    const preserveOptimistic =
      nextStatus !== "connected" && state.currentServer && !snap.currentServer;
    setState({
      ...state,
      status: nextStatus,
      mode: (snap.mode as Mode) || "tun",
      currentServer: preserveOptimistic ? state.currentServer : (snap.currentServer ?? null),
      helperState: (snap.helperState as DashState["helperState"]) || "missing",
      allServers: servers,
      bootstrapped: true,
    });
    maybeAutoProbe(servers);
    maybeAutoConnect(snap);
  } catch (err: any) {
    setState({
      ...state,
      lastError: {
        kind: "bootstrap_failed",
        message: err?.message ?? String(err),
        at: Date.now(),
      },
    });
  }
}

export function extractEndpoint(
  payload: any,
): { socksPort: number; httpPort: number } | null {
  const n = payload?.network;
  if (!n || typeof n.socksPort !== "number" || typeof n.httpPort !== "number") {
    return null;
  }
  return { socksPort: n.socksPort, httpPort: n.httpPort };
}

function onVpnStatus(payload: any) {
  if (!payload || typeof payload.status !== "string") return;
  const nextStatus = payload.status as ChainStatus;
  let next = { ...state, status: nextStatus };

  if (nextStatus === "connected") {
    if (typeof payload.serverId === "string") {
      saveLastServerId(payload.serverId);
      const cached = state.allServers.find(s => s.id === payload.serverId);
      next.currentServer = cached ?? {
        id: payload.serverId,
        name: "",
        country: "",
        address: "",
        transport: "",
        security: "",
        latencyMs: 0,
        origin: "manual",
        favorite: false,
        tags: [],
        uri: "",
      } as ServerView;
    }
    if (typeof payload.mode === "string") next.mode = payload.mode as Mode;
    next.endpoint = extractEndpoint(payload);
    if (state.status !== "connected") {
      // Backend supplies connectedAt for Reconcile-adopted sessions
      // (the chain was already running before we booted; without this
      // the duration counter would restart at zero on every GUI relaunch).
      // bringUp also includes Date.now()-equivalent; falling back to
      // Date.now() keeps older tests / event sources working.
      next.connectedAt =
        typeof payload.connectedAt === "number" ? payload.connectedAt : Date.now();
    }
  }

  if (nextStatus === "idle") {
    next.history = [];
    next.speed = { downBps: 0, upBps: 0, at: 0 };
    next.totals = { down: 0, up: 0 };
    next.connectedAt = null;
    next.endpoint = null;
    resolveIdleAck();
  }

  setState(next);

  if (!state.bootstrapped) bootstrap();
}

function onVpnSpeed(payload: any) {
  if (state.status !== "connected") return;
  if (!payload || typeof payload.downBps !== "number" || typeof payload.upBps !== "number") return;
  const sample = { downBps: payload.downBps, upBps: payload.upBps, at: Date.now() };
  const point: SpeedPoint = { t: sample.at, downBps: sample.downBps, upBps: sample.upBps };
  const history = state.history.length >= HISTORY_CAP
    ? [...state.history.slice(state.history.length - HISTORY_CAP + 1), point]
    : [...state.history, point];
  setState({
    ...state,
    speed: sample,
    history,
    totals: {
      down: state.totals.down + sample.downBps,
      up: state.totals.up + sample.upBps,
    },
  });
}

function onChainError(payload: any) {
  if (!payload) return;
  setState({
    ...state,
    lastError: {
      kind: payload.kind ?? "chain_error",
      message: payload.message ?? "unknown error",
      at: Date.now(),
    },
  });
}

function onHelperState(payload: any) {
  if (!payload || typeof payload.state !== "string") return;
  setState({ ...state, helperState: payload.state as DashState["helperState"] });
}

// onSubSynced re-fetches the snapshot so QuickSwitch sees newly-added servers
// without requiring an app restart. Wired to both sub:synced (subscription
// fan-out) and servers:changed (manual mutations from Tier 6 ServersService.
// Add/Edit/Remove). The bootstrap in-flight guard coalesces rapid bursts
// (e.g., SyncAll fans out N sub:synced events).
function onSubSynced() {
  void bootstrap();
}

// onProbeResult patches both latencies into allServers (so QuickSwitch's
// sort key updates without waiting for the next snapshot reload) AND
// probeState (so the per-row "probing…" pill in Servers settles to ok
// or error). A single backend probe:result event drives both updates.
function onProbeResult(payload: any) {
  if (!payload || !Array.isArray(payload.results)) return;
  const latencyById = new Map<string, number>();
  const probeUpdates = new Map<string, "ok" | "error">();
  for (const r of payload.results) {
    if (!r || typeof r.id !== "string") continue;
    if (r.error) {
      probeUpdates.set(r.id, "error");
    } else if (typeof r.latencyMs === "number") {
      latencyById.set(r.id, r.latencyMs);
      probeUpdates.set(r.id, "ok");
    }
  }
  if (probeUpdates.size === 0) return;
  const nextServers =
    latencyById.size === 0
      ? state.allServers
      : state.allServers.map((s) => {
          const ms = latencyById.get(s.id);
          return ms === undefined ? s : { ...s, latencyMs: ms };
        });
  const nextProbe = new Map(state.probeState);
  for (const [id, st] of probeUpdates) nextProbe.set(id, st);
  setState({ ...state, allServers: nextServers, probeState: nextProbe });
}

function registerEventHandlers() {
  EventsOn("vpn:status", onVpnStatus);
  EventsOn("vpn:speed", onVpnSpeed);
  EventsOn("chain:error", onChainError);
  EventsOn("helper:state", onHelperState);
  EventsOn("sub:synced", onSubSynced);
  EventsOn("servers:changed", onSubSynced);
  EventsOn("probe:result", onProbeResult);
}

// maybeAutoProbe fires a TestLatency batch when the snapshot contains servers
// that have never been probed (latencyMs === 0). Fire-and-forget; results
// arrive via the probe:result handler above.
function maybeAutoProbe(servers: hub.ServerView[]) {
  if (servers.length === 0) return;
  if (servers.every((s) => s.latencyMs > 0)) return;
  void TestLatency("").catch(() => {
    /* probe failures are non-fatal; UI shows em-dash for unprobed servers */
  });
}

// maybeAutoConnect fires a one-shot connect to the last-used server at launch
// when the "connect on start" setting is enabled. Runs at most once per app
// session and only from a clean idle boot with the helper up and a known last
// server still present in the list. Failures surface via the lastError banner.
function maybeAutoConnect(snap: Snapshot): void {
  if (autoConnectDone) return;
  autoConnectDone = true;
  if (snap.settings?.general?.autoConnect !== true) return;
  if (((snap.status as ChainStatus) || "idle") !== "idle") return;
  if ((snap.helperState as DashState["helperState"]) !== "running") return;
  const lastId = loadLastServerId();
  const servers = snap.servers ?? [];
  if (!lastId || !servers.some((s) => s.id === lastId)) return;
  void dashConnect(lastId).catch(() => {
    /* surfaced via lastError */
  });
}

export function useDash(): DashState {
  return useSyncExternalStore(subscribe, () => state, () => state);
}

export function getDashState(): DashState { return state; }

export function effectiveStatus(s: DashState): ChainStatus | "error" {
  if (s.status === "idle" && s.lastError) return "error";
  return s.status;
}

let connectInFlight: Promise<void> | null = null;

export async function dashConnect(serverId: string): Promise<void> {
  if (connectInFlight) {
    // Coalesce: silently absorb concurrent re-entry. Caller can still observe
    // status via useDash(); the in-flight call will set state when it completes.
    return connectInFlight;
  }
  connectInFlight = doConnect(serverId);
  try {
    await connectInFlight;
  } finally {
    connectInFlight = null;
  }
}

async function doConnect(serverId: string): Promise<void> {
  // Optimistic UI: immediately reflect the user's chosen server so the
  // active-row indicator flips before the backend completes Disconnect+
  // Connect. The vpn:status connected event will set the same value;
  // on failure the chain falls back to idle/error but currentServer
  // continues to reflect the user's intent (clearer than reverting).
  const target = state.allServers.find((s) => s.id === serverId);
  if (target && state.currentServer?.id !== serverId) {
    setState({ ...state, currentServer: target });
  }
  try {
    if (state.status === "connected") {
      await Disconnect();
      await waitForIdle();
    }
    await Connect(serverId, state.mode);
  } catch (err: any) {
    if (err?.message === "superseded") return;
    setState({
      ...state,
      lastError: {
        kind: "connect_failed",
        message: err?.message ?? String(err),
        at: Date.now(),
      },
    });
    throw err;
  }
}

export async function dashDisconnect(): Promise<void> {
  try {
    await Disconnect();
  } catch (err: any) {
    setState({
      ...state,
      lastError: {
        kind: "disconnect_failed",
        message: err?.message ?? String(err),
        at: Date.now(),
      },
    });
    throw err;
  }
}

// persistMode writes the chosen mode to config.Network.Mode so the choice
// survives restarts (GetSnapshot seeds the toggle from it). Fire-and-forget.
function persistMode(mode: Mode): void {
  void UpdateSettings("network", { defaultMode: mode }).catch(() => {});
}

export async function dashSwitchMode(mode: Mode): Promise<void> {
  if (state.mode === mode) return;
  persistMode(mode);
  if (state.status !== "connected") {
    setState({ ...state, mode });
    return;
  }
  if (connectInFlight) return connectInFlight;
  const targetId = state.currentServer?.id;
  if (!targetId) return;
  connectInFlight = doSwitchMode(targetId, mode);
  try {
    await connectInFlight;
  } finally {
    connectInFlight = null;
  }
}

async function doSwitchMode(targetId: string, mode: Mode): Promise<void> {
  try {
    await Disconnect();
    await waitForIdle();
    setState({ ...state, mode });
    await Connect(targetId, mode);
  } catch (err: any) {
    if (err?.message === "superseded") return;
    setState({
      ...state,
      lastError: {
        kind: "switch_mode_failed",
        message: err?.message ?? String(err),
        at: Date.now(),
      },
    });
    throw err;
  }
}

// dashReconnect is the AppShell "Reconnect to apply settings" path:
// disconnect-then-connect with an explicit serverId+mode pair sourced
// from the last-connected snapshot. Routes through the existing
// connectInFlight mutex and surfaces failures via lastError so a Connect
// rejection after a successful Disconnect doesn't leave the user
// stranded with only a console.warn.
export async function dashReconnect(serverId: string, mode: Mode): Promise<void> {
  if (connectInFlight) return connectInFlight;
  connectInFlight = doReconnect(serverId, mode);
  try {
    await connectInFlight;
  } finally {
    connectInFlight = null;
  }
}

async function doReconnect(serverId: string, mode: Mode): Promise<void> {
  try {
    if (state.status === "connected") {
      await Disconnect();
      await waitForIdle();
    }
    if (state.mode !== mode) setState({ ...state, mode });
    await Connect(serverId, mode);
  } catch (err: any) {
    if (err?.message === "superseded") return;
    setState({
      ...state,
      lastError: {
        kind: "reconnect_failed",
        message: err?.message ?? String(err),
        at: Date.now(),
      },
    });
    throw err;
  }
}

export function clearLastError(): void {
  setState({ ...state, lastError: null });
}

export function dashSetProbing(ids: string[]): void {
  if (ids.length === 0) return;
  const next = new Map(state.probeState);
  for (const id of ids) next.set(id, "probing");
  setState({ ...state, probeState: next });
}

export async function dashProbeOne(id: string): Promise<void> {
  dashSetProbing([id]);
  try {
    await TestLatency(id);
  } catch {
    const next = new Map(state.probeState);
    next.set(id, "error");
    setState({ ...state, probeState: next });
  }
}

export async function dashProbeAll(): Promise<void> {
  const ids = state.allServers.map((s) => s.id);
  dashSetProbing(ids);
  try {
    await TestLatency("");
  } catch {
    const next = new Map(state.probeState);
    for (const id of ids) next.set(id, "error");
    setState({ ...state, probeState: next });
  }
}

// Test-only: __resetForTest only resets state. It does NOT auto-call bootstrap, so
// tests can set up GetSnapshot mocks before triggering bootstrap via __bootstrapForTest.
export function __resetForTest(): void {
  state = initialState();
  listeners.clear();
  if (pendingIdleAck) {
    clearTimeout(pendingIdleAck.timer);
    pendingIdleAck = null;
  }
  bootstrapInFlight = null;
  connectInFlight = null;
  autoConnectDone = false;
  registerEventHandlers();
}

export async function __bootstrapForTest(): Promise<void> {
  return bootstrap();
}

// Module init
registerEventHandlers();
bootstrap();
