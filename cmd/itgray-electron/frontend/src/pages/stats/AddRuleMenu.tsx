import { useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { cn } from "@/lib/cn";
import type { Conditions, GroupView } from "@/lib/rulesStore";
import type { StatsConn } from "./format";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

export type CreateRuleRequest = {
  groupId: string;
  beforeId?: string;
  prefill: { name: string; conditions: Conditions };
};

type Option = "app" | "exact" | "suffix" | "ip";

function isIP(s: string): boolean {
  return /^\d{1,3}(\.\d{1,3}){3}$/.test(s) || s.includes(":");
}

export function placeNewRule(conn: StatsConn, groups: GroupView[]): { groupId: string; beforeId?: string } {
  if (conn.ruleId && conn.ruleId !== "final" && conn.ruleGroupId && !conn.ruleGroupLocked) {
    return { groupId: conn.ruleGroupId, beforeId: conn.ruleId };
  }
  const target = groups.find((g) => !g.locked && g.enabled) ?? groups.find((g) => !g.locked);
  if (!target) return { groupId: "" };
  const first = target.rules[0]?.id;
  return first ? { groupId: target.id, beforeId: first } : { groupId: target.id };
}

function availableOptions(conn: StatsConn): Option[] {
  const out: Option[] = [];
  if (conn.app) out.push("app");
  const domain = conn.host && !isIP(conn.host) ? conn.host : "";
  if (conn.suffix) out.push("suffix");
  if (domain && domain !== conn.suffix) out.push("exact");
  const ip = conn.destIp || (conn.host && isIP(conn.host) ? conn.host : "");
  if (!domain && ip) out.push("ip");
  return out;
}

function defaultSelection(opts: Option[]): Set<Option> {
  if (opts.includes("suffix")) return new Set(["suffix"]);
  if (opts.includes("exact")) return new Set(["exact"]);
  if (opts.includes("ip")) return new Set(["ip"]);
  return new Set(opts.slice(0, 1));
}

export function buildConditions(conn: StatsConn, picked: Set<Option>): Conditions {
  const c: Conditions = {};
  if (picked.has("app") && conn.app) c.processes = [conn.app];
  const domains: NonNullable<Conditions["domains"]> = [];
  if (picked.has("exact") && conn.host) domains.push({ kind: "exact", value: conn.host });
  if (picked.has("suffix") && conn.suffix) domains.push({ kind: "suffix", value: conn.suffix });
  if (domains.length) c.domains = domains;
  const ip = conn.destIp || conn.host;
  if (picked.has("ip") && ip) c.ip_cidrs = [ip.includes(":") ? `${ip}/128` : `${ip}/32`];
  return c;
}

export function AddRuleMenu({
  conn,
  groups,
  onCreate,
}: {
  conn: StatsConn;
  groups: GroupView[];
  onCreate: (req: CreateRuleRequest) => void;
}) {
  const { t } = useTranslation();
  const opts = useMemo(() => availableOptions(conn), [conn]);
  const [open, setOpen] = useState(false);
  const [picked, setPicked] = useState<Set<Option>>(() => defaultSelection(opts));
  const [coords, setCoords] = useState<{ right: number; top?: number; bottom?: number }>({ right: 0, top: 0 });
  const ref = useRef<HTMLDivElement>(null);
  const popRef = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (!ref.current?.contains(target) && !popRef.current?.contains(target)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const labelFor = (o: Option): string => {
    switch (o) {
      case "app":
        return t("stats.addRule.app", { app: conn.app });
      case "exact":
        return t("stats.addRule.exact", { host: conn.host });
      case "suffix":
        return t("stats.addRule.suffix", { domain: conn.suffix });
      default:
        return t("stats.addRule.ip", { ip: conn.destIp || conn.host });
    }
  };

  const toggle = (o: Option) => {
    const next = new Set(picked);
    if (next.has(o)) next.delete(o);
    else next.add(o);
    setPicked(next);
  };

  const create = () => {
    const place = placeNewRule(conn, groups);
    const name = conn.suffix || conn.host || conn.app || t("stats.addRule.untitled");
    onCreate({ ...place, prefill: { name, conditions: buildConditions(conn, picked) } });
    setOpen(false);
  };

  if (opts.length === 0) return null;

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        ref={btnRef}
        onClick={() => {
          if (!open) {
            setPicked(defaultSelection(opts));
            const r = btnRef.current?.getBoundingClientRect();
            if (r) {
              const right = Math.max(8, window.innerWidth - r.right);
              setCoords(window.innerHeight - r.bottom < 240 ? { right, bottom: window.innerHeight - r.top + 4 } : { right, top: r.bottom + 4 });
            }
          }
          setOpen((v) => !v);
        }}
        aria-label={`+ ${t("stats.addRule.button")}`}
        className="flex h-7 items-center gap-1 whitespace-nowrap rounded-md border border-accent-start/35 px-2 text-[11px] text-accent-start transition-colors hover:bg-accent-start/10"
      >
        <Plus className="h-3 w-3" />
        {t("stats.addRule.button")}
      </button>
      {createPortal(
        <AnimatePresence>
          {open && (
            <motion.div
              initial={{ opacity: 0, y: -4, scale: 0.98 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: -4, scale: 0.98 }}
              transition={{ duration: 0.15, ease: SNAP_EASE }}
              ref={popRef}
              style={{ position: "fixed", right: coords.right, top: coords.top, bottom: coords.bottom }}
              className="z-50 w-[300px] rounded-lg border border-white/[0.18] bg-bg-1/95 p-2 text-left shadow-[0_18px_36px_-10px_rgba(0,0,0,0.6)] backdrop-blur-xl"
            >
              <div className="px-1.5 pb-1.5 text-[10px] font-medium uppercase tracking-[0.14em] text-white/40">
                {t("stats.addRule.title")}
              </div>
              {opts.map((o) => (
                <label key={o} className="flex cursor-pointer items-center gap-2.5 rounded-md px-1.5 py-1.5 text-[12px] text-white/85 hover:bg-white/[0.06]">
                  <input
                    type="checkbox"
                    checked={picked.has(o)}
                    onChange={() => toggle(o)}
                    aria-label={labelFor(o)}
                    className="h-3.5 w-3.5 accent-[#7ed4ff]"
                  />
                  <span className="min-w-0 flex-1 truncate">{labelFor(o)}</span>
                </label>
              ))}
              {picked.size > 1 && <div className="px-1.5 pt-1 text-[10.5px] text-white/40">{t("stats.addRule.andHint")}</div>}
              <button
                type="button"
                disabled={picked.size === 0}
                onClick={create}
                className={cn(
                  "mt-2 h-8 w-full rounded-md bg-gradient-to-br from-accent-start to-accent-mid text-[12px] font-semibold text-white",
                  picked.size === 0 && "cursor-not-allowed opacity-40",
                )}
              >
                {t("stats.addRule.create")}
              </button>
            </motion.div>
          )}
        </AnimatePresence>,
        document.body,
      )}
    </div>
  );
}
