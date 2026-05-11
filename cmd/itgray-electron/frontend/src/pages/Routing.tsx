import { useState, useRef, useEffect, type HTMLAttributes, type SyntheticEvent } from "react";
import { useNavigate } from "react-router-dom";
import { Lock, ChevronRight, Plus, MoreHorizontal, GripVertical } from "lucide-react";
import { DndContext, closestCenter, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import {
  useRules,
  rulesAddGroup,
  rulesEditGroup,
  rulesRemoveGroup,
  rulesAddRule,
  rulesMoveRule,
  rulesRemoveRule,
  rulesReplaceAll,
  type GroupView,
  type RuleView,
} from "@/lib/rulesStore";
import { Toggle } from "@/components/controls/Toggle";
import { ConfirmDialog } from "@/components/controls/ConfirmDialog";
import { cn } from "@/lib/cn";

export function reorderRules(groups: GroupView[], groupId: string, fromIdx: number, toIdx: number): GroupView[] {
  return groups.map((g) => {
    if (g.id !== groupId) return g;
    const next = g.rules.slice();
    const [moved] = next.splice(fromIdx, 1);
    next.splice(toIdx, 0, moved);
    return { ...g, rules: next };
  });
}

export function reorderGroups(groups: GroupView[], activeId: string, overId: string): GroupView[] {
  if (activeId === "safety" || overId === "safety") return groups;
  const fromIdx = groups.findIndex((g) => g.id === activeId);
  const toIdx = groups.findIndex((g) => g.id === overId);
  if (fromIdx < 0 || toIdx < 0) return groups;
  const next = groups.slice();
  const [moved] = next.splice(fromIdx, 1);
  next.splice(toIdx, 0, moved);
  return next;
}

export function Routing() {
  const { groups, defaultAction, lastError } = useRules();
  const [adding, setAdding] = useState(false);

  const safetyGroup = groups.find((g) => g.id === "safety");
  const userGroups = groups.filter((g) => g.id !== "safety");

  function handleRuleDragEnd(groupId: string) {
    return (e: DragEndEvent) => {
      if (!e.over || e.over.id === e.active.id) return;
      const group = groups.find((g) => g.id === groupId);
      if (!group) return;
      const fromIdx = group.rules.findIndex((r) => r.id === e.active.id);
      const toIdx = group.rules.findIndex((r) => r.id === e.over!.id);
      if (fromIdx < 0 || toIdx < 0) return;
      const nextGroups = reorderRules(groups, groupId, fromIdx, toIdx);
      void rulesReplaceAll({ defaultAction, groups: nextGroups });
    };
  }

  function onGroupDragEnd(e: DragEndEvent) {
    if (!e.over || e.over.id === e.active.id) return;
    const nextUser = reorderGroups(userGroups, String(e.active.id), String(e.over.id));
    const finalGroups = safetyGroup ? [safetyGroup, ...nextUser] : nextUser;
    void rulesReplaceAll({ defaultAction, groups: finalGroups });
  }

  return (
    <div className="flex flex-col gap-3">
      <header className="flex items-baseline justify-between pb-2">
        <div>
          <h1 className="text-[20px] font-semibold tracking-tight">Routing rules</h1>
          <p className="mt-1 text-[12px] text-white/55">Per-domain, per-IP, per-process routing. Top of the list matches first.</p>
        </div>
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="flex items-center gap-1.5 rounded-md bg-white/[0.08] px-3 py-1.5 text-[12px] font-medium text-white/85 hover:bg-white/[0.12]"
        >
          <Plus className="h-3.5 w-3.5" /> Add group
        </button>
      </header>
      {lastError && (
        <div role="alert" className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[12px] text-rose-200">
          {lastError}
        </div>
      )}
      {adding && <AddGroupRow onCancel={() => setAdding(false)} />}
      {safetyGroup && <GroupCard group={safetyGroup} onRuleDragEnd={() => {}} allGroups={groups} />}
      <DndContext collisionDetection={closestCenter} onDragEnd={onGroupDragEnd}>
        <SortableContext items={userGroups.map((g) => g.id)} strategy={verticalListSortingStrategy}>
          {userGroups.map((g) => (
            <SortableGroupCard
              key={g.id}
              group={g}
              onRuleDragEnd={handleRuleDragEnd(g.id)}
              allGroups={groups}
            />
          ))}
        </SortableContext>
      </DndContext>
    </div>
  );
}

function AddGroupRow({ onCancel }: { onCancel: () => void }) {
  const [value, setValue] = useState("");
  const ref = useRef<HTMLInputElement>(null);
  const submittedRef = useRef(false);
  useEffect(() => { ref.current?.focus(); }, []);

  async function submit() {
    if (submittedRef.current) return;
    submittedRef.current = true;
    const trimmed = value.trim();
    if (!trimmed) { onCancel(); return; }
    try {
      await rulesAddGroup(trimmed);
    } finally {
      onCancel();
    }
  }

  function cancel() {
    if (submittedRef.current) return;
    submittedRef.current = true;
    onCancel();
  }

  const trimmed = value.trim();
  return (
    <div className="glass-regular flex items-center gap-2 rounded-2xl p-3">
      <input
        ref={ref}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") void submit();
          if (e.key === "Escape") cancel();
        }}
        placeholder="New group name — Enter to add, Esc to cancel"
        className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1 text-[13px] outline-none focus:border-sky-400/40"
      />
      <button
        type="button"
        onClick={() => void submit()}
        disabled={!trimmed}
        className="rounded-md bg-sky-500/30 px-3 py-1 text-[12px] font-medium text-sky-100 hover:bg-sky-500/40 disabled:cursor-not-allowed disabled:opacity-40"
      >
        Add
      </button>
      <button
        type="button"
        onClick={cancel}
        className="rounded-md px-2 py-1 text-[12px] text-white/55 hover:text-white/90"
      >
        Cancel
      </button>
    </div>
  );
}

