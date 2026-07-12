import { describe, it, expect } from "vitest";
import { conditionChips } from "./ConditionSummary";

describe("conditionChips", () => {
  it("domains → '{kind}: {value}'", () => {
    expect(conditionChips("domains", { domains: [{ kind: "suffix", value: "netflix.com" }, { kind: "exact", value: "a.com" }] }))
      .toEqual(["suffix: netflix.com", "exact: a.com"]);
  });
  it("ports → single and range", () => {
    expect(conditionChips("ports", { ports: [{ single: 443 }, { from: 1000, to: 2000 }] }))
      .toEqual(["443", "1000:2000"]);
  });
  it("string lists pass through (ip_cidrs, geo, processes, protocols)", () => {
    expect(conditionChips("ip_cidrs", { ip_cidrs: ["10.0.0.0/8"] })).toEqual(["10.0.0.0/8"]);
    expect(conditionChips("geo", { geo: ["geosite:netflix"] })).toEqual(["geosite:netflix"]);
    expect(conditionChips("protocols", { protocols: ["tcp", "udp"] })).toEqual(["tcp", "udp"]);
  });
  it("missing condition → []", () => {
    expect(conditionChips("domains", {})).toEqual([]);
  });
});
