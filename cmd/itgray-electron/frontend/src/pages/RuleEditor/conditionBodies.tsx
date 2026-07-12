import { useState } from "react";
import type React from "react";
import { useTranslation } from "react-i18next";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import { type RuleView, type DomainMatcher, type PortSpec } from "@/lib/rulesStore";
import { Dropdown } from "@/components/controls/Dropdown";
import { Segmented } from "@/components/controls/Segmented";
import type { ConditionType } from "./types";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];
const LAYOUT_SPRING = { type: "spring", stiffness: 380, damping: 34, mass: 0.85 } as const;

const chipVariants: Variants = {
  hidden: { opacity: 0, scale: 0.88 },
  show: { opacity: 1, scale: 1, transition: { duration: 0.18, ease: SNAP_EASE } },
  exit: { opacity: 0, scale: 0.88, transition: { duration: 0.14, ease: SNAP_EASE } },
};

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
  const { t } = useTranslation();
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
          aria-label={removeLabel ?? t('ruleEditor.remove')}
          className="ml-0.5 rounded-full p-0.5 text-white/40 transition-colors duration-150 hover:bg-rose-500/15 hover:text-rose-300"
        >
          ✕
        </button>
      )}
    </motion.span>
  );
}

// ----- ConditionCardBody — switch by type -----

export function ConditionBody({
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
  const { t } = useTranslation();
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
                removeLabel={t('ruleEditor.domains.removeLabel', { n: i + 1 })}
              >
                <div className="w-[88px] shrink-0">
                  <Dropdown
                    value={m.kind}
                    onChange={(v) => onChange(value.map((x, j) => j === i ? { ...x, kind: v as DomainMatcher["kind"] } : x))}
                    options={DOMAIN_OPTIONS}
                    triggerClassName="border-transparent bg-transparent px-1 py-0.5 text-[11px] uppercase tracking-wider text-white/45 hover:border-white/10 hover:bg-white/[0.06] hover:text-white/80"
                    menuClassName="w-32"
                    ariaLabel={t('ruleEditor.domains.kindLabel', { n: i + 1 })}
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
          aria-label={t('ruleEditor.domains.valueLabel')}
          value={addValue}
          onChange={(e) => setAddValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
          placeholder={t('ruleEditor.domains.valuePlaceholder')}
          className={ADD_INPUT_CLASSES}
        />
      </div>
    </div>
  );
}

// ----- IP CIDRs -----

function CidrsBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  const { t } = useTranslation();
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
                removeLabel={t('ruleEditor.cidrs.removeLabel', { n: i + 1 })}
              >
                <span>{cidr || <span className="text-white/40">{t('ruleEditor.empty')}</span>}</span>
              </Chip>
            ))}
          </AnimatePresence>
        </div>
      )}
      <input
        aria-label={t('ruleEditor.cidrs.valueLabel')}
        value={addValue}
        onChange={(e) => setAddValue(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
        placeholder={t('ruleEditor.cidrs.valuePlaceholder')}
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
  const { t } = useTranslation();
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
                  removeLabel={t('ruleEditor.geoBody.removeLabel', { n: i + 1 })}
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
            ariaLabel={t('ruleEditor.geoBody.prefixLabel')}
          />
        </div>
        <input
          aria-label={t('ruleEditor.geoBody.valueLabel')}
          value={addValue}
          onChange={(e) => setAddValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
          placeholder={t('ruleEditor.geoBody.valuePlaceholder')}
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
  const { t } = useTranslation();
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
                removeLabel={t('ruleEditor.ports.removeLabel', { n: i + 1 })}
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
            { value: "single", label: t('ruleEditor.ports.single') },
            { value: "range", label: t('ruleEditor.ports.range') }
          ]}
        />
        {mode === "single" ? (
          <input
            aria-label={t('ruleEditor.ports.numberLabel')}
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
              aria-label={t('ruleEditor.ports.fromLabel')}
              type="number"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
              placeholder="8000"
              className="w-24 rounded-md border border-white/[0.08] bg-white/[0.02] px-2.5 py-1.5 text-[12.5px] tabular-nums text-white/90 outline-none transition-colors duration-200 placeholder:text-white/30 focus:border-sky-400/45 focus:bg-white/[0.04]"
            />
            <span className="text-white/40">→</span>
            <input
              aria-label={t('ruleEditor.ports.toLabel')}
              type="number"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
              placeholder="9000"
              className="w-24 rounded-md border border-white/[0.08] bg-white/[0.02] px-2.5 py-1.5 text-[12.5px] tabular-nums text-white/90 outline-none transition-colors duration-200 placeholder:text-white/30 focus:border-sky-400/45 focus:bg-white/[0.04]"
            />
          </>
        )}
        <AddChipButton onClick={commit} disabled={!canAdd} label={t('ruleEditor.ports.addPort')} />
      </div>
    </div>
  );
}

// ----- Processes -----

function ProcessesBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  const { t } = useTranslation();
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
                removeLabel={t('ruleEditor.processes.removeLabel', { n: i + 1 })}
              >
                <span>{name || <span className="text-white/40">{t('ruleEditor.empty')}</span>}</span>
              </Chip>
            ))}
          </AnimatePresence>
        </div>
      )}
      <input
        aria-label={t('ruleEditor.processes.nameLabel')}
        value={addValue}
        onChange={(e) => setAddValue(e.target.value)}
        onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); commit(); } }}
        placeholder={t('ruleEditor.processes.namePlaceholder')}
        className={ADD_INPUT_CLASSES}
      />
    </div>
  );
}

// ----- Protocols (toggleable chips, no add input) -----

const PROTOCOL_VALUES = ["tcp", "udp"] as const;

function ProtocolsBody({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  const { t } = useTranslation();
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
            aria-label={t('ruleEditor.protocols.label', { p })}
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
