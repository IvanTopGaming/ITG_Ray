// cmd/itgray-electron/frontend/wails-shim/bindings/AppService.ts
//
// Method names use the same casing the Wails-generated bindings used:
// PascalCase exported function per Go method name. They proxy to
// window.itg.app.<lowerFirst(name)> through the preload.

const itgApp = () => window.itg.app as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function GetSnapshot(): Promise<unknown> {
  return itgApp().getSnapshot?.() ?? Promise.resolve(null);
}

export function GetPublicIP(): Promise<unknown> {
  return itgApp().getPublicIP?.() ?? Promise.resolve(null);
}
