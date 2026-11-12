import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import i18n from "@/i18n";

const helper = vi.hoisted(() => ({
  StatusDetails: vi.fn(),
  Install: vi.fn(() => Promise.resolve()),
  Start: vi.fn(() => Promise.resolve()),
  Reinstall: vi.fn(() => Promise.resolve()),
  InstallLinux: vi.fn(() => Promise.resolve()),
  StartLinux: vi.fn(() => Promise.resolve()),
  RestartLinux: vi.fn(() => Promise.resolve()),
}));
const env = vi.hoisted(() => ({ platform: "linux" }));
const dash = vi.hoisted(() => ({ status: "idle", mode: "tun" }));

vi.mock("@/lib/itg/HelperService", () => helper);
vi.mock("@/lib/itg/runtime", () => ({ Environment: () => Promise.resolve({ buildType: "production", platform: env.platform, arch: "x64" }) }));
vi.mock("@/lib/dashStore", () => ({ getDashState: () => dash }));

import {
  buildHelperNotification,
  isHelperOutdated,
  normalizeVersion,
  useHelperNotifications,
} from "./helperNotifications";
import { getNotifications, __resetNotificationsForTests } from "./notificationStore";

const t = i18n.getFixedT("en");
const off = { connected: false, tunMode: true };
const run = (op: () => Promise<unknown>) => op().then(() => undefined);

beforeEach(() => {
  __resetNotificationsForTests();
  for (const fn of Object.values(helper)) fn.mockClear();
  env.platform = "linux";
  dash.status = "idle";
  dash.mode = "tun";
});
afterEach(() => vi.useRealTimers());

describe("version comparison", () => {
  it("ignores a leading v and unknown dev stamps", () => {
    expect(normalizeVersion("v0.1.3")).toBe("0.1.3");
    expect(normalizeVersion("dev")).toBeNull();
    expect(normalizeVersion("0.0.0-dev")).toBeNull();
    expect(normalizeVersion(undefined)).toBeNull();
  });

  it("flags only a running helper whose version differs", () => {
    expect(isHelperOutdated({ state: "running", packageManaged: false, version: "v0.1.2", expectedVersion: "v0.1.3" })).toBe(true);
    expect(isHelperOutdated({ state: "running", packageManaged: false, version: "0.1.3", expectedVersion: "v0.1.3" })).toBe(false);
    expect(isHelperOutdated({ state: "running", packageManaged: false, version: "dev", expectedVersion: "v0.1.3" })).toBe(false);
    expect(isHelperOutdated({ state: "running", packageManaged: false })).toBe(false);
    expect(isHelperOutdated({ state: "stopped", packageManaged: false, version: "v0.1.2", expectedVersion: "v0.1.3" })).toBe(false);
  });
});

describe("buildHelperNotification", () => {
  it("returns nothing for a healthy helper", () => {
    expect(buildHelperNotification({ state: "running", packageManaged: false, version: "v1", expectedVersion: "v1" }, "linux", off, t, run)).toBeNull();
  });

  it("offers Install for a missing helper on Windows", async () => {
    const n = buildHelperNotification({ state: "missing", packageManaged: false }, "windows", off, t, run)!;
    expect(n.id).toBe("helper-missing");
    expect(n.actions![0].label).toBe("Install");
    await n.actions![0].run();
    expect(helper.Install).toHaveBeenCalled();
  });

  it("offers Start for a missing packaged helper on Linux", async () => {
    const n = buildHelperNotification({ state: "missing", packageManaged: true }, "linux", off, t, run)!;
    expect(n.actions![0].label).toBe("Start");
    await n.actions![0].run();
    expect(helper.StartLinux).toHaveBeenCalled();
    expect(helper.InstallLinux).not.toHaveBeenCalled();
  });

  it("stays quiet about a missing Linux helper outside TUN mode", () => {
    expect(buildHelperNotification({ state: "missing", packageManaged: false }, "linux", { connected: false, tunMode: false }, t, run)).toBeNull();
  });

  it("explains that TUN mode needs the helper on Linux", () => {
    const n = buildHelperNotification({ state: "missing", packageManaged: false }, "linux", off, t, run)!;
    expect(n.body).toMatch(/TUN/);
  });

  it("offers Start for a stopped Windows helper", async () => {
    const n = buildHelperNotification({ state: "stopped", packageManaged: false }, "windows", off, t, run)!;
    expect(n.id).toBe("helper-stopped");
    await n.actions![0].run();
    expect(helper.Start).toHaveBeenCalled();
  });

  it("offers Update (reinstall) for an outdated Windows helper and warns when connected", async () => {
    const n = buildHelperNotification({ state: "running", packageManaged: false, version: "v0.1.2", expectedVersion: "v0.1.3" }, "windows", { connected: true, tunMode: true }, t, run)!;
    expect(n.id).toBe("helper-outdated");
    expect(n.body).toContain("v0.1.2");
    expect(n.body).toContain("v0.1.3");
    expect(n.body).toMatch(/connection/i);
    expect(n.actions![0].label).toBe("Update");
    await n.actions![0].run();
    expect(helper.Reinstall).toHaveBeenCalled();
  });

  it("offers Restart for an outdated packaged Linux helper", async () => {
    const n = buildHelperNotification({ state: "running", packageManaged: true, version: "v0.1.2", expectedVersion: "v0.1.3" }, "linux", off, t, run)!;
    expect(n.actions![0].label).toBe("Restart");
    expect(n.body).not.toMatch(/connection/i);
    await n.actions![0].run();
    expect(helper.RestartLinux).toHaveBeenCalled();
  });
});

describe("useHelperNotifications", () => {
  it("shows and then retracts the outdated notification as the helper catches up", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    helper.StatusDetails.mockResolvedValue({ state: "running", packageManaged: false, version: "v0.1.2", expectedVersion: "v0.1.3" });
    renderHook(() => useHelperNotifications(t));
    await waitFor(() => expect(getNotifications().map((n) => n.id)).toEqual(["helper-outdated"]));
    helper.StatusDetails.mockResolvedValue({ state: "running", packageManaged: false, version: "v0.1.3", expectedVersion: "v0.1.3" });
    await vi.advanceTimersByTimeAsync(5000);
    await waitFor(() => expect(getNotifications()).toEqual([]));
  });

  it("surfaces a failed action as an error notification", async () => {
    helper.StatusDetails.mockResolvedValue({ state: "missing", packageManaged: false });
    helper.InstallLinux.mockRejectedValueOnce(new Error("pkexec dismissed"));
    renderHook(() => useHelperNotifications(t));
    await waitFor(() => expect(getNotifications().map((n) => n.id)).toEqual(["helper-missing"]));
    await getNotifications()[0].actions![0].run();
    await waitFor(() => expect(getNotifications().map((n) => n.id)).toContain("helper-error"));
    expect(getNotifications().find((n) => n.id === "helper-error")!.body).toContain("pkexec dismissed");
  });

  it("does nothing on platforms without a helper", async () => {
    env.platform = "darwin";
    helper.StatusDetails.mockResolvedValue({ state: "missing", packageManaged: false });
    renderHook(() => useHelperNotifications(t));
    await new Promise((r) => setTimeout(r, 20));
    expect(helper.StatusDetails).not.toHaveBeenCalled();
    expect(getNotifications()).toEqual([]);
  });
});
