import { useStore } from "@/store";
import { SubCard } from "@/components/sub-card/SubCard";

// SubsPage renders the subscription card grid. The toolbar (filter / Sync
// all / + Add subscription) is wired in C.T7; in C.T6 the buttons render
// disabled so the layout is locked in alongside the read-only List binding.
export function SubsPage() {
  const subs = useStore((s) => s.subs);
  return (
    <div className="flex flex-col gap-3 h-full min-h-0">
      <div className="flex gap-2">
        <input
          className="flex-1 h-8 bg-white/[0.04] border border-white/10 rounded-md px-3 text-sm"
          placeholder="Filter subs…"
          disabled
        />
        <button
          className="px-3 h-8 rounded-md bg-white/[0.06] border border-white/10 text-sm opacity-50 cursor-not-allowed"
          disabled
          title="Sync all lands in C.T7"
        >
          Sync all
        </button>
        <button
          className="px-3 h-8 rounded-md bg-gradient-to-br from-indigo-500 to-pink-500 text-sm opacity-50 cursor-not-allowed"
          disabled
          title="Add subscription lands in C.T7"
        >
          + Add subscription
        </button>
      </div>
      <div className="flex-1 min-h-0 overflow-auto">
        {subs.length === 0 ? (
          <div className="px-3 py-8 text-center text-text-muted text-sm">
            No subscriptions yet — add one in Settings or Onboarding.
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-3 auto-rows-min">
            {subs.map((s) => (
              <SubCard key={s.id} s={s} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
