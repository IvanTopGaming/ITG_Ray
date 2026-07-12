import { useSyncExternalStore } from "react";
import { EventsOn } from "@/lib/itg/runtime";
import * as Logs from "@/lib/itg/LogsService";

export type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";
export type LogSource = "bridge" | "sing-box" | "xray";
export type LogEntry = {
  seq: number; time: string; level: LogLevel; source: LogSource; message: string;
};

const LEVEL_ORDER: Record<LogLevel, number> = { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 };
const CAP = 6000;

let entries: LogEntry[] = [];
let lastSeq = 0;
const listeners = new Set<() => void>();
function notify() { for (const l of listeners) l(); }

function append(list: LogEntry[]) {
  let changed = false;
  for (const en of list) {
    if (en.seq <= lastSeq) continue;
    entries.push(en);
    lastSeq = en.seq;
    changed = true;
  }
  if (entries.length > CAP) entries = entries.slice(entries.length - CAP);
  if (changed) { entries = entries.slice(); notify(); }
}

let off: (() => void) | null = null;

export async function startLogs(): Promise<void> {
  const res = (await Logs.Start()) as { entries?: LogEntry[] } | null;
  if (res?.entries) append(res.entries);
  if (!off) off = EventsOn("log:line", (p: any) => { if (p && typeof p.seq === "number") append([p as LogEntry]); });
}

export function stopLogs(): void {
  if (off) { off(); off = null; }
  void Logs.Stop();
}

export function clearLogs(): void {
  entries = [];
  notify();
}

export function filterLogs(
  list: LogEntry[],
  opts: { sources: Set<string>; minLevel: LogLevel; search: string },
): LogEntry[] {
  const floor = LEVEL_ORDER[opts.minLevel];
  const q = opts.search.trim().toLowerCase();
  return list.filter(
    (e) =>
      opts.sources.has(e.source) &&
      LEVEL_ORDER[e.level] >= floor &&
      (q === "" || e.message.toLowerCase().includes(q)),
  );
}

export function useLogEntries(): LogEntry[] {
  return useSyncExternalStore(
    (cb) => { listeners.add(cb); return () => listeners.delete(cb); },
    () => entries,
    () => entries,
  );
}
