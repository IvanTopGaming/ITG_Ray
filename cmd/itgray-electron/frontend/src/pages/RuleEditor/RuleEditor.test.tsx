import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { describe, it, expect, vi, beforeEach } from "vitest";

// rulesStore.ts pulls in dashStore + settings at top level; both
// modules touch window.itg via EventsOn during init, which is undefined
// in jsdom. Stub them so vi.importActual('@/lib/rulesStore') doesn't crash.
vi.mock("@/lib/dashStore", () => ({
  getDashState: () => ({ status: "idle" }),
}));
vi.mock("@/lib/settings", () => ({
  setCurrentRulesSignature: () => {},
}));

const useRulesMock = vi.fn();
const rulesEditRuleMock = vi.fn();
const rulesMoveRuleMock = vi.fn();
const rulesAddRuleMock = vi.fn();
vi.mock("@/lib/rulesStore", async () => {
  const actual = await vi.importActual<any>("@/lib/rulesStore");
  return {
    ...actual,
    useRules: () => useRulesMock(),
    rulesEditRule: (...a: any[]) => rulesEditRuleMock(...a),
    rulesMoveRule: (...a: any[]) => rulesMoveRuleMock(...a),
    rulesAddRule: (...a: any[]) => rulesAddRuleMock(...a),
  };
});

import { RuleEditor } from "./index";

const safety = { id: "safety", name: "Safety", locked: true, enabled: true, rules: [] };
const user = { id: "user", name: "My Rules", locked: false, enabled: true, rules: [
  { id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } },
]};

function groupsWith(...rules: any[]) {
  return [safety, { ...user, rules }];
}

const okRules = { defaultAction: "proxy", loading: false, lastError: null, bootstrapped: true };

beforeEach(() => {
  useRulesMock.mockReset();
  rulesEditRuleMock.mockReset();
  rulesMoveRuleMock.mockReset();
  rulesAddRuleMock.mockReset();
});

function renderEditor(ruleId: string) {
  return render(
    <MemoryRouter initialEntries={[`/routing/${ruleId}`]}>
      <Routes>
        <Route path="/routing/:ruleId" element={<RuleEditor />} />
      </Routes>
    </MemoryRouter>,
  );
}

function renderCreateEditor(groupId: string) {
  return render(
    <MemoryRouter initialEntries={[{ pathname: "/routing/new", state: { mode: "create", groupId } }]}>
      <Routes>
        <Route path="/routing/:ruleId" element={<RuleEditor />} />
      </Routes>
    </MemoryRouter>,
  );
}

// Opens the "+ Add condition" popover and picks an item by its visible label.
async function addConditionPick(label: RegExp) {
  await userEvent.click(screen.getByRole("button", { name: /\+ add condition/i }));
  await userEvent.click(screen.getByRole("menuitem", { name: label }));
}

describe("RuleEditor", () => {
  it("loads the rule by id: Name, group, action, and one card per present type", () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderEditor("r1");
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Block ads");
    expect(screen.getByRole("button", { name: /^block$/i, pressed: true })).toBeInTheDocument();
    expect((screen.getByRole("combobox") as HTMLSelectElement).value).toBe("user");
    // Only the IP CIDRs card is present.
    expect(screen.getByRole("heading", { name: /ip cidrs/i })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /domains/i })).not.toBeInTheDocument();
  });

  it("puts an AND connector between two condition cards", () => {
    const rule = { id: "r1", name: "T", enabled: true, action: "proxy" as const, conditions: {
      domains: [{ kind: "suffix" as const, value: "netflix.com" }],
      protocols: ["tcp"],
    } };
    useRulesMock.mockReturnValue({ ...okRules, groups: groupsWith(rule) });
    renderEditor("r1");
    expect(screen.getByRole("heading", { name: /domains/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /protocols/i })).toBeInTheDocument();
    expect(screen.getAllByText("AND").length).toBe(1);
  });

  it("Save is disabled with an 'add a condition' hint when there are no conditions", () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderCreateEditor("user");
    expect(screen.getByRole("button", { name: /create rule/i })).toBeDisabled();
    expect(screen.getByText(/add a condition to save/i)).toBeInTheDocument();
  });

  it("adding a condition opens its card expanded and enables Save once a chip exists", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderCreateEditor("user");
    await addConditionPick(/domain matcher/i);
    // Card is present and expanded — the value input is rendered immediately.
    expect(screen.getByRole("heading", { name: /domains/i })).toBeInTheDocument();
    const input = screen.getByLabelText(/^domain matcher value$/i);
    // An empty card alone does not enable Save.
    expect(screen.getByRole("button", { name: /create rule/i })).toBeDisabled();
    await userEvent.type(input, "example.com{enter}");
    expect(screen.getByRole("button", { name: /create rule/i })).toBeEnabled();
    expect(rulesAddRuleMock).not.toHaveBeenCalled();
  });

  it("card ✕ removes the entire card and clears that type from the draft", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /remove ip cidrs card/i }));
    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /ip cidrs/i })).not.toBeInTheDocument();
    });
    // With no conditions left, Save is disabled.
    expect(screen.getByRole("button", { name: /save changes/i })).toBeDisabled();
  });

  it("changing the action updates the pressed state", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderCreateEditor("user");
    expect(screen.getByRole("button", { name: /^proxy$/i, pressed: true })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /^direct$/i }));
    expect(screen.getByRole("button", { name: /^direct$/i, pressed: true })).toBeInTheDocument();
  });

  it("Save (edit mode) calls rulesEditRule with the assembled rule", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.type(screen.getByLabelText("Name"), " v2");
    await userEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      id: "r1",
      name: "Block ads v2",
      action: "block",
      conditions: expect.objectContaining({ ip_cidrs: ["1.2.3.4/32"] }),
    }));
  });

  it("Save (create mode) calls rulesAddRule with the assembled rule and no id", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    await addConditionPick(/ip cidrs/i);
    await userEvent.type(screen.getByLabelText(/^cidr value$/i), "10.0.0.0/8{enter}");
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      enabled: true,
      action: "proxy",
      conditions: expect.objectContaining({ ip_cidrs: ["10.0.0.0/8"] }),
    }));
    expect(rulesAddRuleMock.mock.calls[0][1]).not.toHaveProperty("id");
  });

  it("once a type is added it disappears from the picker options", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderCreateEditor("user");
    await addConditionPick(/domain matcher/i);
    await userEvent.click(screen.getByRole("button", { name: /\+ add condition/i }));
    const items = screen.getAllByRole("menuitem").map((el) => el.textContent ?? "");
    expect(items).toHaveLength(5);
    expect(items.some((tx) => /domain matcher/i.test(tx))).toBe(false);
  });

  it("shows a discard-changes confirm when leaving with unsaved edits", async () => {
    useRulesMock.mockReturnValue({ ...okRules, groups: [safety, user] });
    renderEditor("r1");
    await userEvent.type(screen.getByLabelText("Name"), " edit");
    await userEvent.click(screen.getByRole("button", { name: /routing/i }));
    expect(screen.getByText(/discard.*changes/i)).toBeInTheDocument();
  });
});
