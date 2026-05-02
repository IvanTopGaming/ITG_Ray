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

async function refresh(): Promise<void> {
  try {
    const views = await ListSubs();
    setState({ ...state, load: { kind: "ready", subs: views.map(backendToFrontend) } });
  } catch (err) {
    setState({ ...state, load: { kind: "error", message: String(err) } });
  }
}

let pendingRefetch: number | null = null;
function scheduleRefetch(): void {
  if (pendingRefetch !== null) return;
  pendingRefetch = window.setTimeout(() => {
    pendingRefetch = null;
    void refresh();
  }, 50);
}

export function useSubs(): {
  state: SubsState;
  actions: {
    add: (name: string, url: string) => Promise<void>;
    edit: (id: string, name: string, url: string) => Promise<void>;
    remove: (id: string) => Promise<void>;
    syncOne: (id: string) => Promise<void>;
    syncAll: () => Promise<void>;
    refresh: () => Promise<void>;
  };
} {
  void ensureInit();
  const snap = useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
  return {
    state: snap,
    actions: {
      add: async () => { throw new Error("add not implemented"); },
      edit: async () => { throw new Error("edit not implemented"); },
      remove: async () => { throw new Error("remove not implemented"); },
      syncOne: async () => { throw new Error("syncOne not implemented"); },
      syncAll: async () => { throw new Error("syncAll not implemented"); },
      refresh,
    },
  };
}

// Suppress unused-binding warnings until Tasks 6-9 wire these in.
void AddSub; void EditSub; void RemoveSub; void SyncOneSub; void SyncAllSubs;

// Test escape hatch — mirrors settings.ts.__resetForTests.
export function __resetForTests(): void {
  state = {
    load: { kind: "loading" },
    inFlight: { syncing: new Set(), removing: new Set(), adding: false, editing: new Set() },
  };
  listeners.clear();
  initStarted = false;
  eventsRegistered = false;
  if (pendingRefetch !== null) {
    window.clearTimeout(pendingRefetch);
    pendingRefetch = null;
  }
}
