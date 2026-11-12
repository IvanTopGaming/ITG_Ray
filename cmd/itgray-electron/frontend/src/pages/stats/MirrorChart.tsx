import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Bar, BarChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Segmented } from "@/components/controls/Segmented";
import { formatBytes, formatBytesShort, type Period, type StatsPoint } from "./format";

type Mode = "direction" | "route";

const DOWN = "#00e892";
const UP = "#7ed4ff";
const PROXY = "#7ed4ff";
const DIRECT = "rgba(255,255,255,0.32)";

function label(start: string, period: Period, locale: string): string {
  const d = new Date(start);
  if (period === "today") return d.toLocaleTimeString(locale, { hour: "2-digit", minute: "2-digit" });
  return d.toLocaleDateString(locale, { day: "2-digit", month: "2-digit" });
}

export function MirrorChart({ series, period }: { series: StatsPoint[]; period: Period }) {
  const { t, i18n } = useTranslation();
  const [mode, setMode] = useState<Mode>("direction");

  const data = useMemo(
    () =>
      series.map((p) => ({
        label: label(p.start, period, i18n.language),
        down: p.down,
        upNeg: -p.up,
        up: p.up,
        proxy: p.downProxy + p.upProxy,
        direct: p.downDirect + p.upDirect,
      })),
    [series, period, i18n.language],
  );

  return (
    <div className="glass-regular flex flex-col gap-3 rounded-2xl p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
          {period === "today" ? t("stats.chart.hourly") : t("stats.chart.daily")}
        </span>
        <Segmented<Mode>
          value={mode}
          onChange={setMode}
          options={[
            { value: "direction", label: t("stats.chart.direction") },
            { value: "route", label: t("stats.chart.route") },
          ]}
        />
      </div>
      <div className="h-[200px]">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} stackOffset={mode === "direction" ? "sign" : "none"} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.06)" vertical={false} />
            <XAxis
              dataKey="label"
              tick={{ fontSize: 9, fill: "rgba(255,255,255,0.4)" }}
              axisLine={false}
              tickLine={false}
              interval="preserveStartEnd"
              minTickGap={16}
            />
            <YAxis
              tick={{ fontSize: 9, fill: "rgba(255,255,255,0.4)", fontFamily: "ui-monospace, monospace" }}
              axisLine={false}
              tickLine={false}
              width={72}
              tickFormatter={(v: number) => formatBytesShort(Math.abs(v))}
            />
            <Tooltip
              cursor={{ fill: "rgba(255,255,255,0.04)" }}
              contentStyle={{ background: "rgba(10,16,38,0.92)", border: "1px solid rgba(255,255,255,0.12)", borderRadius: 10, fontSize: 11 }}
              labelStyle={{ color: "rgba(255,255,255,0.6)" }}
              formatter={(value: number, name: string) => [formatBytes(Math.abs(value)), t(`stats.chart.series.${name}`)]}
            />
            {mode === "direction" ? (
              <>
                <ReferenceLine y={0} stroke="rgba(255,255,255,0.25)" />
                <Bar dataKey="down" stackId="d" fill={DOWN} radius={[3, 3, 0, 0]} isAnimationActive={false} />
                <Bar dataKey="upNeg" stackId="d" fill={UP} radius={[0, 0, 3, 3]} isAnimationActive={false} />
              </>
            ) : (
              <>
                <Bar dataKey="direct" stackId="r" fill={DIRECT} isAnimationActive={false} />
                <Bar dataKey="proxy" stackId="r" fill={PROXY} radius={[3, 3, 0, 0]} isAnimationActive={false} />
              </>
            )}
          </BarChart>
        </ResponsiveContainer>
      </div>
      <div className="flex gap-4 text-[11px] text-white/55">
        {mode === "direction" ? (
          <>
            <Legend color={DOWN} text={t("stats.chart.legendDown")} />
            <Legend color={UP} text={t("stats.chart.legendUp")} />
          </>
        ) : (
          <>
            <Legend color={PROXY} text={t("stats.proxy")} />
            <Legend color={DIRECT} text={t("stats.direct")} />
          </>
        )}
      </div>
    </div>
  );
}

function Legend({ color, text }: { color: string; text: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span className="inline-block h-2 w-2 rounded-[2px]" style={{ background: color }} />
      {text}
    </span>
  );
}
