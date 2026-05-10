// cmd/itgray-electron/frontend/wails-shim/bindings/SubsService.ts
const svc = () => (window.itg.subs ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function List(): Promise<unknown> { return svc().list?.() ?? Promise.resolve([]); }
export function Add(url: string, name: string): Promise<unknown> { return svc().add?.({ url, name }) ?? Promise.resolve(null); }
export function Edit(id: string, url: string, name: string): Promise<unknown> { return svc().edit?.({ id, url, name }) ?? Promise.resolve(null); }
export function Remove(id: string): Promise<unknown> { return svc().remove?.({ id }) ?? Promise.resolve(null); }
export function SyncOne(id: string): Promise<unknown> { return svc().syncOne?.({ id }) ?? Promise.resolve(null); }
export function SyncAll(): Promise<unknown> { return svc().syncAll?.() ?? Promise.resolve(null); }
