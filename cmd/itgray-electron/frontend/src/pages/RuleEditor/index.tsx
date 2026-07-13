import { useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import { ChevronLeft } from "lucide-react";
import { motion, type Variants } from "framer-motion";
import { useRules, rulesAddRule, rulesEditRule, rulesMoveRule, type RuleView, type GroupView } from "@/lib/rulesStore";
import { ConfirmDialog } from "@/components/controls/ConfirmDialog";
import {
  CONDITION_TYPES,
  EMPTY_DRAFT,
  hasAnyCondition,
  type ConditionType,
} from "./types";
import { RuleIdentityCard } from "./RuleIdentityCard";
import { ConditionCard } from "./ConditionCard";
import { AndConnector, ThenConnector } from "./connectors";
import { AddConditionPicker } from "./AddConditionPicker";
import { ActionSelector } from "./ActionSelector";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

const pageVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { delayChildren: 0.05, staggerChildren: 0.04 },
  },
};

const sectionVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.24, ease: SNAP_EASE } },
};

type CreateState = { mode: "create"; groupId: string };

function isCreateState(s: unknown): s is CreateState {
  return !!s && typeof s === "object" && (s as { mode?: unknown }).mode === "create"
    && typeof (s as { groupId?: unknown }).groupId === "string";
}

function initialVisibleTypes(rule: RuleView | null): Set<ConditionType> {
  const set = new Set<ConditionType>();
  if (!rule) return set;
  for (const c of CONDITION_TYPES) {
    const arr = rule.conditions[c.key];
    if (Array.isArray(arr) && arr.length > 0) set.add(c.key);
  }
  return set;
}

