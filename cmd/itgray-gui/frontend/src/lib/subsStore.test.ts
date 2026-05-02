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

describe("subsStore — actions identity", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    const mod = await import("./subsStore");
    mod.__resetForTests();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("actions object identity is stable across re-renders", async () => {
    mockList.mockResolvedValueOnce([]);
    const { useSubs } = await import("./subsStore");
    const { result, rerender } = renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });
    const firstActions = result.current.actions;
    rerender();
    expect(result.current.actions).toBe(firstActions);
  });
});

describe("subsStore — refresh coalescing", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    const mod = await import("./subsStore");
    mod.__resetForTests();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("does NOT fan-out concurrent List calls when sub:synced arrives during in-flight refresh", async () => {
    let registeredHandler: (() => void) | null = null;
    mockEventsOn.mockImplementationOnce((_n: string, h: (...data: unknown[]) => void) => {
      registeredHandler = h as () => void;
      return () => {};
    });
    // Initial List for ensureInit; resolve immediately.
    mockList.mockResolvedValueOnce([]);
    const { useSubs } = await import("./subsStore");
    renderHook(() => useSubs());
    await act(async () => { await vi.runAllTimersAsync(); });
    expect(mockList).toHaveBeenCalledTimes(1);

    // Subsequent List takes time — set up a deferred resolve we control.
    let resolveSecondList: ((v: unknown[]) => void) | null = null;
    mockList.mockImplementationOnce(() => new Promise(resolve => { resolveSecondList = resolve as (v: unknown[]) => void; }));

    // Trigger first event → debounce timer schedules refresh after 50ms.
    act(() => { registeredHandler!(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(60); });
    // List should have been called twice now (initial + first triggered).
    expect(mockList).toHaveBeenCalledTimes(2);

    // While the second List is in flight (not yet resolved), fire 5 more events.
    act(() => {
      registeredHandler!();
      registeredHandler!();
      registeredHandler!();
      registeredHandler!();
      registeredHandler!();
    });
    // No new List calls should happen yet — they're coalesced.
    expect(mockList).toHaveBeenCalledTimes(2);

    // Now resolve the in-flight List; trailing refresh schedules after 50ms.
    mockList.mockResolvedValueOnce([]);  // for the trailing refresh
    await act(async () => {
      resolveSecondList!([]);
      await vi.advanceTimersByTimeAsync(60);
    });
    // Exactly one trailing refresh, regardless of how many events fired.
    expect(mockList).toHaveBeenCalledTimes(3);
  });
});

describe("subsStore.add", () => {
  beforeEach(async () => {
    vi.clearAllMocks();
    const mod = await import("./subsStore");
    mod.__resetForTests();
  });

  it("optimistically inserts a row with status=syncing on success", async () => {
    mockList.mockResolvedValueOnce([]);
    mockAdd.mockResolvedValueOnce({
      id: "s-new", name: "new", url: "https://x",
      lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "",
      serverCount: 0, updateInterval: 3600,
    });
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      await result.current.actions.add("new", "https://x");
    });

    expect(mockAdd).toHaveBeenCalledWith("https://x", "new");
    expect(result.current.state.inFlight.adding).toBe(false);
    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs).toHaveLength(1);
      expect(result.current.state.load.subs[0].id).toBe("s-new");
      expect(result.current.state.load.subs[0].status).toBe("syncing");
    }
  });

  it("rethrows on backend error and leaves state unchanged", async () => {
    mockList.mockResolvedValueOnce([]);
    mockAdd.mockRejectedValueOnce(new Error("invalid url"));
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      await expect(
        result.current.actions.add("bad", "ftp://x"),
      ).rejects.toThrow(/invalid url/);
    });

    expect(result.current.state.inFlight.adding).toBe(false);
    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs).toHaveLength(0);
    }
  });
});

describe("subsStore.remove", () => {
  const seed = [
    { id: "s1", name: "a", url: "https://1", lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "", serverCount: 0, updateInterval: 3600 },
    { id: "s2", name: "b", url: "https://2", lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "", serverCount: 0, updateInterval: 3600 },
  ];

  beforeEach(async () => {
    vi.clearAllMocks();
    const mod = await import("./subsStore");
    mod.__resetForTests();
  });

  it("optimistically removes the row on success", async () => {
    mockList.mockResolvedValueOnce(seed);
    mockRemove.mockResolvedValueOnce(undefined);
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      await result.current.actions.remove("s1");
    });
    expect(mockRemove).toHaveBeenCalledWith("s1");
    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs.map(s => s.id)).toEqual(["s2"]);
    }
  });

  it("reverts the row and rethrows on backend error", async () => {
    mockList.mockResolvedValueOnce(seed);
    mockRemove.mockRejectedValueOnce(new Error("disk full"));
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await Promise.resolve(); });

    await act(async () => {
      await expect(result.current.actions.remove("s1")).rejects.toThrow(/disk full/);
    });

    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs.map(s => s.id).sort()).toEqual(["s1", "s2"]);
    }
  });
});

