import { useId } from "react";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";
import { dashSetRoutingMode, useDash, type RoutingMode } from "@/lib/dashStore";

const MODES: RoutingMode[] = ["rule", "global", "direct"];

export function RoutingModeSwitch() {
  const { t } = useTranslation();
  const dash = useDash();
  const layoutId = useId();
  const enabled = dash.status === "connected";
  return (
    <div
      role="group"
      aria-label={t("routingMode.label")}
      title={enabled ? undefined : t("routingMode.needsConnection")}
      className={cn(
        "inline-flex h-8 shrink-0 items-center gap-0.5 whitespace-nowrap rounded-md border border-white/[0.10] bg-white/[0.06] p-[3px]",
        !enabled && "opacity-50",
      )}
    >
      {MODES.map((m) => {
        const active = dash.routingMode === m;
        return (
          <button
            key={m}
            type="button"
            disabled={!enabled}
            aria-pressed={active}
            onClick={() => void dashSetRoutingMode(m).catch(() => {})}
            className={cn(
              "relative h-full whitespace-nowrap rounded-[5px] px-3 text-[12px] font-medium transition-colors",
              active ? "text-white" : "text-white/55 hover:text-white/85",
              !enabled && "cursor-not-allowed hover:text-white/55",
            )}
          >
            {active && (
              <motion.span
                layoutId={`routing-mode-pill-${layoutId}`}
                className="absolute inset-0 rounded-[5px] bg-white/[0.14] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.10)]"
                transition={{ type: "spring", stiffness: 500, damping: 32 }}
              />
            )}
            <span className="relative">{t(`routingMode.${m}`)}</span>
          </button>
        );
      })}
    </div>
  );
}
