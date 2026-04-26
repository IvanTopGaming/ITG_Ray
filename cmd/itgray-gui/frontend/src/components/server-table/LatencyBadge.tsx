// LatencyBadge renders a coloured pill for a probed RTT value.
// Tone thresholds match the spec: < 80 ms green, < 200 ms amber, else rose.
// A latency of 0 means "never probed" — render an em-dash placeholder.
export function LatencyBadge({ ms }: { ms: number }) {
  if (!ms) return <span className="text-text-muted">—</span>;
  const tone =
    ms < 80
      ? "bg-emerald-500/15 text-emerald-400 border-emerald-500/30"
      : ms < 200
      ? "bg-amber-500/15 text-amber-400 border-amber-500/30"
      : "bg-rose-500/15 text-rose-400 border-rose-500/30";
  return (
    <span className={`px-2 py-0.5 rounded-full border text-xs ${tone}`}>
      {ms}ms
    </span>
  );
}