describe("subsStore.syncOne / syncAll + debounce", () => {
  const seed = [
    { id: "s1", name: "a", url: "https://1", lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "", serverCount: 0, updateInterval: 3600 },
    { id: "s2", name: "b", url: "https://2", lastSyncAt: "0001-01-01T00:00:00Z", lastSyncStatus: "", serverCount: 0, updateInterval: 3600 },
  ];

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    const mod = await import("./subsStore");
    mod.__resetForTests();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("syncOne marks the row 'syncing' during the call and clears it after", async () => {
    mockList.mockResolvedValueOnce(seed);
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await vi.runAllTimersAsync(); });

    let resolveSync: (() => void) | null = null;
    mockSyncOne.mockImplementationOnce(() => new Promise<void>(res => { resolveSync = res; }));

    let syncPromise!: Promise<void>;
    await act(async () => {
      syncPromise = result.current.actions.syncOne("s1");
      await Promise.resolve();
    });

    expect(result.current.state.inFlight.syncing.has("s1")).toBe(true);
    expect(result.current.state.load.kind).toBe("ready");
    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs.find(s => s.id === "s1")?.status).toBe("syncing");
    }

    await act(async () => {
      resolveSync!();
      await syncPromise;
    });

    expect(result.current.state.inFlight.syncing.has("s1")).toBe(false);
  });

  it("sub:synced event triggers a debounced refresh that fires once for a burst", async () => {
    let registeredHandler: (() => void) | null = null;
    mockEventsOn.mockImplementationOnce((_n: string, h: () => void) => {
      registeredHandler = h;
      return () => {};
    });
    mockList.mockResolvedValueOnce(seed);
    const { useSubs } = await import("./subsStore");
    renderHook(() => useSubs());
    await act(async () => { await vi.runAllTimersAsync(); });
    expect(mockList).toHaveBeenCalledTimes(1);

    mockList.mockResolvedValue(seed);
    act(() => {
      registeredHandler!();
      registeredHandler!();
      registeredHandler!();
    });
    await act(async () => { await vi.advanceTimersByTimeAsync(60); });
    expect(mockList).toHaveBeenCalledTimes(2);
  });

  it("syncAll marks all rows 'syncing' and calls the backend once", async () => {
    mockList.mockResolvedValueOnce(seed);
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await vi.runAllTimersAsync(); });

    let resolveSync: (() => void) | null = null;
    mockSyncAll.mockImplementationOnce(() => new Promise<void>(res => { resolveSync = res; }));

    let syncPromise!: Promise<void>;
    await act(async () => {
      syncPromise = result.current.actions.syncAll();
      await Promise.resolve();
    });

    expect(result.current.state.load.kind).toBe("ready");
    if (result.current.state.load.kind === "ready") {
      expect(result.current.state.load.subs.every(s => s.status === "syncing")).toBe(true);
    }
    expect(result.current.state.inFlight.syncing.size).toBe(2);
    expect(mockSyncAll).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveSync!();
      await syncPromise;
    });

    expect(result.current.state.inFlight.syncing.size).toBe(0);
  });

  it("syncOne clears inFlight even if backend rejects, and rethrows", async () => {
    mockList.mockResolvedValueOnce(seed);
    mockSyncOne.mockRejectedValueOnce(new Error("network gone"));
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await vi.runAllTimersAsync(); });

    await act(async () => {
      await expect(result.current.actions.syncOne("s1")).rejects.toThrow(/network gone/);
    });
    expect(result.current.state.inFlight.syncing.has("s1")).toBe(false);
  });

  it("syncAll clears inFlight even if backend rejects, and rethrows", async () => {
    mockList.mockResolvedValueOnce(seed);
    mockSyncAll.mockRejectedValueOnce(new Error("disk on fire"));
    const { useSubs } = await import("./subsStore");
    const { result } = renderHook(() => useSubs());
    await act(async () => { await vi.runAllTimersAsync(); });

    await act(async () => {
      await expect(result.current.actions.syncAll()).rejects.toThrow(/disk on fire/);
    });
    expect(result.current.state.inFlight.syncing.size).toBe(0);
  });
});
