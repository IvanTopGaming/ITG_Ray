import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

const mockList = vi.fn();
const mockAdd = vi.fn();
const mockEdit = vi.fn();
const mockRemove = vi.fn();
const mockSyncOne = vi.fn();
const mockSyncAll = vi.fn();
const mockEventsOn = vi.fn((_eventName: string, _callback: (...data: unknown[]) => void) => () => {});

vi.mock("../../wailsjs/go/bindings/SubsService", () => ({
  List: (...args: unknown[]) => mockList(...args),
  Add: (...args: unknown[]) => mockAdd(...args),
  Edit: (...args: unknown[]) => mockEdit(...args),
  Remove: (...args: unknown[]) => mockRemove(...args),
  SyncOne: (...args: unknown[]) => mockSyncOne(...args),
  SyncAll: (...args: unknown[]) => mockSyncAll(...args),
}));
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (eventName: string, callback: (...data: unknown[]) => void) => mockEventsOn(eventName, callback),
}));

describe("subsStore — init + refresh + useSubs", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    const mod = await import("./subsStore");
    mod.__resetForTests();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("initial mount calls List() and transitions to ready", async () => {
    mockList.mockResolvedValueOnce([
      { id: "s1", name: "alpha", url: "https://x", lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "", serverCount: 0 },
    ]);
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());

    expect(result.current.state.load.kind).toBe("loading");
    await act(async () => { await Promise.resolve(); });
    expect(mockList).toHaveBeenCalledTimes(1);
    expect(result.current.state.load.kind).toBe("ready");
    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs).toHaveLength(1);
      expect(result.current.state.load.subs[0].name).toBe("alpha");
    }
  });

  it("List failure surfaces as error state", async () => {
    mockList.mockRejectedValueOnce(new Error("disk gone"));
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());

    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(result.current.state.load.kind).toBe("error");
    if (result.current.state.load.kind === "error") {
      expect(result.current.state.load.message).toContain("disk gone");
    }
  });

  it("two consumers see the same state via useSyncExternalStore", async () => {
    mockList.mockResolvedValueOnce([]);
    const { useSubs } = await import("./subsStore");
    const a = renderHook(() => useSubs());
    const b = renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });
    expect(a.result.current.state).toBe(b.result.current.state);
  });

  it("EventsOn registers exactly once across N hook mounts", async () => {
    mockList.mockResolvedValueOnce([]);
    const { useSubs } = await import("./subsStore");
    renderHook(() => useSubs());
    renderHook(() => useSubs());
    renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });
    expect(mockEventsOn).toHaveBeenCalledTimes(1);
    expect(mockEventsOn).toHaveBeenCalledWith("sub:synced", expect.any(Function));
  });
});
