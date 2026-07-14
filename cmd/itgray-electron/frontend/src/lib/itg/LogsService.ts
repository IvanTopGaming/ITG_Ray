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

export function ExportLogs(): Promise<{ text: string }> {
  return (svc().export?.() ?? Promise.resolve({ text: "" })) as Promise<{
    text: string;
  }>;
}

export function SaveLogs(text: string): Promise<string | null> {
  return (svc().save?.(text) ?? Promise.resolve(null)) as Promise<
    string | null
  >;
}
