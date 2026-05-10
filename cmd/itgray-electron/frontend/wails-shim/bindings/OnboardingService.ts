// cmd/itgray-electron/frontend/wails-shim/bindings/OnboardingService.ts
const svc = () => (window.itg.onboarding ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function GetState(): Promise<unknown> {
  return svc().getState?.() ?? Promise.resolve({ onboarded: false });
}

export function Complete(): Promise<unknown> {
  return svc().complete?.() ?? Promise.resolve(null);
}

export function Skip(): Promise<unknown> {
  return svc().skip?.() ?? Promise.resolve(null);
}
