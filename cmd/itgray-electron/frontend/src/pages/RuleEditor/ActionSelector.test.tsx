import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ActionSelector } from "./ActionSelector";
const LABELS: Record<string, string> = {
  "ruleEditor.actions.proxy": "Proxy",
  "ruleEditor.actions.direct": "Direct",
  "ruleEditor.actions.block": "Block",
};
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (k: string) => LABELS[k] ?? k }) }));

describe("ActionSelector", () => {
  it("selecting Block calls onChange('block') and marks it pressed", async () => {
    const onChange = vi.fn();
    const { rerender } = render(<ActionSelector value="proxy" onChange={onChange} />);
    await userEvent.click(screen.getByRole("button", { name: /^block$/i }));
    expect(onChange).toHaveBeenCalledWith("block");
    rerender(<ActionSelector value="block" onChange={onChange} />);
    expect(screen.getByRole("button", { name: /^block$/i })).toHaveAttribute("aria-pressed", "true");
  });
});
