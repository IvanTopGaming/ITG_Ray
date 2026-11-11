import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { ChevronDown, Globe, Search } from "lucide-react";
import { CountryFlag } from "@/components/controls/CountryFlag";
import type { hub } from "@/lib/itg/models";

type ServerView = hub.ServerView;

function Flag({ code }: { code: string }) {
  if (!code) return <Globe className="h-[14px] w-[14px] shrink-0 text-white/40" />;
  return <CountryFlag code={code} className="h-[12px] w-[18px] shrink-0 rounded-[2px] object-cover" />;
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
  const rootRef = useRef<HTMLDivElement>(null);
  const selected = value ? servers.find((s) => s.id === value) ?? null : null;
  const label = value ? selected?.name ?? fallbackName ?? value : t("ruleEditor.server.current");

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter((s) => s.name.toLowerCase().includes(q) || s.origin.toLowerCase().includes(q));
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

  return (
    <div ref={rootRef} className="relative mt-2 flex items-center gap-3 px-2">
      <span className="text-[12px] text-white/55">{t("ruleEditor.server.label")}</span>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex min-w-[220px] items-center gap-2 rounded-lg border border-white/12 bg-black/30 px-3 py-1.5 text-left text-[12.5px] text-white/90 hover:bg-white/[0.06]"
      >
        {selected ? <Flag code={selected.country} /> : null}
        <span className="flex-1 truncate">{label}</span>
        <ChevronDown className="h-3.5 w-3.5 text-white/50" />
      </button>
      {open && (
        <div className="absolute left-2 top-full z-30 mt-1 w-[300px] rounded-xl border border-white/10 bg-[#0b1430] p-1.5 shadow-xl">
          <div className="mb-1 flex items-center gap-2 rounded-lg bg-white/[0.05] px-2 py-1.5">
            <Search className="h-3.5 w-3.5 text-white/40" />
            <input
              autoFocus
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("ruleEditor.server.search")}
              className="w-full bg-transparent text-[12.5px] text-white outline-none placeholder:text-white/35"
            />
          </div>
          <div role="listbox" className="max-h-[260px] overflow-y-auto">
            <button
              type="button"
              role="option"
              aria-selected={value === null}
              onClick={() => pick(null)}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12.5px] text-white/85 hover:bg-white/[0.06]"
            >
              {t("ruleEditor.server.current")}
            </button>
            {filtered.map((s) => (
              <button
                key={s.id}
                type="button"
                role="option"
                aria-selected={s.id === value}
                onClick={() => pick(s)}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12.5px] text-white/85 hover:bg-white/[0.06]"
              >
                <Flag code={s.country} />
                <span className="flex-1 truncate">{s.name}</span>
                <span className="truncate text-[10.5px] text-white/35">{s.origin === "manual" ? "" : s.origin}</span>
              </button>
            ))}
            {filtered.length === 0 && (
              <div className="px-2 py-2 text-[12px] text-white/40">{t("ruleEditor.server.none")}</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
