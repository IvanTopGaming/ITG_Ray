// cmd/itgray-electron/frontend/wails-shim/bindings/ServersService.ts
const svc = () => (window.itg.servers ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function List(): Promise<unknown> { return svc().list?.() ?? Promise.resolve([]); }
export function Add(uri: string, name: string): Promise<unknown> { return svc().add?.({ uri, name }) ?? Promise.resolve(null); }
export function Edit(id: string, uri: string, name: string): Promise<unknown> { return svc().edit?.({ id, uri, name }) ?? Promise.resolve(null); }
export function Remove(id: string): Promise<unknown> { return svc().remove?.({ id }) ?? Promise.resolve(null); }
export function ToggleFavorite(id: string): Promise<unknown> { return svc().toggleFavorite?.({ id }) ?? Promise.resolve(null); }
export function TestLatency(id: string): Promise<unknown> { return svc().testLatency?.({ id }) ?? Promise.resolve(null); }
