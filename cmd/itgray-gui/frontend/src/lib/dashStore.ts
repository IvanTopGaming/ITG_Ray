import { useSyncExternalStore } from "react";
import { GetSnapshot } from "../../wailsjs/go/bindings/AppService";
import { Connect, Disconnect } from "../../wailsjs/go/bindings/RunService";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { hub } from "../../wailsjs/go/models";

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
  lastError: { kind: string; message: string; at: number } | null;
  bootstrapped: boolean;
};

const HISTORY_CAP = 60;
const RECONNECT_TIMEOUT_MS = 5000;

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
  lastError: null,
  bootstrapped: false,
});

let state: DashState = initialState();
const listeners = new Set<() => void>();

function notify() { for (const l of listeners) l(); }
function setState(next: DashState) { state = next; notify(); }
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

async function bootstrap() {
  try {
    const snap: Snapshot = await GetSnapshot();
    setState({
      ...state,
      status: (snap.status as ChainStatus) || "idle",
      mode: (snap.mode as Mode) || "tun",
      currentServer: snap.currentServer ?? null,
      helperState: (snap.helperState as DashState["helperState"]) || "missing",
      allServers: snap.servers ?? [],
      bootstrapped: true,
    });
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

function onVpnStatus(payload: any) {
  if (!payload || typeof payload.status !== "string") return;
  const nextStatus = payload.status as ChainStatus;
  let next = { ...state, status: nextStatus };

  if (nextStatus === "connected") {
    if (typeof payload.serverId === "string") {
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
      } as ServerView;
    }
    if (typeof payload.mode === "string") next.mode = payload.mode as Mode;
    if (state.status !== "connected") next.connectedAt = Date.now();
  }

  if (nextStatus === "idle") {
    next.history = [];
    next.speed = { downBps: 0, upBps: 0, at: 0 };
    next.totals = { down: 0, up: 0 };
    next.connectedAt = null;
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

function registerEventHandlers() {
  EventsOn("vpn:status", onVpnStatus);
  EventsOn("vpn:speed", onVpnSpeed);
  EventsOn("chain:error", onChainError);
  EventsOn("helper:state", onHelperState);
}

export function useDash(): DashState {
  return useSyncExternalStore(subscribe, () => state, () => state);
}

export function getDashState(): DashState { return state; }

export function effectiveStatus(s: DashState): ChainStatus | "error" {
  if (s.status === "idle" && s.lastError) return "error";
  return s.status;
}

export async function dashConnect(serverId: string): Promise<void> {
  try {
    if (state.status === "connected") {
      await Disconnect();
      await waitForIdle();
    }
    await Connect(serverId, state.mode);
  } catch (err: any) {
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

export async function dashSwitchMode(mode: Mode): Promise<void> {
  if (state.mode === mode) return;
  if (state.status !== "connected") {
    setState({ ...state, mode });
    return;
  }
  const targetId = state.currentServer?.id;
  if (!targetId) return;
  try {
    await Disconnect();
    await waitForIdle();
    setState({ ...state, mode });
    await Connect(targetId, mode);
  } catch (err: any) {
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

export function clearLastError(): void {
  setState({ ...state, lastError: null });
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
  registerEventHandlers();
}

export async function __bootstrapForTest(): Promise<void> {
  return bootstrap();
}

// Module init
registerEventHandlers();
bootstrap();
