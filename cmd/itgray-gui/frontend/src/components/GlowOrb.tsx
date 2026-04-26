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
  onClick?: () => void;
  disabled?: boolean;
  ariaLabel?: string;
  className?: string;
}

interface OrbStyle {
  outerRing: string;
  outerGlow?: string;
  innerBg: string;
  innerBorder: string;
  innerInsetShadow: string;
  iconColor: string;
  iconFill?: string;
  scale: number;
}

const STYLES: Record<OrbStatus, OrbStyle> = {
  idle: {
    outerRing: "rgba(255,255,255,0.20)",
    innerBg: "rgba(255,255,255,0.02)",
    innerBorder: "rgba(255,255,255,0.10)",
    innerInsetShadow: "inset 0 0 20px rgba(0,0,0,0.30)",
    iconColor: "rgba(255,255,255,0.40)",
    scale: 1,
  },
  connecting: {
    outerRing: "rgba(255,177,60,0.30)",
    innerBg: "rgba(60,40,0,0.20)",
    innerBorder: "rgba(255,177,60,0.30)",
    innerInsetShadow: "inset 0 0 20px rgba(255,177,60,0.18)",
    iconColor: "#ffd28a",
    scale: 1.06,
  },
  connected: {
    outerRing: "rgba(0,230,118,0.55)",
    outerGlow: "0 0 28px rgba(0,230,118,0.40)",
    innerBg: "rgba(0,40,20,0.30)",
    innerBorder: "rgba(0,230,118,0.40)",
    innerInsetShadow: "inset 0 0 24px rgba(0,230,118,0.28)",
    iconColor: "#3effa0",
    iconFill: "#00e676",
    scale: 1.06,
  },
  disconnecting: {
    outerRing: "rgba(0,230,118,0.30)",
    innerBg: "rgba(0,40,20,0.18)",
    innerBorder: "rgba(0,230,118,0.20)",
    innerInsetShadow: "inset 0 0 18px rgba(0,230,118,0.15)",
    iconColor: "rgba(62,255,160,0.55)",
    scale: 1,
  },
  error: {
    outerRing: "rgba(255,94,94,0.50)",
    outerGlow: "0 0 24px rgba(255,94,94,0.35)",
    innerBg: "rgba(60,0,0,0.30)",
    innerBorder: "rgba(255,94,94,0.40)",
    innerInsetShadow: "inset 0 0 20px rgba(255,94,94,0.25)",
    iconColor: "#ff8a8a",
    iconFill: "#ff5e5e",
    scale: 1,
  },
};

const TRANSITION = "all 480ms cubic-bezier(0.16, 1, 0.3, 1)";

export function GlowOrb({
  status,
  size = 104,
  onClick,
  disabled = false,
  ariaLabel,
  className,
}: GlowOrbProps) {
  const s = STYLES[status];
  const interactive = Boolean(onClick) && !disabled;
  const innerSize = Math.round(size * 0.62);
  const iconSize = Math.round(size * 0.34);

  const content = (
    <>
      <div
        className="absolute inset-0 rounded-full"
        style={{
          border: `1px solid ${s.outerRing}`,
          boxShadow: s.outerGlow,
          transition: TRANSITION,
        }}
      />

      {status === "connecting" && (
        <svg
          className="absolute inset-0 -rotate-90 animate-spin"
          style={{ animationDuration: "1.4s" }}
          viewBox="0 0 100 100"
          aria-hidden
        >
          <circle
            cx="50"
            cy="50"
            r="49"
            fill="none"
            stroke="#ffb13c"
            strokeWidth="2"
            strokeLinecap="round"
            pathLength={100}
            strokeDasharray="30 70"
          />
        </svg>
      )}

      <div
        className="flex items-center justify-center rounded-full"
        style={{
          width: innerSize,
          height: innerSize,
          background: s.innerBg,
          border: `1px solid ${s.innerBorder}`,
          boxShadow: s.innerInsetShadow,
          transition: TRANSITION,
        }}
      >
        <Zap
          width={iconSize}
          height={iconSize}
          stroke={s.iconColor}
          fill={s.iconFill ?? "none"}
          strokeWidth={2}
          style={{
            transition: TRANSITION,
            filter: s.iconFill
              ? `drop-shadow(0 0 6px ${s.iconColor})`
              : undefined,
          }}
        />
      </div>
    </>
  );

  const sharedClasses = cn(
    "relative flex shrink-0 items-center justify-center rounded-full p-0",
    status === "error" && "animate-orb-shake",
    className,
  );

  const sharedStyle = {
    width: size,
    height: size,
    transform: `scale(${s.scale})`,
    transition: TRANSITION,
  };

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
      aria-label={ariaLabel ?? "Toggle connection"}
      style={sharedStyle}
      className={cn(
        sharedClasses,
        "outline-none",
        interactive
          ? "cursor-pointer focus-visible:ring-2 focus-visible:ring-accent-start/70 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-1"
          : "cursor-not-allowed",
        // Idle hover: button gets a soft cyan halo to invite the click.
        interactive &&
          status === "idle" &&
          "hover:shadow-[0_0_24px_rgba(120,200,255,0.30)]",
      )}
    >
      {content}
    </button>
  );
}
