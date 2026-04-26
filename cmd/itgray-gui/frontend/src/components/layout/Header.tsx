import type { CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "@/store";
import { WindowControls } from "./WindowControls";

// `WebkitAppRegion` is a non-standard CSSProperty used by Wails/Electron
// frameless windows to mark draggable regions. React's CSSProperties type
// doesn't ship it, so we widen via a typed extension rather than `any`.
type DragCSSProperties = CSSProperties & { WebkitAppRegion?: "drag" | "no-drag" };

export function Header() {
  const { t } = useTranslation();
  const status = useStore((s) => s.status);
  const cs = useStore((s) => s.currentServer);
  const speeds = useStore((s) => s.speeds);
  const dragStyle: DragCSSProperties = { WebkitAppRegion: "drag" };
  const labels: Record<string, string> = {
    idle: t("header.disconnected"),
    connecting: t("header.connecting"),
    connected: t("header.connected"),
    disconnecting: t("header.disconnecting"),
    error: t("header.error"),
  };
  return (
    <header
      style={dragStyle}
      className="h-14 px-4 flex items-center gap-3 border-b border-white/10 bg-white/[0.025]"
    >
      <div className="font-semibold tracking-tight">ITG Ray</div>
      <span className={`w-2 h-2 rounded-full ${dotColor(status)}`} />
      <span className="text-sm text-text-secondary">{labels[status] ?? status}</span>
      {cs && <span className="text-sm text-text-secondary">· {cs.name}</span>}
      <span className="ml-auto text-xs text-text-muted">
        ↑ {fmt(speeds.upBps)}/s  ↓ {fmt(speeds.downBps)}/s
      </span>
      <WindowControls />
    </header>
  );
}

function dotColor(s: string): string {
  return (
    {
      connected: "bg-emerald-500",
      connecting: "bg-amber-500",
      error: "bg-rose-500",
      idle: "bg-white/30",
      disconnecting: "bg-amber-500",
    } as Record<string, string>
  )[s] ?? "bg-white/30";
}

function fmt(b: number): string {
  if (b > 1e6) return (b / 1e6).toFixed(1) + " MB";
  if (b > 1e3) return (b / 1e3).toFixed(1) + " kB";
  return `${b} B`;
}
