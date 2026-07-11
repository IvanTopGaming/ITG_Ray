import { useSyncExternalStore } from 'react';
import { EventsOn } from '@/lib/itg/runtime';

type GeoState = { done: number; total: number; active: boolean };

let state: GeoState = { done: 0, total: 0, active: false };
const listeners = new Set<() => void>();

function notify() {
  for (const l of listeners) l();
}
function setState(next: GeoState) {
  state = next;
  notify();
}

function onProgress(payload: any) {
  if (!payload || typeof payload.done !== 'number' || typeof payload.total !== 'number') return;
  const active = payload.total > 0 && payload.done < payload.total;
  setState({ done: payload.done, total: payload.total, active });
}

let registered = false;
function ensureRegistered() {
  if (registered) return;
  registered = true;
  EventsOn('geo:progress', onProgress);
}
ensureRegistered();

export function geoBegin() {
  setState({ done: 0, total: 0, active: true });
}
export function geoEnd() {
  setState({ done: state.done, total: state.total, active: false });
}

export function useGeoProgress(): GeoState {
  return useSyncExternalStore(
    (cb) => {
      listeners.add(cb);
      return () => listeners.delete(cb);
    },
    () => state,
    () => state,
  );
}