export function RuleEditor() {
  const { t } = useTranslation();
  const { ruleId = "" } = useParams<{ ruleId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
  const { groups } = useRules();

  const isCreate = ruleId === "new";
  const createState = isCreateState(location.state) ? location.state : null;

  const initial = useMemo(
    () => isCreate
      ? { rule: { id: "", ...EMPTY_DRAFT } as RuleView, groupId: createState?.groupId ?? "" }
      : findRule(groups, ruleId),
    [groups, ruleId, isCreate, createState?.groupId],
  );
  const [draft, setDraft] = useState<RuleView | null>(initial.rule);
  const [groupId, setGroupId] = useState<string>(initial.groupId);
  const [visibleTypes, setVisibleTypes] = useState<Set<ConditionType>>(() => initialVisibleTypes(initial.rule));
  const [justAdded, setJustAdded] = useState<ConditionType | null>(null);
  const initialSnapshot = useRef(JSON.stringify({ rule: initial.rule, groupId: initial.groupId }));
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  if (!draft) {
    return (
      <motion.div
        className="flex flex-col gap-3"
        initial={{ opacity: 0, y: 4 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.24, ease: SNAP_EASE }}
      >
        <motion.button
          onClick={() => navigate("/routing")}
          whileHover={{ x: -2 }}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="self-start text-[12.5px] text-white/70 hover:text-white/95"
        >
          ← {t('ruleEditor.back')}
        </motion.button>
        <p className="text-[14px] text-white/70">{t('ruleEditor.ruleNotFound')}</p>
      </motion.div>
    );
  }

  const conditionsEmpty = !hasAnyCondition(draft);

  async function handleSave() {
    if (!draft) return;
    setSaveError(null);
    if (!hasAnyCondition(draft)) {
      setSaveError(t('ruleEditor.errAddCondition'));
      return;
    }
    try {
      if (isCreate) {
        const { id: _id, ...rest } = draft;
        void _id;
        await rulesAddRule(groupId, rest);
      } else {
        if (groupId !== initial.groupId) {
          await rulesMoveRule(draft.id, groupId);
        }
        await rulesEditRule(draft);
      }
      navigate("/routing");
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : String(err));
    }
  }

  const currentSerialized = JSON.stringify({ rule: draft, groupId });
  const dirty = initialSnapshot.current !== currentSerialized;

  function handleBack() {
    if (isCreate) {
      navigate("/routing");
      return;
    }
    if (dirty) setConfirmDiscard(true);
    else navigate("/routing");
  }

  function addConditionType(key: ConditionType) {
    setJustAdded(key);
    setVisibleTypes((prev) => {
      const next = new Set(prev);
      next.add(key);
      return next;
    });
    setDraft((prev) => {
      if (!prev) return prev;
      const existing = prev.conditions[key];
      if (Array.isArray(existing)) return prev;
      return {
        ...prev,
        conditions: { ...prev.conditions, [key]: [] as never },
      };
    });
  }

  function removeConditionType(key: ConditionType) {
    setVisibleTypes((prev) => {
      const next = new Set(prev);
      next.delete(key);
      return next;
    });
    setDraft((prev) => {
      if (!prev) return prev;
      const nextConditions = { ...prev.conditions };
      delete nextConditions[key];
      return { ...prev, conditions: nextConditions };
    });
  }

  const userGroups = groups.filter((g) => !g.locked).map((g) => ({ id: g.id, name: g.name }));
  const visibleConditionList = CONDITION_TYPES.filter((c) => visibleTypes.has(c.key));

  return (
    <motion.div
      className="mx-auto flex w-full max-w-[640px] flex-col gap-4"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      <motion.header variants={sectionVariants} className="flex items-center justify-between pb-1">
        <motion.button
          onClick={handleBack}
          whileHover={{ x: -2 }}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="group flex items-center gap-2 rounded-full border border-white/0 bg-white/0 px-3 py-1.5 text-[13px] font-medium text-white/60 transition-all hover:border-white/10 hover:bg-white/[0.03] hover:text-white/95"
        >
          <ChevronLeft className="h-4 w-4 transition-transform group-hover:-translate-x-0.5" /> {t('ruleEditor.back')}
        </motion.button>
        <div className="flex items-center gap-3">
          {conditionsEmpty && (
            <span className="text-[11.5px] font-medium text-amber-400/70">{t('ruleEditor.addConditionToSave')}</span>
          )}
          <motion.button
            onClick={handleSave}
            disabled={conditionsEmpty}
            whileHover={conditionsEmpty ? undefined : { scale: 1.03 }}
            whileTap={conditionsEmpty ? undefined : { scale: 0.96 }}
            transition={{ duration: 0.18, ease: SNAP_EASE }}
            className={
              conditionsEmpty
                ? "rounded-full bg-white/[0.06] px-5 py-2 text-[13px] font-semibold text-white/40 cursor-not-allowed"
                : "rounded-full bg-sky-500/80 px-5 py-2 text-[13px] font-semibold text-white shadow-[0_4px_20px_-4px_rgba(56,189,248,0.6)] hover:bg-sky-400"
            }
          >
            {isCreate ? t('ruleEditor.createRule') : t('ruleEditor.saveChanges')}
          </motion.button>
        </div>
      </motion.header>

      {saveError && (
        <motion.div
          variants={sectionVariants}
          role="alert"
          className="rounded-md border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-[12px] text-rose-200"
        >
          {saveError}
        </motion.div>
      )}

      <motion.div variants={sectionVariants}>
        <RuleIdentityCard
          name={draft.name}
          enabled={draft.enabled}
          groupId={groupId}
          groups={userGroups}
          onName={(name) => setDraft({ ...draft, name })}
          onEnabled={(enabled) => setDraft({ ...draft, enabled })}
          onGroup={setGroupId}
        />
      </motion.div>

      <motion.section variants={sectionVariants} className="mt-2 flex flex-col gap-1">
        <span className="mb-2 pl-1 text-[10.5px] font-extrabold uppercase tracking-[0.14em] text-amber-400">
          ◆ {t('ruleEditor.matchAllHeader')}
        </span>
        {visibleConditionList.map((c, i) => (
          <div key={c.key} className="flex flex-col gap-1">
            {i > 0 && <AndConnector />}
            <ConditionCard
              def={c}
              draft={draft}
              setDraft={setDraft}
              onRemove={() => removeConditionType(c.key)}
              startExpanded={justAdded === c.key}
            />
          </div>
        ))}
        <div className={visibleConditionList.length > 0 ? "mt-2" : undefined}>
          <AddConditionPicker
            present={visibleTypes}
            onAdd={addConditionType}
            prominent={visibleConditionList.length === 0}
          />
        </div>
      </motion.section>

      <motion.section variants={sectionVariants} className="flex flex-col">
        <ThenConnector />
        <ActionSelector value={draft.action} onChange={(action) => setDraft({ ...draft, action })} />
      </motion.section>

      <ConfirmDialog
        open={confirmDiscard}
        title={t('ruleEditor.discardTitle')}
        description={t('ruleEditor.discardDescription')}
        confirmLabel={t('ruleEditor.discardConfirm')}
        confirmVariant="danger"
        onClose={() => setConfirmDiscard(false)}
        onConfirm={() => navigate("/routing")}
      />
    </motion.div>
  );
}

function findRule(groups: GroupView[], ruleId: string): { rule: RuleView | null; groupId: string } {
  for (const g of groups) {
    for (const r of g.rules) {
      if (r.id === ruleId) return { rule: r, groupId: g.id };
    }
  }
  return { rule: null, groupId: "" };
}
