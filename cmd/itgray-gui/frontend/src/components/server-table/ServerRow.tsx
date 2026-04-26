import type { ServerView } from "@/api/client";
import { LatencyBadge } from "./LatencyBadge";
import { ServerActions } from "./ServerActions";

// ServerRow renders one row of the server table. The 5-column grid mirrors
// the header in ServerTable. Country flag rendering is deferred until the
// geo-IP enrichment lands (server.country is "" today per C.T3 TODO).
export function ServerRow({ s }: { s: ServerView }) {
  return (
    <div className="grid grid-cols-[1.2fr_1fr_0.7fr_0.6fr_0.6fr] gap-3 items-center px-3 py-2 hover:bg-white/[0.03] border-b border-white/[0.04] text-sm">
      <div className="flex items-center gap-2">
        {s.favorite && <span className="text-amber-400">★</span>}
        <span className="font-medium">{s.name}</span>
      </div>
      <div className="text-text-secondary text-xs">
        {s.transport} · {s.security}
      </div>
      <LatencyBadge ms={s.latencyMs} />
      <div className="text-text-muted text-xs">{s.origin}</div>
      <ServerActions id={s.id} favorite={s.favorite} />
    </div>
  );
}
