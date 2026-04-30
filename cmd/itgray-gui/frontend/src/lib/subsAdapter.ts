import type { hub } from "../../wailsjs/go/models";

export type Status = "ok" | "error" | "never" | "syncing";

export interface Sub {
  id: string;
  name: string;
  url: string;
  status: Status;
  lastSyncAt: number | null;
  serverCount: number;
  lastSyncMessage?: string;
  upload?: number;
  download?: number;
  total?: number;
  expire?: number;
}

const ZERO_TIME = "0001-01-01T00:00:00Z";

function parseTime(t: unknown): number | null {
  if (t == null) return null;
  if (t === ZERO_TIME) return null;
  const n = Date.parse(String(t));
  return Number.isNaN(n) ? null : n;
}

function deriveStatus(view: hub.SubView): Status {
  const lastSyncAt = parseTime(view.lastSyncAt);
  // Tolerate legacy uppercase / prefixed values written by older builds:
  // pre-Tier-2a CLI wrote "OK "+summary (e.g. "OK imported=3 invalid=0
  // skipped=0"); accept both the bare enum and the prefixed legacy form.
  const raw = (view.lastSyncStatus ?? "").trim().toLowerCase();
  if (!raw && lastSyncAt === null) return "never";
  if (raw === "ok" || raw.startsWith("ok ")) return "ok";
  return "error"; // fail-safe: any non-ok value (including unknown) → error
}

function nonZero(n: number | undefined): number | undefined {
  return n && n > 0 ? n : undefined;
}

export function backendToFrontend(view: hub.SubView): Sub {
  const expireMs = parseTime(view.expire);
  return {
    id: view.id,
    name: view.name,
    url: view.url,
    status: deriveStatus(view),
    lastSyncAt: parseTime(view.lastSyncAt),
    serverCount: view.serverCount,
    lastSyncMessage: view.lastSyncMessage ? view.lastSyncMessage : undefined,
    upload: nonZero(view.upload),
    download: nonZero(view.download),
    total: nonZero(view.total),
    expire: expireMs ?? undefined,
  };
}
