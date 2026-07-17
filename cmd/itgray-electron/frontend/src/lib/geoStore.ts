import { useSyncExternalStore } from 'react';
import { EventsOn } from '@/lib/itg/runtime';

export type GeoResult = 'ok' | 'error';
type GeoState = {
  done: number;
  total: number;
  active: boolean;
  refreshing: boolean;
  result: GeoResult | null;
};

const initial: GeoState = { done: 0, total: 0, active: false, refreshing: false, result: null };
let state: GeoState = initial;
const listeners = new Set<() => void>();
let clearTimer: ReturnType<typeof setTimeout> | null = null;

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
  setState({ ...state, done: payload.done, total: payload.total, active });
}

let registered = false;
function ensureRegistered() {
  if (registered) return;
  registered = true;
  EventsOn('geo:progress', onProgress);
}
ensureRegistered();

export function geoBegin() {
  if (clearTimer) {
    clearTimeout(clearTimer);
    clearTimer = null;
  }
  setState({ done: 0, total: 0, active: false, refreshing: true, result: null });
}

export function geoEnd(result: GeoResult) {
  setState({ ...state, active: false, refreshing: false, result });
  if (clearTimer) clearTimeout(clearTimer);
  clearTimer = setTimeout(() => {
    clearTimer = null;
    setState({ ...state, result: null });
  }, 2000);
}

export function getGeoSnapshot(): GeoState {
  return state;
}

export function __resetGeoForTest() {
  if (clearTimer) {
    clearTimeout(clearTimer);
    clearTimer = null;
  }
  state = { done: 0, total: 0, active: false, refreshing: false, result: null };
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
