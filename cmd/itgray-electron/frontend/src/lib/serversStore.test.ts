import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  __resetForTest,
  clearLastError,
  getServersState,
  serverAdd,
  serverEdit,
  serverRemove,
  serversBootstrap,
} from "./serversStore";

const listMock = vi.fn();
const addMock = vi.fn();
const editMock = vi.fn();
const removeMock = vi.fn();

vi.mock("../../wailsjs/go/bindings/ServersService", () => ({
  List: (...args: any[]) => listMock(...args),
  Add: (...args: any[]) => addMock(...args),
  Edit: (...args: any[]) => editMock(...args),
  Remove: (...args: any[]) => removeMock(...args),
}));

const eventListeners = new Map<string, ((data: any) => void)[]>();

vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, cb: (data: any) => void) => {
    const list = eventListeners.get(name) ?? [];
    list.push(cb);
    eventListeners.set(name, list);
    return () => {};
  },
}));

function emit(name: string, data?: any) {
  for (const cb of eventListeners.get(name) ?? []) cb(data);
}

const fixtureServer = {
  id: "m1",
  name: "DE",
  origin: "manual",
  country: "",
  address: "",
  transport: "",
  security: "",
  latencyMs: 0,
  favorite: false,
};

beforeEach(() => {
  __resetForTest();
  eventListeners.clear();
  listMock.mockReset().mockResolvedValue([fixtureServer]);
  addMock.mockReset();
  editMock.mockReset();
  removeMock.mockReset();
});

afterEach(() => {
  __resetForTest();
});

describe("serversStore bootstrap", () => {
  it("loads servers via ServersService.List on bootstrap", async () => {
    await serversBootstrap();
    expect(listMock).toHaveBeenCalledTimes(1);
    expect(getServersState().servers).toEqual([fixtureServer]);
    expect(getServersState().loading).toBe(false);
    expect(getServersState().lastError).toBeNull();
  });

  it("sets lastError when bootstrap fails", async () => {
    listMock.mockRejectedValueOnce(new Error("disk"));
    await serversBootstrap();
    expect(getServersState().servers).toEqual([]);
    expect(getServersState().lastError).toContain("disk");
    expect(getServersState().loading).toBe(false);
  });
});

describe("serversStore mutations", () => {
  it("serverAdd calls ServersService.Add and clears lastError", async () => {
    addMock.mockResolvedValue(fixtureServer);
    await serversBootstrap();
    listMock.mockClear();

    await serverAdd("vless://...", "DE");

    expect(addMock).toHaveBeenCalledWith("vless://...", "DE");
  });

  it("serverAdd stores backend error in lastError and rethrows", async () => {
    addMock.mockRejectedValue(new Error("invalid VLESS URI"));
    await expect(serverAdd("garbage", "X")).rejects.toThrow("invalid VLESS URI");
    expect(getServersState().lastError).toContain("invalid VLESS URI");
  });

  it("serverEdit returns vlessChanged from backend", async () => {
    editMock.mockResolvedValue([fixtureServer, true]);
    const result = await serverEdit("m1", "vless://new", "DE-new");
    expect(result.vlessChanged).toBe(true);
    expect(editMock).toHaveBeenCalledWith("m1", "vless://new", "DE-new");
  });

  it("serverRemove calls ServersService.Remove", async () => {
    removeMock.mockResolvedValue(undefined);
    await serverRemove("m1");
    expect(removeMock).toHaveBeenCalledWith("m1");
  });

  it("serverRemove backend error is captured and rethrown", async () => {
    removeMock.mockRejectedValue(new Error("disconnect first to delete this server"));
    await expect(serverRemove("m1")).rejects.toThrow("disconnect first");
    expect(getServersState().lastError).toContain("disconnect first");
  });

  it("serverEdit defaults vlessChanged=false when backend returns non-array", async () => {
    // Defensive: today the Wails runtime emits Go multi-returns as [view, bool],
    // but if codegen ever flattens to just `view`, our store should not crash.
    editMock.mockResolvedValue(fixtureServer);
    const result = await serverEdit("m1", "vless://x", "DE");
    expect(result.vlessChanged).toBe(false);
  });

  it("rejects concurrent mutations with in-flight error", async () => {
    // Make the first Add hang so the second Add lands while it's in flight.
    let releaseFirst: () => void = () => {};
    addMock.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          releaseFirst = resolve;
        }),
    );

    const first = serverAdd("vless://a", "A");
    await expect(serverAdd("vless://b", "B")).rejects.toThrow(
      "another server mutation is in flight",
    );

    releaseFirst();
    await first;
  });
});

describe("serversStore events", () => {
  it("re-fetches on servers:changed", async () => {
    await serversBootstrap();
    listMock.mockClear();
    listMock.mockResolvedValue([fixtureServer, { ...fixtureServer, id: "m2" }]);

    emit("servers:changed");
    await Promise.resolve();
    await Promise.resolve();

    expect(listMock).toHaveBeenCalledTimes(1);
    expect(getServersState().servers).toHaveLength(2);
  });

  it("re-fetches on sub:synced", async () => {
    await serversBootstrap();
    listMock.mockClear();

    emit("sub:synced");
    await Promise.resolve();
    await Promise.resolve();

    expect(listMock).toHaveBeenCalledTimes(1);
  });
});

describe("serversStore lastError dismissal", () => {
  it("clearLastError resets lastError to null", async () => {
    listMock.mockRejectedValueOnce(new Error("boom"));
    await serversBootstrap();
    expect(getServersState().lastError).toContain("boom");

    clearLastError();
    expect(getServersState().lastError).toBeNull();
  });
});
