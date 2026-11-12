import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import { Segmented } from "@/components/controls/Segmented";
import { Overview as FetchOverview } from "@/lib/itg/StatsService";
import { Overview } from "./stats/Overview";
import { Connections } from "./stats/Connections";
import type { Period, StatsOverview } from "./stats/format";

type Tab = "overview" | "connections";

const REFRESH_MS = 60_000;

export function Stats() {
  const { t } = useTranslation();
  const [tab, setTab] = useState<Tab>("overview");
  const [period, setPeriod] = useState<Period>("today");
  const [data, setData] = useState<StatsOverview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [liveCount, setLiveCount] = useState(0);

  useEffect(() => {
    let alive = true;
    const load = () => {
      FetchOverview(period)
        .then((res) => {
          if (!alive) return;
          setData(res as StatsOverview);
          setError(null);
        })
        .catch((err: unknown) => {
          if (alive) setError(err instanceof Error ? err.message : String(err));
        });
    };
    load();
    const timer = window.setInterval(load, REFRESH_MS);
    return () => {
      alive = false;
      window.clearInterval(timer);
    };
  }, [period]);

  return (
    <motion.section initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col gap-4">
      <header className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-3">
        <div className="min-w-[240px] flex-1">
          <h1 className="text-[22px] font-semibold tracking-tight">{t("stats.title")}</h1>
          <p className="mt-1 text-[13px] text-white/50">{t("stats.description")}</p>
        </div>
        {tab === "overview" && (
          <Segmented<Period>
            value={period}
            onChange={setPeriod}
            options={[
              { value: "today", label: t("stats.period.today") },
              { value: "7d", label: t("stats.period.7d") },
              { value: "30d", label: t("stats.period.30d") },
            ]}
          />
        )}
      </header>
      <div className="flex gap-5 border-b border-white/[0.08] text-[13px]">
        {(["overview", "connections"] as const).map((k) => (
          <button
            key={k}
            type="button"
            onClick={() => setTab(k)}
            className={
              "-mb-px border-b-2 pb-2 transition-colors " +
              (tab === k ? "border-accent-start text-accent-start" : "border-transparent text-white/50 hover:text-white/80")
            }
          >
            {k === "overview" ? t("stats.tabs.overview") : t("stats.tabs.connections", { count: liveCount })}
          </button>
        ))}
      </div>
      {tab === "overview" ? (
        error ? (
          <div role="alert" className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[12px] text-rose-200">
            {error}
          </div>
        ) : data ? (
          <Overview data={data} period={period} />
        ) : (
          <div className="glass-regular h-40 animate-pulse rounded-2xl" />
        )
      ) : null}
      <Connections visible={tab === "connections"} onCount={setLiveCount} />
    </motion.section>
  );
}
