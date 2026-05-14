import { useEffect, useMemo, useRef, useState } from "react";
import type React from "react";
import { createPortal } from "react-dom";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import { ChevronLeft } from "lucide-react";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import { useRules, rulesAddRule, rulesEditRule, rulesMoveRule, type RuleView, type GroupView, type DomainMatcher, type PortSpec } from "@/lib/rulesStore";
import { Toggle } from "@/components/controls/Toggle";
import { ConfirmDialog } from "@/components/controls/ConfirmDialog";
import { Dropdown } from "@/components/controls/Dropdown";
import { Segmented } from "@/components/controls/Segmented";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];
const SMOOTH_EASE: [number, number, number, number] = [0.32, 0.72, 0, 1];
const LAYOUT_SPRING = { type: "spring", stiffness: 380, damping: 34, mass: 0.85 } as const;

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

const chipVariants: Variants = {
  hidden: { opacity: 0, scale: 0.88 },
  show: { opacity: 1, scale: 1, transition: { duration: 0.18, ease: SNAP_EASE } },
  exit: { opacity: 0, scale: 0.88, transition: { duration: 0.14, ease: SNAP_EASE } },
};

type ConditionType = "domains" | "ip_cidrs" | "geo" | "ports" | "processes" | "protocols";

type ConditionDef = {
  key: ConditionType;
  icon: string;
  label: string;
};

const CONDITION_TYPES: ConditionDef[] = [
  { key: "domains", icon: "\u{1F310}", label: "Domain matcher" },
  { key: "ip_cidrs", icon: "\u{1F522}", label: "IP CIDRs" },
  { key: "geo", icon: "\u{1F4CD}", label: "Geo" },
  { key: "ports", icon: "\u{1F6AA}", label: "Ports" },
  { key: "processes", icon: "⚙️", label: "Processes" },
  { key: "protocols", icon: "\u{1F4E1}", label: "Protocols" },
];

type CreateState = { mode: "create"; groupId: string };

function isCreateState(s: unknown): s is CreateState {
  return !!s && typeof s === "object" && (s as { mode?: unknown }).mode === "create"
    && typeof (s as { groupId?: unknown }).groupId === "string";
}

const EMPTY_DRAFT: Omit<RuleView, "id"> = {
  name: "",
  enabled: true,
  action: "proxy",
  conditions: {},
};

function initialVisibleTypes(rule: RuleView | null): Set<ConditionType> {
  const set = new Set<ConditionType>();
  if (!rule) return set;
  for (const c of CONDITION_TYPES) {
    const arr = rule.conditions[c.key];
    if (Array.isArray(arr) && arr.length > 0) set.add(c.key);
  }
  return set;
}

function conditionCount(rule: RuleView, key: ConditionType): number {
  const arr = rule.conditions[key];
  return Array.isArray(arr) ? arr.length : 0;
}

