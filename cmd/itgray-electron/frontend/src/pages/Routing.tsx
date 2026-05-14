import { useState, useRef, useEffect, type HTMLAttributes, type SyntheticEvent } from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { Lock, ChevronRight, Plus, MoreHorizontal, GripVertical } from "lucide-react";
import { DndContext, DragOverlay, closestCenter, PointerSensor, useSensor, useSensors, type DragEndEvent, type DragOverEvent, type DragStartEvent } from "@dnd-kit/core";
import { SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import {
  useRules,
  rulesAddGroup,
  rulesEditGroup,
  rulesRemoveGroup,
  rulesRemoveRule,
  rulesReplaceAll,
  type GroupView,
  type RuleView,
} from "@/lib/rulesStore";
import { Toggle } from "@/components/controls/Toggle";
import { ConfirmDialog } from "@/components/controls/ConfirmDialog";
import { cn } from "@/lib/cn";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];
const SMOOTH_EASE: [number, number, number, number] = [0.32, 0.72, 0, 1];

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

const popoverVariants: Variants = {
  hidden: { opacity: 0, y: -4, scale: 0.96 },
  show: { opacity: 1, y: 0, scale: 1, transition: { duration: 0.18, ease: SNAP_EASE } },
  exit: { opacity: 0, y: -4, scale: 0.96, transition: { duration: 0.18, ease: SNAP_EASE } },
};

const groupCardVariants: Variants = {
  initial: { opacity: 0, scale: 0.96, height: 0, overflow: "hidden" },
  animate: { opacity: 1, scale: 1, height: "auto", overflow: "visible", transition: { duration: 0.3, ease: SMOOTH_EASE } },
  exit: { opacity: 0, scale: 0.96, height: 0, overflow: "hidden", transition: { duration: 0.25, ease: SMOOTH_EASE } },
};

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

// Move a rule from its current group into another group at a specific
// index. No-op when the source group is locked, the target group is
// missing/locked, or when the rule isn't found anywhere.
export function moveRuleAcrossGroups(
  groups: GroupView[],
  ruleId: string,
  targetGroupId: string,
  targetIndex: number,
): GroupView[] {
  const sourceGroup = groups.find((g) => g.rules.some((r) => r.id === ruleId));
  if (!sourceGroup || sourceGroup.locked) return groups;
  const targetGroup = groups.find((g) => g.id === targetGroupId);
  if (!targetGroup || targetGroup.locked) return groups;
  if (sourceGroup.id === targetGroupId) return groups;
  const fromIdx = sourceGroup.rules.findIndex((r) => r.id === ruleId);
  if (fromIdx < 0) return groups;
  const rule = sourceGroup.rules[fromIdx];
  return groups.map((g) => {
    if (g.id === sourceGroup.id) {
      return { ...g, rules: g.rules.filter((_, i) => i !== fromIdx) };
    }
    if (g.id === targetGroupId) {
      const next = g.rules.slice();
      next.splice(Math.max(0, Math.min(targetIndex, next.length)), 0, rule);
      return { ...g, rules: next };
    }
    return g;
  });
}

