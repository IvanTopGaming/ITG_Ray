import { Hero } from "@/components/hero/Hero";

// DashboardPage is the landing route: the Hero card on top with the
// Connect affordance, server picker, and mode toggle; below it a
// placeholder surface that becomes the live up/down chart in C.T11.
export function DashboardPage() {
  return (
    <div className="flex flex-col gap-4 h-full">
      <Hero />
      <div className="flex-1 rounded-2xl border border-white/10 bg-white/[0.035] p-4 flex items-center justify-center text-text-muted text-sm">
        Live chart coming in C.T11
      </div>
    </div>
  );
}
