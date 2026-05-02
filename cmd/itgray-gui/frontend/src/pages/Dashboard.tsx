import React, { useEffect, useMemo, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  YAxis,
} from "recharts";
import { ArrowRight, Star } from "lucide-react";
import { motion, type Variants } from "framer-motion";
import { Link } from "react-router-dom";
import { GlowOrb, type OrbStatus } from "@/components/GlowOrb";
import { CountryFlag } from "@/components/controls/CountryFlag";
import { Reveal } from "@/components/controls/Reveal";
import { cn } from "@/lib/cn";
import {
  useDash,
  effectiveStatus,
  dashConnect,
  dashDisconnect,
  dashSwitchMode,
  clearLastError,
  type Mode,
  type SpeedPoint,
} from "@/lib/dashStore";
import { useIp, ipRefresh, ipReset } from "@/lib/ipStore";
import { useSettings } from "@/lib/settings";
import { pickQuickSwitch } from "@/lib/quickSwitch";
import type { hub } from "../../wailsjs/go/models";

type ServerView = hub.ServerView;

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.08, delayChildren: 0.05 },
  },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 12 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.42, ease: SNAP_EASE },
  },
};

export function Dashboard() {
  const dash = useDash();
  const ip = useIp();
  const [settings] = useSettings();
  const eff = effectiveStatus(dash) as OrbStatus;

  const quickSwitchServers = useMemo(
    () => pickQuickSwitch(dash.allServers, 3),
    [dash.allServers],
  );

  // Drive ipStore based on connection lifecycle.
  useEffect(() => {
    if (dash.status === "connected") void ipRefresh();
    else ipReset();
  }, [dash.status]);

  // Tick second-resolution clock for uptime display while connected.
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (dash.status !== "connected") return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [dash.status]);

  const sessionSeconds = dash.connectedAt
    ? Math.floor((now - dash.connectedAt) / 1000)
    : 0;
  const orbDisabled =
    dash.status === "connecting" || dash.status === "disconnecting";

  async function handleOrbClick() {
    if (eff === "idle" || eff === "error") {
      const target = dash.currentServer?.id ?? quickSwitchServers[0]?.id;
      if (!target) return;
      try {
        await dashConnect(target);
      } catch {
        /* lastError set */
      }
    } else if (eff === "connected") {
      try {
        await dashDisconnect();
      } catch {
        /* lastError set */
      }
    }
  }

  async function handleModeChange(next: Mode) {
    if (next === dash.mode || orbDisabled) return;
    try {
      await dashSwitchMode(next);
    } catch {
      /* lastError set */
    }
  }

  async function handleQuickSwitch(serverId: string) {
    if (orbDisabled) return;
    if (serverId === dash.currentServer?.id && dash.status === "connected")
      return;
    try {
      await dashConnect(serverId);
    } catch {
      /* lastError set */
    }
  }

  return (
    <motion.section
      className="flex flex-col gap-5"
      variants={containerVariants}
      initial="hidden"
      animate="show"
    >
      <motion.div
        className="flex items-center justify-between"
        variants={itemVariants}
      >
        <h1 className="text-[22px] font-semibold tracking-tight">Dashboard</h1>
        <ModeToggle
          value={dash.mode}
          onChange={handleModeChange}
          disabled={orbDisabled}
        />
      </motion.div>

      <Reveal show={!!dash.lastError}>
        <div className="mb-5 flex items-start gap-3 rounded-xl border border-[#ff9a9a]/30 bg-[#ff9a9a]/[0.06] px-4 py-3">
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-[#ff9a9a]">
              {dash.lastError?.kind ?? "error"}
            </span>
            <span className="break-words font-mono text-[12px] text-white/85">
              {dash.lastError?.message ?? ""}
            </span>
          </div>
          <button
            onClick={clearLastError}
            className="shrink-0 rounded-md px-2 py-1 text-[11px] font-medium text-white/55 transition-colors hover:bg-white/[0.06] hover:text-white"
          >
            Dismiss
          </button>
        </div>
      </Reveal>

      <motion.div
        variants={itemVariants}
        className="glass-regular min-h-[200px] rounded-2xl p-7"
      >
        <div className="flex items-center gap-6">
          <div className="flex h-[140px] shrink-0 items-center">
            <GlowOrb
              status={eff}
              size={104}
              onClick={handleOrbClick}
              disabled={orbDisabled}
              ariaLabel={ariaLabelFor(eff)}
            />
          </div>

          <div className="flex h-[140px] min-w-0 flex-1 flex-col gap-4">
            <StatusLine status={eff} />
            <ActiveRoute
              status={eff}
              mode={dash.mode}
              server={dash.currentServer}
            />
          </div>

          <div className="h-[140px] w-px shrink-0 bg-white/10" />

          <Stats
            status={eff}
            speed={dash.speed}
            totals={dash.totals}
            sessionSeconds={sessionSeconds}
          />
        </div>
      </motion.div>

      <motion.div variants={itemVariants}>
        <QuickSwitch
          servers={quickSwitchServers}
          activeServerId={dash.currentServer?.id ?? null}
          status={eff}
          onPick={handleQuickSwitch}
        />
      </motion.div>

      <motion.div variants={itemVariants} className="grid grid-cols-3 gap-4">
        <MetricChart
          status={eff}
          speed={dash.speed}
          history={dash.history}
          metric="down"
        />
        <MetricChart
          status={eff}
          speed={dash.speed}
          history={dash.history}
          metric="up"
        />
        <ConnectionInfo
          status={eff}
          server={dash.currentServer}
          publicIp={ip}
          dnsMode={settings.dnsMode}
          dnsCustom={settings.dnsCustom}
          helperState={dash.helperState}
        />
      </motion.div>
    </motion.section>
  );
}

