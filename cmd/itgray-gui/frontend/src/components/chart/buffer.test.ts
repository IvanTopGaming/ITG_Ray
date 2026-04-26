import { describe, it, expect } from "vitest";
import { SpeedBuffer } from "./buffer";

describe("SpeedBuffer", () => {
  it("trims to capacity", () => {
    const b = new SpeedBuffer(3);
    for (let i = 0; i < 5; i++) b.push({ upBps: i, downBps: i * 2, t: i });
    expect(b.values()).toHaveLength(3);
    expect(b.values()[0].upBps).toBe(2);
  });
  it("returns empty initially", () => {
    expect(new SpeedBuffer(5).values()).toEqual([]);
  });
});
