import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";
import { MirrorChart } from "./MirrorChart";
import { formatBytes, formatCount, formatDuration, type Period, type StatsApp, type StatsOverview, type StatsRule } from "./format";

function SplitBar({ proxy, direct }: { proxy: number; direct: number }) {
  const total = proxy + direct;
  const p = total > 0 ? (proxy / total) * 100 : 0;
  return (
    <div className="mt-3 flex h-1.5 overflow-hidden rounded-full bg-white/[0.08]">
      <span className="h-full bg-gradient-to-r from-accent-start to-accent-mid" style={{ width: `${p}%` }} />
      <span className="h-full bg-white/[0.28]" style={{ width: `${total > 0 ? 100 - p : 0}%` }} />
    </div>
  );
}

function Tile({ label, value, hint, children, wide }: { label: string; value: string; hint?: string; children?: React.ReactNode; wide?: boolean }) {
  return (
    <div className={cn("glass-regular flex flex-col rounded-2xl p-4", wide ? "min-w-[230px] flex-[1.4]" : "min-w-[170px] flex-1")}>
      <span className="truncate text-[10px] font-medium uppercase tracking-[0.16em] text-white/45">{label}</span>
      <span className="mt-1.5 whitespace-nowrap font-mono text-[22px] font-semibold tabular-nums text-white/95">{value}</span>
      {hint && <span className="mt-0.5 text-[11px] text-white/45">{hint}</span>}
      {children}
    </div>
  );
}

function Dot({ className }: { className: string }) {
  return <i className={cn("mr-1.5 inline-block h-2 w-2 rounded-[2px] align-middle", className)} />;
}

function DirectionTile({ label, total, proxy, direct }: { label: string; total: number; proxy: number; direct: number }) {
  const { t } = useTranslation();
  return (
    <Tile label={label} value={formatBytes(total)} wide>
      <SplitBar proxy={proxy} direct={direct} />
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px] text-white/55">
        <span>
          <Dot className="bg-accent-start" />
          {t("stats.proxy")} {formatBytes(proxy)}
        </span>
        <span>
          <Dot className="bg-white/30" />
          {t("stats.direct")} {formatBytes(direct)}
        </span>
      </div>
    </Tile>
  );
}

type SortKey = "down" | "up";

function TopApps({ apps }: { apps: StatsApp[] }) {
  const { t } = useTranslation();
  const [sort, setSort] = useState<SortKey>("down");
  const rows = useMemo(() => [...apps].sort((a, b) => b[sort] - a[sort]), [apps, sort]);
  return (
    <div data-testid="stats-apps" className="glass-regular flex min-w-0 flex-col rounded-2xl p-4">
      <span className="mb-2 text-[10px] font-medium uppercase tracking-[0.16em] text-white/45">{t("stats.apps.title")}</span>
      <div className="flex items-center gap-3 pb-1 text-[10px] uppercase tracking-[0.08em] text-white/35">
        <span className="w-[32%]">{t("stats.apps.app")}</span>
        <span className="flex-1">{t("stats.apps.proxyShare")}</span>
        {(["down", "up"] as const).map((k) => (
          <button
            key={k}
            type="button"
            onClick={() => setSort(k)}
            className={cn("w-[64px] text-right uppercase transition-colors", sort === k ? "text-white/85" : "hover:text-white/70")}
          >
            {k === "down" ? "↓" : "↑"}
          </button>
        ))}
      </div>
      {rows.length === 0 ? (
        <span className="py-3 text-[12px] text-white/40">{t("stats.noData")}</span>
      ) : (
        rows.map((a) => {
          const total = a.down + a.up;
          const share = total > 0 ? ((a.downProxy + a.upProxy) / total) * 100 : 0;
          return (
            <div key={a.app} className="flex items-center gap-3 border-t border-white/[0.05] py-1.5 text-[12.5px]">
              <span data-testid="app-name" className="w-[32%] truncate text-white/85">{a.app}</span>
              <span className="flex h-1.5 flex-1 overflow-hidden rounded-full bg-white/[0.08]">
                <span className="h-full bg-gradient-to-r from-accent-start to-accent-mid" style={{ width: `${share}%` }} />
              </span>
              <span className="w-[64px] text-right font-mono text-[11px] tabular-nums text-white/60">{formatBytes(a.down)}</span>
              <span className="w-[64px] text-right font-mono text-[11px] tabular-nums text-white/60">{formatBytes(a.up)}</span>
            </div>
          );
        })
      )}
    </div>
  );
}

