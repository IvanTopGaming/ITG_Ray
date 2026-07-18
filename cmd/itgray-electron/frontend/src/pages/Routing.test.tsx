import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";

// rulesStore.ts pulls in dashStore + settings at top level; both
// modules touch window.itg via EventsOn during init, which is undefined
// in jsdom. Stub them so vi.importActual('@/lib/rulesStore') doesn't
// crash.
vi.mock("@/lib/dashStore", () => ({
  getDashState: () => ({ status: "idle" }),
}));
vi.mock("@/lib/settings", () => ({
  setCurrentRulesSignature: () => {},
}));

const useRulesMock = vi.fn();
const rulesAddGroupMock = vi.fn();
const rulesEditGroupMock = vi.fn();
const rulesRemoveGroupMock = vi.fn();
const rulesAddRuleMock = vi.fn();
const rulesMoveRuleMock = vi.fn();
const rulesRemoveRuleMock = vi.fn();
const rulesExportGroupMock = vi.fn();
vi.mock("@/lib/rulesStore", async () => {
  const actual = await vi.importActual<any>("@/lib/rulesStore");
  return {
    ...actual,
    useRules: () => useRulesMock(),
    rulesAddGroup: (...a: any[]) => rulesAddGroupMock(...a),
    rulesEditGroup: (...a: any[]) => rulesEditGroupMock(...a),
    rulesRemoveGroup: (...a: any[]) => rulesRemoveGroupMock(...a),
    rulesAddRule: (...a: any[]) => rulesAddRuleMock(...a),
    rulesMoveRule: (...a: any[]) => rulesMoveRuleMock(...a),
    rulesRemoveRule: (...a: any[]) => rulesRemoveRuleMock(...a),
    rulesExportGroup: (...a: any[]) => rulesExportGroupMock(...a),
  };
});

const navigateMock = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<any>("react-router-dom");
  return { ...actual, useNavigate: () => navigateMock };
});

import { Routing, reorderRules, reorderGroups, moveRuleAcrossGroups } from "./Routing";

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
    rulesExportGroupMock.mockReset();
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

  it("shares a group as a link to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1", name: "Streaming" }],
      loading: false, lastError: null, bootstrapped: true,
    });
    rulesExportGroupMock.mockResolvedValue("itgray://rules/import/xyz");
    renderRouting();
    await userEvent.click(screen.getByLabelText(/Streaming menu/i));
    await userEvent.click(screen.getByRole("menuitem", { name: /share/i }));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith("itgray://rules/import/xyz"));
    expect(rulesExportGroupMock).toHaveBeenCalledWith("g1");
  });
});

describe("Routing page — add rule", () => {
  beforeEach(() => {
    rulesAddRuleMock.mockReset();
    navigateMock.mockReset();
  });

  it("+ Add rule navigates to the create-flow editor without persisting", async () => {
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1" }],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    await userEvent.click(screen.getByRole("button", { name: /add rule/i }));
    expect(rulesAddRuleMock).not.toHaveBeenCalled();
    expect(navigateMock).toHaveBeenCalledWith("/routing/new", {
      state: { mode: "create", groupId: "g1" },
    });
  });

  it("locked groups do not show + Add rule", () => {
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    expect(screen.queryByRole("button", { name: /add rule/i })).not.toBeInTheDocument();
  });
});

describe("Routing page — rule click", () => {
  beforeEach(() => {
    navigateMock.mockReset();
  });

  it("clicking a rule row navigates to the editor", async () => {
    const ruleA = { id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } };
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1", name: "Group A", rules: [ruleA] }],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    await userEvent.click(screen.getByText("Block ads"));
    expect(navigateMock).toHaveBeenCalledWith("/routing/r1");
  });

  it("clicking the rule menu does NOT navigate", async () => {
    const ruleA = { id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } };
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1", name: "Group A", rules: [ruleA] }],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    await userEvent.click(screen.getByLabelText(/Block ads menu/i));
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("locked-group rule rows are NOT clickable", async () => {
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    await userEvent.click(screen.getByText("Private IPs"));
    expect(navigateMock).not.toHaveBeenCalled();
  });
});

describe("Routing page — per-rule menu", () => {
  beforeEach(() => {
    rulesMoveRuleMock.mockReset();
    rulesRemoveRuleMock.mockReset();
  });

  it("Edit menuitem navigates to the rule editor", async () => {
    const ruleA = { id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } };
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1", name: "Group A", rules: [ruleA] }],
      loading: false, lastError: null, bootstrapped: true,
    });
    navigateMock.mockReset();
    renderRouting();
    await userEvent.click(screen.getByLabelText(/Block ads menu/i));
    await userEvent.click(screen.getByRole("menuitem", { name: /^edit$/i }));
    expect(navigateMock).toHaveBeenCalledWith("/routing/r1");
  });

  it("Delete calls rulesRemoveRule", async () => {
    const ruleA = { id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } };
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety, { ...user, id: "g1", name: "Group A", rules: [ruleA] }],
      loading: false, lastError: null, bootstrapped: true,
    });
    rulesRemoveRuleMock.mockResolvedValue(undefined);
    renderRouting();
    await userEvent.click(screen.getByLabelText(/Block ads menu/i));
    await userEvent.click(screen.getByRole("menuitem", { name: /delete/i }));
    expect(rulesRemoveRuleMock).toHaveBeenCalledWith("r1");
  });

  it("locked groups do not show per-rule menu", () => {
    useRulesMock.mockReturnValue({
      defaultAction: "proxy",
      groups: [safety],
      loading: false, lastError: null, bootstrapped: true,
    });
    renderRouting();
    expect(screen.queryByLabelText(/Private IPs menu/i)).not.toBeInTheDocument();
  });
});