export function RuleEditor() {
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
          ← Routing
        </motion.button>
        <p className="text-[14px] text-white/70">Rule not found.</p>
      </motion.div>
    );
  }

  function hasAnyCondition(rule: RuleView): boolean {
    for (const c of CONDITION_TYPES) {
      const arr = rule.conditions[c.key];
      if (Array.isArray(arr) && arr.length > 0) return true;
    }
    return false;
  }

  const conditionsEmpty = !hasAnyCondition(draft);

  async function handleSave() {
    if (!draft) return;
    setSaveError(null);
    if (!hasAnyCondition(draft)) {
      setSaveError("Add at least one condition before saving.");
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
      // Create-mode drafts only live in memory — backing out is free.
      navigate("/routing");
      return;
    }
    if (dirty) setConfirmDiscard(true);
    else navigate("/routing");
  }

  function addConditionType(key: ConditionType) {
    setVisibleTypes((prev) => {
      const next = new Set(prev);
      next.add(key);
      return next;
    });
    setDraft((prev) => {
      if (!prev) return prev;
      const existing = prev.conditions[key];
      if (Array.isArray(existing)) return prev; // already present
      // Initialize as empty array of correct type (we cast loosely; per-section
      // setters keep types honest going forward).
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

  const userGroups = groups.filter((g) => !g.locked);
  const availableTypes = CONDITION_TYPES.filter((c) => !visibleTypes.has(c.key));
  // Render cards in the canonical order from CONDITION_TYPES so the layout
  // stays stable regardless of add/remove sequence.
  const visibleConditionList = CONDITION_TYPES.filter((c) => visibleTypes.has(c.key));

  return (
    <motion.div
      className="flex flex-col gap-5"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      {/* Header — back link + Save */}
      <motion.header variants={sectionVariants} className="flex items-center justify-between pb-2">
        <motion.button
          onClick={handleBack}
          whileHover={{ x: -2 }}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="group flex items-center gap-2 rounded-full border border-white/0 bg-white/0 px-3 py-1.5 text-[13px] font-medium text-white/60 transition-all hover:border-white/10 hover:bg-white/[0.03] hover:text-white/95"
        >
          <ChevronLeft className="h-4 w-4 transition-transform group-hover:-translate-x-0.5" /> Routing
        </motion.button>
        <div className="flex items-center gap-3">
          {conditionsEmpty && (
            <span className="text-[11.5px] font-medium text-amber-400/70">Add a condition to save</span>
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
            {isCreate ? "Create rule" : "Save changes"}
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

      {/* Rule basics */}
      <motion.div variants={sectionVariants} className="relative overflow-hidden rounded-2xl border border-white/[0.08] bg-white/[0.02] p-5 shadow-sm transition-colors hover:border-white/[0.12] hover:bg-white/[0.03]">
        <div className="absolute -right-12 -top-12 h-32 w-32 rounded-full bg-sky-500/10 blur-[40px] pointer-events-none" />
        <div className="relative flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4">
            <input
              aria-label="Name"
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              placeholder="Rule name"
              className="min-w-0 flex-1 border-0 bg-transparent p-0 text-[20px] font-medium text-white/95 outline-none placeholder:text-white/20 focus:ring-0"
            />
            <div className="flex shrink-0 items-center gap-2 rounded-full border border-white/10 bg-black/20 px-3 py-1.5 shadow-inner">
              <span className="text-[10.5px] font-medium uppercase tracking-[0.12em] text-white/50">Enabled</span>
              <Toggle value={draft.enabled} aria-label="Enabled" onChange={(v) => setDraft({ ...draft, enabled: v })} />
            </div>
          </div>
          {!isCreate && userGroups.length > 0 && (
            <div className="flex items-center gap-3 border-t border-white/[0.06] pt-4">
              <span className="text-[11px] font-medium uppercase tracking-[0.12em] text-white/40">Group</span>
              <select
                aria-label="Group"
                value={groupId}
                onChange={(e) => setGroupId(e.target.value)}
                className="rounded-full border border-white/10 bg-black/20 px-3 py-1.5 text-[12px] font-medium text-white/85 outline-none transition-colors duration-200 hover:bg-black/40 focus:border-sky-400/40"
              >
                {userGroups.map((g) => <option key={g.id} value={g.id} className="bg-[#1c1f2a]">{g.name}</option>)}
              </select>
            </div>
          )}
        </div>
      </motion.div>

      {/* IF (conditions) */}
      <motion.section
        layout
        variants={sectionVariants}
        transition={{ layout: LAYOUT_SPRING }}
        className="mt-2 flex flex-col gap-3"
      >
        <span className="pl-1 text-[11px] font-medium uppercase tracking-[0.18em] text-white/50">
          If matches all of
        </span>
        {visibleConditionList.length === 0 ? (
          <div key="if-empty" className="flex flex-col items-center justify-center gap-4 rounded-2xl border border-dashed border-white/[0.15] bg-white/[0.01] px-4 py-12 text-center hover:bg-white/[0.02]">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-white/[0.04]">
              <span className="text-xl opacity-60">🔮</span>
            </div>
            <div className="flex flex-col gap-1">
              <p className="text-[14px] font-medium text-white/80">No conditions specified</p>
              <p className="text-[12.5px] text-white/40">Add rules to define what traffic should be routed.</p>
            </div>
            <div className="mt-2">
              <AddConditionButton
                availableTypes={availableTypes}
                onPick={addConditionType}
                prominent
              />
            </div>
          </div>
        ) : (
          <div key="if-list" className="relative flex flex-col gap-3">
            <AnimatePresence initial={false} mode="popLayout">
              {visibleConditionList.map((c) => {
                const count = conditionCount(draft, c.key);
                return (
                  <motion.div
                    key={c.key}
                    layout
                    variants={sectionVariants}
                    initial="hidden"
                    animate="show"
                    exit={{
                      opacity: 0,
                      scale: 0.94,
                      y: -6,
                      pointerEvents: "none",
                      transition: { duration: 0.22, ease: SMOOTH_EASE },
                    }}
                    transition={{ layout: LAYOUT_SPRING }}
                    className="relative flex flex-col gap-3 rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4 shadow-sm transition-colors duration-200 hover:border-white/[0.12] hover:bg-white/[0.03]"
                  >
                    <div className="absolute bottom-0 left-0 top-0 w-1 rounded-l-2xl bg-gradient-to-b from-sky-400/40 to-transparent opacity-40" />
                    <div className="flex items-center justify-between gap-2 pl-2">
                      <h3 className="flex items-center gap-2.5 text-[14px] font-medium text-white/90">
                        <span aria-hidden className="flex h-7 w-7 items-center justify-center rounded-md bg-white/[0.06] text-[14px] shadow-inner">{c.icon}</span>
                        <span>{c.label === "Domain matcher" ? "Domains" : c.label}</span>
                        {count > 0 && (
                          <span className="rounded-full bg-white/10 px-2 py-0.5 text-[10.5px] font-medium text-white/60">
                            {count}
                          </span>
                        )}
                      </h3>
                      <motion.button
                        type="button"
                        onClick={() => removeConditionType(c.key)}
                        aria-label={`Remove ${c.label} card`}
                        whileHover={{ scale: 1.15 }}
                        whileTap={{ scale: 0.85 }}
                        transition={{ duration: 0.18, ease: SNAP_EASE }}
                        className="flex h-7 w-7 items-center justify-center rounded-full text-white/40 hover:bg-rose-500/15 hover:text-rose-300"
                      >
                        ✕
                      </motion.button>
                    </div>
                    <div className="pl-2">
                      <ConditionCardBody
                        type={c.key}
                        draft={draft}
                        setDraft={setDraft}
                      />
                    </div>
                  </motion.div>
                );
              })}
            </AnimatePresence>
            <AnimatePresence initial={false} mode="popLayout">
              {availableTypes.length > 0 && (
                <motion.div
                  key="add-more"
                  layout
                  initial={{ opacity: 0, y: -6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6, pointerEvents: "none" }}
                  transition={{ duration: 0.22, ease: SMOOTH_EASE, layout: LAYOUT_SPRING }}
                  className="flex"
                >
                  <AddConditionButton
                    availableTypes={availableTypes}
                    onPick={addConditionType}
                  />
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        )}
      </motion.section>

      {/* THEN (action) */}
      <motion.section
        layout
        variants={sectionVariants}
        transition={{ layout: LAYOUT_SPRING }}
        className="mt-4 flex flex-col gap-3"
      >
        <span className="pl-1 text-[11px] font-medium uppercase tracking-[0.18em] text-white/50">
          Then action
        </span>
        <ActionPicker
          value={draft.action}
          onChange={(v) => setDraft({ ...draft, action: v })}
        />
      </motion.section>

      <ConfirmDialog
        open={confirmDiscard}
        title="Discard changes?"
        description="You have unsaved changes. Leave anyway?"
        confirmLabel="Discard"
        confirmVariant="danger"
        onClose={() => setConfirmDiscard(false)}
        onConfirm={() => navigate("/routing")}
      />
    </motion.div>
  );
}

// ----- ActionPicker: custom 3-button row where each selected pill matches
// the wrapper hue. The wrapper itself carries a soft glow (shadow) in the same
// color so "Block" feels distinctly red, "Direct" warmly amber, "Proxy" cool sky.
// Tests rely on:
//   - role="button" with name=/^block$/i and aria-pressed reflecting selection
//   - a rounded-2xl ancestor whose className contains the hue (e.g. "rose").

type ActionValue = RuleView["action"];

const ACTION_DEFS: ReadonlyArray<{ value: ActionValue; label: string; on: string; ring: string }> = [
  {
    value: "proxy",
    label: "Proxy",
    on: "bg-sky-500 text-white shadow-[0_2px_12px_-2px_rgba(56,189,248,0.7)]",
    ring: "ring-sky-300/30",
  },
  {
    value: "direct",
    label: "Direct",
    on: "bg-amber-500 text-amber-50 shadow-[0_2px_12px_-2px_rgba(245,158,11,0.6)]",
    ring: "ring-amber-300/30",
  },
  {
    value: "block",
    label: "Block",
    on: "bg-rose-500 text-rose-50 shadow-[0_2px_12px_-2px_rgba(244,63,94,0.7)]",
    ring: "ring-rose-300/30",
  },
];

function actionWrapperTint(action: ActionValue): string {
  switch (action) {
    case "proxy":
      return "bg-sky-500/[0.04] border-sky-500/30 shadow-[0_0_40px_-10px_rgba(56,189,248,0.15)]";
    case "direct":
      return "bg-amber-500/[0.03] border-amber-500/25 shadow-[0_0_36px_-10px_rgba(245,158,11,0.12)]";
    case "block":
      return "bg-rose-500/[0.04] border-rose-500/30 shadow-[0_0_40px_-10px_rgba(244,63,94,0.18)]";
  }
}

function ActionPicker({ value, onChange }: { value: ActionValue; onChange: (v: ActionValue) => void }) {
  return (
    <div
      className={`flex flex-col rounded-2xl border p-2 transition-all duration-300 ${actionWrapperTint(value)}`}
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
              <span className="relative z-10">{a.label}</span>
            </motion.button>
          );
        })}
      </div>
    </div>
  );
}

// ----- AddConditionButton (custom popover dropdown) -----

function AddConditionButton({
  availableTypes,
  onPick,
  prominent,
}: {
  availableTypes: ConditionDef[];
  onPick: (k: ConditionType) => void;
  prominent?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [coords, setCoords] = useState<{ left: number; top: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const popRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open || !btnRef.current) return;
    const r = btnRef.current.getBoundingClientRect();
    setCoords({ left: r.left, top: r.bottom + 4 });
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      const t = e.target as Node;
      if (btnRef.current?.contains(t) || popRef.current?.contains(t)) return;
      setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", esc);
    };
  }, [open]);

  if (availableTypes.length === 0) return null;

  return (
    <>
      <motion.button
        ref={btnRef}
        type="button"
        onClick={() => setOpen((o) => !o)}
        whileHover={{ scale: 1.02 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        aria-haspopup="menu"
        aria-expanded={open}
        className={
          prominent
            ? "flex items-center gap-2 rounded-full border border-sky-400/30 bg-sky-500/15 px-4 py-2 text-[13px] font-medium text-sky-100 shadow-[0_4px_20px_-4px_rgba(56,189,248,0.3)] transition-all hover:bg-sky-500/25"
            : "group flex items-center gap-2 self-start rounded-full border border-white/[0.08] bg-white/[0.03] px-3.5 py-1.5 text-[12.5px] font-medium text-white/70 transition-all hover:border-white/[0.15] hover:bg-white/[0.08] hover:text-white/95"
        }
      >
        <span className="text-lg leading-none opacity-70 group-hover:opacity-100">{open ? "×" : "+"}</span>
        <span>Add condition</span>
      </motion.button>
      {open && coords && createPortal(
        <motion.div
          ref={popRef}
          role="menu"
          initial={{ opacity: 0, y: -4, scale: 0.96, filter: "blur(4px)" }}
          animate={{ opacity: 1, y: 0, scale: 1, filter: "blur(0px)" }}
          transition={{ duration: 0.16, ease: SNAP_EASE }}
          style={{ position: "fixed", left: coords.left, top: coords.top, zIndex: 1000 }}
          className="flex w-56 flex-col gap-0.5 rounded-xl border border-white/[0.12] bg-[#1a1c24]/95 p-1.5 shadow-[0_16px_40px_-8px_rgba(0,0,0,0.6)] backdrop-blur-xl"
        >
          {availableTypes.map((t) => (
            <button
              key={t.key}
              type="button"
              role="menuitem"
              onClick={() => { onPick(t.key); setOpen(false); }}
              className="group flex items-center gap-3 rounded-lg px-2.5 py-2 text-left text-[13px] font-medium text-white/70 transition-colors duration-150 hover:bg-white/[0.06] hover:text-white/95"
            >
              <span aria-hidden className="flex h-6 w-6 items-center justify-center rounded-md bg-white/[0.04] text-[14px] shadow-sm transition-colors group-hover:bg-white/[0.08]">{t.icon}</span>
              <span>{t.label}</span>
            </button>
          ))}
        </motion.div>,
        document.body,
      )}
    </>
  );
}

// ----- Chip primitive -----

function Chip({
  children,
  onRemove,
  removeLabel,
}: {
  children: React.ReactNode;
  onRemove?: () => void;
  removeLabel?: string;
}) {
  return (
    <motion.span
      layout
      variants={chipVariants}
      initial="hidden"
      animate="show"
      exit="exit"
      whileHover={{ y: -1 }}
      transition={{ duration: 0.18, ease: SNAP_EASE, layout: LAYOUT_SPRING }}
      className="group inline-flex items-center gap-1 rounded-full border border-white/[0.08] bg-white/[0.05] px-2.5 py-1 text-[12px] text-white/90 transition-colors duration-200 hover:border-white/[0.16] hover:bg-white/[0.10]"
    >
      {children}
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          aria-label={removeLabel ?? "Remove"}
          className="ml-0.5 rounded-full p-0.5 text-white/40 transition-colors duration-150 hover:bg-rose-500/15 hover:text-rose-300"
        >
          ✕
        </button>
      )}
    </motion.span>
  );
}

