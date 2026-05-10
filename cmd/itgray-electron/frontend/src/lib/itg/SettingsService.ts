// cmd/itgray-electron/frontend/wails-shim/bindings/SettingsService.ts
const svc = () => (window.itg.settings ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function Get(): Promise<unknown> {
  return svc().get?.() ?? Promise.resolve(null);
}

export function Update(section: string, patch: Record<string, unknown>): Promise<unknown> {
  return svc().update?.(section, patch) ?? Promise.resolve(null);
}
