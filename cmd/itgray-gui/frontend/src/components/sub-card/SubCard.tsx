import type { SubView } from "@/api/client";
import { QuotaBar } from "./QuotaBar";

// SubCard renders one subscription as a compact tile. C.T6 ships display
// only — Sync now / Edit / Export / Delete buttons are styled but disabled;
// C.T7 wires Sync now and Delete; Edit/Export remain placeholders.
export function SubCard({ s }: { s: SubView }) {
  const sinceSync = humanSince(s.lastSyncAt);
  const tone =
    s.lastSyncStatus === "OK"
      ? "text-emerald-400 border-emerald-500/30 bg-emerald-500/10"
      : s.lastSyncStatus === "ERROR"
      ? "text-rose-400 border-rose-500/30 bg-rose-500/10"
      : "text-text-muted border-white/10 bg-white/5";
  return (
    <div className="bg-white/[0.035] border border-white/[0.08] rounded-xl p-3 flex flex-col gap-2">
      <h4 className="font-medium text-sm flex items-center gap-2">{s.name || s.id}</h4>
      <div className="text-[10px] text-text-muted font-mono break-all">{maskUrl(s.url)}</div>
      <div className="flex flex-wrap gap-1 items-center">
        <span className={`px-2 py-0.5 rounded-full text-[10px] border ${tone}`}>
          last sync: {sinceSync} · {s.lastSyncStatus || "—"}
        </span>
        <span className="px-2 py-0.5 rounded-full text-[10px] bg-white/[0.06] border border-white/10 text-text-muted">
          {s.serverCount} servers
        </span>
      </div>
      {s.lastSyncMessage && <div className="text-xs text-rose-400/80">{s.lastSyncMessage}</div>}
      <QuotaBar percent={0} />
      <div className="flex gap-1 mt-auto pt-1 text-xs">
        <button className="px-2 h-7 rounded bg-white/[0.06] border border-white/10 opacity-50 cursor-not-allowed" disabled>Sync now</button>
        <button className="px-2 h-7 rounded bg-white/[0.06] border border-white/10 opacity-50 cursor-not-allowed" disabled>Edit</button>
        <button className="px-2 h-7 rounded bg-white/[0.06] border border-white/10 opacity-50 cursor-not-allowed" disabled>Export</button>
        <button className="px-2 h-7 rounded text-rose-400 border border-rose-500/30 ml-auto opacity-50 cursor-not-allowed" disabled>Delete</button>
      </div>
    </div>
  );
}

// maskUrl preserves the origin and path shape but elides long hex/UUID-ish
// path segments so secret tokens embedded in the URL don't leak into the UI.
function maskUrl(u: string): string {
  try {
    const p = new URL(u);
    return `${p.origin}${p.pathname.replace(/[a-f0-9-]{16,}/gi, "…")}`;
  } catch {
    return u;
  }
}

// humanSince renders a coarse "Xs/Xm/Xh ago" relative timestamp. Go's
// zero-time JSON encoding ("0001-01-01T00:00:00Z") and the empty string
// both map to "never" — matches the SubView spec where LastSyncAt.IsZero()
// means "not yet synced".
function humanSince(iso: string): string {
  if (iso === "0001-01-01T00:00:00Z" || !iso) return "never";
  const ms = Date.now() - new Date(iso).getTime();
  if (ms < 60_000) return `${Math.round(ms / 1000)}s ago`;
  if (ms < 3_600_000) return `${Math.round(ms / 60_000)}m ago`;
  return `${Math.round(ms / 3_600_000)}h ago`;
}