// ----- ConditionCardBody — switch by type -----

function ConditionCardBody({
  type,
  draft,
  setDraft,
}: {
  type: ConditionType;
  draft: RuleView;
  setDraft: React.Dispatch<React.SetStateAction<RuleView | null>>;
}) {
  function patch<K extends ConditionType>(key: K, value: NonNullable<RuleView["conditions"][K]>) {
    setDraft((prev) => prev ? { ...prev, conditions: { ...prev.conditions, [key]: value } } : prev);
  }
  switch (type) {
    case "domains":
      return (
        <DomainsBody
          value={draft.conditions.domains ?? []}
          onChange={(v) => patch("domains", v)}
        />
      );
    case "ip_cidrs":
      return (
        <CidrsBody
          value={draft.conditions.ip_cidrs ?? []}
          onChange={(v) => patch("ip_cidrs", v)}
        />
      );
    case "geo":
      return (
        <GeoBody
          value={draft.conditions.geo ?? []}
          onChange={(v) => patch("geo", v)}
        />
      );
    case "ports":
      return (
        <PortsBody
          value={draft.conditions.ports ?? []}
          onChange={(v) => patch("ports", v)}
        />
      );
    case "processes":
      return (
        <ProcessesBody
          value={draft.conditions.processes ?? []}
          onChange={(v) => patch("processes", v)}
        />
      );
    case "protocols":
      return (
        <ProtocolsBody
          value={draft.conditions.protocols ?? []}
          onChange={(v) => patch("protocols", v)}
        />
      );
  }
}

