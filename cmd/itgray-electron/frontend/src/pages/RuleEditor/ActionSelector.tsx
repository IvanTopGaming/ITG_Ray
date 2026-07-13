import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import type { Action } from "@/lib/rulesStore";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

const ACTION_DEFS: ReadonlyArray<{ value: Action; on: string }> = [
  {
    value: "proxy",
    on: "bg-sky-500 text-white shadow-[0_2px_12px_-2px_rgba(56,189,248,0.7)]",
  },
  {
    value: "direct",
    on: "bg-amber-500 text-amber-50 shadow-[0_2px_12px_-2px_rgba(245,158,11,0.6)]",
  },
  {
    value: "block",
    on: "bg-rose-500 text-rose-50 shadow-[0_2px_12px_-2px_rgba(244,63,94,0.7)]",
  },
];

function actionWrapperTint(action: Action): string {
  switch (action) {
    case "proxy":
      return "bg-sky-500/[0.04] border-sky-500/30 shadow-[0_0_40px_-10px_rgba(56,189,248,0.15)]";
    case "direct":
      return "bg-amber-500/[0.03] border-amber-500/25 shadow-[0_0_36px_-10px_rgba(245,158,11,0.12)]";
    case "block":
      return "bg-rose-500/[0.04] border-rose-500/30 shadow-[0_0_40px_-10px_rgba(244,63,94,0.18)]";
  }
}

export function ActionSelector({ value, onChange }: { value: Action; onChange: (v: Action) => void }) {
  const { t } = useTranslation();
  return (
    <div
      className={`flex w-full flex-col rounded-2xl border p-2 transition-all duration-300 ${actionWrapperTint(value)}`}
    >
      <div className="flex w-full gap-1 rounded-xl bg-black/40 p-1 shadow-inner">
        {ACTION_DEFS.map((a) => {
          const selected = value === a.value;
          return (
            <motion.button
              key={a.value}
              type="button"
              role="button"
              aria-pressed={selected}
              onClick={() => onChange(a.value)}
              whileTap={{ scale: 0.97 }}
              transition={{ duration: 0.18, ease: SNAP_EASE }}
              className={
                "relative flex-1 rounded-lg px-4 py-2.5 text-[13.5px] font-medium transition-all duration-200 " +
                (selected ? "text-white" : "text-white/50 hover:text-white/80 hover:bg-white/[0.03]")
              }
            >
              {selected && (
                <motion.span
                  layoutId="action-pill"
                  className={`absolute inset-0 rounded-lg ${a.on}`}
                  transition={{ type: "spring", stiffness: 400, damping: 30 }}
                />
              )}
              <span className="relative z-10">{t(`ruleEditor.actions.${a.value}`)}</span>
            </motion.button>
          );
        })}
      </div>
    </div>
  );
}
