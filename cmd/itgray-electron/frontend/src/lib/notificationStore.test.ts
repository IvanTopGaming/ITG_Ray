import { describe, it, expect, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  notify,
  dismiss,
  retract,
  getNotifications,
  useNotifications,
  __resetNotificationsForTests,
} from "./notificationStore";

beforeEach(() => __resetNotificationsForTests());

describe("notificationStore", () => {
  it("puts the newest notification first", () => {
    notify({ id: "a", tone: "info", title: "A" });
    notify({ id: "b", tone: "info", title: "B" });
    expect(getNotifications().map((n) => n.id)).toEqual(["b", "a"]);
  });

  it("updates an existing id in place instead of duplicating it", () => {
    notify({ id: "a", tone: "info", title: "A" });
    notify({ id: "b", tone: "info", title: "B" });
    notify({ id: "a", tone: "warning", title: "A2" });
    expect(getNotifications().map((n) => `${n.id}:${n.title}`)).toEqual(["b:B", "a:A2"]);
  });

  it("keeps the same snapshot when an identical notification is re-sent", () => {
    notify({ id: "a", tone: "info", title: "A", actions: [{ label: "Go", run: () => {} }] });
    const before = getNotifications();
    notify({ id: "a", tone: "info", title: "A", actions: [{ label: "Go", run: () => {} }] });
    expect(getNotifications()).toBe(before);
  });

  it("refreshes the action handler of an unchanged notification", () => {
    const calls: string[] = [];
    notify({ id: "a", tone: "info", title: "A", actions: [{ label: "Go", run: () => { calls.push("old"); } }] });
    notify({ id: "a", tone: "info", title: "A", actions: [{ label: "Go", run: () => { calls.push("new"); } }] });
    void getNotifications()[0].actions![0].run();
    expect(calls).toEqual(["new"]);
  });

  it("a user dismissal sticks until the notification is retracted", () => {
    notify({ id: "a", tone: "info", title: "A" });
    dismiss("a");
    notify({ id: "a", tone: "info", title: "A" });
    expect(getNotifications()).toEqual([]);
    retract("a");
    notify({ id: "a", tone: "info", title: "A" });
    expect(getNotifications().map((n) => n.id)).toEqual(["a"]);
  });

  it("retract removes without marking as dismissed", () => {
    notify({ id: "a", tone: "info", title: "A" });
    retract("a");
    expect(getNotifications()).toEqual([]);
    notify({ id: "a", tone: "info", title: "A" });
    expect(getNotifications()).toHaveLength(1);
  });

  it("useNotifications re-renders on change", () => {
    const { result } = renderHook(() => useNotifications());
    expect(result.current).toEqual([]);
    act(() => notify({ id: "a", tone: "info", title: "A" }));
    expect(result.current.map((n) => n.id)).toEqual(["a"]);
  });
});
