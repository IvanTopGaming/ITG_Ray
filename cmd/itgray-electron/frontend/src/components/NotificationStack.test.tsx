import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { NotificationStack, AUTO_HIDE_MS } from "./NotificationStack";
import { notify, getNotifications, __resetNotificationsForTests } from "@/lib/notificationStore";

beforeEach(() => __resetNotificationsForTests());
afterEach(() => vi.useRealTimers());

describe("NotificationStack", () => {
  it("renders title, body and actions", async () => {
    const run = vi.fn();
    render(<NotificationStack />);
    act(() =>
      notify({ id: "a", tone: "warning", title: "Helper out of date", body: "v1 → v2", persistent: true, actions: [{ label: "Update", run, primary: true }] }),
    );
    expect(screen.getByText("Helper out of date")).toBeInTheDocument();
    expect(screen.getByText("v1 → v2")).toBeInTheDocument();
    await act(async () => fireEvent.click(screen.getByRole("button", { name: "Update" })));
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("shows at most three notifications, newest first", () => {
    render(<NotificationStack />);
    act(() => {
      for (const id of ["1", "2", "3", "4"]) notify({ id, tone: "info", title: `N${id}`, persistent: true });
    });
    expect(screen.queryByText("N1")).toBeNull();
    const titles = screen.getAllByText(/^N\d$/).map((el) => el.textContent);
    expect(titles).toEqual(["N4", "N3", "N2"]);
  });

  it("dismisses on the close button", () => {
    render(<NotificationStack />);
    act(() => notify({ id: "a", tone: "info", title: "A", persistent: true }));
    act(() => fireEvent.click(screen.getByRole("button", { name: "Dismiss" })));
    expect(getNotifications()).toEqual([]);
  });

  it("auto-hides non-persistent notifications but not persistent ones", () => {
    vi.useFakeTimers();
    render(<NotificationStack />);
    act(() => {
      notify({ id: "info", tone: "info", title: "Info" });
      notify({ id: "sticky", tone: "warning", title: "Sticky", persistent: true });
    });
    act(() => vi.advanceTimersByTime(AUTO_HIDE_MS + 10));
    expect(getNotifications().map((n) => n.id)).toEqual(["sticky"]);
  });

  it("pauses auto-hide while hovered", () => {
    vi.useFakeTimers();
    render(<NotificationStack />);
    act(() => notify({ id: "info", tone: "info", title: "Info" }));
    fireEvent.mouseEnter(screen.getByTestId("notification-info"));
    act(() => vi.advanceTimersByTime(AUTO_HIDE_MS * 2));
    expect(getNotifications()).toHaveLength(1);
    fireEvent.mouseLeave(screen.getByTestId("notification-info"));
    act(() => vi.advanceTimersByTime(AUTO_HIDE_MS + 10));
    expect(getNotifications()).toEqual([]);
  });
});
