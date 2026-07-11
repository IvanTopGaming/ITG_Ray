import React from "react";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { Zap } from "lucide-react";
import { cn } from "@/lib/cn";

export type OrbStatus =
  | "idle"
  | "connecting"
  | "connected"
  | "disconnecting"
  | "error";

interface GlowOrbProps {
  status: OrbStatus;
  size?: number;
  onClick?: (e: React.MouseEvent<HTMLButtonElement>) => void;
  disabled?: boolean;
  ariaLabel?: string;
  className?: string;
  progress?: number | null;
}

interface OrbStyle {
  border: string;
  background: string;
  boxShadow: string;
  iconColor: string;
  iconFill: string;
  scale: number;
}

const STYLES: Record<OrbStatus, OrbStyle> = {
  idle: {
    border: "rgba(180,210,235,0.32)",
    background: "rgba(140,180,220,0.06)",
    boxShadow: "inset 0 0 26px rgba(0,0,0,0.45)",
    iconColor: "rgba(195,220,245,0.65)",
    iconFill: "none",
    scale: 1,
  },
  connecting: {
    border: "rgba(255,188,90,0.65)",
    background: "rgba(55,32,5,0.30)",
    boxShadow: "inset 0 0 30px rgba(255,188,90,0.28)",
    iconColor: "#ffe097",
    iconFill: "none",
    scale: 1.06,
  },
  connected: {
    border: "rgba(40,240,170,0.78)",
    background: "rgba(0,52,38,0.42)",
    boxShadow:
      "0 0 38px rgba(40,240,170,0.55), inset 0 0 32px rgba(40,240,170,0.32)",
    iconColor: "#6affd0",
    iconFill: "#00f099",
    scale: 1.06,
  },
  disconnecting: {
    border: "rgba(40,240,170,0.36)",
    background: "rgba(0,52,38,0.20)",
    boxShadow: "inset 0 0 24px rgba(40,240,170,0.16)",
    iconColor: "rgba(106,255,208,0.62)",
    iconFill: "none",
    scale: 1,
  },
  error: {
    border: "rgba(255,110,110,0.72)",
    background: "rgba(50,0,0,0.40)",
    boxShadow:
      "0 0 32px rgba(255,110,110,0.48), inset 0 0 26px rgba(255,110,110,0.28)",
    iconColor: "#ff9a9a",
    iconFill: "#ff5e5e",
    scale: 1,
  },
};

const TRANSITION = "all 480ms cubic-bezier(0.16, 1, 0.3, 1)";

export const GlowOrb = React.memo(function GlowOrb({
  status,
  size = 104,
  onClick,
  disabled = false,
  ariaLabel,
  className,
  progress = null,
}: GlowOrbProps) {
  const { t } = useTranslation();
  const s = STYLES[status];
  const pct = progress == null ? null : Math.max(0, Math.min(100, progress));
  const interactive = Boolean(onClick) && !disabled;
  const iconSize = Math.round(size * 0.34);

  const sharedClasses = cn(
    "relative shrink-0 flex items-center justify-center rounded-full p-0",
    status === "error" && "animate-orb-shake",
    className,
  );

  const sharedStyle = {
    width: size,
    height: size,
    background: s.background,
    border: `1px solid ${s.border}`,
    boxShadow: s.boxShadow,
    transform: `scale(${s.scale})`,
    transition: TRANSITION,
  };

  const content = (
    <>
      {status === "connecting" && (
        <>
          <motion.div
            aria-hidden
            className="absolute inset-0 rounded-full"
            style={{ boxShadow: "0 0 24px rgba(255,188,90,0.45)" }}
            animate={{ opacity: [0.3, 0.75, 0.3], scale: [0.97, 1.05, 0.97] }}
            transition={{ repeat: Infinity, duration: 1.9, ease: "easeInOut" }}
          />
          <svg aria-hidden className="absolute inset-0" viewBox="0 0 100 100">
            <circle
              cx="50"
              cy="50"
              r="46"
              fill="none"
              stroke="rgba(255,194,102,0.14)"
              strokeWidth="3"
            />
          </svg>
          {pct == null ? (
            <>
              <motion.div
                aria-hidden
                className="absolute inset-0"
                animate={{ rotate: 360 }}
                transition={{ repeat: Infinity, ease: "linear", duration: 1.1 }}
              >
                <svg className="h-full w-full" viewBox="0 0 100 100">
                  <circle
                    cx="50"
                    cy="50"
                    r="46"
                    fill="none"
                    stroke="#ffc266"
                    strokeWidth="3"
                    strokeLinecap="round"
                    pathLength={100}
                    strokeDasharray="30 100"
                  />
                </svg>
              </motion.div>
              <motion.div
                aria-hidden
                className="absolute inset-0"
                animate={{ rotate: -360 }}
                transition={{ repeat: Infinity, ease: "linear", duration: 1.7 }}
              >
                <svg className="h-full w-full" viewBox="0 0 100 100">
                  <circle
                    cx="50"
                    cy="50"
                    r="38"
                    fill="none"
                    stroke="rgba(255,224,151,0.65)"
                    strokeWidth="2"
                    strokeLinecap="round"
                    pathLength={100}
                    strokeDasharray="16 100"
                  />
                </svg>
              </motion.div>
            </>
          ) : (
            <svg aria-hidden className="absolute inset-0 -rotate-90" viewBox="0 0 100 100">
              <circle
                cx="50"
                cy="50"
                r="46"
                fill="none"
                stroke="#ffc266"
                strokeWidth="3"
                strokeLinecap="round"
                pathLength={100}
                strokeDasharray={`${pct} 100`}
                style={{
                  transition: "stroke-dasharray 220ms cubic-bezier(0.16, 1, 0.3, 1)",
                }}
              />
            </svg>
          )}
        </>
      )}
      <Zap
        width={iconSize}
        height={iconSize}
        stroke={s.iconColor}
        fill={s.iconFill}
        strokeWidth={2}
        style={{
          transition: "stroke 200ms cubic-bezier(0.16, 1, 0.3, 1)",
          filter:
            s.iconFill !== "none"
              ? `drop-shadow(0 0 6px ${s.iconColor})`
              : undefined,
        }}
      />
    </>
  );

  if (!onClick) {
    return (
      <div aria-hidden className={sharedClasses} style={sharedStyle}>
        {content}
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel ?? t("common.toggleConnection")}
      style={sharedStyle}
      className={cn(
        sharedClasses,
        "outline-none",
        interactive
          ? "cursor-pointer focus-visible:ring-2 focus-visible:ring-accent-start/70 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-1"
          : "cursor-not-allowed",
      )}
    >
      {content}
    </button>
  );
});
