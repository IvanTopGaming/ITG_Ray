export function Dashboard() {
  return (
    <section className="flex flex-col gap-4">
      <h1 className="text-[22px] font-semibold tracking-tight">Dashboard</h1>
      <div className="glass-regular rounded-2xl p-8">
        <div className="flex items-center gap-6">
          <div
            className="h-24 w-24 shrink-0 rounded-full border-2 border-dashed border-white/15 bg-white/[0.03]"
            aria-hidden
          />
          <div className="flex flex-col gap-1.5">
            <div className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/55">
              Disconnected
            </div>
            <div className="text-[24px] font-bold tracking-tight">
              No active connection
            </div>
            <div className="text-[13px] text-white/55">
              Hero with Connect / Disconnect lands here next.
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
