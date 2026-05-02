import { useSyncExternalStore } from "react";
import {
  List as ListSubs,
  Add as AddSub,
  Edit as EditSub,
  Remove as RemoveSub,
  SyncOne as SyncOneSub,
  SyncAll as SyncAllSubs,
} from "../../wailsjs/go/bindings/SubsService";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { backendToFrontend, type Sub } from "./subsAdapter";

export type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; subs: Sub[] }
  | { kind: "error"; message: string };

export type InFlight = {
  syncing: Set<string>;
  removing: Set<string>;
  adding: boolean;
  editing: Set<string>;
};

export type SubsState = {
  load: LoadState;
  inFlight: InFlight;
};

let state: SubsState = {
  load: { kind: "loading" },
  inFlight: {
    syncing: new Set(),
    removing: new Set(),
    adding: false,
    editing: new Set(),
  },
};

const listeners = new Set<() => void>();

function notifyListeners(): void {
  for (const l of listeners) l();
}

function setState(next: SubsState): void {
  state = next;
  notifyListeners();
}

function getSnapshot(): SubsState {
  return state;
}

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

let eventsRegistered = false;
function ensureEventsRegistered(): void {
  if (eventsRegistered) return;
  eventsRegistered = true;
  EventsOn("sub:synced", () => {
    scheduleRefetch();
  });
}

let initStarted = false;
async function ensureInit(): Promise<void> {
  ensureEventsRegistered();
  if (initStarted) return;
  initStarted = true;
  await refresh();
}

let refreshInFlight = false;
let coalescedTrailing = false;

async function refresh(): Promise<void> {
  if (refreshInFlight) {
    // Direct caller (e.g. manual reload) hit during another refresh —
    // coalesce into a trailing refresh instead of running concurrently.
    coalescedTrailing = true;
    return;
  }
  refreshInFlight = true;
  try {
    const views = await ListSubs();
    setState({ ...state, load: { kind: "ready", subs: views.map(backendToFrontend) } });
  } catch (err) {
    setState({ ...state, load: { kind: "error", message: String(err) } });
  } finally {
    refreshInFlight = false;
    if (coalescedTrailing) {
      coalescedTrailing = false;
      scheduleRefetch();
    }
  }
}

let pendingRefetch: number | null = null;
function scheduleRefetch(): void {
  if (refreshInFlight) {
    coalescedTrailing = true;
    return;
  }
  if (pendingRefetch !== null) return;
  pendingRefetch = window.setTimeout(() => {
    pendingRefetch = null;
    void refresh();
  }, 50);
}

export type SubsActions = {
  add: (name: string, url: string) => Promise<void>;
  edit: (id: string, name: string, url: string) => Promise<void>;
  remove: (id: string) => Promise<void>;
  syncOne: (id: string) => Promise<void>;
  syncAll: () => Promise<void>;
  refresh: () => Promise<void>;
};

