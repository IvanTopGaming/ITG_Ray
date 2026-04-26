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
  className?: string;
}

export function GlowOrb({ status, size = 96, className }: GlowOrbProps) {
  return (
    <div
      style={{ width: size, height: size }}
      className={cn(
        "shrink-0 rounded-full transition-all duration-standard ease-snap",
        status === "idle" &&
          "border-2 border-dashed border-white/20 bg-white/[0.03]",
        status === "connecting" &&
          "animate-spin-slow border border-white/20 bg-orb-warn shadow-[0_0_24px_rgba(255,177,60,0.5)]",
        status === "connected" &&
          "animate-orb-pulse bg-orb-accent shadow-[0_0_36px_rgba(120,200,255,0.65),inset_0_-10px_22px_rgba(0,0,0,0.3)]",
        status === "disconnecting" &&
          "bg-orb-accent opacity-50 [filter:blur(0.5px)]",
        status === "error" &&
          "animate-orb-shake bg-orb-danger shadow-[0_0_28px_rgba(255,94,94,0.55)]",
        className,
      )}
      aria-hidden
    />
  );
}
