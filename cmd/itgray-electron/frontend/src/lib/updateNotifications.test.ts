import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import i18n from "@/i18n";

const upd = vi.hoisted(() => {
  const listeners: Array<(u: { latest: string; htmlUrl?: string }) => void> = [];
  return {
    listeners,
    Pending: vi.fn(),
    Skip: vi.fn(() => Promise.resolve()),
    OpenReleases: vi.fn(() => Promise.resolve()),
    OnAvailable: vi.fn((cb: (u: { latest: string; htmlUrl?: string }) => void) => {
      listeners.push(cb);
      return () => {};
    }),
  };
});
const pkg = vi.hoisted(() => ({ managed: false }));

vi.mock("@/lib/itg/UpdateService", () => upd);
vi.mock("@/lib/itg/HelperService", () => ({ StatusPackageManaged: () => Promise.resolve(pkg.managed) }));

import { useUpdateNotifications } from "./updateNotifications";
import { getNotifications, __resetNotificationsForTests } from "./notificationStore";

const t = i18n.getFixedT("en");

beforeEach(() => {
  __resetNotificationsForTests();
  upd.listeners.length = 0;
  upd.Pending.mockReset();
  upd.Skip.mockClear();
  upd.OpenReleases.mockClear();
  pkg.managed = false;
});

describe("useUpdateNotifications", () => {
  it("shows an update that main already found", async () => {
    upd.Pending.mockResolvedValue({ latest: "0.1.4", htmlUrl: "https://github.com/o/r/releases/tag/v0.1.4" });
    renderHook(() => useUpdateNotifications(t));
    await waitFor(() => expect(getNotifications().map((n) => n.id)).toEqual(["update-0.1.4"]));
    const n = getNotifications()[0];
    expect(n.title).toBe("Version 0.1.4 is available");
    expect(n.persistent).toBe(true);
    expect(n.actions!.map((a) => a.label)).toEqual(["Skip this version", "Open release"]);
  });

  it("shows an update announced after mount", async () => {
    upd.Pending.mockResolvedValue(null);
    renderHook(() => useUpdateNotifications(t));
    await waitFor(() => expect(upd.listeners).toHaveLength(1));
    upd.listeners[0]({ latest: "0.1.5" });
    await waitFor(() => expect(getNotifications().map((n) => n.id)).toEqual(["update-0.1.5"]));
  });

  it("points package-managed installs at the package manager", async () => {
    pkg.managed = true;
    upd.Pending.mockResolvedValue({ latest: "0.1.4" });
    renderHook(() => useUpdateNotifications(t));
    await waitFor(() => expect(getNotifications()[0]?.body).toMatch(/package manager/));
  });

  it("skip persists the version and closes the notification", async () => {
    upd.Pending.mockResolvedValue({ latest: "0.1.4" });
    renderHook(() => useUpdateNotifications(t));
    await waitFor(() => expect(getNotifications()).toHaveLength(1));
    await getNotifications()[0].actions![0].run();
    expect(upd.Skip).toHaveBeenCalledWith("0.1.4");
    expect(getNotifications()).toEqual([]);
  });

  it("open hands the release URL to main and closes the notification", async () => {
    upd.Pending.mockResolvedValue({ latest: "0.1.4", htmlUrl: "https://github.com/o/r/releases/tag/v0.1.4" });
    renderHook(() => useUpdateNotifications(t));
    await waitFor(() => expect(getNotifications()).toHaveLength(1));
    await getNotifications()[0].actions![1].run();
    expect(upd.OpenReleases).toHaveBeenCalledWith("https://github.com/o/r/releases/tag/v0.1.4");
    expect(getNotifications()).toEqual([]);
  });
});
