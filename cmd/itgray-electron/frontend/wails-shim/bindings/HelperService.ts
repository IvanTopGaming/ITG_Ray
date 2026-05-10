// cmd/itgray-electron/frontend/wails-shim/bindings/HelperService.ts
const svc = () => (window.itg.helper ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function Status(): Promise<unknown> { return svc().status?.() ?? Promise.resolve({ state: "unknown" }); }
export function Install(): Promise<unknown> { return svc().install?.() ?? Promise.resolve(null); }
export function Start(): Promise<unknown> { return svc().start?.() ?? Promise.resolve(null); }
export function Stop(): Promise<unknown> { return svc().stop?.() ?? Promise.resolve(null); }
export function Restart(): Promise<unknown> { return svc().restart?.() ?? Promise.resolve(null); }
export function Reinstall(): Promise<unknown> { return svc().reinstall?.() ?? Promise.resolve(null); }
