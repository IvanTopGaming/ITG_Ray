import { useSyncExternalStore } from "react";

export type NotificationTone = "info" | "warning" | "danger";

export type NotificationAction = {
  label: string;
  run: () => void | Promise<void>;
  primary?: boolean;
};

export type AppNotification = {
  id: string;
  tone: NotificationTone;
  title: string;
  body?: string;
  actions?: NotificationAction[];
  persistent?: boolean;
};

let items: AppNotification[] = [];
const dismissed = new Set<string>();
const listeners = new Set<() => void>();

function emit(): void {
  for (const l of listeners) l();
}

function sameContent(a: AppNotification, b: AppNotification): boolean {
  const aa = a.actions ?? [];
  const ba = b.actions ?? [];
  return (
    a.tone === b.tone &&
    a.title === b.title &&
    a.body === b.body &&
    !!a.persistent === !!b.persistent &&
    aa.length === ba.length &&
    aa.every((x, i) => x.label === ba[i].label && !!x.primary === !!ba[i].primary)
  );
}

export function notify(n: AppNotification): void {
  if (dismissed.has(n.id)) return;
  const idx = items.findIndex((x) => x.id === n.id);
  if (idx === -1) {
    items = [n, ...items];
    emit();
    return;
  }
  const existing = items[idx];
  if (sameContent(existing, n)) {
    existing.actions = n.actions;
    return;
  }
  items = items.map((x, i) => (i === idx ? n : x));
  emit();
}

export function dismiss(id: string): void {
  dismissed.add(id);
  if (!items.some((x) => x.id === id)) return;
  items = items.filter((x) => x.id !== id);
  emit();
}

export function retract(id: string): void {
  dismissed.delete(id);
  if (!items.some((x) => x.id === id)) return;
  items = items.filter((x) => x.id !== id);
  emit();
}

export function getNotifications(): AppNotification[] {
  return items;
}

function subscribe(cb: () => void): () => void {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

export function useNotifications(): AppNotification[] {
  return useSyncExternalStore(subscribe, getNotifications, getNotifications);
}

export function __resetNotificationsForTests(): void {
  items = [];
  dismissed.clear();
  emit();
}
