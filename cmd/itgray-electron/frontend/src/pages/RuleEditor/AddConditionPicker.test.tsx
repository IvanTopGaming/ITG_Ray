import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AddConditionPicker } from "./AddConditionPicker";

const LABELS: Record<string, string> = {
  "ruleEditor.addCondition": "Add condition",
  "ruleEditor.conditions.domains": "Domains",
  "ruleEditor.conditions.ip_cidrs": "IP CIDRs",
  "ruleEditor.conditions.geo": "Geo",
  "ruleEditor.conditions.ports": "Ports",
  "ruleEditor.conditions.processes": "Processes",
  "ruleEditor.conditions.protocols": "Protocols",
};
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (k: string) => LABELS[k] ?? k }) }));

describe("AddConditionPicker", () => {
  it("omits already-present types and calls onAdd", async () => {
    const onAdd = vi.fn();
    render(<AddConditionPicker present={new Set(["domains"])} onAdd={onAdd} />);
    await userEvent.click(screen.getByRole("button", { name: /add condition/i }));
    // "domains" is present → not offered; "ports" is offered
    expect(screen.queryByRole("menuitem", { name: /domains/i })).toBeNull();
    await userEvent.click(screen.getByRole("menuitem", { name: /ports/i }));
    expect(onAdd).toHaveBeenCalledWith("ports");
  });
});
