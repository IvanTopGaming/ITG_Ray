import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

const useServersMock = vi.fn();
const useDashMock = vi.fn();
const serverAddMock = vi.fn();
const serverEditMock = vi.fn();
const serverRemoveMock = vi.fn();
const clearLastErrorMock = vi.fn();
const dashConnectMock = vi.fn();
const dashProbeOneMock = vi.fn();
const dashProbeAllMock = vi.fn();

vi.mock("@/lib/serversStore", () => ({
  useServers: () => useServersMock(),
  serverAdd: (...args: any[]) => serverAddMock(...args),
  serverEdit: (...args: any[]) => serverEditMock(...args),
  serverRemove: (...args: any[]) => serverRemoveMock(...args),
  clearLastError: () => clearLastErrorMock(),
}));

vi.mock("@/lib/dashStore", () => ({
  useDash: () => useDashMock(),
  effectiveStatus: (s: any) =>
    s.status === "idle" && s.lastError ? "error" : s.status,
  dashConnect: (...args: any[]) => dashConnectMock(...args),
  dashProbeOne: (...args: any[]) => dashProbeOneMock(...args),
  dashProbeAll: (...args: any[]) => dashProbeAllMock(...args),
}));

const toggleFavoriteMock = vi.fn();
vi.mock("@/lib/itg/ServersService", () => ({
  ToggleFavorite: (...args: any[]) => toggleFavoriteMock(...args),
}));

const markActiveServerEditedMock = vi.fn();
const setDesiredServerMock = vi.fn();
const useDesiredServerMock = vi.fn();
vi.mock("@/lib/settings", () => ({
  markActiveServerEdited: () => markActiveServerEditedMock(),
  setDesiredServer: (...args: any[]) => setDesiredServerMock(...args),
  useDesiredServer: () => useDesiredServerMock(),
}));

vi.mock("@/lib/itg/runtime", () => ({
  EventsOn: () => () => {},
}));

import { Servers, parseVless, buildVless } from "./Servers";

function makeServer(over: Partial<any> = {}) {
  return {
    id: "s1",
    name: "Manual Server",
    country: "DE",
    address: "host.example.com:443",
    transport: "tcp",
    security: "tls",
    latencyMs: 42,
    origin: "manual",
    favorite: false,
    tags: [],
    uri: "vless://00000000-0000-0000-0000-000000000000@host.example.com:443?type=tcp&security=tls#Manual%20Server",
    ...over,
  };
}

const baseDash = {
  status: "idle" as const,
  mode: "tun" as const,
  currentServer: null as any,
  helperState: "running" as const,
  allServers: [] as any[],
  speed: { downBps: 0, upBps: 0, at: 0 },
  history: [],
  totals: { down: 0, up: 0 },
  connectedAt: null,
  lastError: null,
  bootstrapped: true,
  probeState: new Map<string, "probing" | "ok" | "error">(),
};

beforeEach(() => {
  useServersMock.mockReset();
  useServersMock.mockReturnValue({
    servers: [],
    loading: false,
    lastError: null,
  });
  useDashMock.mockReset();
  useDashMock.mockReturnValue({ ...baseDash });
  serverAddMock.mockReset().mockResolvedValue(undefined);
  serverEditMock.mockReset().mockResolvedValue({ vlessChanged: false });
  serverRemoveMock.mockReset().mockResolvedValue(undefined);
  clearLastErrorMock.mockReset();
  toggleFavoriteMock.mockReset().mockResolvedValue(undefined);
  markActiveServerEditedMock.mockReset();
  setDesiredServerMock.mockReset();
  useDesiredServerMock.mockReset().mockReturnValue(null);
  dashConnectMock.mockReset().mockResolvedValue(undefined);
  dashProbeOneMock.mockReset().mockResolvedValue(undefined);
  dashProbeAllMock.mockReset().mockResolvedValue(undefined);
});

