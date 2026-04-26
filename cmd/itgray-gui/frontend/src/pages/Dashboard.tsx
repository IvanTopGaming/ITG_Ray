import React, { useEffect, useRef, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  YAxis,
} from "recharts";
import { ArrowRight, Star } from "lucide-react";
import { motion, type Variants } from "framer-motion";
import { GlowOrb, type OrbStatus } from "@/components/GlowOrb";
import { cn } from "@/lib/cn";

type Mode = "sysproxy" | "tun";

interface Speed {
  downBps: number;
  upBps: number;
}

interface Totals {
  down: number;
  up: number;
}

interface SpeedPoint {
  t: number;
  down: number;
  up: number;
}

interface FakeServer {
  id: string;
  flag: string;
  city: string;
  code: string;
  pingMs: number;
  favorite: boolean;
}

const FAKE_SERVERS: FakeServer[] = [
  {
    id: "s1",
    flag: "🇳🇱",
    city: "Amsterdam",
    code: "NL-AMS-03",
    pingMs: 12,
    favorite: true,
  },
  {
    id: "s2",
    flag: "🇩🇪",
    city: "Frankfurt",
    code: "DE-FRA-01",
    pingMs: 28,
    favorite: false,
  },
  {
    id: "s3",
    flag: "🇫🇮",
    city: "Helsinki",
    code: "FI-HEL-02",
    pingMs: 34,
    favorite: true,
  },
];

