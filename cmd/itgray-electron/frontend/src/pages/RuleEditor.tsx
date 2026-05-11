import { useState, useMemo, useRef } from "react";
import type React from "react";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import { ChevronLeft } from "lucide-react";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import { useRules, rulesAddRule, rulesEditRule, rulesMoveRule, type RuleView, type GroupView, type DomainMatcher, type PortSpec } from "@/lib/rulesStore";
import { Segmented } from "@/components/controls/Segmented";
import { Toggle } from "@/components/controls/Toggle";
import { ConfirmDialog } from "@/components/controls/ConfirmDialog";
import { Reveal } from "@/components/controls/Reveal";

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

const rowVariants: Variants = {
  hidden: { opacity: 0, x: -8 },
  show: { opacity: 1, x: 0, transition: { duration: 0.22, ease: SNAP_EASE } },
  exit: { opacity: 0, x: 8, transition: { duration: 0.18, ease: SNAP_EASE } },
};

type CreateState = { mode: "create"; groupId: string };

function isCreateState(s: unknown): s is CreateState {
  return !!s && typeof s === "object" && (s as { mode?: unknown }).mode === "create"
    && typeof (s as { groupId?: unknown }).groupId === "string";
}

const EMPTY_DRAFT: Omit<RuleView, "id"> = {
  name: "New rule",
  enabled: true,
  action: "proxy",
  conditions: {},
};

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
          className="self-start text-[12px] text-white/55 hover:text-white/90"
        >
          ← Routing
        </motion.button>
        <p className="text-[14px] text-white/70">Rule not found.</p>
      </motion.div>
    );
  }

  async function handleSave() {
    if (!draft) return;
    setSaveError(null);
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

  const userGroups = groups.filter((g) => !g.locked);

  return (
    <motion.div
      className="flex flex-col gap-4"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      <motion.header variants={sectionVariants} className="flex items-center justify-between">
        <motion.button
          onClick={handleBack}
          whileHover={{ x: -2 }}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="flex items-center gap-1 text-[12px] text-white/55 hover:text-white/90"
        >
          <ChevronLeft className="h-3.5 w-3.5" /> Routing
        </motion.button>
        <motion.button
          onClick={handleSave}
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="rounded-md bg-sky-500/60 px-5 py-2 text-[13px] font-semibold text-white shadow-[0_6px_18px_-8px_rgba(56,189,248,0.65)] hover:bg-sky-400/70"
        >
          {isCreate ? "Create rule" : "Save"}
        </motion.button>
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
      <motion.div variants={sectionVariants} className="glass-regular flex flex-col gap-3 rounded-2xl p-4">
        <label className="flex flex-col gap-1">
          <span className="text-[11.5px] uppercase tracking-wider text-white/55">Name</span>
          <input
            aria-label="Name"
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            className="rounded-md border border-white/15 bg-transparent px-3 py-1.5 text-[13px] outline-none focus:border-sky-400/40"
          />
        </label>
        <div className={`flex flex-wrap items-end gap-3 rounded-lg p-2 transition-colors duration-200 ${actionTint(draft.action)}`}>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <span className="text-[11.5px] uppercase tracking-wider text-white/55">Action</span>
            <Segmented
              value={draft.action}
              onChange={(v) => setDraft({ ...draft, action: v as RuleView["action"] })}
              options={[
                { value: "proxy", label: "Proxy" },
                { value: "direct", label: "Direct" },
                { value: "block", label: "Block" },
              ] as const}
            />
          </div>
          <div className="flex flex-col items-end gap-1">
            <span className="text-[11.5px] uppercase tracking-wider text-white/55">Enabled</span>
            <Toggle value={draft.enabled} aria-label="Enabled" onChange={(v) => setDraft({ ...draft, enabled: v })} />
          </div>
        </div>
        {!isCreate && (
          <label className="flex flex-col gap-1">
            <span className="text-[11.5px] uppercase tracking-wider text-white/55">Group</span>
            <select
              aria-label="Group"
              value={groupId}
              onChange={(e) => setGroupId(e.target.value)}
              className="rounded-md border border-white/15 bg-[#1c1f2a] px-3 py-1.5 text-[13px] outline-none focus:border-sky-400/40"
            >
              {userGroups.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
            </select>
          </label>
        )}
      </motion.div>
      <motion.div variants={sectionVariants} layout className="grid grid-cols-2 items-start gap-3">
        <Section
          title="Domains"
          count={draft.conditions.domains?.length ?? 0}
          defaultOpen={(draft.conditions.domains?.length ?? 0) > 0}
        >
          <DomainsSection
            value={draft.conditions.domains ?? []}
            onChange={(next) => setDraft({ ...draft, conditions: { ...draft.conditions, domains: next } })}
          />
        </Section>
        <Section
          title="IP CIDRs"
          count={draft.conditions.ip_cidrs?.length ?? 0}
          defaultOpen={(draft.conditions.ip_cidrs?.length ?? 0) > 0}
        >
          <CidrsSection
            value={draft.conditions.ip_cidrs ?? []}
            onChange={(next) => setDraft({ ...draft, conditions: { ...draft.conditions, ip_cidrs: next } })}
          />
        </Section>
        <Section
          title="Geo"
          count={draft.conditions.geo?.length ?? 0}
          defaultOpen={(draft.conditions.geo?.length ?? 0) > 0}
        >
          <GeoSection
            value={draft.conditions.geo ?? []}
            onChange={(next) => setDraft({ ...draft, conditions: { ...draft.conditions, geo: next } })}
          />
        </Section>
        <Section
          title="Ports"
          count={draft.conditions.ports?.length ?? 0}
          defaultOpen={(draft.conditions.ports?.length ?? 0) > 0}
        >
          <PortsSection
            value={draft.conditions.ports ?? []}
            onChange={(next) => setDraft({ ...draft, conditions: { ...draft.conditions, ports: next } })}
          />
        </Section>
        <Section
          title="Processes"
          count={draft.conditions.processes?.length ?? 0}
          defaultOpen={(draft.conditions.processes?.length ?? 0) > 0}
        >
          <ProcessesSection
            value={draft.conditions.processes ?? []}
            onChange={(next) => setDraft({ ...draft, conditions: { ...draft.conditions, processes: next } })}
          />
        </Section>
        <Section
          title="Protocols"
          count={draft.conditions.protocols?.length ?? 0}
          defaultOpen={(draft.conditions.protocols?.length ?? 0) > 0}
        >
          <ProtocolsSection
            value={draft.conditions.protocols ?? []}
            onChange={(next) => setDraft({ ...draft, conditions: { ...draft.conditions, protocols: next } })}
          />
        </Section>
      </motion.div>
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

function Section({ title, count, defaultOpen, children }: { title: string; count: number; defaultOpen: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <motion.section
      layout
      variants={sectionVariants}
      transition={{ layout: { duration: 0.22, ease: SNAP_EASE } }}
      className={`glass-regular flex flex-col rounded-2xl ${open ? "gap-2 p-4" : "py-2.5 px-4"}`}
    >
      <button type="button" onClick={() => setOpen(!open)} className="flex items-center justify-between text-left">
        <span className="text-[13px] font-medium text-white/90">
          <motion.span
            animate={{ rotate: open ? 0 : -90 }}
            transition={{ duration: 0.18, ease: SNAP_EASE }}
            className="mr-1 inline-block w-3"
          >
            ▼
          </motion.span>
          {title}{count > 0 ? ` (${count})` : ` · empty`}
        </span>
      </button>
      <Reveal show={open}>
        <div className="flex flex-col gap-2 pt-1">{children}</div>
      </Reveal>
    </motion.section>
  );
}

function actionTint(action: RuleView["action"]): string {
  switch (action) {
    case "proxy":
      return "bg-sky-500/10 ring-1 ring-inset ring-sky-400/20";
    case "direct":
      return "bg-amber-500/10 ring-1 ring-inset ring-amber-400/20";
    case "block":
      return "bg-rose-500/10 ring-1 ring-inset ring-rose-400/20";
    default:
      return "";
  }
}

function DomainsSection({ value, onChange }: { value: DomainMatcher[]; onChange: (next: DomainMatcher[]) => void }) {
  return (
    <>
      <AnimatePresence initial={false}>
        {value.map((m, i) => (
          <motion.div
            key={i}
            variants={rowVariants}
            initial="hidden"
            animate="show"
            exit="exit"
            className="flex items-center gap-2"
          >
            <select
              aria-label="Domain matcher kind"
              value={m.kind}
              onChange={(e) => onChange(value.map((x, j) => j === i ? { ...x, kind: e.target.value as DomainMatcher["kind"] } : x))}
              className="rounded-md border border-white/10 bg-[#1c1f2a] px-2 py-1 text-[12px]"
            >
              <option value="exact">exact</option>
              <option value="suffix">suffix</option>
              <option value="keyword">keyword</option>
              <option value="regex">regex</option>
            </select>
            <input
              aria-label="Domain matcher value"
              value={m.value}
              onChange={(e) => onChange(value.map((x, j) => j === i ? { ...x, value: e.target.value } : x))}
              className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
            />
            <motion.button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              aria-label={`Remove domain matcher ${i + 1}`}
              whileHover={{ scale: 1.15 }}
              whileTap={{ scale: 0.85 }}
              transition={{ duration: 0.18, ease: SNAP_EASE }}
              className="text-white/45 hover:text-rose-300"
            >
              ✕
            </motion.button>
          </motion.div>
        ))}
      </AnimatePresence>
      <motion.button
        type="button"
        onClick={() => onChange([...value, { kind: "suffix", value: "" }])}
        whileHover={{ y: -1 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add domain matcher
      </motion.button>
    </>
  );
}

function CidrsSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <>
      <AnimatePresence initial={false}>
        {value.map((cidr, i) => (
          <motion.div
            key={i}
            variants={rowVariants}
            initial="hidden"
            animate="show"
            exit="exit"
            className="flex items-center gap-2"
          >
            <input
              aria-label={`CIDR value ${i + 1}`}
              value={cidr}
              onChange={(e) => onChange(value.map((x, j) => j === i ? e.target.value : x))}
              placeholder="e.g. 10.0.0.0/8 or 1.2.3.4"
              className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
            />
            <motion.button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              aria-label={`Remove CIDR ${i + 1}`}
              whileHover={{ scale: 1.15 }}
              whileTap={{ scale: 0.85 }}
              transition={{ duration: 0.18, ease: SNAP_EASE }}
              className="text-white/45 hover:text-rose-300"
            >
              ✕
            </motion.button>
          </motion.div>
        ))}
      </AnimatePresence>
      <motion.button
        type="button"
        onClick={() => onChange([...value, ""])}
        whileHover={{ y: -1 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add CIDR
      </motion.button>
    </>
  );
}

function GeoSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  function split(entry: string): { prefix: string; rest: string } {
    const idx = entry.indexOf(":");
    if (idx < 0) return { prefix: "geosite", rest: entry };
    return { prefix: entry.slice(0, idx), rest: entry.slice(idx + 1) };
  }
  return (
    <>
      <AnimatePresence initial={false}>
        {value.map((entry, i) => {
          const { prefix, rest } = split(entry);
          return (
            <motion.div
              key={i}
              variants={rowVariants}
              initial="hidden"
              animate="show"
              exit="exit"
              className="flex items-center gap-2"
            >
              <select
                aria-label={`Geo prefix ${i + 1}`}
                value={prefix}
                onChange={(e) => onChange(value.map((x, j) => j === i ? `${e.target.value}:${split(x).rest}` : x))}
                className="rounded-md border border-white/10 bg-[#1c1f2a] px-2 py-1 text-[12px]"
              >
                <option value="geosite">geosite</option>
                <option value="geoip">geoip</option>
              </select>
              <input
                aria-label={`Geo value ${i + 1}`}
                value={rest}
                onChange={(e) => onChange(value.map((x, j) => j === i ? `${split(x).prefix}:${e.target.value}` : x))}
                placeholder="e.g. cn, google, ru"
                className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
              />
              <motion.button
                type="button"
                onClick={() => onChange(value.filter((_, j) => j !== i))}
                aria-label={`Remove geo ${i + 1}`}
                whileHover={{ scale: 1.15 }}
                whileTap={{ scale: 0.85 }}
                transition={{ duration: 0.18, ease: SNAP_EASE }}
                className="text-white/45 hover:text-rose-300"
              >
                ✕
              </motion.button>
            </motion.div>
          );
        })}
      </AnimatePresence>
      <motion.button
        type="button"
        onClick={() => onChange([...value, "geosite:"])}
        whileHover={{ y: -1 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add geo
      </motion.button>
    </>
  );
}

function PortsSection({ value, onChange }: { value: PortSpec[]; onChange: (next: PortSpec[]) => void }) {
  function setRow(i: number, next: PortSpec) {
    onChange(value.map((x, j) => (j === i ? next : x)));
  }
  return (
    <>
      <AnimatePresence initial={false}>
        {value.map((p, i) => {
          // Track mode by field presence, not by truthiness. A range with
          // both endpoints at 0 (`from === 0, to === 0`) is still a range.
          const mode: "single" | "range" =
            p.from !== undefined || p.to !== undefined ? "range" : "single";
          return (
            <motion.div
              key={i}
              variants={rowVariants}
              initial="hidden"
              animate="show"
              exit="exit"
              className="flex items-center gap-2"
            >
              <Segmented
                value={mode}
                onChange={(v) => {
                  if (v === "single") setRow(i, { single: p.single ?? p.from ?? 0 });
                  else setRow(i, { from: p.from ?? p.single ?? 0, to: p.to ?? p.single ?? 0 });
                }}
                options={[
                  { value: "single", label: "Single" },
                  { value: "range", label: "Range" },
                ] as const}
              />
              {mode === "single" ? (
                <input
                  aria-label={`Port number ${i + 1}`}
                  type="number"
                  value={p.single ?? ""}
                  onChange={(e) => setRow(i, { single: Number(e.target.value) })}
                  className="w-24 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
                />
              ) : (
                <>
                  <input
                    aria-label={`Port from ${i + 1}`}
                    type="number"
                    value={p.from ?? ""}
                    onChange={(e) => setRow(i, { from: Number(e.target.value), to: p.to ?? 0 })}
                    className="w-24 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
                  />
                  <span className="text-white/45">→</span>
                  <input
                    aria-label={`Port to ${i + 1}`}
                    type="number"
                    value={p.to ?? ""}
                    onChange={(e) => setRow(i, { from: p.from ?? 0, to: Number(e.target.value) })}
                    className="w-24 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
                  />
                </>
              )}
              <motion.button
                type="button"
                onClick={() => onChange(value.filter((_, j) => j !== i))}
                aria-label={`Remove port ${i + 1}`}
                whileHover={{ scale: 1.15 }}
                whileTap={{ scale: 0.85 }}
                transition={{ duration: 0.18, ease: SNAP_EASE }}
                className="ml-auto text-white/45 hover:text-rose-300"
              >
                ✕
              </motion.button>
            </motion.div>
          );
        })}
      </AnimatePresence>
      <motion.button
        type="button"
        onClick={() => onChange([...value, { single: 0 }])}
        whileHover={{ y: -1 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add port
      </motion.button>
    </>
  );
}

function ProcessesSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <>
      <AnimatePresence initial={false}>
        {value.map((name, i) => (
          <motion.div
            key={i}
            variants={rowVariants}
            initial="hidden"
            animate="show"
            exit="exit"
            className="flex items-center gap-2"
          >
            <input
              aria-label={`Process name ${i + 1}`}
              value={name}
              onChange={(e) => onChange(value.map((x, j) => j === i ? e.target.value : x))}
              onBlur={() => onChange(value.map((x, j) => j === i ? x.trim() : x))}
              placeholder="e.g. chrome.exe"
              className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
            />
            <motion.button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              aria-label={`Remove process ${i + 1}`}
              whileHover={{ scale: 1.15 }}
              whileTap={{ scale: 0.85 }}
              transition={{ duration: 0.18, ease: SNAP_EASE }}
              className="text-white/45 hover:text-rose-300"
            >
              ✕
            </motion.button>
          </motion.div>
        ))}
      </AnimatePresence>
      <motion.button
        type="button"
        onClick={() => onChange([...value, ""])}
        whileHover={{ y: -1 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add process
      </motion.button>
    </>
  );
}

function ProtocolsSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <>
      <AnimatePresence initial={false}>
        {value.map((proto, i) => (
          <motion.div
            key={i}
            variants={rowVariants}
            initial="hidden"
            animate="show"
            exit="exit"
            className="flex items-center gap-2"
          >
            <Segmented
              value={proto === "udp" ? "udp" : "tcp"}
              onChange={(v) => onChange(value.map((x, j) => j === i ? v : x))}
              options={[
                { value: "tcp", label: "tcp" },
                { value: "udp", label: "udp" },
              ] as const}
            />
            <motion.button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              aria-label={`Remove protocol ${i + 1}`}
              whileHover={{ scale: 1.15 }}
              whileTap={{ scale: 0.85 }}
              transition={{ duration: 0.18, ease: SNAP_EASE }}
              className="ml-auto text-white/45 hover:text-rose-300"
            >
              ✕
            </motion.button>
          </motion.div>
        ))}
      </AnimatePresence>
      <motion.button
        type="button"
        onClick={() => onChange([...value, "tcp"])}
        whileHover={{ y: -1 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add protocol
      </motion.button>
    </>
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