type DragHandleProps = {
  attributes: HTMLAttributes<HTMLElement>;
  listeners: Record<string, (event: SyntheticEvent) => void> | undefined;
};

function GroupCard({ group, onRuleDragEnd, dragHandle, allGroups }: { group: GroupView; onRuleDragEnd: (e: DragEndEvent) => void; dragHandle?: DragHandleProps; allGroups: GroupView[] }) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const navigate = useNavigate();

  async function handleAddRule() {
    const id = await rulesAddRule(group.id, {
      name: "New rule",
      enabled: true,
      action: "proxy",
      conditions: { ip_cidrs: ["0.0.0.0/0"] },
    });
    // freshFromAdd lets RuleEditor know this rule was just stubbed on
    // the user's behalf — if they back out without saving, the editor
    // deletes it so the routing list stays clean.
    navigate(`/routing/${id}`, { state: { freshFromAdd: true } });
  }

  return (
    <>
      <section className={cn("glass-regular flex flex-col gap-2 rounded-2xl p-4", !group.enabled && "opacity-60")}>
        <header className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {dragHandle && (
              <button
                type="button"
                {...dragHandle.attributes}
                {...dragHandle.listeners}
                aria-label={`Drag ${group.name}`}
                className="cursor-grab text-white/35 hover:text-white/65"
              >
                <GripVertical className="h-3.5 w-3.5" />
              </button>
            )}
            {group.locked && <Lock aria-label="locked" className="h-3.5 w-3.5 text-white/55" />}
            {renaming
              ? <InlineRename
                  initial={group.name}
                  onCancel={() => setRenaming(false)}
                  onCommit={async (next) => {
                    setRenaming(false);
                    if (next !== group.name) {
                      await rulesEditGroup(group.id, next, group.enabled);
                    }
                  }}
                />
              : <span className="text-[14px] font-medium text-white/90">{group.name}</span>
            }
            <span className="text-[11px] text-white/45">· {group.rules.length} rule{group.rules.length === 1 ? "" : "s"}</span>
          </div>
          <div className="flex items-center gap-2">
            {!group.locked && (
              <Toggle
                value={group.enabled}
                aria-label={`Toggle ${group.name}`}
                onChange={(next) => { void rulesEditGroup(group.id, group.name, next); }}
              />
            )}
            {!group.locked && (
              <div className="relative">
                <button
                  type="button"
                  aria-label={`${group.name} menu`}
                  onClick={() => setMenuOpen((v) => !v)}
                  className="rounded-md p-1 text-white/55 hover:bg-white/[0.08] hover:text-white/90"
                >
                  <MoreHorizontal className="h-4 w-4" />
                </button>
                {menuOpen && (
                  <div role="menu" className="absolute right-0 z-10 mt-1 w-32 rounded-md border border-white/10 bg-[#1c1f2a] py-1 text-[12.5px] shadow-lg">
                    <button
                      role="menuitem"
                      type="button"
                      className="block w-full px-3 py-1.5 text-left hover:bg-white/[0.06]"
                      onClick={() => { setRenaming(true); setMenuOpen(false); }}
                    >
                      Rename
                    </button>
                    <button
                      role="menuitem"
                      type="button"
                      className="block w-full px-3 py-1.5 text-left text-rose-300 hover:bg-rose-500/10"
                      onClick={() => { setConfirmDelete(true); setMenuOpen(false); }}
                    >
                      Delete
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        </header>
        {group.rules.length === 0 ? (
          <div className="rounded-md border border-white/[0.06] bg-white/[0.02] px-3 py-3 text-[11.5px] text-white/45">
            No rules in this group.
          </div>
        ) : group.locked ? (
          <ul className="flex flex-col gap-1">
            {group.rules.map((r) => (
              <li key={r.id}>
                <RuleRow rule={r} groupLocked />
              </li>
            ))}
          </ul>
        ) : (
          <DndContext collisionDetection={closestCenter} onDragEnd={onRuleDragEnd}>
            <SortableContext items={group.rules.map((r) => r.id)} strategy={verticalListSortingStrategy}>
              <ul className="flex flex-col gap-1">
                {group.rules.map((r) => <SortableRuleRow key={r.id} rule={r} group={group} allGroups={allGroups} />)}
              </ul>
            </SortableContext>
          </DndContext>
        )}
        {!group.locked && (
          <button
            type="button"
            onClick={handleAddRule}
            className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08] hover:text-white/90"
          >
            + Add rule
          </button>
        )}
      </section>
      <ConfirmDialog
        open={confirmDelete}
        title="Delete group?"
        description={`"${group.name}" and all its ${group.rules.length} rule${group.rules.length === 1 ? "" : "s"} will be removed.`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onClose={() => setConfirmDelete(false)}
        onConfirm={() => { void rulesRemoveGroup(group.id); }}
      />
    </>
  );
}

function InlineRename({ initial, onCancel, onCommit }: { initial: string; onCancel: () => void; onCommit: (next: string) => void | Promise<void> }) {
  const [value, setValue] = useState(initial);
  const ref = useRef<HTMLInputElement>(null);
  const finishedRef = useRef(false);
  useEffect(() => { ref.current?.focus(); ref.current?.select(); }, []);

  function commit() {
    if (finishedRef.current) return;
    finishedRef.current = true;
    void onCommit(value.trim() || initial);
  }

  function cancel() {
    if (finishedRef.current) return;
    finishedRef.current = true;
    onCancel();
  }

  return (
    <input
      ref={ref}
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onKeyDown={(e) => {
        if (e.key === "Enter") commit();
        if (e.key === "Escape") cancel();
      }}
      onBlur={commit}
      className="rounded-md border border-white/10 bg-transparent px-2 py-0.5 text-[14px] font-medium text-white/90 outline-none focus:border-sky-400/40"
    />
  );
}

function SortableGroupCard({ group, onRuleDragEnd, allGroups }: { group: GroupView; onRuleDragEnd: (e: DragEndEvent) => void; allGroups: GroupView[] }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: group.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.7 : 1,
  };
  return (
    <div ref={setNodeRef} style={style}>
      <GroupCard group={group} onRuleDragEnd={onRuleDragEnd} allGroups={allGroups} dragHandle={{ attributes: attributes as HTMLAttributes<HTMLElement>, listeners: listeners as Record<string, (event: SyntheticEvent) => void> | undefined }} />
    </div>
  );
}

function SortableRuleRow({ rule, group, allGroups }: { rule: RuleView; group: GroupView; allGroups: GroupView[] }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: rule.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.7 : 1,
  };
  return (
    <li ref={setNodeRef} style={style} className="flex items-center gap-2">
      <button
        type="button"
        {...attributes}
        {...listeners}
        aria-label={`Drag ${rule.name}`}
        className="cursor-grab text-white/35 hover:text-white/65"
      >
        <GripVertical className="h-3.5 w-3.5" />
      </button>
      <div className="flex-1">
        <RuleRow rule={rule} groupLocked={false} group={group} allGroups={allGroups} />
      </div>
    </li>
  );
}

function RuleRow({
  rule,
  groupLocked,
  group,
  allGroups,
}: {
  rule: RuleView;
  groupLocked: boolean;
  group?: GroupView;
  allGroups?: GroupView[];
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const actionStyle =
    rule.action === "proxy" ? "bg-sky-500/20 text-sky-200"
    : rule.action === "direct" ? "bg-amber-500/20 text-amber-200"
    : "bg-rose-500/20 text-rose-200";
  const targetGroups =
    !groupLocked && group && allGroups
      ? allGroups.filter((g) => !g.locked && g.id !== group.id)
      : [];
  return (
    <div
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
      {!groupLocked && (
        <div className="flex items-center gap-1">
          <div className="relative" onClick={(e) => e.stopPropagation()}>
            <button
              type="button"
              aria-label={`${rule.name} menu`}
              onClick={(e) => { e.stopPropagation(); setMenuOpen((v) => !v); }}
              className="rounded-md p-1 text-white/55 hover:bg-white/[0.08] hover:text-white/90"
            >
              <MoreHorizontal className="h-4 w-4" />
            </button>
            {menuOpen && (
              <div role="menu" className="absolute right-0 z-10 mt-1 w-44 rounded-md border border-white/10 bg-[#1c1f2a] py-1 text-[12.5px] shadow-lg">
                {targetGroups.map((g) => (
                  <button
                    key={g.id}
                    role="menuitem"
                    type="button"
                    className="block w-full px-3 py-1.5 text-left hover:bg-white/[0.06]"
                    onClick={() => { setMenuOpen(false); void rulesMoveRule(rule.id, g.id); }}
                  >
                    Move to {g.name}
                  </button>
                ))}
                <button
                  role="menuitem"
                  type="button"
                  className="block w-full px-3 py-1.5 text-left text-rose-300 hover:bg-rose-500/10"
                  onClick={() => { setMenuOpen(false); void rulesRemoveRule(rule.id); }}
                >
                  Delete
                </button>
              </div>
            )}
          </div>
          <ChevronRight className="h-3.5 w-3.5 text-white/40 transition-transform group-hover:translate-x-0.5" />
        </div>
      )}
    </div>
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
