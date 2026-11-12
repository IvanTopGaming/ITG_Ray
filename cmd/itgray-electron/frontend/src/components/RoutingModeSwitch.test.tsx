import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";

const dash = { status: "idle", routingMode: "rule" };
const setMock = vi.fn((_m: string) => Promise.resolve());
vi.mock("@/lib/dashStore", () => ({
  useDash: () => dash,
  dashSetRoutingMode: (m: string) => setMock(m),
}));

import { RoutingModeSwitch } from "./RoutingModeSwitch";

beforeEach(() => {
  setMock.mockClear();
  dash.status = "idle";
  dash.routingMode = "rule";
});

describe("RoutingModeSwitch", () => {
  it("is disabled while not connected", () => {
    render(<RoutingModeSwitch />);
    for (const name of [/by rules/i, /all via proxy/i, /all direct/i]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
  });

  it("marks the active mode and switches on click when connected", async () => {
    dash.status = "connected";
    dash.routingMode = "global";
    render(<RoutingModeSwitch />);
    expect(screen.getByRole("button", { name: /all via proxy/i })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: /all direct/i }));
    expect(setMock).toHaveBeenCalledWith("direct");
  });
});
