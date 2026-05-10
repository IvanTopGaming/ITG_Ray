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

// Keyed on Wails-style event name (with `:` separator) so that EventsOff
// can match by (topic, callback) pair. Same callback can be registered
// for multiple topics independently.
const offByTopic = new Map<string, Map<Callback, () => void>>();

export function EventsOn(eventName: string, cb: Callback): () => void {
  const off = window.itg.on(toBridgeTopic(eventName), cb);
  let perTopic = offByTopic.get(eventName);
  if (!perTopic) {
    perTopic = new Map();
    offByTopic.set(eventName, perTopic);
  }
  perTopic.set(cb, off);
  return () => {
    off();
    perTopic!.delete(cb);
    if (perTopic!.size === 0) offByTopic.delete(eventName);
  };
}

export function EventsOnMultiple(eventName: string, cb: Callback, _maxCallbacks = 0): () => void {
  return EventsOn(eventName, cb);
}

export function EventsOff(eventName: string, ...callbacks: Callback[]): void {
  const perTopic = offByTopic.get(eventName);
  if (!perTopic) return;
  if (callbacks.length === 0) {
    // Wails behavior: no callbacks → unsubscribe ALL listeners on this topic.
    for (const off of perTopic.values()) off();
    offByTopic.delete(eventName);
    return;
  }
  for (const cb of callbacks) {
    const off = perTopic.get(cb);
    if (off) {
      off();
      perTopic.delete(cb);
    }
  }
  if (perTopic.size === 0) offByTopic.delete(eventName);
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

// Wails runtime additionally exposes a synchronous Environment(buildType, platform, arch)
// query and a Quit() shortcut. The Electron build does not currently need real values
// here — Phase 5 (native shell) will replace these via window.itg if anything depends
// on them. helperAdapter.ts caches the result, so a stable async resolver is fine.
export function Environment(): Promise<{ buildType: string; platform: string; arch: string }> {
  return Promise.resolve({
    buildType: "production",
    platform: typeof navigator !== "undefined" && (navigator as any).platform
      ? String((navigator as any).platform).toLowerCase()
      : "unknown",
    arch: "unknown",
  });
}

export function Quit(): void {
  // No-op in Phase 2. Phase 5 wires this to Electron's app.quit() via window.itg.
}
