import { useState, useMemo, useRef } from "react";
import type React from "react";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import { ChevronLeft } from "lucide-react";
import { useRules, rulesEditRule, rulesMoveRule, rulesRemoveRule, type RuleView, type GroupView, type DomainMatcher, type PortSpec } from "@/lib/rulesStore";
import { Segmented } from "@/components/controls/Segmented";
import { Toggle } from "@/components/controls/Toggle";
import { ConfirmDialog } from "@/components/controls/ConfirmDialog";

export function RuleEditor() {
  const { ruleId = "" } = useParams<{ ruleId: string }>();
  const navigate = useNavigate();
  const { groups } = useRules();

  const initial = useMemo(() => findRule(groups, ruleId), [groups, ruleId]);
  const [draft, setDraft] = useState<RuleView | null>(initial.rule);
  const [groupId, setGroupId] = useState<string>(initial.groupId);
  const initialSnapshot = useRef(JSON.stringify({ rule: initial.rule, groupId: initial.groupId }));
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const location = useLocation();
  const freshFromAdd = (location.state as { freshFromAdd?: boolean } | null)?.freshFromAdd === true;
  const savedRef = useRef(false);

  if (!draft) {
    return (
      <div className="flex flex-col gap-3">
        <button onClick={() => navigate("/routing")} className="self-start text-[12px] text-white/55 hover:text-white/90">
          ← Routing
        </button>
        <p className="text-[14px] text-white/70">Rule not found.</p>
      </div>
    );
  }

  async function handleSave() {
    if (!draft) return;
    if (groupId !== initial.groupId) {
      await rulesMoveRule(draft.id, groupId);
    }
    await rulesEditRule(draft);
    // After a successful Save the rule is no longer a fresh stub —
    // future back-clicks should behave like ordinary edits.
    savedRef.current = true;
    navigate("/routing");
  }

  const currentSerialized = JSON.stringify({ rule: draft, groupId });
  const dirty = initialSnapshot.current !== currentSerialized;

  async function handleBack() {
    if (freshFromAdd && !savedRef.current) {
      // User clicked + Add rule, then backed out without saving.
      // Delete the stub so the routing list doesn't accumulate junk
      // placeholder rules.
      try { await rulesRemoveRule(draft!.id); } catch { /* best-effort */ }
      navigate("/routing");
      return;
    }
    if (dirty) setConfirmDiscard(true);
    else navigate("/routing");
  }

  const userGroups = groups.filter((g) => !g.locked);

  return (
    <div className="flex flex-col gap-4">
      <header className="flex items-center justify-between">
        <button
          onClick={handleBack}
          className="flex items-center gap-1 text-[12px] text-white/55 hover:text-white/90"
        >
          <ChevronLeft className="h-3.5 w-3.5" /> Routing
        </button>
        <button
          onClick={handleSave}
          className="rounded-md bg-sky-500/30 px-3 py-1.5 text-[12.5px] font-medium text-sky-100 hover:bg-sky-500/40"
        >
          Save
        </button>
      </header>
      <div className="glass-regular flex flex-col gap-3 rounded-2xl p-4">
        <label className="flex flex-col gap-1">
          <span className="text-[11.5px] uppercase tracking-wider text-white/55">Name</span>
          <input
            aria-label="Name"
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            className="rounded-md border border-white/10 bg-transparent px-3 py-1.5 text-[13px] outline-none focus:border-sky-400/40"
          />
        </label>
        <div className="flex items-center justify-between">
          <span className="text-[11.5px] uppercase tracking-wider text-white/55">Enabled</span>
          <Toggle value={draft.enabled} aria-label="Enabled" onChange={(v) => setDraft({ ...draft, enabled: v })} />
        </div>
        <div className="flex flex-col gap-1">
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
        <label className="flex flex-col gap-1">
          <span className="text-[11.5px] uppercase tracking-wider text-white/55">Group</span>
          <select
            aria-label="Group"
            value={groupId}
            onChange={(e) => setGroupId(e.target.value)}
            className="rounded-md border border-white/10 bg-[#1c1f2a] px-3 py-1.5 text-[13px] outline-none focus:border-sky-400/40"
          >
            {userGroups.map((g) => <option key={g.id} value={g.id}>{g.name}</option>)}
          </select>
        </label>
      </div>
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
      <ConfirmDialog
        open={confirmDiscard}
        title="Discard changes?"
        description="You have unsaved changes. Leave anyway?"
        confirmLabel="Discard"
        confirmVariant="danger"
        onClose={() => setConfirmDiscard(false)}
        onConfirm={() => navigate("/routing")}
      />
    </div>
  );
}

