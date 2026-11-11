import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";
import { dashSetRoutingMode, useDash, type RoutingMode } from "@/lib/dashStore";

const MODES: RoutingMode[] = ["rule", "global", "direct"];

export function RoutingModeSwitch() {
  const { t } = useTranslation();
  const dash = useDash();
  const enabled = dash.status === "connected";
  return (
    <div role="group" aria-label={t("routingMode.label")} className="inline-flex rounded-lg bg-white/[0.05] p-0.5 text-[11.5px]">
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
              "rounded-md px-3 py-1 transition-colors",
              active ? "bg-gradient-to-br from-accent-start to-accent-mid font-semibold text-white" : "text-white/60 hover:text-white",
              !enabled && "cursor-not-allowed opacity-50",
            )}
          >
            {t(`routingMode.${m}`)}
          </button>
        );
      })}
    </div>
  );
}
