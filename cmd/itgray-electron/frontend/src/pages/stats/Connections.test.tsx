import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, it, expect, vi, beforeEach } from "vitest";

const connectionsMock = vi.hoisted(() => vi.fn());
vi.mock("@/lib/itg/StatsService", () => ({ Connections: () => connectionsMock(), Overview: vi.fn(), Clear: vi.fn() }));
vi.mock("@/lib/rulesStore", () => ({ useRules: () => ({ groups: [] }) }));
vi.mock("@/lib/dashStore", () => ({ useDash: () => ({ status: "connected" }) }));

import { Connections } from "./Connections";

const conns = [
  { id: "1", app: "firefox", host: "rr3.googlevideo.com", suffix: "googlevideo.com", route: "proxy", ruleId: "r3", ruleName: "YouTube", serverName: "🇳🇱 Netherlands 2", downBps: 8 * 1024 * 1024, upBps: 40 * 1024 },
  { id: "2", app: "telegram-desktop", host: "149.154.167.51", destIp: "149.154.167.51", route: "proxy", ruleId: "final", downBps: 0, upBps: 0 },
  { id: "3", app: "firefox", host: "ya.ru", suffix: "ya.ru", route: "direct", ruleId: "r1", ruleName: "Bypass RU", downBps: 0, upBps: 0 },
];

beforeEach(() => {
  connectionsMock.mockReset();
  connectionsMock.mockResolvedValue({ conns });
});

function renderIt(onCount = vi.fn()) {
  render(
    <MemoryRouter>
      <Connections visible onCount={onCount} />
    </MemoryRouter>,
  );
  return onCount;
}

describe("Connections", () => {
  it("lists live connections with rule, route and server", async () => {
    const onCount = renderIt();
    expect(await screen.findByText("rr3.googlevideo.com")).toBeInTheDocument();
    expect(screen.getByText("YouTube")).toBeInTheDocument();
    expect(screen.getByText("🇳🇱 Netherlands 2")).toBeInTheDocument();
    expect(screen.getByText("Default")).toBeInTheDocument();
    expect(screen.getByText("8.0 MB/s")).toBeInTheDocument();
    await waitFor(() => expect(onCount).toHaveBeenLastCalledWith(3));
  });

  it("filters by app or host", async () => {
    renderIt();
    await screen.findByText("rr3.googlevideo.com");
    await userEvent.type(screen.getByPlaceholderText(/filter/i), "ya.ru");
    expect(screen.queryByText("rr3.googlevideo.com")).toBeNull();
    expect(screen.getByText("ya.ru")).toBeInTheDocument();
  });

  it("shows the empty state", async () => {
    connectionsMock.mockResolvedValue({ conns: [] });
    renderIt();
    expect(await screen.findByText(/no active connections/i)).toBeInTheDocument();
  });
});