function Section({ title, count, defaultOpen, children }: { title: string; count: number; defaultOpen: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className="glass-regular flex flex-col gap-2 rounded-2xl p-4">
      <button type="button" onClick={() => setOpen(!open)} className="flex items-center justify-between text-left">
        <span className="text-[13px] font-medium text-white/90">
          <span className="mr-1 inline-block w-3">{open ? "▼" : "▶"}</span>
          {title}{count > 0 ? ` (${count})` : ` · empty`}
        </span>
      </button>
      {open && <div className="flex flex-col gap-2 pt-1">{children}</div>}
    </section>
  );
}

function DomainsSection({ value, onChange }: { value: DomainMatcher[]; onChange: (next: DomainMatcher[]) => void }) {
  return (
    <>
      {value.map((m, i) => (
        <div key={i} className="flex items-center gap-2">
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
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => j !== i))}
            aria-label={`Remove domain matcher ${i + 1}`}
            className="text-white/45 hover:text-rose-300"
          >
            ✕
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange([...value, { kind: "suffix", value: "" }])}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add domain matcher
      </button>
    </>
  );
}

function CidrsSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <>
      {value.map((cidr, i) => (
        <div key={i} className="flex items-center gap-2">
          <input
            aria-label={`CIDR value ${i + 1}`}
            value={cidr}
            onChange={(e) => onChange(value.map((x, j) => j === i ? e.target.value : x))}
            placeholder="e.g. 10.0.0.0/8 or 1.2.3.4"
            className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
          />
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => j !== i))}
            aria-label={`Remove CIDR ${i + 1}`}
            className="text-white/45 hover:text-rose-300"
          >
            ✕
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange([...value, ""])}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add CIDR
      </button>
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
      {value.map((entry, i) => {
        const { prefix, rest } = split(entry);
        return (
          <div key={i} className="flex items-center gap-2">
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
            <button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              aria-label={`Remove geo ${i + 1}`}
              className="text-white/45 hover:text-rose-300"
            >
              ✕
            </button>
          </div>
        );
      })}
      <button
        type="button"
        onClick={() => onChange([...value, "geosite:"])}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add geo
      </button>
    </>
  );
}

function PortsSection({ value, onChange }: { value: PortSpec[]; onChange: (next: PortSpec[]) => void }) {
  function setRow(i: number, next: PortSpec) {
    onChange(value.map((x, j) => (j === i ? next : x)));
  }
  return (
    <>
      {value.map((p, i) => {
        const mode: "single" | "range" = p.single ? "single" : (p.from || p.to ? "range" : "single");
        return (
          <div key={i} className="flex items-center gap-2">
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
                  onChange={(e) => setRow(i, { ...p, from: Number(e.target.value), single: undefined })}
                  className="w-24 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
                />
                <span className="text-white/45">→</span>
                <input
                  aria-label={`Port to ${i + 1}`}
                  type="number"
                  value={p.to ?? ""}
                  onChange={(e) => setRow(i, { ...p, to: Number(e.target.value), single: undefined })}
                  className="w-24 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
                />
              </>
            )}
            <button
              type="button"
              onClick={() => onChange(value.filter((_, j) => j !== i))}
              aria-label={`Remove port ${i + 1}`}
              className="ml-auto text-white/45 hover:text-rose-300"
            >
              ✕
            </button>
          </div>
        );
      })}
      <button
        type="button"
        onClick={() => onChange([...value, { single: 0 }])}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add port
      </button>
    </>
  );
}

function ProcessesSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <>
      {value.map((name, i) => (
        <div key={i} className="flex items-center gap-2">
          <input
            aria-label={`Process name ${i + 1}`}
            value={name}
            onChange={(e) => onChange(value.map((x, j) => j === i ? e.target.value : x))}
            onBlur={() => onChange(value.map((x, j) => j === i ? x.trim() : x))}
            placeholder="e.g. chrome.exe"
            className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[12.5px] outline-none focus:border-sky-400/40"
          />
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => j !== i))}
            aria-label={`Remove process ${i + 1}`}
            className="text-white/45 hover:text-rose-300"
          >
            ✕
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange([...value, ""])}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add process
      </button>
    </>
  );
}

function ProtocolsSection({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <>
      {value.map((proto, i) => (
        <div key={i} className="flex items-center gap-2">
          <Segmented
            value={proto === "udp" ? "udp" : "tcp"}
            onChange={(v) => onChange(value.map((x, j) => j === i ? v : x))}
            options={[
              { value: "tcp", label: "tcp" },
              { value: "udp", label: "udp" },
            ] as const}
          />
          <button
            type="button"
            onClick={() => onChange(value.filter((_, j) => j !== i))}
            aria-label={`Remove protocol ${i + 1}`}
            className="ml-auto text-white/45 hover:text-rose-300"
          >
            ✕
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={() => onChange([...value, "tcp"])}
        className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08]"
      >
        + Add protocol
      </button>
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
