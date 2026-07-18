let pending: string | null = null;
const listeners = new Set<() => void>();

export function setPendingImportLink(link: string): void {
  pending = link;
  for (const l of listeners) l();
}

export function consumePendingImportLink(): string | null {
  const link = pending;
  pending = null;
  return link;
}

export function subscribePendingImport(cb: () => void): () => void {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}
