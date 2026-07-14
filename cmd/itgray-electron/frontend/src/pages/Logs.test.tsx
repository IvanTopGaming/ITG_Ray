import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { LogEntry } from "@/lib/logStore";

const entries: LogEntry[] = [
  {
    seq: 1,
    time: "2026-07-12T04:00:00.000Z",
    level: "INFO",
    source: "bridge",
    message: "bridge-line-alpha",
  },
  {
    seq: 2,
    time: "2026-07-12T04:00:01.000Z",
    level: "INFO",
    source: "xray",
    message: "xray-line-beta",
  },
];

const startLogsMock = vi.fn();
const stopLogsMock = vi.fn();

vi.mock("@/lib/logStore", async () => {
  const actual = await vi.importActual<typeof import("@/lib/logStore")>(
    "@/lib/logStore",
  );
  return {
    ...actual,
    useLogEntries: () => entries,
    startLogs: (...args: unknown[]) => startLogsMock(...args),
    stopLogs: (...args: unknown[]) => stopLogsMock(...args),
  };
});

import { Logs } from "./Logs";

beforeEach(() => {
  startLogsMock.mockReset();
  stopLogsMock.mockReset();
  localStorage.clear();
  (window as any).itg = {
    logs: {
      start: vi.fn().mockResolvedValue({ entries: [] }),
      stop: vi.fn().mockResolvedValue(null),
      openFolder: vi.fn().mockResolvedValue(null),
      export: vi.fn().mockResolvedValue({ text: "combined-log-text" }),
      save: vi.fn().mockResolvedValue("/tmp/itgray-logs.txt"),
    },
  };
});

describe("Logs page", () => {
  it("renders the title", () => {
    render(<Logs />);
    expect(
      screen.getByRole("heading", { name: /Logs/i }),
    ).toBeInTheDocument();
  });

  it("toggling a source chip filters its rows out", async () => {
    const user = userEvent.setup();
    render(<Logs />);
    expect(screen.getByText("bridge-line-alpha")).toBeInTheDocument();
    expect(screen.getByText("xray-line-beta")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "bridge" }));

    expect(screen.queryByText("bridge-line-alpha")).not.toBeInTheDocument();
    expect(screen.getByText("xray-line-beta")).toBeInTheDocument();
  });

  it("Export fetches combined logs then hands them to the save dialog", async () => {
    const user = userEvent.setup();
    render(<Logs />);
    await user.click(screen.getByRole("button", { name: /Export/i }));
    const itg = (window as any).itg.logs;
    expect(itg.export).toHaveBeenCalledTimes(1);
    expect(itg.save).toHaveBeenCalledWith("combined-log-text");
  });

  it("persists the source filter across a remount (tab switch)", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<Logs />);
    await user.click(screen.getByRole("button", { name: "bridge" }));
    expect(screen.queryByText("bridge-line-alpha")).not.toBeInTheDocument();
    unmount();

    render(<Logs />);
    expect(screen.queryByText("bridge-line-alpha")).not.toBeInTheDocument();
    expect(screen.getByText("xray-line-beta")).toBeInTheDocument();
  });
});
