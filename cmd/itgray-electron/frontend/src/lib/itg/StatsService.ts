const svc = () => (window.itg.stats ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function Overview(period: string): Promise<unknown> { return svc().overview?.({ period }) ?? Promise.resolve(null); }
export function Connections(): Promise<unknown> { return svc().connections?.() ?? Promise.resolve({ conns: [] }); }
export function Clear(): Promise<unknown> { return svc().clear?.() ?? Promise.resolve(null); }