function ariaLabelFor(status: OrbStatus): string {
  switch (status) {
    case "idle":
      return "Connect";
    case "connected":
      return "Disconnect";
    case "error":
      return "Try connecting again";
    case "connecting":
      return "Connecting";
    case "disconnecting":
      return "Disconnecting";
  }
}

function StatusLine({ status }: { status: OrbStatus }) {
  const label = {
    idle: "DISCONNECTED",
    connecting: "CONNECTING…",
    connected: "CONNECTED",
    disconnecting: "DISCONNECTING…",
    error: "ERROR",
  }[status];
  const color = {
    idle: "text-white/55",
    connecting: "text-warn",
    connected: "text-success",
    disconnecting: "text-white/55",
    error: "text-[#ff9a9a]",
  }[status];

  return (
    <div className="flex h-[14px] items-center leading-none">
      <span
        className={cn(
          "text-[10px] font-medium uppercase tracking-[0.18em]",
          color,
        )}
      >
        {label}
      </span>
    </div>
  );
}

function ActiveRoute({
  status,
  mode,
  server,
}: {
  status: OrbStatus;
  mode: Mode;
  server: ServerView | null;
}) {
  if (status === "idle" || status === "error" || !server) {
    return (
      <div className="flex h-[48px] flex-col">
        <div className="h-[28px] text-[24px] font-bold leading-[28px] tracking-tight">
          {status === "error" ? "Connection failed" : "No active connection"}
        </div>
        <div className="mt-1 h-[16px] font-mono text-[12px] leading-[16px] tabular-nums text-white/55">
          {status === "error"
            ? "Click the orb to retry."
            : "Click the orb to connect."}
        </div>
      </div>
    );
  }
  return (
    <div className="flex h-[48px] flex-col">
      <div className="flex h-[28px] items-center gap-2 text-[24px] font-bold leading-[28px] tracking-tight">
        {server.country && (
          <CountryFlag
            code={server.country}
            className="h-[18px] w-[27px] shrink-0 rounded-[2px] object-cover shadow-[0_0_0_1px_rgba(255,255,255,0.08)]"
          />
        )}
        <span>{server.name || server.id}</span>
      </div>
      <div className="mt-1 h-[16px] font-mono text-[12px] leading-[16px] tabular-nums text-white/55">
        {server.address || "—"} ·{" "}
        {server.latencyMs ? `${server.latencyMs} ms` : "— ms"} · {mode}
      </div>
    </div>
  );
}

