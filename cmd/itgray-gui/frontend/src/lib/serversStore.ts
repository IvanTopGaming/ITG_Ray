import { useSyncExternalStore } from "react";

import {
  Add,
  Edit,
  List,
  Remove,
} from "../../wailsjs/go/bindings/ServersService";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import type { hub } from "../../wailsjs/go/models";

export type ServersState = {
  servers: hub.ServerView[];
  loading: boolean;
  lastError: string | null;
};

const initial = (): ServersState => ({ servers: [], loading: false, lastError: null });

let state: ServersState = initial();
const listeners = new Set<() => void>();
let bootstrapped = false;
let mutationInFlight = false;

function notify() {
  for (const l of listeners) l();
}

function setState(next: ServersState) {
  state = next;
  notify();
}

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

async function refetch(): Promise<void> {
  try {
    const list = await List();
    setState({ servers: list ?? [], loading: false, lastError: null });
  } catch (err: any) {
    setState({
      servers: state.servers,
      loading: false,
      lastError: err?.message ?? String(err),
    });
  }
}

export async function serversBootstrap(): Promise<void> {
  if (bootstrapped) return;
  bootstrapped = true;

  EventsOn("servers:changed", () => {
    void refetch();
  });
  EventsOn("sub:synced", () => {
    void refetch();
  });

  setState({ ...state, loading: true, lastError: null });
  await refetch();
}

export function useServers(): ServersState {
  // Lazy bootstrap on first hook use. serversBootstrap is idempotent.
  if (!bootstrapped) void serversBootstrap();
  return useSyncExternalStore(subscribe, () => state, () => state);
}

export function getServersState(): ServersState {
  return state;
}

export async function serverAdd(uri: string, name: string): Promise<void> {
  if (mutationInFlight) {
    throw new Error("another server mutation is in flight");
  }
  mutationInFlight = true;
  setState({ ...state, loading: true, lastError: null });
  try {
    await Add(uri, name);
    setState({ ...state, loading: false });
  } catch (err: any) {
    const msg = err?.message ?? String(err);
    setState({ ...state, loading: false, lastError: msg });
    throw err;
  } finally {
    mutationInFlight = false;
  }
}

export async function serverEdit(
  id: string,
  uri: string,
  name: string,
): Promise<{ vlessChanged: boolean }> {
  if (mutationInFlight) {
    throw new Error("another server mutation is in flight");
  }
  mutationInFlight = true;
  setState({ ...state, loading: true, lastError: null });
  try {
    const result: unknown = await Edit(id, uri, name);
    // Wails returns Go multi-return as a tuple [view, vlessChanged].
    const vlessChanged = Array.isArray(result) ? Boolean(result[1]) : false;
    setState({ ...state, loading: false });
    return { vlessChanged };
  } catch (err: any) {
    const msg = err?.message ?? String(err);
    setState({ ...state, loading: false, lastError: msg });
    throw err;
  } finally {
    mutationInFlight = false;
  }
}

export async function serverRemove(id: string): Promise<void> {
  if (mutationInFlight) {
    throw new Error("another server mutation is in flight");
  }
  mutationInFlight = true;
  setState({ ...state, loading: true, lastError: null });
  try {
    await Remove(id);
    setState({ ...state, loading: false });
  } catch (err: any) {
    const msg = err?.message ?? String(err);
    setState({ ...state, loading: false, lastError: msg });
    throw err;
  } finally {
    mutationInFlight = false;
  }
}

export function clearLastError(): void {
  setState({ ...state, lastError: null });
}

export function __resetForTest(): void {
  state = initial();
  listeners.clear();
  bootstrapped = false;
  mutationInFlight = false;
}
