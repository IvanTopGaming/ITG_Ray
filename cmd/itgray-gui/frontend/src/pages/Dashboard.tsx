import { useEffect, useRef, useState } from "react";
import { GlowOrb, type OrbStatus } from "@/components/GlowOrb";
import { cn } from "@/lib/cn";

type Mode = "sysproxy" | "tun";

interface Speed {
  downBps: number;
  upBps: number;
}

const FAKE_SERVER = {
  flag: "🇳🇱",
  city: "Amsterdam",
  code: "NL-AMS-03",
  pingMs: 12,
};

export function Dashboard() {
  const [status, setStatus] = useState<OrbStatus>("idle");
  const [mode, setMode] = useState<Mode>("sysproxy");
  const [connectedAt, setConnectedAt] = useState<number | null>(null);
  const [now, setNow] = useState(Date.now());
  const [speed, setSpeed] = useState<Speed>({ downBps: 0, upBps: 0 });
  const speedTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const tickTimer = useRef<ReturnType<typeof setInterval> | null>(null);

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
      setSpeed({
        downBps: 25_000_000 + Math.random() * 8_000_000,
        upBps: 1_750_000 + Math.random() * 1_000_000,
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

  function handleOrbClick() {
    if (status === "idle" || status === "error") setStatus("connecting");
    else if (status === "connected") setStatus("disconnecting");
  }

  return (
    <section className="flex flex-col gap-5">
      <div className="flex items-center justify-between">
        <h1 className="text-[22px] font-semibold tracking-tight">Dashboard</h1>
        <ModeToggle
          value={mode}
          onChange={setMode}
          disabled={status !== "idle"}
        />
      </div>

      <div className="glass-regular rounded-2xl p-7 min-h-[200px]">
        <div className="flex items-center gap-7">
          <GlowOrb
            status={status}
            size={104}
            onClick={handleOrbClick}
            disabled={orbDisabled}
            ariaLabel={ariaLabelFor(status)}
          />

          <div className="flex min-w-0 flex-1 flex-col gap-4">
            <StatusLine status={status} sessionSeconds={sessionSeconds} />
            <ActiveRoute status={status} mode={mode} />
            <Metrics status={status} speed={speed} />
          </div>
        </div>
      </div>
    </section>
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

function StatusLine({
  status,
  sessionSeconds,
}: {
  status: OrbStatus;
  sessionSeconds: number;
}) {
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
    connected: "text-accent-start",
    disconnecting: "text-white/55",
    error: "text-[#ff9a9a]",
  }[status];

  return (
    <div className="flex items-center gap-3">
      <span
        className={cn(
          "text-[10px] font-medium uppercase tracking-[0.18em]",
          color,
        )}
      >
        {label}
      </span>
      {status === "connected" && (
        <span className="font-mono text-[11px] tabular-nums text-white/45">
          {formatDuration(sessionSeconds)}
        </span>
      )}
    </div>
  );
}

function ActiveRoute({ status, mode }: { status: OrbStatus; mode: Mode }) {
  if (status === "idle" || status === "error") {
    return (
      <div>
        <div className="text-[24px] font-bold tracking-tight">
          {status === "error" ? "Connection failed" : "No active connection"}
        </div>
        <div className="mt-1 text-[13px] text-white/55">
          {status === "error"
            ? "Click the orb to retry."
            : "Click the orb to connect."}
        </div>
      </div>
    );
  }
  return (
    <div>
      <div className="text-[24px] font-bold tracking-tight">
        {FAKE_SERVER.flag} {FAKE_SERVER.city}
      </div>
      <div className="mt-1 font-mono text-[12px] tabular-nums text-white/55">
        {FAKE_SERVER.code} · {FAKE_SERVER.pingMs} ms · {mode}
      </div>
    </div>
  );
}

function Metrics({ status, speed }: { status: OrbStatus; speed: Speed }) {
  if (status !== "connected") return null;
  return (
    <div className="grid grid-cols-2 gap-5 pt-1">
      <Metric label="↓ Down" value={speed.downBps} />
      <Metric label="↑ Up" value={speed.upBps} />
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  const mbps = (value / 125_000).toFixed(1);
  return (
    <div>
      <div className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
        {label}
      </div>
      <div className="font-mono text-[20px] font-semibold tabular-nums">
        {mbps}
        <span className="ml-1 text-[12px] font-normal text-white/55">
          Mbps
        </span>
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
            "rounded-lg px-3.5 py-1.5 text-[11px] font-medium transition-colors duration-instant ease-snap",
            value === m
              ? "bg-white/[0.14] text-white"
              : "text-white/55 hover:text-white",
          )}
        >
          {m === "sysproxy" ? "SysProxy" : "TUN"}
        </button>
      ))}
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