describe("Servers page", () => {
  it("renders header on empty state", () => {
    render(<Servers />);
    expect(
      screen.getByRole("heading", { name: /Servers/i }),
    ).toBeInTheDocument();
    expect(screen.getByText(/No servers yet/i)).toBeInTheDocument();
  });

  it("renders rows from dash.allServers", () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [
        makeServer({ id: "m1", name: "Toronto", origin: "manual" }),
        makeServer({
          id: "s1",
          name: "Amsterdam",
          origin: "Main provider",
          latencyMs: 12,
        }),
      ],
    });
    render(<Servers />);
    expect(screen.getByText("Toronto")).toBeInTheDocument();
    expect(screen.getByText("Amsterdam")).toBeInTheDocument();
    expect(screen.getByText("Manual")).toBeInTheDocument();
    expect(screen.getByText("Main provider")).toBeInTheDocument();
    expect(screen.getByText(/12 ms/)).toBeInTheDocument();
  });

  it("shows lastError as a dismissible banner", async () => {
    useServersMock.mockReturnValue({
      servers: [],
      loading: false,
      lastError: "duplicate vless URI",
    });
    render(<Servers />);
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(/duplicate vless URI/)).toBeInTheDocument();
    await userEvent.click(
      within(alert).getByRole("button", { name: /Dismiss error/i }),
    );
    expect(clearLastErrorMock).toHaveBeenCalled();
  });

  it("Add success closes the modal", async () => {
    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getByRole("button", { name: /Add server/i }));
    expect(screen.getByText("Add manual server")).toBeInTheDocument();

    // Use the Quick paste path to fill required fields with a single action.
    const paste = screen.getByPlaceholderText(/vless:\/\/…/);
    await user.type(
      paste,
      "vless://00000000-0000-0000-0000-000000000000@host.example.com:443?type=tcp&security=tls#NewServer",
    );
    await user.click(screen.getByRole("button", { name: /^Parse$/ }));

    await user.click(screen.getByRole("button", { name: /^Add$/ }));
    await waitFor(() => expect(serverAddMock).toHaveBeenCalledTimes(1));
    const [uriArg, nameArg] = serverAddMock.mock.calls[0];
    expect(nameArg).toBe("NewServer");
    expect(uriArg).toContain("vless://");
    // Modal closes — title gone.
    await waitFor(() =>
      expect(screen.queryByText("Add manual server")).not.toBeInTheDocument(),
    );
  });

  it("Add error keeps the modal open", async () => {
    serverAddMock.mockRejectedValueOnce(new Error("invalid URI"));
    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getByRole("button", { name: /Add server/i }));

    const paste = screen.getByPlaceholderText(/vless:\/\/…/);
    await user.type(
      paste,
      "vless://00000000-0000-0000-0000-000000000000@host.example.com:443?type=tcp&security=tls#X",
    );
    await user.click(screen.getByRole("button", { name: /^Parse$/ }));
    await user.click(screen.getByRole("button", { name: /^Add$/ }));

    expect(serverAddMock).toHaveBeenCalledTimes(1);
    // Modal stays open
    expect(screen.getByText("Add manual server")).toBeInTheDocument();
    // Local error banner surfaces the message
    expect(await screen.findByText(/invalid URI/)).toBeInTheDocument();
  });

  it("Edit with vlessChanged on the active connected server marks active-edited", async () => {
    const active = makeServer({
      id: "m1",
      name: "Toronto",
      origin: "manual",
    });
    useDashMock.mockReturnValue({
      ...baseDash,
      status: "connected",
      currentServer: active,
      allServers: [active],
    });
    serverEditMock.mockResolvedValueOnce({ vlessChanged: true });

    const user = userEvent.setup();
    render(<Servers />);
    // Click the row's edit-pencil action.
    const editButtons = screen.getAllByTitle("Edit");
    await user.click(editButtons[0]);
    expect(screen.getByText("Edit server")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^Save$/ }));
    expect(serverEditMock).toHaveBeenCalledTimes(1);

    await waitFor(() =>
      expect(markActiveServerEditedMock).toHaveBeenCalledTimes(1),
    );
  });

  it("Delete error surfaces in the modal without closing", async () => {
    const target = makeServer({
      id: "m1",
      name: "Toronto",
      origin: "manual",
    });
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [target],
    });
    serverRemoveMock.mockRejectedValueOnce(new Error("server in use"));

    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getAllByTitle("Edit")[0]);
    await user.click(screen.getByRole("button", { name: /^Delete$/ }));

    expect(serverRemoveMock).toHaveBeenCalledWith("m1");
    // Modal still open
    expect(screen.getByText("Edit server")).toBeInTheDocument();
    expect(await screen.findByText(/server in use/)).toBeInTheDocument();
  });

  it("Subscription server opens a view-only modal with the View action", async () => {
    const subServer = makeServer({
      id: "s9",
      name: "Frankfurt",
      origin: "Main provider",
    });
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [subServer],
    });
    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getByTitle("View details"));
    expect(screen.getByText("Server details")).toBeInTheDocument();
    // No Save button on view-only modal.
    expect(screen.queryByRole("button", { name: /^Save$/ })).toBeNull();
  });

  it("Probe-all triggers dashProbeAll", async () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [makeServer({ id: "m1" })],
    });
    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getByRole("button", { name: /Probe all/i }));
    expect(dashProbeAllMock).toHaveBeenCalled();
  });

  it("Toggling favorite calls ToggleFavorite", async () => {
    const target = makeServer({ id: "m1", favorite: false });
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [target],
    });
    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getByTitle("Favourite"));
    expect(toggleFavoriteMock).toHaveBeenCalledWith("m1");
  });

  it("row click switches active server when not active", async () => {
    const user = userEvent.setup();
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [makeServer({ id: "s1", name: "A" })],
      currentServer: null,
      bootstrapped: true,
    });
    render(<Servers />);
    await user.click(screen.getByText("A"));
    expect(dashConnectMock).toHaveBeenCalledWith("s1");
  });

  it("row click does nothing when already active", async () => {
    const user = userEvent.setup();
    const active = makeServer({ id: "s1", name: "A" });
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [active],
      currentServer: active,
      status: "connected",
      bootstrapped: true,
    });
    render(<Servers />);
    await user.click(screen.getByText("A"));
    expect(dashConnectMock).not.toHaveBeenCalled();
  });

  it("row click does nothing during connecting/disconnecting", async () => {
    const user = userEvent.setup();
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [makeServer({ id: "s1", name: "A" })],
      currentServer: null,
      status: "connecting",
      bootstrapped: true,
    });
    render(<Servers />);
    await user.click(screen.getByText("A"));
    expect(dashConnectMock).not.toHaveBeenCalled();
  });

  it("clicking a server while CONNECTED sets a pending pick, does not reconnect", async () => {
    const a = makeServer({ id: "a", name: "Server A" });
    const b = makeServer({ id: "b", name: "Server B" });
    useDashMock.mockReturnValue({
      ...baseDash,
      status: "connected",
      currentServer: a,
      allServers: [a, b],
      bootstrapped: true,
    });
    render(<Servers />);
    await userEvent.click(screen.getByText("Server B"));
    expect(setDesiredServerMock).toHaveBeenCalledWith("b");
    expect(dashConnectMock).not.toHaveBeenCalled();
  });

  it("clicking the connected row while a pending pick exists reverts it", async () => {
    const a = makeServer({ id: "a", name: "Server A" });
    const b = makeServer({ id: "b", name: "Server B" });
    useDesiredServerMock.mockReturnValue("b");
    useDashMock.mockReturnValue({
      ...baseDash,
      status: "connected",
      currentServer: a,
      allServers: [a, b],
      bootstrapped: true,
    });
    render(<Servers />);
    await userEvent.click(screen.getByText("Server A"));
    expect(setDesiredServerMock).toHaveBeenCalledWith("a");
    expect(dashConnectMock).not.toHaveBeenCalled();
  });

  it("clicking a server while IDLE connects immediately", async () => {
    const a = makeServer({ id: "a", name: "Server A" });
    const b = makeServer({ id: "b", name: "Server B" });
    useDashMock.mockReturnValue({
      ...baseDash,
      status: "idle",
      currentServer: null,
      allServers: [a, b],
      bootstrapped: true,
    });
    render(<Servers />);
    await userEvent.click(screen.getByText("Server B"));
    expect(dashConnectMock).toHaveBeenCalledWith("b");
    expect(setDesiredServerMock).not.toHaveBeenCalled();
  });

  it("row click is suppressed when clicking inner buttons", async () => {
    const user = userEvent.setup();
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [makeServer({ id: "s1", name: "A", favorite: false })],
      bootstrapped: true,
    });
    render(<Servers />);
    await user.click(screen.getByTitle("Favourite"));
    expect(dashConnectMock).not.toHaveBeenCalled();
    expect(toggleFavoriteMock).toHaveBeenCalledWith("s1");
  });

  it("disables flow with an incompatible hint on non-tcp transport", async () => {
    const user = userEvent.setup();
    render(<Servers />);
    await user.click(screen.getByRole("button", { name: /Add server/i }));

    const paste = screen.getByPlaceholderText(/vless:\/\/…/);
    await user.type(
      paste,
      "vless://00000000-0000-0000-0000-000000000000@host.example.com:443?type=ws&security=tls#WS",
    );
    await user.click(screen.getByRole("button", { name: /^Parse$/ }));

    expect(
      screen.getByText(/flow needs TCP \+ TLS\/Reality/i),
    ).toBeInTheDocument();
  });

  it("does not render empty-state copy before bootstrap completes", () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [],
      bootstrapped: false,
    });
    render(<Servers />);
    expect(screen.queryByText(/no servers yet/i)).toBeNull();
  });

  it("renders empty-state copy after bootstrap with no servers", () => {
    useDashMock.mockReturnValue({
      ...baseDash,
      allServers: [],
      bootstrapped: true,
    });
    render(<Servers />);
    expect(screen.getByText(/no servers yet/i)).toBeInTheDocument();
  });
});

describe("vless round-trip", () => {
  it("carries xhttp host+mode and grpc serviceName", () => {
    const uri =
      "vless://u@ex.com:443?encryption=none&type=xhttp&path=%2Fx&host=front.ex.com&mode=auto&security=tls&sni=a.com";
    const c = parseVless(uri);
    expect(c.type).toBe("xhttp");
    expect(c.hostHeader).toBe("front.ex.com");
    expect(c.mode).toBe("auto");
    const back = buildVless(c);
    expect(back).toContain("type=xhttp");
    expect(back).toContain("host=front.ex.com");
    expect(back).toContain("mode=auto");
  });
});
