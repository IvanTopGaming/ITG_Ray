import { describe, expect, it } from "vitest";
import { pickQuickSwitch } from "./quickSwitch";
import type { hub } from "../../wailsjs/go/models";

type ServerView = hub.ServerView;

function s(over: Partial<ServerView>): ServerView {
  return {
    id: over.id ?? "x",
    name: over.name ?? "X",
    country: "",
    address: "",
    transport: "tcp",
    security: "tls",
    latencyMs: 0,
    origin: "manual",
    favorite: false,
    tags: [],
    ...over,
  };
}

describe("pickQuickSwitch", () => {
  it("returns empty list for empty input", () => {
    expect(pickQuickSwitch([], 3)).toEqual([]);
  });

  it("favorites come before non-favorites", () => {
    const all = [
      s({ id: "a", favorite: false, latencyMs: 10 }),
      s({ id: "b", favorite: true, latencyMs: 50 }),
      s({ id: "c", favorite: true, latencyMs: 20 }),
    ];
    const out = pickQuickSwitch(all, 3);
    expect(out.map(x => x.id)).toEqual(["c", "b", "a"]);
  });

  it("within each group, sorts by latency ascending", () => {
    const all = [
      s({ id: "a", favorite: false, latencyMs: 100 }),
      s({ id: "b", favorite: false, latencyMs: 30 }),
      s({ id: "c", favorite: false, latencyMs: 60 }),
    ];
    const out = pickQuickSwitch(all, 3);
    expect(out.map(x => x.id)).toEqual(["b", "c", "a"]);
  });

  it("treats latencyMs=0 as 'never probed' (sorts last via Infinity)", () => {
    const all = [
      s({ id: "a", favorite: false, latencyMs: 0 }),
      s({ id: "b", favorite: false, latencyMs: 60 }),
      s({ id: "c", favorite: false, latencyMs: 30 }),
    ];
    const out = pickQuickSwitch(all, 3);
    expect(out.map(x => x.id)).toEqual(["c", "b", "a"]);
  });

  it("caps result at n", () => {
    const all = [
      s({ id: "a", favorite: true, latencyMs: 10 }),
      s({ id: "b", favorite: true, latencyMs: 20 }),
      s({ id: "c", favorite: true, latencyMs: 30 }),
      s({ id: "d", favorite: true, latencyMs: 40 }),
    ];
    expect(pickQuickSwitch(all, 3).map(x => x.id)).toEqual(["a", "b", "c"]);
  });
});
