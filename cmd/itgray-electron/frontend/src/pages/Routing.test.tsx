import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

const useRulesMock = vi.fn();
vi.mock("@/lib/rulesStore", async () => {
  const actual = await vi.importActual<any>("@/lib/rulesStore");
  return { ...actual, useRules: () => useRulesMock() };
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
