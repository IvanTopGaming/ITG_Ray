import { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Search } from "lucide-react";
import { Connections as FetchConnections } from "@/lib/itg/StatsService";
import { useRules } from "@/lib/rulesStore";
import { cn } from "@/lib/cn";
import { AddRuleMenu, type CreateRuleRequest } from "./AddRuleMenu";
import { formatRate, type StatsConn } from "./format";

const VISIBLE_MS = 1000;
const HIDDEN_MS = 5000;

function routeBadge(route: string): string {
  switch (route) {
    case "direct":
      return "bg-amber-500/20 text-amber-200";
    case "block":
      return "bg-rose-500/20 text-rose-200";
    default:
      return "bg-sky-500/20 text-sky-200";
  }
}

export function Connections({ visible, onCount }: { visible: boolean; onCount: (n: number) => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { groups } = useRules();
  const [conns, setConns] = useState<StatsConn[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [filter, setFilter] = useState("");

  useEffect(() => {
    let alive = true;
    const load = () => {
      FetchConnections()
        .then((res) => {
          if (!alive) return;
          const list = ((res as { conns?: StatsConn[] } | null)?.conns ?? []) as StatsConn[];
          setConns(list);
          setLoaded(true);
          onCount(list.length);
        })
        .catch(() => {
          if (alive) setLoaded(true);
        });
    };
    load();
    const timer = window.setInterval(load, visible ? VISIBLE_MS : HIDDEN_MS);
    return () => {
      alive = false;
      window.clearInterval(timer);
    };
  }, [visible, onCount]);

  const rows = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return conns;
    return conns.filter((c) => c.app.toLowerCase().includes(q) || c.host.toLowerCase().includes(q));
  }, [conns, filter]);

  if (!visible) return null;

  const create = (req: CreateRuleRequest) => {
    navigate("/routing/new", { state: { mode: "create", groupId: req.groupId, beforeId: req.beforeId, prefill: req.prefill } });
  };

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-[11px] text-amber-300/80">● {t("stats.live.memoryOnly")}</span>
        <div className="flex h-8 w-[260px] items-center gap-2 rounded-md border border-white/[0.10] bg-white/[0.05] px-2.5">
          <Search className="h-3.5 w-3.5 shrink-0 text-white/40" />
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder={t("stats.live.filter")}
            className="w-full bg-transparent text-[12px] text-white outline-none placeholder:text-white/35"
          />
        </div>
      </div>
      {loaded && rows.length === 0 ? (
        <div className="glass-regular rounded-2xl p-8 text-center text-[13px] text-white/50">
          {conns.length === 0 ? t("stats.live.empty") : t("stats.live.noMatch")}
        </div>
      ) : (
        <div className="glass-regular overflow-x-auto rounded-2xl p-1.5">
          <table className="w-full border-collapse text-[12.5px]">
            <thead>
              <tr className="text-left text-[10px] uppercase tracking-[0.08em] text-white/40">
                <th className="px-2.5 py-2 font-medium">{t("stats.live.app")}</th>
                <th className="px-2.5 py-2 font-medium">{t("stats.live.host")}</th>
                <th className="px-2.5 py-2 font-medium">{t("stats.live.rule")}</th>
                <th className="px-2.5 py-2 font-medium">{t("stats.live.route")}</th>
                <th className="px-2.5 py-2 text-right font-medium">↓ / ↑</th>
                <th className="px-2.5 py-2" />
              </tr>
            </thead>
            <tbody>
              {rows.map((c) => (
                <tr key={c.id} className="border-t border-white/[0.05]">
                  <td className="max-w-[140px] truncate px-2.5 py-2 text-white/85">{c.app || "—"}</td>
                  <td className="max-w-[260px] truncate px-2.5 py-2 font-mono text-[11.5px] text-white/80" title={c.host}>
                    {c.host || "—"}
                  </td>
                  <td className="max-w-[160px] truncate px-2.5 py-2 text-white/55">
                    {c.ruleId === "final" ? t("stats.rules.default") : c.ruleName || "—"}
                  </td>
                  <td className="whitespace-nowrap px-2.5 py-2">
                    <span className={cn("rounded px-2 py-0.5 text-[9.5px] font-semibold uppercase tracking-wider", routeBadge(c.route))}>
                      {t(`stats.live.routes.${c.route || "proxy"}`)}
                    </span>
                    {c.serverName && <span className="ml-2 text-[11px] text-sky-100/70">{c.serverName}</span>}
                  </td>
                  <td className="whitespace-nowrap px-2.5 py-2 text-right font-mono text-[11px] tabular-nums text-white/55">
                    <span>{formatRate(c.downBps)}</span>
                    <span className="px-1 text-white/30">·</span>
                    <span>{formatRate(c.upBps)}</span>
                  </td>
                  <td className="px-2.5 py-2 text-right">
                    <AddRuleMenu conn={c} groups={groups} onCreate={create} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
