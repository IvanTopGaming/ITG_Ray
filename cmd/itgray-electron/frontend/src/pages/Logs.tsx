import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Dropdown } from "@/components/controls/Dropdown";
import { ExportLogs, SaveLogs } from "@/lib/itg/LogsService";
import {
  useLogEntries,
  startLogs,
  stopLogs,
  clearLogs,
  filterLogs,
  type LogLevel,
  type LogSource,
} from "@/lib/logStore";

const SOURCES: LogSource[] = ["bridge", "sing-box", "xray"];
const LEVELS: LogLevel[] = ["DEBUG", "INFO", "WARN", "ERROR"];

const SOURCES_STORAGE_KEY = "itg.logs.sources";

function loadSources(): Set<string> {
  try {
    const raw = localStorage.getItem(SOURCES_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as unknown;
      if (Array.isArray(parsed))
        return new Set(parsed.filter((s) => SOURCES.includes(s as LogSource)));
    }
  } catch {
    // malformed / unavailable storage → fall back to all sources
  }
  return new Set(SOURCES);
}
const levelClass: Record<LogLevel, string> = {
  DEBUG: "text-white/40",
  INFO: "text-accent-start",
  WARN: "text-warn",
  ERROR: "text-danger",
};
const sourceClass: Record<LogSource, string> = {
  bridge: "text-accent-start",
  "sing-box": "text-[#c9a6ff]",
  xray: "text-success",
};

export function Logs() {
  const { t } = useTranslation();
  const all = useLogEntries();
  const [sources, setSources] = useState<Set<string>>(loadSources);
  const [minLevel, setMinLevel] = useState<LogLevel>("INFO");
  const [search, setSearch] = useState("");
  const [wrap, setWrap] = useState(true);
  const [pinned, setPinned] = useState(true);
  const [copied, setCopied] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    void startLogs();
    return () => stopLogs();
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem(SOURCES_STORAGE_KEY, JSON.stringify([...sources]));
    } catch {
      // storage unavailable → filter simply won't persist
    }
  }, [sources]);

  const rows = useMemo(
    () => filterLogs(all, { sources, minLevel, search }),
    [all, sources, minLevel, search],
  );

  useEffect(() => {
    if (pinned && scrollRef.current)
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [rows, pinned]);

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
    setPinned(atBottom);
  };

  const toggleSource = (s: string) =>
    setSources((prev) => {
      const next = new Set(prev);
      next.has(s) ? next.delete(s) : next.add(s);
      return next;
    });

  const copyVisible = () => {
    void navigator.clipboard.writeText(rows.map(fmtLine).join("\n"));
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  };
  const onExport = async () => {
    const { text } = await ExportLogs();
    await SaveLogs(text);
  };

  const errors = rows.filter((r) => r.level === "ERROR").length;
  const warns = rows.filter((r) => r.level === "WARN").length;

  return (
    <div className="flex h-full flex-col">
      <div className="mb-4">
        <h1 className="text-[22px] font-semibold tracking-tight">
          {t("logs.title")}
        </h1>
        <p className="mt-1 text-[13px] text-white/50">{t("logs.subtitle")}</p>
      </div>

      <div className="mb-3 flex flex-wrap items-center gap-2.5">
        <div className="flex gap-1 rounded-[10px] glass-dim p-1">
          {SOURCES.map((s) => (
            <button
              key={s}
              onClick={() => toggleSource(s)}
              className={`rounded-md px-3 py-1 text-[12px] ${sources.has(s) ? "bg-white/15 text-white" : "text-white/55"}`}
            >
              {s}
            </button>
          ))}
        </div>
        <Dropdown
          value={minLevel}
          onChange={(v) => setMinLevel(v as LogLevel)}
          options={LEVELS.map((l) => ({ value: l, label: `${l}+` }))}
          ariaLabel={t("logs.level")}
        />
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("logs.search")}
          className="min-w-[140px] flex-1 rounded-[10px] glass-dim px-3 py-1.5 text-[12.5px] outline-none"
        />
        <button
          onClick={() => setWrap((w) => !w)}
          className={`rounded-[10px] px-2.5 py-1.5 text-[12px] ${wrap ? "text-accent-start" : "glass-dim text-white/70"}`}
        >
          {t("logs.wrap")}
        </button>
        <button
          onClick={() => void window.itg.logs.openFolder()}
          className="rounded-[10px] glass-dim px-2.5 py-1.5 text-[12px] text-white/80"
        >
          {t("logs.folder")}
        </button>
        <button
          onClick={copyVisible}
          className={`rounded-[10px] px-2.5 py-1.5 text-[12px] transition-colors duration-instant ${copied ? "bg-success/20 text-success" : "glass-dim text-white/80"}`}
        >
          {copied ? t("logs.copied") : t("logs.copy")}
        </button>
        <button
          onClick={() => void onExport()}
          className="rounded-[10px] glass-dim px-2.5 py-1.5 text-[12px] text-white/80"
        >
          {t("logs.export")}
        </button>
        <button
          onClick={() => clearLogs()}
          className="rounded-[10px] glass-dim px-2.5 py-1.5 text-[12px] text-white/80 hover:text-danger"
        >
          {t("logs.clear")}
        </button>
      </div>

      <div className="relative min-h-0 flex-1 overflow-hidden rounded-xl glass-dim">
        <div
          ref={scrollRef}
          onScroll={onScroll}
          className="h-full overflow-auto p-2.5 font-mono text-[12px] leading-relaxed"
        >
          {rows.length === 0 && (
            <div className="p-4 text-white/40">{t("logs.empty")}</div>
          )}
          {rows.map((r) => (
            <div
              key={r.seq}
              className={`px-2 ${r.level === "ERROR" ? "bg-danger/10" : r.level === "WARN" ? "bg-warn/[0.08]" : ""}`}
            >
              <span className="text-white/35">{r.time.slice(11, 23)}</span>{" "}
              <span className={`font-semibold ${levelClass[r.level]}`}>
                {r.level}
              </span>{" "}
              <span className={sourceClass[r.source]}>{r.source}</span>{" "}
              <span
                className={
                  wrap
                    ? "whitespace-pre-wrap break-words text-white/85"
                    : "whitespace-pre text-white/85"
                }
              >
                {r.message}
              </span>
            </div>
          ))}
        </div>
        {!pinned && (
          <button
            onClick={() => setPinned(true)}
            className="absolute bottom-3 right-3 rounded-full bg-btn-accent px-3 py-1.5 text-[11.5px] font-semibold text-[#04121f]"
          >
            {t("logs.jump")}
          </button>
        )}
      </div>

      <div className="flex justify-between px-0.5 pb-1 pt-2 font-mono text-[11px] text-white/40">
        <span>
          {t("logs.status", { sources: sources.size, lines: rows.length })}
        </span>
        <span>
          <span className="text-danger">●</span> {errors} &nbsp;{" "}
          <span className="text-warn">●</span> {warns}
        </span>
      </div>
    </div>
  );
}

function fmtLine(r: {
  time: string;
  level: string;
  source: string;
  message: string;
}): string {
  return `${r.time} ${r.level} ${r.source} ${r.message}`;
}
