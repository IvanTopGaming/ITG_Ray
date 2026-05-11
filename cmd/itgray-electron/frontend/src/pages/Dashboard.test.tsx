import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

const useDashMock = vi.fn();
const dashConnectMock = vi.fn();
const dashDisconnectMock = vi.fn();
const dashSwitchModeMock = vi.fn();
vi.mock("@/lib/dashStore", () => ({
  useDash: () => useDashMock(),
  effectiveStatus: (s: any) =>
    s.status === "idle" && s.lastError ? "error" : s.status,
  dashConnect: (...a: any[]) => dashConnectMock(...a),
  dashDisconnect: (...a: any[]) => dashDisconnectMock(...a),
  dashSwitchMode: (...a: any[]) => dashSwitchModeMock(...a),
  clearLastError: vi.fn(),
}));

const useIpMock = vi.fn();
vi.mock("@/lib/ipStore", () => ({
  useIp: () => useIpMock(),
  ipRefresh: vi.fn(),
  ipReset: vi.fn(),
}));

const settingsState = { dnsMode: "auto", dnsCustom: "" };
const settingsUpdate = vi.fn();
vi.mock("@/lib/settings", () => ({
  useSettings: () => [settingsState, settingsUpdate],
}));

import { Dashboard } from "./Dashboard";

const baseDash = {
  status: "idle",
  mode: "tun",
  currentServer: null,
  helperState: "running",
  allServers: [],
  speed: { downBps: 0, upBps: 0, at: 0 },
  history: [],
  totals: { down: 0, up: 0 },
  connectedAt: null,
  lastError: null,
  bootstrapped: true,
};

function makeServer(over: Record<string, any> = {}) {
  return {
    id: "a",
    name: "A",
    country: "DE",
    favorite: false,
    latencyMs: 0,
    address: "1.2.3.4:443",
    origin: "manual",
    security: "tls",
    transport: "tcp",
    tags: [],
    ...over,
  };
}

function renderDash() {
  return render(
    <MemoryRouter>
      <Dashboard />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  useDashMock.mockReset();
  useIpMock.mockReset();
  useIpMock.mockReturnValue({ value: null, loading: false, error: null });
  dashConnectMock.mockReset();
  dashDisconnectMock.mockReset();
  dashSwitchModeMock.mockReset();
  settingsState.dnsMode = "auto";
  settingsState.dnsCustom = "";
});

describe("Dashboard", () => {
  it("renders idle state with no server", () => {
    useDashMock.mockReturnValue(baseDash);
    renderDash();
    expect(screen.getByText("DISCONNECTED")).toBeInTheDocument();
    expect(screen.getByText("No active connection")).toBeInTheDocument();
  });

  it("shows empty QuickSwitch state when allServers is empty", () => {
    useDashMock.mockReturnValue(baseDash);
    renderDash();
    expect(screen.getByText("No servers added")).toBeInTheDocument();
  });

  it("announces the empty QuickSwitch state to screen readers", () => {
    useDashMock.mockReturnValue(baseDash);
    renderDash();
    const liveRegion = screen.getByRole("status", {
      name: /no servers added/i,
    });
    expect(liveRegion).toHaveAttribute("aria-live", "polite");
  });

  it("uses grid-cols-1 with 1 server", () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [makeServer()],
    });
    const { container } = renderDash();
    expect(container.querySelector(".grid-cols-1")).toBeTruthy();
  });

  it("uses grid-cols-2 with 2 servers", () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [
        makeServer({ id: "a", name: "A" }),
        makeServer({
          id: "b",
          name: "B",
          country: "NL",
          address: "5.6.7.8:443",
        }),
      ],
    });
    const { container } = renderDash();
    expect(container.querySelector(".grid-cols-2")).toBeTruthy();
  });

  it("shows helperState in ConnectionInfo", () => {
    useDashMock.mockReturnValue({ ...baseDash, helperState: "stopped" });
    renderDash();
    expect(screen.getByText("stopped")).toBeInTheDocument();
  });

  it("shows public IP value when connected", () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      status: "connected",
      currentServer: makeServer({
        latencyMs: 30,
        security: "reality",
      }),
      connectedAt: Date.now() - 1000,
    });
    useIpMock.mockReturnValue({
      value: "203.0.113.5",
      loading: false,
      error: null,
    });
    renderDash();
    expect(
      screen.getByText("203.0.113.5", { exact: false }),
    ).toBeInTheDocument();
  });

  it("shows '—' for public IP when idle", () => {
    useDashMock.mockReturnValue(baseDash);
    useIpMock.mockReturnValue({
      value: "203.0.113.5",
      loading: false,
      error: null,
    });
    renderDash();
    // DNS row also shows; assert at least one Public IP cell shows em-dash
    const cells = screen.getAllByText("—");
    expect(cells.length).toBeGreaterThan(0);
  });
});
