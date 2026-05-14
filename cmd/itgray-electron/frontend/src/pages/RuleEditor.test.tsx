import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { describe, it, expect, vi, beforeEach } from "vitest";

// rulesStore.ts pulls in dashStore + settings at top level; both
// modules touch window.itg via EventsOn during init, which is undefined
// in jsdom. Stub them so vi.importActual('@/lib/rulesStore') doesn't
// crash.
vi.mock("@/lib/dashStore", () => ({
  getDashState: () => ({ status: "idle" }),
}));
vi.mock("@/lib/settings", () => ({
  markRulesDirty: () => {},
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

import { RuleEditor } from "./RuleEditor";

const safety = { id: "safety", name: "Safety", locked: true, enabled: true, rules: [] };
const user = { id: "user", name: "My Rules", locked: false, enabled: true, rules: [
  { id: "r1", name: "Block ads", enabled: true, action: "block", conditions: { ip_cidrs: ["1.2.3.4/32"] } },
]};

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
  it("loads the rule by id and renders Name / Group; action is below conditions", () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderEditor("r1");
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Block ads");
    // Action segmented: pressed=true on the block button
    expect(screen.getByRole("button", { name: /block/i, pressed: true })).toBeInTheDocument();
    expect((screen.getByLabelText(/Group/i) as HTMLSelectElement).value).toBe("user");
    // IP CIDRs card is visible (the rule has one); other cards are not.
    expect(screen.getByRole("heading", { name: /ip cidrs/i })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /domains/i })).not.toBeInTheDocument();
  });

  it("Save calls rulesEditRule with the edited shape", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.type(screen.getByLabelText("Name"), " v2");
    await userEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({ id: "r1", name: "Block ads v2", action: "block" }));
  });

  it("empty rule (create mode): only + Add condition button is visible; no condition cards", () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    expect(screen.getByRole("button", { name: /\+ add condition/i })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /domains/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /ip cidrs/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /geo/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /ports/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /processes/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: /protocols/i })).not.toBeInTheDocument();
  });

  it("Add condition dropdown shows 6 items", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    await userEvent.click(screen.getByRole("button", { name: /\+ add condition/i }));
    expect(screen.getAllByRole("menuitem")).toHaveLength(6);
  });

  it("picking Domain matcher adds a Domains card; type + Enter creates a chip; chip ✕ removes it", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    await addConditionPick(/domain matcher/i);
    expect(screen.getByRole("heading", { name: /domains/i })).toBeInTheDocument();
    const input = screen.getByLabelText(/^domain matcher value$/i);
    await userEvent.type(input, "example.com{enter}");
    // chip rendered
    expect(screen.getByRole("button", { name: /remove domain matcher 1/i })).toBeInTheDocument();
    // remove the chip
    await userEvent.click(screen.getByRole("button", { name: /remove domain matcher 1/i }));
    expect(screen.queryByRole("button", { name: /remove domain matcher 1/i })).not.toBeInTheDocument();
  });

  it("card ✕ removes the entire card AND clears that condition type from draft", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    // r1 has ip_cidrs: ["1.2.3.4/32"]. Add a domains card too so save still has conditions.
    await addConditionPick(/domain matcher/i);
    await userEvent.type(screen.getByLabelText(/^domain matcher value$/i), "example.com{enter}");
    // Remove the IP CIDRs card.
    await userEvent.click(screen.getByRole("button", { name: /remove ip cidrs card/i }));
    // AnimatePresence keeps the exiting element mounted briefly — wait for it.
    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: /ip cidrs/i })).not.toBeInTheDocument();
    });
    await userEvent.click(screen.getByRole("button", { name: /save changes/i }));
    // After remove, ip_cidrs should not be present in the saved conditions.
    const arg = rulesEditRuleMock.mock.calls[0][0];
    expect(arg.conditions.ip_cidrs).toBeUndefined();
    expect(arg.conditions.domains).toEqual([{ kind: "suffix", value: "example.com" }]);
  });

  it("saves IP CIDR conditions via chip-input", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderCreateEditor("user");
    await addConditionPick(/ip cidrs/i);
    await userEvent.type(screen.getByLabelText(/^cidr value$/i), "10.0.0.0/8{enter}");
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      conditions: expect.objectContaining({ ip_cidrs: ["10.0.0.0/8"] }),
    }));
  });

  it("saves geo conditions with prefix and value via chip-input", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    await addConditionPick(/^geo$/i);
    await userEvent.click(screen.getByRole("button", { name: /^geo prefix$/i }));
    await userEvent.click(screen.getByRole("button", { name: /^geoip$/i }));
    await userEvent.type(screen.getByLabelText(/^geo value$/i), "ru{enter}");
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      conditions: expect.objectContaining({ geo: ["geoip:ru"] }),
    }));
  });

  it("saves single port condition via chip-input", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    await addConditionPick(/^ports$/i);
    await userEvent.type(screen.getByLabelText(/^port number$/i), "443{enter}");
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      conditions: expect.objectContaining({ ports: [{ single: 443 }] }),
    }));
  });

  it("ports Range mode saves {from, to} after a Range click + type", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    await addConditionPick(/^ports$/i);
    await userEvent.click(screen.getByRole("button", { name: /^range$/i, pressed: false }));
    await userEvent.type(screen.getByLabelText(/^port from$/i), "8000");
    await userEvent.type(screen.getByLabelText(/^port to$/i), "9000");
    // Click the chip +
    await userEvent.click(screen.getByRole("button", { name: /^add port$/i }));
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      conditions: expect.objectContaining({ ports: [{ from: 8000, to: 9000 }] }),
    }));
  });

  it("saves protocol conditions by toggling chip buttons", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    await addConditionPick(/^protocols$/i);
    // Toggle udp on (tcp stays off).
    await userEvent.click(screen.getByRole("button", { name: /protocol udp/i }));
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      conditions: expect.objectContaining({ protocols: ["udp"] }),
    }));
  });

  it("saves process conditions, trimmed", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    await addConditionPick(/^processes$/i);
    await userEvent.type(screen.getByLabelText(/^process name$/i), "  chrome.exe  {enter}");
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      conditions: expect.objectContaining({ processes: ["chrome.exe"] }),
    }));
  });

  it("action block: wrapper has rose tint class", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    // Find the wrapper around the Block segmented option.
    const blockBtn = screen.getByRole("button", { name: /^block$/i });
    await userEvent.click(blockBtn);
    // Climb to the wrapper: Segmented is wrapped in <div class="rounded-2xl border …" >
    let el: HTMLElement | null = blockBtn;
    while (el && !el.className.includes("rounded-2xl")) el = el.parentElement;
    expect(el).not.toBeNull();
    expect(el!.className).toMatch(/rose/);
  });

  it("Save is disabled until at least one chip exists; never persists empty conditions", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    expect(screen.getByRole("button", { name: /create rule/i })).toBeDisabled();
    // Adding an empty card alone doesn't enable Save — backend rejects all-empty conditions.
    await addConditionPick(/domain matcher/i);
    expect(screen.getByRole("button", { name: /create rule/i })).toBeDisabled();
    // Adding a chip enables Save.
    await userEvent.type(screen.getByLabelText(/^domain matcher value$/i), "example.com{enter}");
    expect(screen.getByRole("button", { name: /create rule/i })).toBeEnabled();
    expect(rulesAddRuleMock).not.toHaveBeenCalled();
  });

  it("once a type is added, it disappears from the dropdown options", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    await addConditionPick(/domain matcher/i);
    await userEvent.click(screen.getByRole("button", { name: /\+ add condition/i }));
    const menuItems = screen.getAllByRole("menuitem").map((el) => el.textContent ?? "");
    expect(menuItems.some((t) => /domain matcher/i.test(t))).toBe(false);
    expect(menuItems).toHaveLength(5);
  });

  it("shows a discard-changes confirm when leaving with unsaved edits", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderEditor("r1");
    await userEvent.type(screen.getByLabelText("Name"), " edit");
    await userEvent.click(screen.getByRole("button", { name: /routing/i }));
    expect(screen.getByText(/discard.*changes/i)).toBeInTheDocument();
  });

  it("create mode hides the Group dropdown and Save calls rulesAddRule", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesAddRuleMock.mockResolvedValue("r-new");
    renderCreateEditor("user");
    expect(screen.queryByLabelText(/^Group$/i)).not.toBeInTheDocument();
    await addConditionPick(/ip cidrs/i);
    await userEvent.type(screen.getByLabelText(/^cidr value$/i), "10.0.0.0/8{enter}");
    await userEvent.click(screen.getByRole("button", { name: /create rule/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      name: expect.any(String),
      enabled: true,
      action: "proxy",
      conditions: expect.objectContaining({ ip_cidrs: ["10.0.0.0/8"] }),
    }));
    const payload = rulesAddRuleMock.mock.calls[0][1];
    expect(payload).not.toHaveProperty("id");
  });

  it("create mode back-out does NOT persist anything", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    await userEvent.click(screen.getByRole("button", { name: /routing/i }));
    expect(rulesAddRuleMock).not.toHaveBeenCalled();
  });

  it("existing condition's chip kind dropdown updates the chip's kind", async () => {
    // Start with a domain that's `exact:example.com`, then switch kind to `regex`
    const userWithRule = { ...user, rules: [{
      id: "r1", name: "T", enabled: true, action: "proxy" as const,
      conditions: { domains: [{ kind: "exact" as const, value: "example.com" }] },
    }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    const chipKindTrigger = screen.getByRole("button", { name: /^domain matcher kind 1$/i });
    expect(chipKindTrigger).toHaveTextContent(/exact/i);
    await userEvent.click(chipKindTrigger);
    await userEvent.click(screen.getByRole("button", { name: /^regex$/i }));
    await userEvent.click(screen.getByRole("button", { name: /save changes/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ domains: [{ kind: "regex", value: "example.com" }] }),
    }));
  });
});

// Silence eslint: import is referenced via within(); keep it to support
// future scoping if needed.
void within;
