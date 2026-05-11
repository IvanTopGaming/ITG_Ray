import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";

const useRulesMock = vi.fn();
const rulesAddGroupMock = vi.fn();
const rulesEditGroupMock = vi.fn();
const rulesRemoveGroupMock = vi.fn();
vi.mock("@/lib/rulesStore", async () => {
  const actual = await vi.importActual<any>("@/lib/rulesStore");
  return {
    ...actual,
    useRules: () => useRulesMock(),
    rulesAddGroup: (...a: any[]) => rulesAddGroupMock(...a),
    rulesEditGroup: (...a: any[]) => rulesEditGroupMock(...a),
    rulesRemoveGroup: (...a: any[]) => rulesRemoveGroupMock(...a),
  };
});

import { Routing } from "./Routing";

const safety = {
  id: "safety",
  name: "Safety",
  locked: true,
  enabled: true,
  rules: [{ id: "private", name: "Private IPs", enabled: true, action: "direct", conditions: { ip_cidrs: ["10.0.0.0/8"] } }],
};
const user = { id: "user", name: "My Rules", locked: false, enabled: true, rules: [] };

beforeEach(() => {
  useRulesMock.mockReset();
});

function renderRouting() {
  return render(
    <MemoryRouter>
      <Routing />
    </MemoryRouter>,
  );
}

describe("Routing page", () => {
  it("renders the safety group with a lock indicator", () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderRouting();
    expect(screen.getByText("Safety")).toBeInTheDocument();
    expect(screen.getByLabelText(/locked/i)).toBeInTheDocument();
  });

  it("renders user group rules", () => {
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, rules: [{ id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } }] }],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    expect(screen.getByText("Block ads")).toBeInTheDocument();
  });
});

describe("Routing page — group actions", () => {
  beforeEach(() => {
    rulesAddGroupMock.mockReset();
    rulesEditGroupMock.mockReset();
    rulesRemoveGroupMock.mockReset();
  });

  it("Add group flow calls rulesAddGroup", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddGroupMock.mockResolvedValue(undefined);
    renderRouting();
    await userEvent.click(screen.getByRole("button", { name: /add group/i }));
    const input = screen.getByPlaceholderText(/new group name/i);
    await userEvent.type(input, "Streaming{Enter}");
    expect(rulesAddGroupMock).toHaveBeenCalledWith("Streaming");
  });

  it("toggling a user group calls rulesEditGroup", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesEditGroupMock.mockResolvedValue(undefined);
    renderRouting();
    await userEvent.click(screen.getByLabelText(/Toggle My Rules/i));
    expect(rulesEditGroupMock).toHaveBeenCalledWith("user", "My Rules", false);
  });

  it("delete group asks for confirmation", async () => {
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1", name: "Streaming" }],
      loading: false, lastError: null, bootstrapped: true,
    });
    rulesRemoveGroupMock.mockResolvedValue(undefined);
    renderRouting();
    await userEvent.click(screen.getByLabelText(/Streaming menu/i));
    await userEvent.click(screen.getByRole("menuitem", { name: /delete/i }));
    await userEvent.click(screen.getByRole("button", { name: /^delete$/i }));
    expect(rulesRemoveGroupMock).toHaveBeenCalledWith("g1");
  });
});
