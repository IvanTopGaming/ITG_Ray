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
async function addAction(_name: string, _url: string): Promise<void> {
  throw new Error("add not implemented");
}
async function editAction(_id: string, _name: string, _url: string): Promise<void> {
  throw new Error("edit not implemented");
}
async function removeAction(_id: string): Promise<void> {
  throw new Error("remove not implemented");
}
async function syncOneAction(_id: string): Promise<void> {
  throw new Error("syncOne not implemented");
}
async function syncAllAction(): Promise<void> {
  throw new Error("syncAll not implemented");
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

// Module scope — re-exported for Tasks 6-9 to wire into the action bodies.
// TS would warn 'imported but unused' otherwise; the void expressions are
// no-ops kept until the next tasks reference each binding directly.
void AddSub;
void EditSub;
void RemoveSub;
void SyncOneSub;
void SyncAllSubs;

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
