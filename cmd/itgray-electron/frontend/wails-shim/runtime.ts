// cmd/itgray-electron/frontend/wails-shim/runtime.ts
//
// Drop-in replacement for the Wails-generated wailsjs/runtime/runtime.{ts,js}.
// All exports route through the Electron preload bridge (window.itg).
// Methods unrelated to events are no-ops in Phase 2; later phases will
// add real implementations as needed.

declare global {
  interface Window {
    itg: {
      app: { ping: () => Promise<{ pong: number; version: string }> };
      on: (topic: string, cb: (payload: unknown) => void) => () => void;
    } & Record<string, Record<string, (...args: unknown[]) => Promise<unknown>>>;
  }
}

type Callback = (payload: unknown) => void;

const offByPair = new WeakMap<Callback, () => void>();

export function EventsOn(eventName: string, cb: Callback): () => void {
  const off = window.itg.on(toBridgeTopic(eventName), cb);
  offByPair.set(cb, off);
  return off;
}

export function EventsOnMultiple(eventName: string, cb: Callback, _maxCallbacks = 0): () => void {
  return EventsOn(eventName, cb);
}

export function EventsOff(eventName: string, ...callbacks: Callback[]): void {
  if (callbacks.length === 0) return;
  for (const cb of callbacks) {
    const off = offByPair.get(cb);
    if (off) {
      off();
      offByPair.delete(cb);
    }
  }
  void eventName;
}

export function EventsEmit(_eventName: string, ..._data: unknown[]): void {
  // Renderer-side EventsEmit was rare in the Wails frontend (mostly used
  // for diagnostic broadcasts). Until we audit usage, this is a no-op.
}

export function LogPrint(..._args: unknown[]): void {}
export function LogTrace(..._args: unknown[]): void {}
export function LogDebug(..._args: unknown[]): void {}
export function LogInfo(..._args: unknown[]): void {}
export function LogWarning(..._args: unknown[]): void {}
export function LogError(..._args: unknown[]): void {}
export function LogFatal(..._args: unknown[]): void {}

// Wails event names use ":" as separator; bridge uses ".".
function toBridgeTopic(name: string): string {
  return name.replace(/:/g, ".");
}
