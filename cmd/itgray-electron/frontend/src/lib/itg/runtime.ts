// cmd/itgray-electron/frontend/src/lib/itg/runtime.ts
//
// Replaces wails-shim/runtime.ts. EventsOn translates Wails-style colon
// topic names ("vpn:status") to the dotted protocol topics ("vpn.status")
// before subscribing through window.itg.on — the renderer's stores were
// authored against the Wails event surface and still use colon. The
// translation here keeps that contract working without touching every
// call site. Window* + Quit are unchanged.

declare global {
  interface Window {
    itg: {
      app: { ping: () => Promise<{ pong: number; version: string }>; quit?: () => Promise<void> } & Record<
        string,
        (...args: unknown[]) => Promise<unknown>
      >;
      on: (topic: string, cb: (payload: unknown) => void) => () => void;
    } & Record<string, Record<string, (...args: unknown[]) => Promise<unknown>>>;
  }
}

type Callback = (payload: unknown) => void;

const offByTopic = new Map<string, Map<Callback, () => void>>();

function toBridgeTopic(name: string): string {
  return name.replace(/:/g, ".");
}

export function EventsOn(topic: string, cb: Callback): () => void {
  const off = window.itg.on(toBridgeTopic(topic), cb);
  let perTopic = offByTopic.get(topic);
  if (!perTopic) {
    perTopic = new Map();
    offByTopic.set(topic, perTopic);
  }
  perTopic.set(cb, off);
  return () => {
    off();
    perTopic!.delete(cb);
    if (perTopic!.size === 0) offByTopic.delete(topic);
  };
}

export function EventsOnMultiple(topic: string, cb: Callback, _max = 0): () => void {
  return EventsOn(topic, cb);
}

export function EventsOff(topic: string, ...callbacks: Callback[]): void {
  const perTopic = offByTopic.get(topic);
  if (!perTopic) return;
  if (callbacks.length === 0) {
    for (const off of perTopic.values()) off();
    offByTopic.delete(topic);
    return;
  }
  for (const cb of callbacks) {
    const off = perTopic.get(cb);
    if (off) {
      off();
      perTopic.delete(cb);
    }
  }
  if (perTopic.size === 0) offByTopic.delete(topic);
}

export function EventsEmit(_topic: string, ..._data: unknown[]): void {}

function detectPlatform(): string {
  if (typeof navigator === "undefined") return "unknown";
  const p = String((navigator as any).platform ?? "").toLowerCase();
  if (p.startsWith("win")) return "windows";
  if (p.startsWith("mac")) return "darwin";
  if (p.startsWith("linux")) return "linux";
  return p || "unknown";
}

export function Environment(): Promise<{ buildType: string; platform: string; arch: string }> {
  return Promise.resolve({ buildType: "production", platform: detectPlatform(), arch: "unknown" });
}

export function Quit(): void {
  void window.itg.app.quit?.();
}

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
export function WindowClose(): void {
  void (window.itg as any).window?.close?.();
}

export function LogPrint(..._args: unknown[]): void {}
export function LogTrace(..._args: unknown[]): void {}
export function LogDebug(..._args: unknown[]): void {}
export function LogInfo(..._args: unknown[]): void {}
export function LogWarning(..._args: unknown[]): void {}
export function LogError(..._args: unknown[]): void {}
export function LogFatal(..._args: unknown[]): void {}
