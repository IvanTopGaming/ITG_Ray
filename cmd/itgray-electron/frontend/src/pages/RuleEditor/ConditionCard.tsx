import { useState } from "react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { motion } from "framer-motion";
import type { RuleView } from "@/lib/rulesStore";
import type { ConditionDef } from "./types";
import { ConditionSummary } from "./ConditionSummary";
import { ConditionBody } from "./conditionBodies";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

export function ConditionCard({
  def,
  draft,
  setDraft,
  onRemove,
  startExpanded,
}: {
  def: ConditionDef;
  draft: RuleView;
  setDraft: React.Dispatch<React.SetStateAction<RuleView | null>>;
  onRemove: () => void;
  startExpanded?: boolean;
}) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(startExpanded ?? false);
  const label = def.key === "domains" ? t('ruleEditor.conditions.domainsCard') : t(`ruleEditor.conditions.${def.key}`);

  return (
    <motion.div
      layout
      transition={{ layout: { type: "spring", stiffness: 380, damping: 34, mass: 0.85 } }}
      className="relative flex flex-col gap-3 rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4 shadow-sm transition-colors duration-200 hover:border-white/[0.12] hover:bg-white/[0.03]"
    >
      <div className="absolute bottom-0 left-0 top-0 w-1 rounded-l-2xl bg-gradient-to-b from-sky-400/40 to-transparent opacity-40" />
      <div className="flex items-center justify-between gap-2 pl-2">
        <h3 className="flex items-center gap-2.5 text-[14px] font-medium text-white/90">
          <span aria-hidden className="flex h-7 w-7 items-center justify-center rounded-md bg-white/[0.06] text-[14px] shadow-inner">{def.icon}</span>
          <span>{label}</span>
        </h3>
        <div className="flex items-center gap-1">
          <motion.button
            type="button"
            onClick={() => setExpanded((v) => !v)}
            aria-label={t('ruleEditor.editCard', { label })}
            aria-expanded={expanded}
            whileHover={{ scale: 1.1 }}
            whileTap={{ scale: 0.85 }}
            transition={{ duration: 0.18, ease: SNAP_EASE }}
            className="flex h-7 w-7 items-center justify-center rounded-full text-white/40 hover:bg-white/[0.08] hover:text-white/80"
          >
            ✎
          </motion.button>
          <motion.button
            type="button"
            onClick={onRemove}
            aria-label={t('ruleEditor.removeCard', { label })}
            whileHover={{ scale: 1.15 }}
            whileTap={{ scale: 0.85 }}
            transition={{ duration: 0.18, ease: SNAP_EASE }}
            className="flex h-7 w-7 items-center justify-center rounded-full text-white/40 hover:bg-rose-500/15 hover:text-rose-300"
          >
            ✕
          </motion.button>
        </div>
      </div>
      <motion.div layout className="pl-2">
        {expanded ? (
          <ConditionBody type={def.key} draft={draft} setDraft={setDraft} />
        ) : (
          <ConditionSummary conditions={draft.conditions} chipsFor={def.key} />
        )}
      </motion.div>
    </motion.div>
  );
}
