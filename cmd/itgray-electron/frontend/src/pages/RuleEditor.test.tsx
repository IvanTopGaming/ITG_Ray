import { render, screen } from "@testing-library/react";
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

describe("RuleEditor", () => {
  it("loads the rule by id and renders Name / Action / Group", () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderEditor("r1");
    expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("Block ads");
    // Action segmented: pressed=true on the block button
    expect(screen.getByRole("button", { name: /block/i, pressed: true })).toBeInTheDocument();
    expect((screen.getByLabelText(/Group/i) as HTMLSelectElement).value).toBe("user");
  });

  it("Save calls rulesEditRule with the edited shape", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.type(screen.getByLabelText("Name"), " v2");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({ id: "r1", name: "Block ads v2", action: "block" }));
  });

  it("adds a domain matcher row on +Add", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /domains/i }));
    await userEvent.click(screen.getByRole("button", { name: /add domain matcher/i }));
    expect(screen.getAllByLabelText(/domain matcher kind/i)).toHaveLength(1);
  });

  it("saves domain matcher conditions", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /domains/i }));
    await userEvent.click(screen.getByRole("button", { name: /add domain matcher/i }));
    await userEvent.selectOptions(screen.getByLabelText(/domain matcher kind/i), "suffix");
    await userEvent.type(screen.getByLabelText(/domain matcher value/i), "example.com");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ domains: [{ kind: "suffix", value: "example.com" }] }),
    }));
  });

  it("saves IP CIDR conditions", async () => {
    const userWithRule = { ...user, rules: [{ id: "r1", name: "T", enabled: true, action: "proxy", conditions: { ip_cidrs: [] } }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /ip cidrs/i }));
    await userEvent.click(screen.getByRole("button", { name: /add cidr/i }));
    await userEvent.type(screen.getByLabelText(/cidr value/i), "10.0.0.0/8");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ ip_cidrs: ["10.0.0.0/8"] }),
    }));
  });

  it("saves geo conditions with prefix and value", async () => {
    const userWithRule = { ...user, rules: [{ id: "r1", name: "T", enabled: true, action: "proxy", conditions: { geo: [] } }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /geo/i }));
    await userEvent.click(screen.getByRole("button", { name: /add geo/i }));
    await userEvent.selectOptions(screen.getByLabelText(/geo prefix/i), "geoip");
    await userEvent.type(screen.getByLabelText(/geo value/i), "ru");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ geo: ["geoip:ru"] }),
    }));
  });

  it("saves single port condition", async () => {
    const userWithRule = { ...user, rules: [{ id: "r1", name: "T", enabled: true, action: "proxy", conditions: { ports: [] } }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /ports/i }));
    await userEvent.click(screen.getByRole("button", { name: /add port/i }));
    await userEvent.type(screen.getByLabelText(/port number/i), "443");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ ports: [{ single: 443 }] }),
    }));
  });

  it("saves protocol conditions", async () => {
    const userWithRule = { ...user, rules: [{ id: "r1", name: "T", enabled: true, action: "proxy", conditions: { protocols: [] } }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /protocols/i }));
    await userEvent.click(screen.getByRole("button", { name: /add protocol/i }));
    // Default added row is "tcp"; switch to "udp"
    await userEvent.click(screen.getByRole("button", { name: /^udp$/i, pressed: false }));
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ protocols: ["udp"] }),
    }));
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
    // Group dropdown is hidden in create mode.
    expect(screen.queryByLabelText(/^Group$/i)).not.toBeInTheDocument();
    // Fill in a condition so the rule has something to save.
    await userEvent.click(screen.getByRole("button", { name: /ip cidrs/i }));
    await userEvent.click(screen.getByRole("button", { name: /add cidr/i }));
    await userEvent.type(screen.getByLabelText(/cidr value/i), "10.0.0.0/8");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesAddRuleMock).toHaveBeenCalledWith("user", expect.objectContaining({
      name: expect.any(String),
      enabled: true,
      action: "proxy",
      conditions: expect.objectContaining({ ip_cidrs: ["10.0.0.0/8"] }),
    }));
    // Ensure the persisted draft does NOT carry an id field.
    const payload = rulesAddRuleMock.mock.calls[0][1];
    expect(payload).not.toHaveProperty("id");
  });

  it("create mode back-out does NOT persist anything", async () => {
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, user], loading: false, lastError: null, bootstrapped: true });
    renderCreateEditor("user");
    await userEvent.click(screen.getByRole("button", { name: /routing/i }));
    expect(rulesAddRuleMock).not.toHaveBeenCalled();
  });

  it("ports Range mode saves {from, to} after a Range click + type", async () => {
    const userWithRule = { ...user, rules: [{ id: "r1", name: "T", enabled: true, action: "proxy", conditions: { ports: [] } }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /ports/i }));
    await userEvent.click(screen.getByRole("button", { name: /add port/i }));
    // Switch to Range — the Range button should toggle to pressed.
    await userEvent.click(screen.getByRole("button", { name: /^range$/i, pressed: false }));
    // Both range inputs should now be visible.
    const fromInput = screen.getByLabelText(/port from/i) as HTMLInputElement;
    const toInput = screen.getByLabelText(/port to/i) as HTMLInputElement;
    await userEvent.type(fromInput, "8000");
    await userEvent.type(toInput, "9000");
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ ports: [{ from: 8000, to: 9000 }] }),
    }));
  });

  it("saves process conditions, trimmed on blur", async () => {
    const userWithRule = { ...user, rules: [{ id: "r1", name: "T", enabled: true, action: "proxy", conditions: { processes: [] } }] };
    useRulesMock.mockReturnValue({ defaultAction: "proxy", groups: [safety, userWithRule], loading: false, lastError: null, bootstrapped: true });
    rulesEditRuleMock.mockResolvedValue(undefined);
    renderEditor("r1");
    await userEvent.click(screen.getByRole("button", { name: /processes/i }));
    await userEvent.click(screen.getByRole("button", { name: /add process/i }));
    const input = screen.getByLabelText(/process name/i);
    await userEvent.type(input, "  chrome.exe  ");
    await userEvent.tab(); // blur
    await userEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(rulesEditRuleMock).toHaveBeenCalledWith(expect.objectContaining({
      conditions: expect.objectContaining({ processes: ["chrome.exe"] }),
    }));
  });
});
