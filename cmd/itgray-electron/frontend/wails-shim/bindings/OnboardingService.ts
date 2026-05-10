// cmd/itgray-electron/frontend/wails-shim/bindings/OnboardingService.ts
const svc = () => (window.itg.onboarding ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function GetState(): Promise<unknown> { return svc().getState?.() ?? Promise.resolve({ seen: [] }); }
export function MarkSeen(key: string): Promise<unknown> { return svc().markSeen?.({ key }) ?? Promise.resolve(null); }