export function Routing() {
  const { groups: backendGroups, defaultAction, lastError } = useRules();
  const [localGroups, setLocalGroups] = useState(backendGroups);

  useEffect(() => {
    setLocalGroups(backendGroups);
  }, [backendGroups]);

  const [adding, setAdding] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);

  // PointerSensor with a 3px activation threshold: drag starts as soon
  // as the pointer moves enough to indicate intent, but a stray click
  // on the handle (e.g. user just trying to focus or hover) doesn't
  // accidentally initiate a drag.
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 3 } }),
  );

  const safetyGroup = localGroups.find((g) => g.id === "safety");
  const userGroups = localGroups.filter((g) => g.id !== "safety");

  // Resolve the active drag target so DragOverlay can render its ghost
  // outside of any clipping/stacking-context container (glass-regular
  // creates a stacking context via backdrop-filter, which clips the
  // dragged element when it visually crosses into another card).
  const activeRule = activeId
    ? localGroups.flatMap((g) => g.rules).find((r) => r.id === activeId)
    : null;

  function onDragStart(e: DragStartEvent) {
    setActiveId(String(e.active.id));
  }

  // Cross-group live preview. While the user drags a rule into a
  // different group, rebuild localGroups so the rule visually appears in
  // the target group. dnd-kit's sortable strategy then animates the
  // target group's rules to make room (and the source group to close
  // the gap) using its CSS transitions. State updates are no-ops when
  // the rule is already in the target group (moveRuleAcrossGroups
  // returns the same reference), so this is safe to call on every drag
  // event.
  function onDragOver(e: DragOverEvent) {
    const { active, over } = e;
    if (!over) return;
    const activeId = String(active.id);
    const overId = String(over.id);
    if (activeId === overId) return;

    // Skip group reordering — handled visually by the sortable strategy
    // inside the user-groups SortableContext.
    if (localGroups.some((g) => g.id === activeId)) return;

    const sourceGroup = localGroups.find((g) => g.rules.some((r) => r.id === activeId));
    if (!sourceGroup) return;

    let targetGroup: GroupView | undefined;
    let overRuleId: string | null = null;
    const directOverGroup = localGroups.find((g) => g.id === overId);
    if (directOverGroup) {
      targetGroup = directOverGroup;
    } else {
      targetGroup = localGroups.find((g) => g.rules.some((r) => r.id === overId));
      overRuleId = overId;
    }
    if (!targetGroup || targetGroup.locked) return;
    // Same-group sort is handled by dnd-kit's strategy transforms; no
    // state update needed mid-drag.
    if (sourceGroup.id === targetGroup.id) return;

    // Insert before/after the over rule based on whether the active
    // rule's *center* is below the over rule's center. Earlier we
    // compared active-TOP to over-MIDDLE, which is asymmetric and made
    // it impossible to drop *after* the last row in a group (the active
    // top would still be above the last row's middle even when the
    // user was clearly aiming below it).
    let newIndex: number;
    if (overRuleId) {
      const overIdx = targetGroup.rules.findIndex((r) => r.id === overRuleId);
      const activeRect = active.rect.current.translated;
      const activeMid = activeRect ? activeRect.top + activeRect.height / 2 : 0;
      const overMid = over.rect.top + over.rect.height / 2;
      const isBelowOverItem = activeMid > overMid;
      newIndex = overIdx + (isBelowOverItem ? 1 : 0);
    } else {
      newIndex = targetGroup.rules.length;
    }

    const targetId = targetGroup.id;
    setLocalGroups((prev) => moveRuleAcrossGroups(prev, activeId, targetId, newIndex));
  }

  function onDragEnd(e: DragEndEvent) {
    setActiveId(null);
    if (!e.over) {
      // Dropped outside any droppable. Revert any onDragOver preview
      // moves so we don't persist a partially-thought-out change.
      if (localGroups !== backendGroups) setLocalGroups(backendGroups);
      return;
    }
    const activeId = String(e.active.id);
    const overId = String(e.over.id);

    // Group reorder
    if (localGroups.some((g) => g.id === activeId)) {
      if (activeId === overId) return;
      const nextUser = reorderGroups(userGroups, activeId, overId);
      const finalGroups = safetyGroup ? [safetyGroup, ...nextUser] : nextUser;
      setLocalGroups(finalGroups);
      void rulesReplaceAll({ defaultAction, groups: finalGroups });
      return;
    }

    // Rule drop. After onDragOver, the active rule already lives in
    // whichever group the user dragged it into; we only need to settle
    // the final position within that group and persist.
    const activeGroup = localGroups.find((g) => g.rules.some((r) => r.id === activeId));
    if (!activeGroup || activeGroup.locked) {
      if (localGroups !== backendGroups) setLocalGroups(backendGroups);
      return;
    }

    // If dropped on another rule in the same group, finalize the sort
    // position; otherwise the onDragOver-applied state is already right.
    const overInGroup = activeGroup.rules.find((r) => r.id === overId);
    if (overInGroup && activeId !== overId) {
      const fromIdx = activeGroup.rules.findIndex((r) => r.id === activeId);
      const toIdx = activeGroup.rules.findIndex((r) => r.id === overId);
      if (fromIdx !== toIdx) {
        const nextGroups = reorderRules(localGroups, activeGroup.id, fromIdx, toIdx);
        setLocalGroups(nextGroups);
        void rulesReplaceAll({ defaultAction, groups: nextGroups });
        return;
      }
    }

    // Persist whatever state we ended up in (may already differ from
    // backend because of onDragOver moves).
    if (localGroups !== backendGroups) {
      void rulesReplaceAll({ defaultAction, groups: localGroups });
    }
  }

  return (
    <motion.div
      className="flex flex-col gap-3"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      <motion.header variants={sectionVariants} className="flex items-baseline justify-between pb-2">
        <div>
          <h1 className="text-[20px] font-semibold tracking-tight">Routing rules</h1>
          <p className="mt-1 text-[12px] text-white/55">Per-domain, per-IP, per-process routing. Top of the list matches first.</p>
        </div>
        <motion.button
          type="button"
          onClick={() => setAdding(true)}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="flex items-center gap-1.5 rounded-md bg-white/[0.08] px-3 py-1.5 text-[12px] font-medium text-white/85 hover:bg-white/[0.12]"
        >
          <Plus className="h-3.5 w-3.5" /> Add group
        </motion.button>
      </motion.header>
      {lastError && (
        <motion.div
          variants={sectionVariants}
          role="alert"
          className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-[12px] text-rose-200"
        >
          {lastError}
        </motion.div>
      )}
      <AnimatePresence initial={false}>
        {adding && <AddGroupRow key="add-group-row" onCancel={() => setAdding(false)} />}
      </AnimatePresence>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={onDragStart}
        onDragOver={onDragOver}
        onDragEnd={onDragEnd}
        onDragCancel={() => {
          setActiveId(null);
          if (localGroups !== backendGroups) setLocalGroups(backendGroups);
        }}
      >
        {safetyGroup && <GroupCard group={safetyGroup} allGroups={localGroups} />}
        <SortableContext items={userGroups.map((g) => g.id)} strategy={verticalListSortingStrategy}>
          <AnimatePresence initial={false}>
            {userGroups.map((g) => (
              <SortableGroupCard
                key={g.id}
                group={g}
                allGroups={localGroups}
              />
            ))}
          </AnimatePresence>
        </SortableContext>
        {/* DragOverlay portals its child to document.body, so the ghost
            escapes every group card's stacking context (glass-regular's
            backdrop-filter creates one, which would otherwise clip the
            element when it crosses card boundaries). */}
        <DragOverlay dropAnimation={null} style={{ cursor: "grabbing" }}>
          {activeRule ? (
            <div className="flex items-center gap-2">
              {/* Grip handle replica — matches the actual SortableRuleRow handle */}
              <div className="rounded p-1.5 text-white/55">
                <GripVertical className="h-4 w-4" />
              </div>
              {/* Rule card — same colors as the in-list rule row, sized to
                  content so the overlay is visibly shorter than the real row. */}
              <div className="inline-flex items-center gap-3 rounded-md bg-white/[0.06] px-3 py-2 text-[12.5px] shadow-[0_18px_36px_-10px_rgba(0,0,0,0.55)] ring-1 ring-white/15">
                <span
                  className={cn(
                    "rounded px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider",
                    activeRule.action === "proxy" ? "bg-sky-500/20 text-sky-200"
                    : activeRule.action === "direct" ? "bg-amber-500/20 text-amber-200"
                    : "bg-rose-500/20 text-rose-200",
                  )}
                >
                  {activeRule.action}
                </span>
                <span className="text-white/85">{activeRule.name}</span>
                <span className="text-[11px] text-white/45">{summarise(activeRule.conditions)}</span>
              </div>
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
    </motion.div>
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
    <motion.div
      initial={{ opacity: 0, height: 0, marginBottom: -12 }}
      animate={{ opacity: 1, height: "auto", marginBottom: 0 }}
      exit={{ opacity: 0, height: 0, marginBottom: -12 }}
      transition={{ duration: 0.28, ease: SMOOTH_EASE }}
      style={{ overflow: "hidden" }}
    >
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
        <motion.button
          type="button"
          onClick={() => void submit()}
          disabled={!trimmed}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="rounded-md bg-sky-500/30 px-3 py-1 text-[12px] font-medium text-sky-100 hover:bg-sky-500/40 disabled:cursor-not-allowed disabled:opacity-40"
        >
          Add
        </motion.button>
        <motion.button
          type="button"
          onClick={cancel}
          whileTap={{ scale: 0.96 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="rounded-md px-2 py-1 text-[12px] text-white/55 hover:text-white/90"
        >
          Cancel
        </motion.button>
      </div>
    </motion.div>
  );
}

type DragHandleProps = {
  attributes: HTMLAttributes<HTMLElement>;
  listeners: Record<string, (event: SyntheticEvent) => void> | undefined;
};

function GroupCard({ group, dragHandle, allGroups }: { group: GroupView; dragHandle?: DragHandleProps; allGroups: GroupView[] }) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [menuCoords, setMenuCoords] = useState<{ left: number; top: number } | null>(null);
  const menuBtnRef = useRef<HTMLButtonElement>(null);
  const menuPopRef = useRef<HTMLDivElement>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    if (!menuOpen || !menuBtnRef.current) return;
    const r = menuBtnRef.current.getBoundingClientRect();
    // w-32 is 128px
    setMenuCoords({ left: r.right - 128, top: r.bottom + 4 });
  }, [menuOpen]);

  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      const t = e.target as Node;
      if (menuBtnRef.current?.contains(t) || menuPopRef.current?.contains(t)) return;
      setMenuOpen(false);
    };
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") setMenuOpen(false); };
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", esc);
    };
  }, [menuOpen]);

  function handleAddRule() {
    // Don't persist a stub on the server — navigate to the editor in
    // "create" mode and let the user Save when they're ready.
    navigate("/routing/new", { state: { mode: "create", groupId: group.id } });
  }

  return (
    <>
      <motion.section
        variants={sectionVariants}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className={cn("glass-regular flex flex-col gap-2 rounded-2xl p-4", !group.enabled && "opacity-60")}
      >
        <header className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {dragHandle && (
              <button
                type="button"
                {...dragHandle.attributes}
                {...dragHandle.listeners}
                aria-label={`Drag ${group.name}`}
                className="-ml-1 cursor-grab rounded p-1.5 text-white/35 hover:bg-white/[0.06] hover:text-white/80 active:cursor-grabbing"
              >
                <GripVertical className="h-4 w-4" />
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
                  ref={menuBtnRef}
                  type="button"
                  aria-label={`${group.name} menu`}
                  onClick={(e) => { e.stopPropagation(); setMenuOpen((v) => !v); }}
                  className="rounded-md p-1 text-white/55 hover:bg-white/[0.08] hover:text-white/90"
                >
                  <MoreHorizontal className="h-4 w-4" />
                </button>
                {menuCoords && createPortal(
                  <AnimatePresence>
                    {menuOpen && (
                      <motion.div
                        ref={menuPopRef}
                        role="menu"
                        onClick={(e) => e.stopPropagation()}
                        variants={popoverVariants}
                        initial="hidden"
                        animate="show"
                        exit="exit"
                        style={{ position: "fixed", left: menuCoords.left, top: menuCoords.top, zIndex: 1000, transformOrigin: "top right" }}
                        className="w-32 rounded-lg border border-white/[0.18] bg-bg-1/95 p-1 shadow-[0_18px_36px_-10px_rgba(0,0,0,0.6)] backdrop-blur-xl"
                      >
                        <button
                          role="menuitem"
                          type="button"
                          className="flex w-full items-center rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap text-white/75 hover:bg-white/[0.06] hover:text-white"
                          onClick={() => { setRenaming(true); setMenuOpen(false); }}
                        >
                          Rename
                        </button>
                        <button
                          role="menuitem"
                          type="button"
                          className="flex w-full items-center rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap text-rose-400 hover:bg-rose-500/15 hover:text-rose-300"
                          onClick={() => { setConfirmDelete(true); setMenuOpen(false); }}
                        >
                          Delete
                        </button>
                      </motion.div>
                    )}
                  </AnimatePresence>,
                  document.body
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
          <SortableContext items={group.rules.map((r) => r.id)} strategy={verticalListSortingStrategy}>
            <ul className="flex flex-col gap-1">
              {group.rules.map((r) => <SortableRuleRow key={r.id} rule={r} group={group} allGroups={allGroups} />)}
            </ul>
          </SortableContext>
        )}
        {!group.locked && (
          <motion.button
            type="button"
            onClick={handleAddRule}
            whileHover={{ y: -1 }}
            whileTap={{ scale: 0.96 }}
            transition={{ duration: 0.18, ease: SNAP_EASE }}
            className="self-start rounded-md bg-white/[0.04] px-3 py-1.5 text-[11.5px] text-white/65 hover:bg-white/[0.08] hover:text-white/90"
          >
            + Add rule
          </motion.button>
        )}
      </motion.section>
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

function SortableGroupCard({ group, allGroups }: { group: GroupView; allGroups: GroupView[] }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: group.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.7 : 1,
  };
  return (
    <motion.div
      variants={groupCardVariants}
      initial="initial"
      animate="animate"
      exit="exit"
    >
      <div ref={setNodeRef} style={style}>
        <GroupCard group={group} allGroups={allGroups} dragHandle={{ attributes: attributes as HTMLAttributes<HTMLElement>, listeners: listeners as Record<string, (event: SyntheticEvent) => void> | undefined }} />
      </div>
    </motion.div>
  );
}

function SortableRuleRow({ rule, group, allGroups }: { rule: RuleView; group: GroupView; allGroups: GroupView[] }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: rule.id });
  // While this row is the active drag target, hide it entirely — the
  // DragOverlay ghost is what the user sees following the cursor.
  // Keeping opacity 0 (not display:none) preserves the slot height so
  // the surrounding list layout doesn't collapse. No framer-motion
  // wrapper here: when onDragOver moves the row across groups, React
  // remounts it under a new parent, and any AnimatePresence-driven
  // exit/enter would duplicate the slot visually (height collapsing in
  // source + height growing in target = overlap). dnd-kit's own CSS
  // transitions handle the visible reorder smoothly.
  const style = {
    transform: CSS.Transform.toString(transform),
    transition: isDragging ? "none" : transition,
    opacity: isDragging ? 0 : 1,
  };
  return (
    <li ref={setNodeRef} style={style} className="flex items-center gap-2">
      <button
        type="button"
        {...attributes}
        {...listeners}
        aria-label={`Drag ${rule.name}`}
        className="cursor-grab rounded p-1.5 text-white/35 hover:bg-white/[0.06] hover:text-white/80 active:cursor-grabbing"
      >
        <GripVertical className="h-4 w-4" />
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
  const [menuCoords, setMenuCoords] = useState<{ left: number; top: number } | null>(null);
  const menuBtnRef = useRef<HTMLButtonElement>(null);
  const menuPopRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();
  // group + allGroups are kept in the signature for backwards compatibility
  // with parent call sites; the menu no longer reads cross-group context.
  void group;
  void allGroups;
  const actionStyle =
    rule.action === "proxy" ? "bg-sky-500/20 text-sky-200"
    : rule.action === "direct" ? "bg-amber-500/20 text-amber-200"
    : "bg-rose-500/20 text-rose-200";

  useEffect(() => {
    if (!menuOpen || !menuBtnRef.current) return;
    const r = menuBtnRef.current.getBoundingClientRect();
    // Right-align: pop's right edge = button's right edge; width 11rem = 176px.
    setMenuCoords({ left: r.right - 176, top: r.bottom + 4 });
  }, [menuOpen]);

  useEffect(() => {
    if (!menuOpen) return;
    const handler = (e: MouseEvent) => {
      const t = e.target as Node;
      if (menuBtnRef.current?.contains(t) || menuPopRef.current?.contains(t)) return;
      setMenuOpen(false);
    };
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") setMenuOpen(false); };
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", esc);
    };
  }, [menuOpen]);
  return (
    <motion.div
      onClick={groupLocked ? undefined : () => navigate(`/routing/${rule.id}`)}
      className={cn(
        "group flex items-center justify-between rounded-md text-[12.5px]",
        groupLocked ? "bg-white/[0.02] px-3 py-2" : "bg-white/[0.04] hover:bg-white/[0.06] cursor-pointer",
      )}
    >
      <motion.div
        whileHover={groupLocked ? undefined : { y: -1 }}
        whileTap={groupLocked ? undefined : { scale: 0.99 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        className={cn(
          "flex min-w-0 flex-1 items-center gap-3",
          groupLocked ? "" : "px-3 py-2",
        )}
      >
        <span className={cn("rounded px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider", actionStyle)}>
          {rule.action}
        </span>
        <span className="text-white/85">{rule.name}</span>
        <span className="text-[11px] text-white/45">{summarise(rule.conditions)}</span>
      </motion.div>
      {!groupLocked && (
        <div
          className="flex items-center gap-1 pr-3"
          onClick={(e) => e.stopPropagation()}
          onPointerDown={(e) => e.stopPropagation()}
        >
          <div className="relative">
            <button
              ref={menuBtnRef}
              type="button"
              aria-label={`${rule.name} menu`}
              onClick={(e) => { e.stopPropagation(); setMenuOpen((v) => !v); }}
              className="rounded-md p-1 text-white/55 hover:bg-white/[0.08] hover:text-white/90"
            >
              <MoreHorizontal className="h-4 w-4" />
            </button>
          {menuCoords && createPortal(
            <AnimatePresence>
              {menuOpen && (
                <motion.div
                  ref={menuPopRef}
                  role="menu"
                  onClick={(e) => e.stopPropagation()}
                  initial={{ opacity: 0, y: -4, scale: 0.96 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -4, scale: 0.96 }}
                  transition={{ duration: 0.16, ease: SNAP_EASE }}
                  style={{ position: "fixed", left: menuCoords.left, top: menuCoords.top, zIndex: 1000 }}
                  className="w-44 rounded-lg border border-white/[0.18] bg-bg-1/95 p-1 shadow-[0_18px_36px_-10px_rgba(0,0,0,0.6)] backdrop-blur-xl"
                >
                  <button
                    role="menuitem"
                    type="button"
                    className="flex w-full items-center rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap text-white/75 hover:bg-white/[0.06] hover:text-white"
                    onClick={() => { setMenuOpen(false); navigate(`/routing/${rule.id}`); }}
                  >
                    Edit
                  </button>
                  <button
                    role="menuitem"
                    type="button"
                    className="flex w-full items-center rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap text-rose-400 hover:bg-rose-500/15 hover:text-rose-300"
                    onClick={() => { setMenuOpen(false); void rulesRemoveRule(rule.id); }}
                  >
                    Delete
                  </button>
                </motion.div>
              )}
            </AnimatePresence>,
            document.body
          )}
          </div>
          <ChevronRight className="h-3.5 w-3.5 text-white/40 transition-transform group-hover:translate-x-0.5" />
        </div>
      )}
    </motion.div>
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
