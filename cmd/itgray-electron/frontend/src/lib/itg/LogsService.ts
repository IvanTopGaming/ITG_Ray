const svc = () => ((window.itg as any).logs ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function Start(): Promise<unknown> {
  return svc().start?.() ?? Promise.resolve(null);
}

export function Stop(): Promise<unknown> {
  return svc().stop?.() ?? Promise.resolve(null);
}

export function OpenFolder(): Promise<unknown> {
  return svc().openFolder?.() ?? Promise.resolve(null);
}

export function DirInfo(): Promise<unknown> {
  return svc().dirInfo?.() ?? Promise.resolve(null);
}