function Stats({
  status,
  speed,
  totals,
  sessionSeconds,
}: {
  status: OrbStatus;
  speed: { downBps: number; upBps: number; at: number };
  totals: { down: number; up: number };
  sessionSeconds: number;
}) {
  const live = status === "connected";
  return (
    <div className="flex h-[140px] w-[152px] shrink-0 flex-col justify-between">
      <Stat
        label="↓ Down"
        value={live ? `${(speed.downBps / 125_000).toFixed(1)} Mbps` : "—"}
      />
      <Stat
        label="↑ Up"
        value={live ? `${(speed.upBps / 125_000).toFixed(1)} Mbps` : "—"}
      />
      <Stat
        label="Uptime"
        value={live ? formatDuration(sessionSeconds) : "—"}
      />
      <Stat
        label="Transferred"
        value={live ? formatBytes(totals.down + totals.up) : "—"}
      />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <div className="h-[11px] text-[9px] font-medium uppercase leading-[11px] tracking-[0.16em] text-white/40">
        {label}
      </div>
      <div className="h-[16px] font-mono text-[13px] font-semibold leading-[16px] tabular-nums text-white/90">
        {value}
      </div>
    </div>
  );
}

function ModeToggle({
  value,
  onChange,
  disabled,
}: {
  value: Mode;
  onChange: (m: Mode) => void;
  disabled?: boolean;
}) {
  return (
    <div
      className={cn(
        "glass-regular flex gap-0 rounded-xl p-1",
        disabled && "pointer-events-none opacity-50",
      )}
    >
      {(["sysproxy", "tun"] as const).map((m) => (
        <button
          key={m}
          onClick={() => onChange(m)}
          className={cn(
            "relative rounded-lg px-3.5 py-1.5 text-[11px] font-medium transition-colors duration-standard ease-snap",
            value === m ? "text-white" : "text-white/55 hover:text-white",
          )}
        >
          {value === m && (
            <motion.div
              layoutId="mode-toggle-pill"
              className="absolute inset-0 rounded-lg bg-white/[0.14]"
              transition={{ type: "spring", stiffness: 380, damping: 32 }}
            />
          )}
          <span className="relative z-10">
            {m === "sysproxy" ? "SysProxy" : "TUN"}
          </span>
        </button>
      ))}
    </div>
  );
}

