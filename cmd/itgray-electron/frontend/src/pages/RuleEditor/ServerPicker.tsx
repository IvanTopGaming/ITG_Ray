import { useEffect, useMemo, useRef, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { Check, ChevronDown, Globe, Search, Zap } from "lucide-react";
import { CountryFlag } from "@/components/controls/CountryFlag";
import { cn } from "@/lib/cn";
import type { hub } from "@/lib/itg/models";

type ServerView = hub.ServerView;

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

function pingTone(ms: number): string {
  if (ms <= 0) return "text-white/35";
  if (ms < 25) return "text-success";
  if (ms < 60) return "text-warn";
  if (ms < 120) return "text-white/65";
  return "text-danger";
}

function Flag({ code }: { code: string }) {
  if (!code) return <Globe className="h-[14px] w-[14px] shrink-0 text-white/40" />;
  return (
    <CountryFlag
      code={code}
      className="h-[13px] w-[19px] shrink-0 rounded-[2px] object-cover shadow-[0_0_0_1px_rgba(255,255,255,0.08)]"
    />
  );
}

function Ping({ ms }: { ms: number }) {
  return (
    <span className={cn("min-w-[52px] text-right font-mono text-[10.5px] font-semibold tabular-nums", pingTone(ms))}>
      {ms > 0 ? `${ms} ms` : "—"}
    </span>
  );
}

export function ServerPicker({
  value,
  servers,
  onChange,
  fallbackName,
}: {
  value: string | null;
  servers: ServerView[];
  onChange: (s: ServerView | null) => void;
  fallbackName?: string;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [dropUp, setDropUp] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const selected = value ? servers.find((s) => s.id === value) ?? null : null;
  const missing = value !== null && selected === null;

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const visible = q
      ? servers.filter((s) => s.name.toLowerCase().includes(q) || s.origin.toLowerCase().includes(q))
      : servers;
    const byOrigin = new Map<string, ServerView[]>();
    for (const s of visible) {
      const key = s.origin || "manual";
      const list = byOrigin.get(key);
      if (list) list.push(s);
      else byOrigin.set(key, [s]);
    }
    return [...byOrigin.entries()];
  }, [servers, query]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
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

  const pick = (s: ServerView | null) => {
    onChange(s);
    setOpen(false);
    setQuery("");
  };

  const triggerLabel = selected?.name ?? (missing ? fallbackName ?? value : t("ruleEditor.server.current"));

  return (
    <div ref={rootRef} className="relative flex items-center gap-3 px-1 pb-1 pt-2.5">
      <span className="shrink-0 text-[10px] font-medium uppercase tracking-[0.12em] text-white/40">
        {t("ruleEditor.server.label")}
      </span>
      <button
        type="button"
        ref={triggerRef}
        onClick={() => {
          const rect = triggerRef.current?.getBoundingClientRect();
          if (rect) setDropUp(window.innerHeight - rect.bottom < 360 && rect.top > window.innerHeight - rect.bottom);
          setOpen((o) => !o);
        }}
        aria-haspopup="listbox"
        aria-expanded={open}
        className={cn(
          "flex min-w-0 flex-1 items-center gap-2.5 rounded-lg border bg-white/[0.04] px-3 py-2 text-left text-[12.5px] text-white transition-colors duration-instant ease-snap",
          open ? "border-accent-start/50 bg-white/[0.06]" : "border-white/15 hover:border-white/25 hover:bg-white/[0.05]",
          missing && "border-amber-400/40",
        )}
      >
        {selected ? <Flag code={selected.country} /> : missing ? <Globe className="h-[14px] w-[14px] shrink-0 text-amber-300/80" /> : <Zap className="h-[14px] w-[14px] shrink-0 text-accent-start" />}
        <span className={cn("min-w-0 flex-1 truncate", missing && "text-amber-200")}>{triggerLabel}</span>
        {selected && selected.origin !== "manual" && (
          <span className="hidden max-w-[40%] truncate text-[11px] text-white/40 sm:inline">{selected.origin}</span>
        )}
        {selected && <Ping ms={selected.latencyMs} />}
        <ChevronDown className={cn("h-3.5 w-3.5 shrink-0 text-white/45 transition-transform duration-instant ease-snap", open && "rotate-180")} />
      </button>

      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: dropUp ? 4 : -4, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: dropUp ? 4 : -4, scale: 0.98 }}
            transition={{ duration: 0.15, ease: SNAP_EASE }}
            className={cn(
              "absolute right-1 z-30 w-[min(26rem,90vw)]",
              dropUp ? "bottom-full mb-1" : "top-full mt-1",
            )}
            data-drop={dropUp ? "up" : "down"}
          >
            <div className="rounded-lg rounded-lg border border-white/[0.18] bg-bg-1/95 p-1 shadow-[0_18px_36px_-10px_rgba(0,0,0,0.6)] backdrop-blur-xl">
            <div className="mb-1 flex items-center gap-2 rounded-md border border-white/[0.08] bg-white/[0.04] px-2.5 py-1.5">
              <Search className="h-3.5 w-3.5 shrink-0 text-white/40" />
              <input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("ruleEditor.server.search")}
                className="w-full bg-transparent text-[12px] text-white outline-none placeholder:text-white/35"
              />
            </div>
            <div role="listbox" className="max-h-[260px] overflow-y-auto">
              <button
                type="button"
                role="option"
                aria-selected={value === null}
                onClick={() => pick(null)}
                className={cn(
                  "flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors duration-instant ease-snap",
                  value === null ? "bg-white/[0.12] text-white" : "text-white/80 hover:bg-white/[0.06] hover:text-white",
                )}
              >
                <Zap className="h-[14px] w-[14px] shrink-0 text-accent-start" />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="text-[12px]">{t("ruleEditor.server.current")}</span>
                  <span className="text-[10.5px] text-white/40">{t("ruleEditor.server.currentHint")}</span>
                </span>
                {value === null && <Check className="h-3.5 w-3.5 shrink-0 text-accent-start" />}
              </button>
              {groups.map(([origin, list]) => (
                <div key={origin} className="mt-1">
                  <div className="px-2.5 pb-0.5 pt-1.5 text-[9.5px] font-medium uppercase tracking-[0.14em] text-white/35">
                    {origin === "manual" ? t("ruleEditor.server.manual") : origin}
                  </div>
                  {list.map((s) => {
                    const active = s.id === value;
                    return (
                      <button
                        key={s.id}
                        type="button"
                        role="option"
                        aria-selected={active}
                        onClick={() => pick(s)}
                        className={cn(
                          "flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap",
                          active ? "bg-white/[0.12] text-white" : "text-white/75 hover:bg-white/[0.06] hover:text-white",
                        )}
                      >
                        <Flag code={s.country} />
                        <span className="min-w-0 flex-1 truncate">{s.name}</span>
                        <Ping ms={s.latencyMs} />
                        <span className="flex w-3.5 shrink-0 justify-end">
                          {active && <Check className="h-3.5 w-3.5 text-accent-start" />}
                        </span>
                      </button>
                    );
                  })}
                </div>
              ))}
              {groups.length === 0 && (
                <div className="px-2.5 py-3 text-center text-[11.5px] text-white/40">{t("ruleEditor.server.none")}</div>
              )}
            </div>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
