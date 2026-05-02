import { beforeEach, describe, expect, it, vi } from "vitest";

const getPublicIPMock = vi.fn();
vi.mock("../../wailsjs/go/bindings/AppService", () => ({
  GetPublicIP: () => getPublicIPMock(),
}));

import { __resetIpForTest, getIpState, ipRefresh, ipReset } from "./ipStore";

beforeEach(() => {
  getPublicIPMock.mockReset();
  __resetIpForTest();
});

describe("ipStore", () => {
  it("starts with null value, not loading, no error", () => {
    expect(getIpState()).toEqual({ value: null, loading: false, error: null });
  });

  it("ipRefresh sets value on success", async () => {
    getPublicIPMock.mockResolvedValue("203.0.113.7");
    await ipRefresh();
    expect(getIpState()).toEqual({ value: "203.0.113.7", loading: false, error: null });
  });

  it("ipRefresh sets error on failure, leaves value", async () => {
    getPublicIPMock.mockRejectedValue(new Error("not connected"));
    await ipRefresh();
    expect(getIpState().error).toContain("not connected");
    expect(getIpState().value).toBeNull();
  });

  it("ipReset clears all", async () => {
    getPublicIPMock.mockResolvedValue("1.2.3.4");
    await ipRefresh();
    ipReset();
    expect(getIpState()).toEqual({ value: null, loading: false, error: null });
  });

  it("loading flag flips during refresh", async () => {
    let resolveFn!: (v: string) => void;
    getPublicIPMock.mockImplementation(() => new Promise((r) => { resolveFn = r; }));
    const p = ipRefresh();
    expect(getIpState().loading).toBe(true);
    resolveFn("9.9.9.9");
    await p;
    expect(getIpState().loading).toBe(false);
  });
});
