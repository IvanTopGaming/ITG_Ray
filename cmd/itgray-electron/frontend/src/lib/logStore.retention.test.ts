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

import { CAP, __entriesForTest, __resetForTest, clearLogs, startLogs, stopLogs } from "./logStore";
import type { LogEntry } from "./logStore";

const line = (seq: number): LogEntry => ({
  seq, time: "t", level: "INFO", source: "bridge", message: `m${seq}`,
});

const emit = (...list: LogEntry[]) => eventHandlers["log:batch"]?.({ entries: list });

describe("logStore retention", () => {
  beforeEach(() => {
    startMock.mockReset();
    stopMock.mockReset();
    stopLogs();
    __resetForTest();
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

    for (let seq = 1; seq <= CAP + 500; seq += 50)
      emit(...Array.from({ length: 50 }, (_, i) => line(seq + i)));

    const entries = __entriesForTest();
    expect(entries).toHaveLength(CAP);
    expect(entries[entries.length - 1].seq).toBe(CAP + 500);
    expect(entries[0].seq).toBe(501);
  });

  it("retains exactly the newest 2000 lines", () => {
    expect(CAP).toBe(2000);
  });

  // The listener is registered before Start resolves so nothing published in
  // between is lost; the seed then overlaps those live lines.
  it("merges a Start seed that overlaps lines already received live", async () => {
    let resolveStart: (v: unknown) => void = () => {};
    startMock.mockReturnValue(new Promise((r) => { resolveStart = r; }));
    const pending = startLogs();
    emit(line(4), line(5));
    resolveStart({ entries: [line(2), line(3), line(4)] });
    await pending;

    expect(__entriesForTest().map((e) => e.seq)).toEqual([2, 3, 4, 5]);
  });

  it("re-seeding after reopening the tab keeps the window and adds only new lines", async () => {
    startMock.mockResolvedValue({ entries: [line(1), line(2)] });
    await startLogs();
    stopLogs();
    startMock.mockResolvedValue({ entries: [line(1), line(2), line(3)] });
    await startLogs();

    expect(__entriesForTest().map((e) => e.seq)).toEqual([1, 2, 3]);
  });

  it("does not bring cleared lines back when the tab re-seeds", async () => {
    startMock.mockResolvedValue({ entries: [line(1), line(2)] });
    await startLogs();
    clearLogs();
    stopLogs();
    startMock.mockResolvedValue({ entries: [line(1), line(2), line(3)] });
    await startLogs();

    expect(__entriesForTest().map((e) => e.seq)).toEqual([3]);
  });
});