// Module-scope action implementations. Tasks 6-9 replace the throwing stubs.
async function addAction(name: string, url: string): Promise<void> {
  setState({ ...state, inFlight: { ...state.inFlight, adding: true } });
  try {
    // Note: public action takes (name, url); Go binding takes (url, name).
    const view = await AddSub(url, name);
    // Backend kicks off background SyncOne — reflect that optimistically
    // with status "syncing" before the sub:synced event lands.
    const newSub: Sub = { ...backendToFrontend(view), status: "syncing" };
    setState({
      ...state,
      load: state.load.kind === "ready"
        ? { kind: "ready", subs: [...state.load.subs, newSub] }
        : state.load,
      inFlight: { ...state.inFlight, adding: false },
    });
  } catch (err) {
    setState({ ...state, inFlight: { ...state.inFlight, adding: false } });
    throw err;
  }
}
async function editAction(id: string, name: string, url: string): Promise<void> {
  const editingNext = new Set(state.inFlight.editing);
  editingNext.add(id);
  setState({ ...state, inFlight: { ...state.inFlight, editing: editingNext } });
  try {
    // Note: public action takes (id, name, url); Go binding takes (id, url, name).
    const view = await EditSub(id, url, name);
    const updated = backendToFrontend(view);
    const editingDone = new Set(state.inFlight.editing);
    editingDone.delete(id);
    setState({
      ...state,
      load: state.load.kind === "ready"
        ? { kind: "ready", subs: state.load.subs.map(s => s.id === id ? updated : s) }
        : state.load,
      inFlight: { ...state.inFlight, editing: editingDone },
    });
  } catch (err) {
    const editingDone = new Set(state.inFlight.editing);
    editingDone.delete(id);
    setState({ ...state, inFlight: { ...state.inFlight, editing: editingDone } });
    throw err;
  }
}
async function removeAction(id: string): Promise<void> {
  if (state.load.kind !== "ready") return;
  const before = state.load.subs;
  const removingNext = new Set(state.inFlight.removing);
  removingNext.add(id);
  setState({
    ...state,
    load: { kind: "ready", subs: before.filter(s => s.id !== id) },
    inFlight: { ...state.inFlight, removing: removingNext },
  });
  try {
    await RemoveSub(id);
    const removingDone = new Set(state.inFlight.removing);
    removingDone.delete(id);
    setState({ ...state, inFlight: { ...state.inFlight, removing: removingDone } });
  } catch (err) {
    const removingDone = new Set(state.inFlight.removing);
    removingDone.delete(id);
    setState({
      ...state,
      load: { kind: "ready", subs: before },
      inFlight: { ...state.inFlight, removing: removingDone },
    });
    throw err;
  }
}
async function syncOneAction(id: string): Promise<void> {
  if (state.load.kind !== "ready") return;
  const syncingNext = new Set(state.inFlight.syncing);
  syncingNext.add(id);
  setState({
    ...state,
    load: {
      kind: "ready",
      subs: state.load.subs.map(s => s.id === id ? { ...s, status: "syncing" } : s),
    },
    inFlight: { ...state.inFlight, syncing: syncingNext },
  });
  try {
    await SyncOneSub(id);
  } finally {
    const syncingDone = new Set(state.inFlight.syncing);
    syncingDone.delete(id);
    setState({ ...state, inFlight: { ...state.inFlight, syncing: syncingDone } });
  }
}

async function syncAllAction(): Promise<void> {
  if (state.load.kind !== "ready") return;
  const ids = state.load.subs.map(s => s.id);
  const syncingNext = new Set([...state.inFlight.syncing, ...ids]);
  setState({
    ...state,
    load: {
      kind: "ready",
      subs: state.load.subs.map(s => ({ ...s, status: "syncing" })),
    },
    inFlight: { ...state.inFlight, syncing: syncingNext },
  });
  try {
    await SyncAllSubs();
  } finally {
    const syncingDone = new Set(state.inFlight.syncing);
    for (const id of ids) syncingDone.delete(id);
    setState({ ...state, inFlight: { ...state.inFlight, syncing: syncingDone } });
  }
}

const actions: SubsActions = {
  add: addAction,
  edit: editAction,
  remove: removeAction,
  syncOne: syncOneAction,
  syncAll: syncAllAction,
  refresh,
};

export function useSubs(): { state: SubsState; actions: SubsActions } {
  void ensureInit();
  const snap = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  return { state: snap, actions };
}

// Test escape hatch — mirrors settings.ts.__resetForTests.
export function __resetForTests(): void {
  state = {
    load: { kind: "loading" },
    inFlight: { syncing: new Set(), removing: new Set(), adding: false, editing: new Set() },
  };
  listeners.clear();
  initStarted = false;
  eventsRegistered = false;
  refreshInFlight = false;
  coalescedTrailing = false;
  if (pendingRefetch !== null) {
    window.clearTimeout(pendingRefetch);
    pendingRefetch = null;
  }
}

/**
 * Convert a backend error (`Error` or string from a rejected Wails RPC) into
 * a user-friendly UI message. Sentinels checked here mirror the Go bindings
 * in `cmd/itgray-gui/bindings/subs.go`: `errInvalidURL`, `errSubNotFound`,
 * and disk-save failures wrapped via `fmt.Errorf("sub.Save: %w", ...)` /
 * `fmt.Errorf("server.Save: %w", ...)`. Unknown errors fall back to the
 * raw message with any leading `Error: ` prefix stripped.
 */
export function humanizeError(err: unknown): string {
  const s = err instanceof Error ? err.message : String(err);
  if (/invalid url|must be http or https/i.test(s)) {
    return "URL must be a valid http(s) address";
  }
  if (/subscription not found/i.test(s)) return "Subscription no longer exists";
  if (/sub\.Save/.test(s))     return "Failed to save subscription file";
  if (/server\.Save/.test(s))  return "Failed to save server list";
  return s.replace(/^Error:\s*/, "");
}