const HISTORY_LENGTH = 60;

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
  const [status, setStatus] = useState<OrbStatus>("idle");
  const [mode, setMode] = useState<Mode>("sysproxy");
  const [activeServerId, setActiveServerId] = useState<string>("s1");
  const [connectedAt, setConnectedAt] = useState<number | null>(null);
  const [now, setNow] = useState(Date.now());
  const [speed, setSpeed] = useState<Speed>({ downBps: 0, upBps: 0 });
  const [totals, setTotals] = useState<Totals>({ down: 0, up: 0 });
  const [history, setHistory] = useState<SpeedPoint[]>([]);
  const speedTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const tickTimer = useRef<ReturnType<typeof setInterval> | null>(null);

  const activeServer =
    FAKE_SERVERS.find((s) => s.id === activeServerId) ?? FAKE_SERVERS[0];

  useEffect(() => {
    if (status === "connecting") {
      const t = setTimeout(() => {
        setStatus("connected");
        setConnectedAt(Date.now());
      }, 900);
      return () => clearTimeout(t);
    }
    if (status === "disconnecting") {
      const t = setTimeout(() => {
        setStatus("idle");
        setConnectedAt(null);
        setSpeed({ downBps: 0, upBps: 0 });
        setTotals({ down: 0, up: 0 });
        setHistory([]);
      }, 450);
      return () => clearTimeout(t);
    }
  }, [status]);

  useEffect(() => {
    if (status !== "connected") {
      if (speedTimer.current) clearInterval(speedTimer.current);
      if (tickTimer.current) clearInterval(tickTimer.current);
      return;
    }
    speedTimer.current = setInterval(() => {
      const next: Speed = {
        downBps: 25_000_000 + Math.random() * 8_000_000,
        upBps: 1_750_000 + Math.random() * 1_000_000,
      };
      setSpeed(next);
      setTotals((prev) => ({
        down: prev.down + next.downBps,
        up: prev.up + next.upBps,
      }));
      setHistory((prev) => {
        const point: SpeedPoint = {
          t: Date.now(),
          down: next.downBps / 125_000,
          up: next.upBps / 125_000,
        };
        const updated = [...prev, point];
        return updated.length > HISTORY_LENGTH
          ? updated.slice(updated.length - HISTORY_LENGTH)
          : updated;
      });
    }, 1000);
    tickTimer.current = setInterval(() => setNow(Date.now()), 1000);
    setSpeed({ downBps: 26_500_000, upBps: 1_900_000 });
    return () => {
      if (speedTimer.current) clearInterval(speedTimer.current);
      if (tickTimer.current) clearInterval(tickTimer.current);
    };
  }, [status]);

  const sessionSeconds = connectedAt
    ? Math.floor((now - connectedAt) / 1000)
    : 0;

  const orbDisabled = status === "connecting" || status === "disconnecting";

  function handleOrbClick(e: React.MouseEvent<HTMLButtonElement>) {
    if (e.shiftKey) {
      setStatus("error");
      return;
    }
    if (status === "idle" || status === "error") setStatus("connecting");
    else if (status === "connected") setStatus("disconnecting");
  }

  function handleModeChange(next: Mode) {
    if (next === mode) return;
    if (status === "connecting" || status === "disconnecting") return;
    setMode(next);
    if (status === "connected") {
      setConnectedAt(null);
      setSpeed({ downBps: 0, upBps: 0 });
      setTotals({ down: 0, up: 0 });
      setHistory([]);
      setStatus("connecting");
    }
  }

  function handleQuickSwitch(serverId: string) {
    if (status === "connecting" || status === "disconnecting") return;
    if (serverId === activeServerId && status === "connected") return;
    setActiveServerId(serverId);
    if (status === "connected") {
      setConnectedAt(null);
      setSpeed({ downBps: 0, upBps: 0 });
      setTotals({ down: 0, up: 0 });
      setHistory([]);
      setStatus("connecting");
    } else if (status === "idle" || status === "error") {
      setStatus("connecting");
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
          value={mode}
          onChange={handleModeChange}
          disabled={status === "connecting" || status === "disconnecting"}
        />
      </motion.div>

      <motion.div
        variants={itemVariants}
        className="glass-regular min-h-[200px] rounded-2xl p-7"
      >
        <div className="flex items-center gap-6">
          <div className="flex h-[140px] shrink-0 items-center">
            <GlowOrb
              status={status}
              size={104}
              onClick={handleOrbClick}
              disabled={orbDisabled}
              ariaLabel={ariaLabelFor(status)}
            />
          </div>

          <div className="flex h-[140px] min-w-0 flex-1 flex-col gap-4">
            <StatusLine status={status} />
            <ActiveRoute status={status} mode={mode} server={activeServer} />
          </div>

          <div className="h-[140px] w-px shrink-0 bg-white/10" />

          <Stats
            status={status}
            speed={speed}
            totals={totals}
            sessionSeconds={sessionSeconds}
          />
        </div>
      </motion.div>

      <motion.div variants={itemVariants}>
        <QuickSwitch
          servers={FAKE_SERVERS}
          activeServerId={activeServerId}
          status={status}
          onPick={handleQuickSwitch}
        />
      </motion.div>

      <motion.div variants={itemVariants} className="grid grid-cols-3 gap-4">
        <MetricChart
          status={status}
          speed={speed}
          history={history}
          metric="down"
        />
        <MetricChart
          status={status}
          speed={speed}
          history={history}
          metric="up"
        />
        <ConnectionInfo status={status} server={activeServer} />
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
  server: FakeServer;
}) {
  if (status === "idle" || status === "error") {
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
      <div className="h-[28px] text-[24px] font-bold leading-[28px] tracking-tight">
        {server.flag} {server.city}
      </div>
      <div className="mt-1 h-[16px] font-mono text-[12px] leading-[16px] tabular-nums text-white/55">
        {server.code} · {server.pingMs} ms · {mode}
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
  speed: Speed;
  totals: Totals;
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
  servers: FakeServer[];
  activeServerId: string;
  status: OrbStatus;
  onPick: (id: string) => void;
}) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between px-1">
        <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
          Quick switch
        </span>
        <button className="group flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-medium text-white/75 transition-colors duration-instant ease-snap hover:bg-white/[0.06] hover:text-white">
          See all
          <ArrowRight className="h-3 w-3 transition-transform duration-instant ease-snap group-hover:translate-x-0.5" />
        </button>
      </div>
      <div className="grid grid-cols-3 gap-3">
        {servers.map((server) => {
          const active = server.id === activeServerId && status === "connected";
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
              <span className="relative z-10 text-[26px] leading-none">
                {server.flag}
              </span>
              <div className="relative z-10 flex min-w-0 flex-1 flex-col gap-0.5">
                <div className="flex items-center gap-1.5">
                  <span className="truncate text-[13px] font-semibold">
                    {server.city}
                  </span>
                  {server.favorite && (
                    <Star
                      className="h-3 w-3 fill-warn stroke-warn"
                      strokeWidth={2}
                    />
                  )}
                </div>
                <span className="font-mono text-[10px] tabular-nums text-white/45">
                  {server.code}
                </span>
              </div>
              <div className="relative z-10 flex shrink-0 flex-col items-end gap-1">
                <span
                  className={cn(
                    "font-mono text-[11px] font-semibold tabular-nums",
                    server.pingMs < 25
                      ? "text-success"
                      : server.pingMs < 60
                        ? "text-warn"
                        : "text-white/65",
                  )}
                >
                  {server.pingMs} ms
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
  speed: Speed;
  history: SpeedPoint[];
  metric: "down" | "up";
}) {
  const live = status === "connected";
  const currentMbps = live
    ? (metric === "down" ? speed.downBps : speed.upBps) / 125_000
    : 0;
  const peakMbps = history.length
    ? Math.max(0, ...history.map((p) => p[metric]))
    : 0;
  const label = metric === "down" ? "↓ Download" : "↑ Upload";
  const color = metric === "down" ? "#00e892" : "#7ed4ff";
  const gradId = `grad-${metric}`;

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
        {live && history.length > 1 ? (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart
              data={history}
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
                dataKey={metric}
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
}: {
  status: OrbStatus;
  server: FakeServer;
}) {
  const live = status === "connected";
  const dash = "—";
  const fakePublicIP: Record<string, string> = {
    s1: "89.46.•••.62",
    s2: "62.171.•••.21",
    s3: "92.118.•••.48",
  };
  const rows: Array<{ label: string; value: string; accent?: string }> = [
    { label: "Protocol", value: "VLESS · Reality" },
    { label: "Transport", value: "WebSocket" },
    {
      label: "Public IP",
      value: live ? `${fakePublicIP[server.id] ?? dash} · ${server.flag}` : dash,
    },
    { label: "DNS", value: "1.1.1.1, 8.8.8.8" },
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
            <span className="inline-block h-1.5 w-1.5 rounded-full bg-success shadow-[0_0_6px_rgba(0,230,118,0.7)]" />
            running
          </span>
        </div>
      </div>
    </div>
  );
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
