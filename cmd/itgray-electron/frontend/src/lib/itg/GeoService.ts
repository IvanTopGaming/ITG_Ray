const svc = () => ((window.itg as any).geo ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function Refresh(): Promise<unknown> {
  return svc().refresh?.() ?? Promise.resolve(null);
}
