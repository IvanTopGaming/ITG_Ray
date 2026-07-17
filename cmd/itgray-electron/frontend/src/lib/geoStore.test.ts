import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { eventHandlers } = vi.hoisted(() => ({
  eventHandlers: {} as Record<string, (...args: any[]) => void>,
}));

vi.mock("@/lib/itg/runtime", () => ({
  EventsOn: (name: string, cb: (...args: any[]) => void) => {
    eventHandlers[name] = cb;
    return () => {
      delete eventHandlers[name];
    };
  },
}));

import { geoBegin, geoEnd, getGeoSnapshot, __resetGeoForTest } from "./geoStore";

function fireProgress(done: number, total: number) {
  eventHandlers["geo:progress"]?.({ done, total });
}

beforeEach(() => {
  vi.useFakeTimers();
  __resetGeoForTest();
});
afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe("geoStore", () => {
  it("geoBegin marks refreshing and clears any prior result", () => {
    geoEnd("ok");
    expect(getGeoSnapshot().result).toBe("ok");
    geoBegin();
    const s = getGeoSnapshot();
    expect(s.refreshing).toBe(true);
    expect(s.result).toBeNull();
    expect(s.done).toBe(0);
    expect(s.total).toBe(0);
  });

  it("progress events update counters and download-active without ending the refresh", () => {
    geoBegin();
    fireProgress(3, 10);
    let s = getGeoSnapshot();
    expect(s.done).toBe(3);
    expect(s.total).toBe(10);
    expect(s.active).toBe(true);
    expect(s.refreshing).toBe(true);
    expect(s.result).toBeNull();

    // a final done==total event ends download-active but the refresh stays in flight
    fireProgress(10, 10);
    s = getGeoSnapshot();
    expect(s.active).toBe(false);
    expect(s.refreshing).toBe(true);
  });

  it("geoEnd records the result, stops refreshing, keeps counters (survives tab switch)", () => {
    geoBegin();
    fireProgress(10, 10);
    geoEnd("ok");
    const s = getGeoSnapshot();
    expect(s.refreshing).toBe(false);
    expect(s.result).toBe("ok");
    expect(s.total).toBe(10);
    // state lives at module scope, so a remounted component reads it unchanged
    expect(getGeoSnapshot().result).toBe("ok");
  });

  it("geoEnd('error') records the error result", () => {
    geoBegin();
    geoEnd("error");
    expect(getGeoSnapshot().result).toBe("error");
  });

  it("auto-clears the result after 2s", () => {
    geoBegin();
    geoEnd("ok");
    expect(getGeoSnapshot().result).toBe("ok");
    vi.advanceTimersByTime(1999);
    expect(getGeoSnapshot().result).toBe("ok");
    vi.advanceTimersByTime(1);
    expect(getGeoSnapshot().result).toBeNull();
  });
});
