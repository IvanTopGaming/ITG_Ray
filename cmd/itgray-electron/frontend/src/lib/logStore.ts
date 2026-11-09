import { useSyncExternalStore } from "react";
import { EventsOn } from "@/lib/itg/runtime";
import * as Logs from "@/lib/itg/LogsService";

export type LogLevel = "DEBUG" | "INFO" | "WARN" | "ERROR";
export type LogSource = "bridge" | "sing-box" | "xray" | "helper";
export type LogEntry = {
  seq: number; time: string; level: LogLevel; source: LogSource; message: string;
};

const LEVEL_ORDER: Record<LogLevel, number> = { DEBUG: 0, INFO: 1, WARN: 2, ERROR: 3 };

// How many lines the renderer keeps, and how many the bridge hands back when
// the tab opens (startTailLimit). The page shows the newest CAP lines across
// sources and then follows live batches, so the window stays at CAP however
// long the tab is open; the full history stays available through Export.
export const CAP = 2000;

let entries: LogEntry[] = [];
// Highest seq ever ingested, and the seq Clear was pressed at. Lines at or
// below clearedThrough are ignored so reopening the tab (which re-seeds from
// the bridge tail) does not bring cleared lines back.
let highestSeq = 0;
let clearedThrough = 0;
const listeners = new Set<() => void>();
function notify() { for (const l of listeners) l(); }

// ingest merges lines into the store by seq. Live batches normally arrive in
// order and strictly after what is already held, which is the cheap path; the
// Start seed can overlap or precede lines that already arrived through the
// live subscription (the listener is registered before Start resolves so no
// line is lost in between), so that case dedupes and re-sorts.
function ingest(list: LogEntry[]) {
  const fresh = list.filter((e) => typeof e?.seq === "number" && e.seq > clearedThrough);
  if (fresh.length === 0) return;
  const tail = entries.length ? entries[entries.length - 1].seq : 0;
  let next: LogEntry[];
  if (fresh[0].seq > tail && isAscending(fresh)) {
    next = entries.concat(fresh);
  } else {
    const known = new Set(entries.map((e) => e.seq));
    next = entries.concat(fresh.filter((e) => !known.has(e.seq) && (known.add(e.seq), true)));
    if (next.length === entries.length) return;
    next.sort((a, b) => a.seq - b.seq);
  }
  if (next.length > CAP) next = next.slice(next.length - CAP);
  for (const e of fresh) if (e.seq > highestSeq) highestSeq = e.seq;
  entries = next;
  notify();
}

function isAscending(list: LogEntry[]): boolean {
  for (let i = 1; i < list.length; i++) if (list[i].seq <= list[i - 1].seq) return false;
  return true;
}

let off: (() => void) | null = null;

export async function startLogs(): Promise<void> {
  if (!off) {
    off = EventsOn("log:batch", (p: any) => {
      if (p && Array.isArray(p.entries)) ingest(p.entries as LogEntry[]);
    });
  }
  const res = (await Logs.Start()) as { entries?: LogEntry[] } | null;
  if (res?.entries) ingest(res.entries);
}

export function stopLogs(): void {
  if (off) { off(); off = null; }
  void Logs.Stop();
}

export function clearLogs(): void {
  const tail = entries.length ? entries[entries.length - 1].seq : 0;
  clearedThrough = Math.max(clearedThrough, highestSeq, tail);
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

// Test-only view of the retained entries.
export function __entriesForTest(): LogEntry[] { return entries; }

// Test-only reset of module state between cases.
export function __resetForTest(): void {
  entries = [];
  highestSeq = 0;
  clearedThrough = 0;
}

export function useLogEntries(): LogEntry[] {
  return useSyncExternalStore(
    (cb) => { listeners.add(cb); return () => listeners.delete(cb); },
    () => entries,
    () => entries,
  );
}
