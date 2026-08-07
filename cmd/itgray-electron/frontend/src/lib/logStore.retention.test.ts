import { beforeEach, describe, expect, it, vi } from "vitest";

const { startMock, stopMock, eventHandlers } = vi.hoisted(() => ({
  startMock: vi.fn(),
  stopMock: vi.fn(),
  eventHandlers: {} as Record<string, (p: unknown) => void>,
}));

vi.mock("@/lib/itg/runtime", () => ({
  EventsOn: (name: string, cb: (p: unknown) => void) => {
    eventHandlers[name] = cb;
    return () => { delete eventHandlers[name]; };
  },
}));

vi.mock("@/lib/itg/LogsService", () => ({
  Start: () => startMock(),
  Stop: () => stopMock(),
}));

import { CAP, __entriesForTest, clearLogs, startLogs, stopLogs } from "./logStore";
import type { LogEntry } from "./logStore";

const line = (seq: number): LogEntry => ({
  seq, time: "t", level: "INFO", source: "bridge", message: `m${seq}`,
});

const emit = (entry: LogEntry) => eventHandlers["log:line"]?.(entry);

describe("logStore retention", () => {
  beforeEach(() => {
    startMock.mockReset();
    stopMock.mockReset();
    stopLogs();
    clearLogs();
  });

  it("seeds from the bridge tail and appends live lines after it", async () => {
    startMock.mockResolvedValue({ entries: [line(1), line(2)] });
    await startLogs();
    emit(line(3));

    expect(__entriesForTest().map((e) => e.seq)).toEqual([1, 2, 3]);
  });

  // The page only ever renders the newest slice, so the store must stay
  // bounded however long the tab is left open on a chatty tunnel — every
  // incoming line is filtered against the whole store.
  it("evicts the oldest entries once the cap is reached", async () => {
    startMock.mockResolvedValue({ entries: [] });
    await startLogs();

    for (let seq = 1; seq <= CAP + 500; seq++) emit(line(seq));

    const entries = __entriesForTest();
    expect(entries).toHaveLength(CAP);
    expect(entries[entries.length - 1].seq).toBe(CAP + 500);
    expect(entries[0].seq).toBe(501);
  });

  it("keeps the retained window close to what the page renders", () => {
    // 6000 retained against an 800-line render window meant most of the
    // filtering work on each new line was for entries nobody could see.
    expect(CAP).toBeLessThanOrEqual(2000);
  });
});
