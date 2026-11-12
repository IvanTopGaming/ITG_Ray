import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, it, expect, vi, beforeEach } from "vitest";

const overviewMock = vi.hoisted(() => vi.fn());
const connectionsMock = vi.hoisted(() => vi.fn());
vi.mock("@/lib/itg/StatsService", () => ({
  Overview: (p: string) => overviewMock(p),
  Connections: () => connectionsMock(),
  Clear: vi.fn(),
}));

import { Stats } from "./Stats";

class NoopResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
(globalThis as { ResizeObserver?: unknown }).ResizeObserver ??= NoopResizeObserver;

const GB = 1024 ** 3;
const MB = 1024 ** 2;

function overview(over: Record<string, unknown> = {}) {
  return {
    period: "today",
    down: 1.5 * GB, up: 300 * MB,
    downProxy: 0.5 * GB, downDirect: 1 * GB, upProxy: 100 * MB, upDirect: 200 * MB,
    blocked: 42, onlineSeconds: 3 * 3600 + 25 * 60, sessions: 2,
    series: Array.from({ length: 24 }, (_, i) => ({ start: `2026-11-12T${String(i).padStart(2, "0")}:00:00+03:00`, down: 0, up: 0, downProxy: 0, downDirect: 0, upProxy: 0, upDirect: 0 })),
    apps: [
      { app: "steam", down: 900 * MB, up: 1 * MB, downProxy: 0, upProxy: 0 },
      { app: "obs", down: 10 * MB, up: 180 * MB, downProxy: 10 * MB, upProxy: 180 * MB },
    ],
    rules: [
      { ruleId: "r1", ruleName: "Video", action: "proxy", route: "proxy", serverName: "🇫🇮 Finland", conns: 900 },
      { ruleId: "gone", ruleName: "", action: "block", route: "block", deleted: true, conns: 12 },
      { ruleId: "final", ruleName: "", action: "proxy", route: "proxy", conns: 5 },
    ],
    ...over,
  };
}

function renderStats() {
  return render(
    <MemoryRouter>
      <Stats />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  overviewMock.mockReset();
  connectionsMock.mockReset();
  connectionsMock.mockResolvedValue({ conns: [] });
});

describe("Stats overview", () => {
  it("shows the totals in the tiles", async () => {
    overviewMock.mockResolvedValue(overview());
    renderStats();
    expect(await screen.findByText("1.50 GB")).toBeInTheDocument();
    expect(screen.getByText("300.0 MB")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("3h 25m")).toBeInTheDocument();
  });

  it("asks for the chosen period", async () => {
    overviewMock.mockResolvedValue(overview());
    renderStats();
    await screen.findByText("1.50 GB");
    await userEvent.click(screen.getByRole("button", { name: "7 days" }));
    await waitFor(() => expect(overviewMock).toHaveBeenLastCalledWith("7d"));
  });

  it("sorts apps by upload when the ↑ header is clicked", async () => {
    overviewMock.mockResolvedValue(overview());
    renderStats();
    const table = await screen.findByTestId("stats-apps");
    expect(within(table).getAllByTestId("app-name")[0]).toHaveTextContent("steam");
    await userEvent.click(within(table).getByRole("button", { name: /↑/ }));
    expect(within(table).getAllByTestId("app-name")[0]).toHaveTextContent("obs");
  });

  it("labels deleted rules and the default route", async () => {
    overviewMock.mockResolvedValue(overview());
    renderStats();
    const rules = await screen.findByTestId("stats-rules");
    expect(within(rules).getByText("Deleted rule")).toBeInTheDocument();
    expect(within(rules).getByText("Default")).toBeInTheDocument();
    expect(within(rules).getByText("🇫🇮 Finland")).toBeInTheDocument();
  });

  it("shows the empty state when nothing was recorded", async () => {
    overviewMock.mockResolvedValue(overview({ down: 0, up: 0, downProxy: 0, downDirect: 0, upProxy: 0, upDirect: 0, blocked: 0, onlineSeconds: 0, sessions: 0, apps: [], rules: [] }));
    renderStats();
    expect(await screen.findByText(/no traffic recorded/i)).toBeInTheDocument();
  });
});
