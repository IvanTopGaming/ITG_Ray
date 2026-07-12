import { describe, it, expect } from "vitest";
import { filterLogs, type LogEntry } from "./logStore";

const e = (over: Partial<LogEntry>): LogEntry => ({
  seq: 1, time: "t", level: "INFO", source: "bridge", message: "m", ...over,
});

describe("filterLogs", () => {
  it("filters by source, level floor, and search substring", () => {
    const entries = [
      e({ seq: 1, source: "bridge", level: "DEBUG", message: "verbose" }),
      e({ seq: 2, source: "sing-box", level: "WARN", message: "dns timeout" }),
      e({ seq: 3, source: "xray", level: "ERROR", message: "reset" }),
    ];
    const out = filterLogs(entries, {
      sources: new Set(["sing-box", "xray"]),
      minLevel: "INFO",
      search: "",
    });
    expect(out.map((x) => x.seq)).toEqual([2, 3]);

    const searched = filterLogs(entries, {
      sources: new Set(["bridge", "sing-box", "xray"]),
      minLevel: "DEBUG",
      search: "dns",
    });
    expect(searched.map((x) => x.seq)).toEqual([2]);
  });
});
