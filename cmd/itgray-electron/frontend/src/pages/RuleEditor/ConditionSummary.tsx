import type { Conditions } from "@/lib/rulesStore";
import type { ConditionType } from "./types";

export function conditionChips(key: ConditionType, c: Conditions): string[] {
  switch (key) {
    case "domains":
      return (c.domains ?? []).map((d) => `${d.kind}: ${d.value}`);
    case "ports":
      return (c.ports ?? []).map((p) => (p.single != null ? String(p.single) : `${p.from}:${p.to}`));
    case "ip_cidrs":
      return c.ip_cidrs ?? [];
    case "geo":
      return c.geo ?? [];
    case "processes":
      return c.processes ?? [];
    case "protocols":
      return c.protocols ?? [];
  }
}

export function ConditionSummary({ conditions, chipsFor }: { conditions: Conditions; chipsFor: ConditionType }) {
  const chips = conditionChips(chipsFor, conditions);
  if (chips.length === 0) return <span className="text-[12px] text-white/35">—</span>;
  return (
    <div className="flex flex-wrap gap-1.5">
      {chips.map((label, i) => (
        <span key={i} className="rounded-md border border-sky-400/25 bg-sky-400/10 px-2 py-0.5 font-mono text-[11.5px] text-sky-100/90">
          {label}
        </span>
      ))}
    </div>
  );
}
