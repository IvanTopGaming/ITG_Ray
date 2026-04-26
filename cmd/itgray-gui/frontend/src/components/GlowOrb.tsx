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
  className?: string;
  ariaLabel?: string;
}

export function GlowOrb({
  status,
  size = 96,
  onClick,
  disabled = false,
  className,
  ariaLabel,
}: GlowOrbProps) {
  const visual = cn(
    "rounded-full transition-all duration-standard ease-snap",
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
  );

  if (!onClick) {
    return (
      <div
        aria-hidden
        style={{ width: size, height: size }}
        className={cn("shrink-0", visual, className)}
      />
    );
  }

  const interactive = !disabled;

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel ?? "Toggle connection"}
      style={{ width: size, height: size }}
      className={cn(
        "shrink-0 p-0 outline-none",
        visual,
        interactive
          ? "cursor-pointer hover:brightness-110 hover:saturate-125 active:brightness-90"
          : "cursor-not-allowed",
        // idle has no glow — brighten the dashed border on hover instead
        status === "idle" &&
          interactive &&
          "hover:border-accent-start/60 hover:bg-white/[0.06]",
        // focus ring for keyboard
        interactive &&
          "focus-visible:ring-2 focus-visible:ring-accent-start/70 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-1",
        className,
      )}
    />
  );
}