function actionStyle(action: string): string {
  switch (action) {
    case "direct":
      return "bg-amber-500/20 text-amber-200";
    case "block":
      return "bg-rose-500/20 text-rose-200";
    default:
      return "bg-sky-500/20 text-sky-200";
  }
}

function TopRules({ rules }: { rules: StatsRule[] }) {
  const { t, i18n } = useTranslation();
  const max = rules.reduce((m, r) => Math.max(m, r.conns), 0);
  return (
    <div data-testid="stats-rules" className="glass-regular flex min-w-0 flex-col rounded-2xl p-4">
      <span className="mb-2 text-[10px] font-medium uppercase tracking-[0.16em] text-white/45">{t("stats.rules.title")}</span>
      {rules.length === 0 ? (
        <span className="py-3 text-[12px] text-white/40">{t("stats.noData")}</span>
      ) : (
        rules.map((r) => {
          const name = r.ruleId === "final" ? t("stats.rules.default") : r.deleted ? t("stats.rules.deleted") : r.ruleName || r.ruleId;
          return (
            <div key={`${r.ruleId}|${r.route}|${r.serverId ?? ""}`} className="flex items-center gap-2.5 border-t border-white/[0.05] py-1.5 text-[12.5px]">
              <span className={cn("min-w-0 flex-1 truncate", r.deleted ? "italic text-white/45" : "text-white/85")}>{name}</span>
              <span className={cn("rounded px-2 py-0.5 text-[9.5px] font-semibold uppercase tracking-wider", actionStyle(r.action))}>
                {t(`ruleEditor.actions.${r.action}`, { defaultValue: r.action })}
              </span>
              {r.serverName && <span className="max-w-[28%] truncate text-[11px] text-sky-100/70">{r.serverName}</span>}
              <span className="flex h-1.5 w-[18%] overflow-hidden rounded-full bg-white/[0.08]">
                <span className="h-full bg-white/[0.35]" style={{ width: `${max > 0 ? (r.conns / max) * 100 : 0}%` }} />
              </span>
              <span className="w-[56px] text-right font-mono text-[11px] tabular-nums text-white/60">{formatCount(r.conns, i18n.language)}</span>
            </div>
          );
        })
      )}
    </div>
  );
}

export function Overview({ data, period }: { data: StatsOverview; period: Period }) {
  const { t, i18n } = useTranslation();
  const empty = data.down + data.up === 0 && data.blocked === 0 && data.onlineSeconds === 0;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-3">
        <DirectionTile label={t("stats.tiles.down")} total={data.down} proxy={data.downProxy} direct={data.downDirect} />
        <DirectionTile label={t("stats.tiles.up")} total={data.up} proxy={data.upProxy} direct={data.upDirect} />
        <Tile label={t("stats.tiles.blocked")} value={formatCount(data.blocked, i18n.language)} hint={t("stats.tiles.blockedHint")} />
        <Tile
          label={t("stats.tiles.online")}
          value={formatDuration(data.onlineSeconds, t)}
          hint={t("stats.tiles.sessions", { count: data.sessions })}
        />
      </div>
      {empty ? (
        <div className="glass-regular rounded-2xl p-8 text-center text-[13px] text-white/50">{t("stats.empty")}</div>
      ) : (
        <>
          <MirrorChart series={data.series} period={period} />
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <TopApps apps={data.apps} />
            <TopRules rules={data.rules} />
          </div>
        </>
      )}
    </div>
  );
}