// ----- Shared input styling -----

const ADD_INPUT_CLASSES =
  "flex-1 rounded-md border border-white/[0.08] bg-white/[0.02] px-2.5 py-1.5 text-[12.5px] text-white/90 outline-none transition-colors duration-200 placeholder:text-white/30 focus:border-sky-400/45 focus:bg-white/[0.04]";

// ----- Domains -----

const DOMAIN_KINDS: DomainMatcher["kind"][] = ["exact", "suffix", "keyword", "regex"];
const DOMAIN_OPTIONS = DOMAIN_KINDS.map(k => ({ value: k, label: k }));

function DomainsBody({ value, onChange }: { value: DomainMatcher[]; onChange: (next: DomainMatcher[]) => void }) {
  const [addKind, setAddKind] = useState<DomainMatcher["kind"]>("suffix");
  const [addValue, setAddValue] = useState("");
  function commit() {
    const v = addValue.trim();
    if (!v) return;
    onChange([...value, { kind: addKind, value: v }]);
    setAddValue("");
  }

  return (
    <div className="flex flex-col gap-2.5">
      {value.length > 0 && (
        <div className="relative flex flex-wrap items-center gap-1.5">
          <AnimatePresence initial={false} mode="popLayout">
            {value.map((m, i) => (
              <Chip
                key={`${i}-${m.kind}-${m.value}`}
                onRemove={() => onChange(value.filter((_, j) => j !== i))}
                removeLabel={`Remove domain matcher ${i + 1}`}
              >
                <div className="w-[88px] shrink-0">
                  <Dropdown
                    value={m.kind}
                    onChange={(v) => onChange(value.map((x, j) => j === i ? { ...x, kind: v as DomainMatcher["kind"] } : x))}
                    options={DOMAIN_OPTIONS}
                    triggerClassName="border-transparent bg-transparent px-1 py-0.5 text-[11px] uppercase tracking-wider text-white/45 hover:border-white/10 hover:bg-white/[0.06] hover:text-white/80"
                    menuClassName="w-32"
                    ariaLabel={`Domain matcher kind ${i + 1}`}
                  />
                </div>
                <span className="text-white/25">·</span>
                <span className="text-white/95">{m.value}</span>
              </Chip>
            ))}
          </AnimatePresence>
        </div>
      )}
      <div className="flex items-center gap-2">
        <div className="w-32 shrink-0">
          <Dropdown
            value={addKind}
            onChange={(v) => setAddKind(v as DomainMatcher["kind"])}
            options={DOMAIN_OPTIONS}
          />
        </div>
        <input
          aria-label="Domain matcher value"
          value={addValue}
          onChange={(e) => setAddValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
          placeholder="example.com — press Enter to add"
          className={ADD_INPUT_CLASSES}
        />
      </div>
    </div>
  );
}

// ----- IP CIDRs -----

function CidrsBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  const [addValue, setAddValue] = useState("");
  function commit() {
    const v = addValue.trim();
    if (!v) return;
    onChange([...value, v]);
    setAddValue("");
  }
  return (
    <div className="flex flex-col gap-2.5">
      {value.length > 0 && (
        <div className="relative flex flex-wrap items-center gap-1.5">
          <AnimatePresence initial={false} mode="popLayout">
            {value.map((cidr, i) => (
              <Chip
                key={`${i}-${cidr}`}
                onRemove={() => onChange(value.filter((_, j) => j !== i))}
                removeLabel={`Remove CIDR ${i + 1}`}
              >
                <span>{cidr || <span className="text-white/40">(empty)</span>}</span>
              </Chip>
            ))}
          </AnimatePresence>
        </div>
      )}
      <input
        aria-label="CIDR value"
        value={addValue}
        onChange={(e) => setAddValue(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
        placeholder="10.0.0.0/8 or 1.2.3.4 — press Enter to add"
        className={ADD_INPUT_CLASSES}
      />
    </div>
  );
}

// ----- Geo -----

const GEO_PREFIXES = ["geosite", "geoip"] as const;
type GeoPrefix = typeof GEO_PREFIXES[number];
const GEO_OPTIONS = GEO_PREFIXES.map(p => ({ value: p, label: p }));

function splitGeo(entry: string): { prefix: GeoPrefix; rest: string } {
  const idx = entry.indexOf(":");
  if (idx < 0) return { prefix: "geosite", rest: entry };
  const p = entry.slice(0, idx);
  const prefix: GeoPrefix = p === "geoip" ? "geoip" : "geosite";
  return { prefix, rest: entry.slice(idx + 1) };
}

function GeoBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  const [addPrefix, setAddPrefix] = useState<GeoPrefix>("geosite");
  const [addValue, setAddValue] = useState("");
  function commit() {
    const v = addValue.trim();
    if (!v) return;
    onChange([...value, `${addPrefix}:${v}`]);
    setAddValue("");
  }
  return (
    <div className="flex flex-col gap-2.5">
      {value.length > 0 && (
        <div className="relative flex flex-wrap items-center gap-1.5">
          <AnimatePresence initial={false} mode="popLayout">
            {value.map((entry, i) => {
              const { prefix, rest } = splitGeo(entry);
              return (
                <Chip
                  key={`${i}-${entry}`}
                  onRemove={() => onChange(value.filter((_, j) => j !== i))}
                  removeLabel={`Remove geo ${i + 1}`}
                >
                  <div className="w-[88px] shrink-0">
                    <Dropdown
                      value={prefix}
                      onChange={(v) => onChange(value.map((x, j) => j === i ? `${v}:${splitGeo(x).rest}` : x))}
                      options={GEO_OPTIONS}
                      triggerClassName="border-transparent bg-transparent px-1 py-0.5 text-[11px] uppercase tracking-wider text-white/45 hover:border-white/10 hover:bg-white/[0.06] hover:text-white/80"
                      menuClassName="w-32"
                    />
                  </div>
                  <span className="text-white/25">·</span>
                  <span className="text-white/95">{rest}</span>
                </Chip>
              );
            })}
          </AnimatePresence>
        </div>
      )}
      <div className="flex items-center gap-2">
        <div className="w-28 shrink-0">
          <Dropdown
            value={addPrefix}
            onChange={(v) => setAddPrefix(v as GeoPrefix)}
            options={GEO_OPTIONS}
            ariaLabel="Geo prefix"
          />
        </div>
        <input
          aria-label="Geo value"
          value={addValue}
          onChange={(e) => setAddValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
          placeholder="cn, google, ru — press Enter to add"
          className={ADD_INPUT_CLASSES}
        />
      </div>
    </div>
  );
}

// ----- Ports -----

function portChipLabel(p: PortSpec): string {
  if (p.from !== undefined || p.to !== undefined) {
    return `${p.from ?? 0}–${p.to ?? 0}`;
  }
  return String(p.single ?? 0);
}

function PortsBody({ value, onChange }: { value: PortSpec[]; onChange: (next: PortSpec[]) => void }) {
  const [mode, setMode] = useState<"single" | "range">("single");
  const [single, setSingle] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  function commit() {
    if (mode === "single") {
      const n = Number(single);
      if (!Number.isFinite(n) || single === "") return;
      onChange([...value, { single: n }]);
      setSingle("");
    } else {
      if (from === "" || to === "") return;
      const f = Number(from);
      const t = Number(to);
      if (!Number.isFinite(f) || !Number.isFinite(t)) return;
      onChange([...value, { from: f, to: t }]);
      setFrom("");
      setTo("");
    }
  }

  const canAdd = mode === "single" ? single !== "" : (from !== "" && to !== "");

  return (
    <div className="flex flex-col gap-2.5">
      {value.length > 0 && (
        <div className="relative flex flex-wrap items-center gap-1.5">
          <AnimatePresence initial={false} mode="popLayout">
            {value.map((p, i) => (
              <Chip
                key={`${i}-${portChipLabel(p)}`}
                onRemove={() => onChange(value.filter((_, j) => j !== i))}
                removeLabel={`Remove port ${i + 1}`}
              >
                <span>{portChipLabel(p)}</span>
              </Chip>
            ))}
          </AnimatePresence>
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">
        {/* Small inline mode switch — single | range — matches the rest of the visual language. */}
        <Segmented
          value={mode}
          onChange={(v) => setMode(v as "single" | "range")}
          options={[
            { value: "single", label: "Single" },
            { value: "range", label: "Range" }
          ]}
        />
        {mode === "single" ? (
          <input
            aria-label="Port number"
            type="number"
            value={single}
            onChange={(e) => setSingle(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
            placeholder="443"
            className="w-28 rounded-md border border-white/[0.08] bg-white/[0.02] px-2.5 py-1.5 text-[12.5px] tabular-nums text-white/90 outline-none transition-colors duration-200 placeholder:text-white/30 focus:border-sky-400/45 focus:bg-white/[0.04]"
          />
        ) : (
          <>
            <input
              aria-label="Port from"
              type="number"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
              placeholder="8000"
              className="w-24 rounded-md border border-white/[0.08] bg-white/[0.02] px-2.5 py-1.5 text-[12.5px] tabular-nums text-white/90 outline-none transition-colors duration-200 placeholder:text-white/30 focus:border-sky-400/45 focus:bg-white/[0.04]"
            />
            <span className="text-white/40">→</span>
            <input
              aria-label="Port to"
              type="number"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
              placeholder="9000"
              className="w-24 rounded-md border border-white/[0.08] bg-white/[0.02] px-2.5 py-1.5 text-[12.5px] tabular-nums text-white/90 outline-none transition-colors duration-200 placeholder:text-white/30 focus:border-sky-400/45 focus:bg-white/[0.04]"
            />
          </>
        )}
        <AddChipButton onClick={commit} disabled={!canAdd} label="Add port" />
      </div>
    </div>
  );
}

// ----- Processes -----

function ProcessesBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  const [addValue, setAddValue] = useState("");
  function commit() {
    const v = addValue.trim();
    if (!v) return;
    onChange([...value, v]);
    setAddValue("");
  }
  return (
    <div className="flex flex-col gap-2.5">
      {value.length > 0 && (
        <div className="relative flex flex-wrap items-center gap-1.5">
          <AnimatePresence initial={false} mode="popLayout">
            {value.map((name, i) => (
              <Chip
                key={`${i}-${name}`}
                onRemove={() => onChange(value.filter((_, j) => j !== i))}
                removeLabel={`Remove process ${i + 1}`}
              >
                <span>{name || <span className="text-white/40">(empty)</span>}</span>
              </Chip>
            ))}
          </AnimatePresence>
        </div>
      )}
      <input
        aria-label="Process name"
        value={addValue}
        onChange={(e) => setAddValue(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
        placeholder="chrome.exe — press Enter to add"
        className={ADD_INPUT_CLASSES}
      />
    </div>
  );
}

// ----- Protocols (toggleable chips, no add input) -----

const PROTOCOL_VALUES = ["tcp", "udp"] as const;

function ProtocolsBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  function toggle(p: string) {
    if (value.includes(p)) onChange(value.filter((x) => x !== p));
    else onChange([...value, p]);
  }
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {PROTOCOL_VALUES.map((p) => {
        const on = value.includes(p);
        return (
          <motion.button
            key={p}
            type="button"
            onClick={() => toggle(p)}
            aria-label={`Protocol ${p}`}
            aria-pressed={on}
            whileTap={{ scale: 0.94 }}
            transition={{ duration: 0.16, ease: SNAP_EASE }}
            className={
              on
                ? "inline-flex items-center gap-1 rounded-full border border-sky-400/45 bg-sky-500/20 px-3.5 py-1 text-[12px] font-medium uppercase tracking-wider text-sky-100 shadow-[0_2px_10px_-2px_rgba(56,189,248,0.35)]"
                : "inline-flex items-center gap-1 rounded-full border border-white/[0.08] bg-white/[0.03] px-3.5 py-1 text-[12px] font-medium uppercase tracking-wider text-white/55 transition-colors duration-200 hover:border-white/[0.16] hover:bg-white/[0.06] hover:text-white/85"
            }
          >
            {p}
          </motion.button>
        );
      })}
    </div>
  );
}

// ----- AddChipButton — for the one body (Ports) where Enter-to-add is awkward
// due to the dual-input range mode. Kept only there for consistency.

function AddChipButton({ onClick, disabled, label }: { onClick: () => void; disabled?: boolean; label: string }) {
  return (
    <motion.button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={label}
      whileHover={disabled ? undefined : { scale: 1.05 }}
      whileTap={disabled ? undefined : { scale: 0.92 }}
      transition={{ duration: 0.18, ease: SNAP_EASE }}
      className={
        disabled
          ? "rounded-md bg-white/[0.04] px-2.5 py-1.5 text-[12.5px] text-white/30 cursor-not-allowed"
          : "rounded-md bg-sky-500/25 px-2.5 py-1.5 text-[12.5px] font-medium text-sky-100 transition-colors duration-200 hover:bg-sky-500/40"
      }
    >
      +
    </motion.button>
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
