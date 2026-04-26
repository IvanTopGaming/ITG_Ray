// QuotaBar renders a slim 1px-tall progress bar with a gradient fill. The
// percent prop is clamped to [0, 100]; callers pass 0 when no quota data is
// available — the empty track stays visible as a placeholder for the C.T7
// data-usage telemetry.
export function QuotaBar({ percent }: { percent: number }) {
  const w = Math.max(0, Math.min(100, percent));
  return (
    <div className="h-1 rounded-full bg-white/10 overflow-hidden">
      <div
        className="h-full bg-gradient-to-br from-indigo-500 to-pink-500"
        style={{ width: `${w}%` }}
      />
    </div>
  );
}
