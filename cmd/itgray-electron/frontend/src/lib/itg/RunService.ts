// cmd/itgray-electron/frontend/wails-shim/bindings/RunService.ts
const svc = () => (window.itg.run ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function Connect(serverId: string, mode: string): Promise<unknown> { return svc().connect?.({ serverId, mode }) ?? Promise.resolve(null); }
export function Disconnect(): Promise<unknown> { return svc().disconnect?.() ?? Promise.resolve(null); }
export function Reconnect(serverId: string, mode: string): Promise<unknown> { return svc().reconnect?.({ serverId, mode }) ?? Promise.resolve(null); }
export function SwitchMode(mode: string): Promise<unknown> { return svc().switchMode?.({ mode }) ?? Promise.resolve(null); }
