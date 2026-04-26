import { Hero } from "@/components/hero/Hero";
import { LiveChart } from "@/components/chart/LiveChart";

// DashboardPage is the landing route: the Hero card on top with the
// Connect affordance, server picker, and mode toggle; below it the
// live up/down throughput chart fed by vpn:speed events.
export function DashboardPage() {
  return (
    <div className="flex flex-col gap-4 h-full">
      <Hero />
      <LiveChart />
    </div>
  );
}