function QuickSwitch({
  servers,
  activeServerId,
  status,
  onPick,
}: {
  servers: ServerView[];
  activeServerId: string | null;
  status: OrbStatus;
  onPick: (id: string) => void;
}) {
  const cols =
    servers.length === 0
      ? null
      : servers.length === 1
        ? "grid-cols-1"
        : servers.length === 2
          ? "grid-cols-2"
          : "grid-cols-3";

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between px-1">
        <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
          Quick switch
        </span>
        <Link
          to="/servers"
          className="group flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium text-white/75 transition-colors duration-instant ease-snap hover:bg-white/[0.06] hover:text-white"
        >
          See all
          <ArrowRight className="h-3 w-3 transition-transform duration-instant ease-snap group-hover:translate-x-0.5" />
        </Link>
      </div>
      {cols === null ? (
        <QuickSwitchEmpty />
      ) : (
        <div className={cn("grid gap-3", cols)}>
          {servers.map((server) => {
            const active =
              server.id === activeServerId && status === "connected";
            return (
              <motion.button
                key={server.id}
                onClick={() => onPick(server.id)}
                whileHover={{ y: -2 }}
                whileTap={{ scale: 0.97 }}
                transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
                className={cn(
                  "glass-regular relative flex items-center gap-3 rounded-xl px-4 py-3 text-left",
                  !active && "hover:bg-white/[0.04]",
                )}
              >
                {active && (
                  <motion.div
                    layoutId="quick-switch-active-ring"
                    className="absolute -inset-px rounded-xl border-2 border-success/65 bg-success/[0.07] shadow-[0_0_22px_rgba(0,230,118,0.32),inset_0_0_18px_rgba(0,230,118,0.10)]"
                    transition={{ type: "spring", stiffness: 380, damping: 32 }}
                  />
                )}
                {server.country && (
                  <CountryFlag
                    code={server.country}
                    className="relative z-10 h-[20px] w-[30px] shrink-0 rounded-[2px] object-cover shadow-[0_0_0_1px_rgba(255,255,255,0.08)]"
                  />
                )}
                <div className="relative z-10 flex min-w-0 flex-1 flex-col gap-0.5">
                  <div className="flex items-center gap-1.5">
                    <span className="truncate text-[13px] font-semibold">
                      {server.name || server.id}
                    </span>
                    {server.favorite && (
                      <Star
                        className="h-3 w-3 fill-warn stroke-warn"
                        strokeWidth={2}
                      />
                    )}
                  </div>
                  <span className="font-mono text-[10px] tabular-nums text-white/45">
                    {server.address}
                  </span>
                </div>
                <div className="relative z-10 flex shrink-0 flex-col items-end gap-1">
                  <span
                    className={cn(
                      "font-mono text-[11px] font-semibold tabular-nums",
                      server.latencyMs === 0
                        ? "text-white/45"
                        : server.latencyMs < 25
                          ? "text-success"
                          : server.latencyMs < 60
                            ? "text-warn"
                            : "text-white/65",
                    )}
                  >
                    {server.latencyMs ? `${server.latencyMs} ms` : "— ms"}
                  </span>
                  {active ? (
                    <motion.span
                      animate={{ opacity: [0.55, 1, 0.55] }}
                      transition={{
                        duration: 1.8,
                        repeat: Infinity,
                        ease: "easeInOut",
                      }}
                      className="flex items-center gap-1 text-[9px] font-semibold uppercase tracking-[0.16em] text-success"
                    >
                      <span className="h-1 w-1 rounded-full bg-success shadow-[0_0_4px_rgba(0,230,118,0.8)]" />
                      Active
                    </motion.span>
                  ) : (
                    <span className="text-[9px] uppercase tracking-[0.14em] text-white/35">
                      Tap
                    </span>
                  )}
                </div>
              </motion.button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function QuickSwitchEmpty() {
  return (
    <div className="glass-regular flex items-center justify-between gap-4 rounded-xl px-5 py-4">
      <div className="flex flex-col gap-1">
        <span className="text-[13px] font-semibold text-white/85">
          No servers added
        </span>
        <span className="text-[11px] text-white/55">
          Add a subscription or browse servers to start connecting.
        </span>
      </div>
      <div className="flex items-center gap-2">
        <Link
          to="/subscriptions"
          className="rounded-md bg-white/[0.08] px-3 py-1.5 text-[11px] font-medium text-white/85 hover:bg-white/[0.12]"
        >
          Add subscription
        </Link>
        <Link
          to="/servers"
          className="rounded-md px-3 py-1.5 text-[11px] font-medium text-white/65 hover:bg-white/[0.06] hover:text-white"
        >
          Browse
        </Link>
      </div>
    </div>
  );
}

function MetricChart({
  status,
  speed,
  history,
  metric,
}: {
  status: OrbStatus;
  speed: { downBps: number; upBps: number; at: number };
  history: SpeedPoint[];
  metric: "down" | "up";
}) {
  const live = status === "connected";
  const currentMbps = live
    ? (metric === "down" ? speed.downBps : speed.upBps) / 125_000
    : 0;
  const histKey = metric === "down" ? "downBps" : "upBps";
  const peakMbps = history.length
    ? Math.max(0, ...history.map((p) => p[histKey] / 125_000))
    : 0;
  const label = metric === "down" ? "↓ Download" : "↑ Upload";
  const color = metric === "down" ? "#00e892" : "#7ed4ff";
  const gradId = `grad-${metric}`;

  // history needs a per-metric Mbps key for recharts dataKey
  const data = useMemo(
    () => history.map((p) => ({ t: p.t, value: p[histKey] / 125_000 })),
    [history, histKey],
  );

  return (
    <div className="glass-regular flex flex-col gap-3 rounded-2xl p-6">
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
          {label}
        </span>
        <div className="flex items-baseline gap-3">
          {live && peakMbps > 0 && (
            <span className="font-mono text-[10px] tabular-nums text-white/35">
              peak {peakMbps.toFixed(0)}
            </span>
          )}
          <motion.span
            key={live ? currentMbps.toFixed(1) : "idle"}
            initial={{ opacity: 0.55 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.35, ease: SNAP_EASE }}
            className="font-mono text-[12px] tabular-nums"
            style={{ color: live ? color : "rgba(255,255,255,0.3)" }}
          >
            {live ? `${currentMbps.toFixed(1)} Mbps` : "— Mbps"}
          </motion.span>
        </div>
      </div>
      <div className="h-[140px]">
        {live && data.length > 1 ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart
              data={data}
              margin={{ top: 5, right: 6, bottom: 0, left: 0 }}
            >
              <defs>
                <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={color} stopOpacity={0.45} />
                  <stop offset="100%" stopColor={color} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="rgba(255,255,255,0.06)"
                horizontal
                vertical={false}
              />
              <YAxis
                domain={[0, (max: number) => Math.ceil(max * 1.15)]}
                tick={{
                  fontSize: 9,
                  fill: "rgba(255,255,255,0.4)",
                  fontFamily: "ui-monospace, monospace",
                }}
                axisLine={false}
                tickLine={false}
                width={32}
                tickFormatter={(v) => (v >= 1 ? v.toFixed(0) : "")}
              />
              <Area
                type="monotone"
                dataKey="value"
                stroke={color}
                strokeWidth={2}
                fill={`url(#${gradId})`}
                isAnimationActive={false}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="flex h-full items-center justify-center text-[12px] text-white/35">
            {live
              ? "Collecting samples…"
              : `${metric === "down" ? "Download" : "Upload"} graph appears when connected.`}
          </div>
        )}
      </div>
    </div>
  );
}

function ConnectionInfo({
  status,
  server,
  publicIp,
  dnsMode,
  dnsCustom,
  helperState,
}: {
  status: OrbStatus;
  server: ServerView | null;
  publicIp: { value: string | null; loading: boolean; error: string | null };
  dnsMode: string;
  dnsCustom: string;
  helperState: "running" | "stopped" | "missing";
}) {
  const live = status === "connected";
  const dash = "—";

  const protocol = server?.security
    ? `VLESS · ${capitalizeFirst(server.security)}`
    : dash;
  const transport = server?.transport ?? dash;

  const dns =
    dnsMode === "custom" && dnsCustom.trim() ? dnsCustom : "Auto";

  const helperColor = {
    running: "bg-success",
    stopped: "bg-warn",
    missing: "bg-[#ff9a9a]",
  }[helperState];

  const ipNode: React.ReactNode = !live
    ? dash
    : publicIp.loading
      ? <span className="opacity-55">Resolving…</span>
      : publicIp.error
        ? dash
        : publicIp.value
          ? (
            <>
              {publicIp.value}
              {server?.country && (
                <>
                  {" · "}
                  <CountryFlag
                    code={server.country}
                    className="inline-block h-[10px] w-[15px] rounded-[2px] align-[-1px] shadow-[0_0_0_1px_rgba(255,255,255,0.08)]"
                  />
                </>
              )}
            </>
          )
          : dash;

  const rows: Array<{ label: string; value: React.ReactNode }> = [
    { label: "Protocol", value: live ? protocol : dash },
    { label: "Transport", value: live ? transport : dash },
    { label: "Public IP", value: ipNode },
    { label: "DNS", value: dns },
  ];

  return (
    <div className="glass-regular flex h-full flex-col gap-3 rounded-2xl p-6">
      <div className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
        Connection
      </div>
      <div className="flex flex-col gap-2.5">
        {rows.map((row) => (
          <div
            key={row.label}
            className="flex items-baseline justify-between gap-3"
          >
            <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-white/40">
              {row.label}
            </span>
            <span className="font-mono text-[11px] tabular-nums text-white/85">
              {row.value}
            </span>
          </div>
        ))}
        <div className="my-1 h-px bg-white/8" />
        <div className="flex items-center justify-between gap-3">
          <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-white/40">
            Helper
          </span>
          <span className="flex items-center gap-1.5 font-mono text-[11px] text-white/85">
            <span
              className={cn(
                "inline-block h-1.5 w-1.5 rounded-full",
                helperColor,
              )}
            />
            {helperState}
          </span>
        </div>
      </div>
    </div>
  );
}

function capitalizeFirst(s: string): string {
  if (!s) return "";
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function formatDuration(s: number): string {
  const safe = Math.max(0, Math.floor(s));
  const hh = String(Math.floor(safe / 3600)).padStart(2, "0");
  const mm = String(Math.floor((safe % 3600) / 60)).padStart(2, "0");
  const ss = String(safe % 60).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const KB = 1024;
  const MB = KB * 1024;
  const GB = MB * 1024;
  if (bytes < KB) return `${bytes.toFixed(0)} B`;
  if (bytes < MB) return `${(bytes / KB).toFixed(0)} KB`;
  if (bytes < GB) return `${(bytes / MB).toFixed(1)} MB`;
  return `${(bytes / GB).toFixed(2)} GB`;
}
