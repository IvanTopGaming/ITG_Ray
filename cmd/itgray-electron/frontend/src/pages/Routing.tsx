import { Lock, ChevronRight } from "lucide-react";
import { useRules, type GroupView, type RuleView } from "@/lib/rulesStore";
import { Toggle } from "@/components/controls/Toggle";
import { cn } from "@/lib/cn";

export function Routing() {
  const { groups, lastError } = useRules();

  return (
    <div className="flex flex-col gap-3">
      <header className="flex items-baseline justify-between pb-2">
        <div>
          <h1 className="text-[20px] font-semibold tracking-tight">Routing rules</h1>
          <p className="mt-1 text-[12px] text-white/55">Per-domain, per-IP, per-process routing. Top of the list matches first.</p>
        </div>
      </header>
      {lastError && (
        <div role="alert" className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[12px] text-rose-200">
          {lastError}
        </div>
      )}
      {groups.map((g) => <GroupCard key={g.id} group={g} />)}
    </div>
  );
}

function GroupCard({ group }: { group: GroupView }) {
  return (
    <section
      className={cn(
        "glass-regular flex flex-col gap-2 rounded-2xl p-4",
        !group.enabled && "opacity-60",
      )}
    >
      <header className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {group.locked && <Lock aria-label="locked" className="h-3.5 w-3.5 text-white/55" />}
          <span className="text-[14px] font-medium text-white/90">{group.name}</span>
          <span className="text-[11px] text-white/45">· {group.rules.length} rule{group.rules.length === 1 ? "" : "s"}</span>
        </div>
        {!group.locked && (
          <Toggle value={group.enabled} aria-label={`Toggle ${group.name}`} onChange={() => { /* wired in T15 */ }} />
        )}
      </header>
      {group.rules.length === 0 ? (
        <div className="rounded-md border border-white/[0.06] bg-white/[0.02] px-3 py-3 text-[11.5px] text-white/45">
          No rules in this group.
        </div>
      ) : (
        <ul className="flex flex-col gap-1">
          {group.rules.map((r) => <RuleRow key={r.id} groupLocked={group.locked} rule={r} />)}
        </ul>
      )}
    </section>
  );
}

function RuleRow({ rule, groupLocked }: { rule: RuleView; groupLocked: boolean }) {
  const actionStyle =
    rule.action === "proxy" ? "bg-sky-500/20 text-sky-200"
    : rule.action === "direct" ? "bg-amber-500/20 text-amber-200"
    : "bg-rose-500/20 text-rose-200";
  return (
    <li
      className={cn(
        "group flex items-center justify-between rounded-md px-3 py-2 text-[12.5px]",
        groupLocked ? "bg-white/[0.02]" : "bg-white/[0.04] hover:bg-white/[0.06] cursor-pointer",
      )}
    >
      <div className="flex items-center gap-3">
        <span className={cn("rounded px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider", actionStyle)}>
          {rule.action}
        </span>
        <span className="text-white/85">{rule.name}</span>
        <span className="text-[11px] text-white/45">{summarise(rule.conditions)}</span>
      </div>
      {!groupLocked && <ChevronRight className="h-3.5 w-3.5 text-white/40 transition-transform group-hover:translate-x-0.5" />}
    </li>
  );
}

function summarise(c: { domains?: unknown[]; ip_cidrs?: unknown[]; geo?: unknown[]; ports?: unknown[]; processes?: unknown[]; protocols?: unknown[] }): string {
  const parts: string[] = [];
  if (c.domains?.length) parts.push(`${c.domains.length} domain${c.domains.length === 1 ? "" : "s"}`);
  if (c.ip_cidrs?.length) parts.push(`${c.ip_cidrs.length} IP CIDR${c.ip_cidrs.length === 1 ? "" : "s"}`);
  if (c.geo?.length) parts.push(`${c.geo.length} geo`);
  if (c.ports?.length) parts.push(`${c.ports.length} port${c.ports.length === 1 ? "" : "s"}`);
  if (c.processes?.length) parts.push(`${c.processes.length} process${c.processes.length === 1 ? "" : "es"}`);
  if (c.protocols?.length) parts.push(`${c.protocols.length} protocol${c.protocols.length === 1 ? "" : "s"}`);
  return parts.join(" · ") || "no conditions";
}