describe("reorderRules", () => {
  it("moves an element within a group", () => {
    const g = {
      id: "g1",
      name: "G",
      locked: false,
      enabled: true,
      rules: [
        { id: "a", name: "A", enabled: true, action: "proxy", conditions: { ip_cidrs: ["1.0.0.0/8"] } },
        { id: "b", name: "B", enabled: true, action: "proxy", conditions: { ip_cidrs: ["2.0.0.0/8"] } },
        { id: "c", name: "C", enabled: true, action: "proxy", conditions: { ip_cidrs: ["3.0.0.0/8"] } },
      ],
    } as any;
    const out = reorderRules([safety as any, g], "g1", 0, 2);
    expect(out[1].rules.map((r: any) => r.id)).toEqual(["b", "c", "a"]);
  });

  it("returns groups unchanged when groupId not found", () => {
    const g = {
      id: "g1",
      name: "G",
      locked: false,
      enabled: true,
      rules: [
        { id: "a", name: "A", enabled: true, action: "proxy", conditions: {} },
        { id: "b", name: "B", enabled: true, action: "proxy", conditions: {} },
      ],
    } as any;
    const out = reorderRules([g], "missing", 0, 1);
    expect(out[0].rules.map((r: any) => r.id)).toEqual(["a", "b"]);
  });
});

describe("reorderGroups", () => {
  it("reorderGroups moves a non-locked group", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [] } as any;
    const b = { id: "b", name: "B", locked: false, enabled: true, rules: [] } as any;
    const out = reorderGroups([safety as any, a, b], "a", "b");
    expect(out.map((g) => g.id)).toEqual(["safety", "b", "a"]);
  });

  it("reorderGroups refuses to move into the safety slot", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [] } as any;
    const out = reorderGroups([safety as any, a], "a", "safety");
    expect(out.map((g) => g.id)).toEqual(["safety", "a"]);
  });
});

describe("moveRuleAcrossGroups", () => {
  const mkRule = (id: string) => ({ id, name: id.toUpperCase(), enabled: true, action: "proxy", conditions: { ip_cidrs: ["1.0.0.0/8"] } });

  it("moves a rule between groups at the specified index", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [mkRule("r1"), mkRule("r2")] } as any;
    const b = { id: "b", name: "B", locked: false, enabled: true, rules: [mkRule("r3")] } as any;
    const out = moveRuleAcrossGroups([a, b], "r1", "b", 0);
    expect(out[0].rules.map((r: any) => r.id)).toEqual(["r2"]);
    expect(out[1].rules.map((r: any) => r.id)).toEqual(["r1", "r3"]);
  });

  it("moves into an empty group", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [mkRule("r1")] } as any;
    const b = { id: "b", name: "B", locked: false, enabled: true, rules: [] } as any;
    const out = moveRuleAcrossGroups([a, b], "r1", "b", 0);
    expect(out[0].rules).toEqual([]);
    expect(out[1].rules.map((r: any) => r.id)).toEqual(["r1"]);
  });

  it("clamps targetIndex into the valid range", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [mkRule("r1")] } as any;
    const b = { id: "b", name: "B", locked: false, enabled: true, rules: [mkRule("r2"), mkRule("r3")] } as any;
    const out = moveRuleAcrossGroups([a, b], "r1", "b", 999);
    expect(out[1].rules.map((r: any) => r.id)).toEqual(["r2", "r3", "r1"]);
  });

  it("refuses moves out of a locked source group", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [] } as any;
    const out = moveRuleAcrossGroups([safety as any, a], "private", "a", 0);
    expect(out).toBe(out);
    expect(out[0].rules.map((r: any) => r.id)).toEqual(["private"]);
    expect(out[1].rules).toEqual([]);
  });

  it("refuses moves into a locked target group", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [mkRule("r1")] } as any;
    const out = moveRuleAcrossGroups([safety as any, a], "r1", "safety", 0);
    expect(out[0].rules.map((r: any) => r.id)).toEqual(["private"]);
    expect(out[1].rules.map((r: any) => r.id)).toEqual(["r1"]);
  });

  it("returns input unchanged when source group equals target group", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [mkRule("r1")] } as any;
    const out = moveRuleAcrossGroups([a], "r1", "a", 0);
    expect(out).toBe(out);
    expect(out[0].rules.map((r: any) => r.id)).toEqual(["r1"]);
  });

  it("returns input unchanged when rule is not found", () => {
    const a = { id: "a", name: "A", locked: false, enabled: true, rules: [mkRule("r1")] } as any;
    const b = { id: "b", name: "B", locked: false, enabled: true, rules: [] } as any;
    const input = [a, b];
    const out = moveRuleAcrossGroups(input, "missing", "b", 0);
    expect(out).toBe(input);
  });
});
