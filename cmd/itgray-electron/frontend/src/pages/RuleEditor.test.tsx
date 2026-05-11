import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Routes, Route } from "react-router-dom";
import { describe, it, expect, vi, beforeEach } from "vitest";

const useRulesMock = vi.fn();
const rulesEditRuleMock = vi.fn();
const rulesMoveRuleMock = vi.fn();
vi.mock("@/lib/rulesStore", async () => {
  const actual = await vi.importActual<any>("@/lib/rulesStore");
  return {
    ...actual,
    useRules: () => useRulesMock(),
    rulesEditRule: (...a: any[]) => rulesEditRuleMock(...a),
    rulesMoveRule: (...a: any[]) => rulesMoveRuleMock(...a),
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
});
