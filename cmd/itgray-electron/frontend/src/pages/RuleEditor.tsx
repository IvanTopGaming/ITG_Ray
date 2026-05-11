import { useState, useMemo } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { ChevronLeft } from "lucide-react";
import { useRules, rulesEditRule, rulesMoveRule, type RuleView, type GroupView } from "@/lib/rulesStore";
import { Segmented } from "@/components/controls/Segmented";
import { Toggle } from "@/components/controls/Toggle";

export function RuleEditor() {
  const { ruleId = "" } = useParams<{ ruleId: string }>();
  const navigate = useNavigate();
  const { groups } = useRules();

  const initial = useMemo(() => findRule(groups, ruleId), [groups, ruleId]);
  const [draft, setDraft] = useState<RuleView | null>(initial.rule);
  const [groupId, setGroupId] = useState<string>(initial.groupId);

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
    navigate("/routing");
  }

  const userGroups = groups.filter((g) => !g.locked);

  return (
    <div className="flex flex-col gap-4">
      <header className="flex items-center justify-between">
        <button
          onClick={() => navigate("/routing")}
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
      {/* Condition sections land in Tasks 21-26 */}
    </div>
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
