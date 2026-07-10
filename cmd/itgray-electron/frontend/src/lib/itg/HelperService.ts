// cmd/itgray-electron/frontend/wails-shim/bindings/HelperService.ts
const svc = () => (window.itg.helper ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

// Wails' Helper.Status() returned a bare string ("running"/"stopped"/
// "missing"); the bridge wraps it into HelperStatusResult{state}. Unwrap
// the .state field here so renderer code (helperAdapter.mapHelperStatus
// and friends) keeps treating Status as string-returning.
export function Status(): Promise<string> {
  const fn = svc().status;
  if (!fn) return Promise.resolve("unknown");
  return fn().then((r) => {
    if (r && typeof r === "object" && "state" in r) {
      const s = (r as { state?: unknown }).state;
      return typeof s === "string" ? s : "unknown";
    }
    return typeof r === "string" ? r : "unknown";
  });
}
export function Install(): Promise<unknown> { return svc().install?.() ?? Promise.resolve(null); }
export function Start(): Promise<unknown> { return svc().start?.() ?? Promise.resolve(null); }
export function Stop(): Promise<unknown> { return svc().stop?.() ?? Promise.resolve(null); }
export function Restart(): Promise<unknown> { return svc().restart?.() ?? Promise.resolve(null); }
export function Reinstall(): Promise<unknown> { return svc().reinstall?.() ?? Promise.resolve(null); }
export function InstallLinux(): Promise<unknown> { return svc().installLinux?.() ?? Promise.resolve(null); }
export function UninstallLinux(): Promise<unknown> { return svc().uninstallLinux?.() ?? Promise.resolve(null); }
