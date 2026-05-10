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
function detectPlatform(): string {
  // Map navigator.platform to the canonical Wails-style strings the
  // renderer expects ("windows" / "linux" / "darwin"). Raw navigator.
  // platform returns "Win32" / "Linux x86_64" / "MacIntel" — using the
  // raw value broke helperAdapter's `platform === 'windows'` check
  // (Settings → Helper showed "uses native APIs on this platform" on
  // actual Windows builds because "win32" !== "windows").
  if (typeof navigator === "undefined") return "unknown";
  const p = String((navigator as any).platform ?? "").toLowerCase();
  if (p.startsWith("win")) return "windows";
  if (p.startsWith("mac")) return "darwin";
  if (p.startsWith("linux")) return "linux";
  return p || "unknown";
}

export function Environment(): Promise<{ buildType: string; platform: string; arch: string }> {
  return Promise.resolve({
    buildType: "production",
    platform: detectPlatform(),
    arch: "unknown",
  });
}

export function Quit(): void {
  void window.itg.app.quit?.();
}

// Window controls — route through Electron via the preload bridge. The
// custom frameless title bar (TitleBar.tsx) calls these, so Phase 5 wires
// them to BrowserWindow.minimize / maximize / isMaximized.
export function WindowMinimise(): void {
  void (window.itg as any).window?.minimise?.();
}
export function WindowToggleMaximise(): void {
  void (window.itg as any).window?.toggleMaximise?.();
}
export function WindowIsMaximised(): Promise<boolean> {
  const ns = (window.itg as any).window;
  return ns?.isMaximised ? ns.isMaximised() : Promise.resolve(false);
}

// WindowClose closes the visible window (NOT the app). With Phase 5's
// tray-only mode (window-all-closed is a no-op), this hides the UI to
// the tray; the user re-summons via tray click. To actually quit the
// app, use the tray's "Quit" menu item (which calls app.quit).
export function WindowClose(): void {
  void (window.itg as any).window?.close?.();
}
