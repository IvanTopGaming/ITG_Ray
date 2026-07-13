import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RuleIdentityCard } from "./RuleIdentityCard";
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (k: string) => k }) }));

const groups = [{ id: "g1", name: "Streaming" }, { id: "g2", name: "Ads" }];

describe("RuleIdentityCard", () => {
  it("edits name and shows the OR hint", async () => {
    const onName = vi.fn();
    render(<RuleIdentityCard name="" enabled groupId="g1" groups={groups} onName={onName} onEnabled={() => {}} onGroup={() => {}} />);
    await userEvent.type(screen.getByLabelText("ruleEditor.name"), "X");
    expect(onName).toHaveBeenCalledWith("X");
    expect(screen.getByText("ruleEditor.orHint")).toBeTruthy();
  });
  it("changing the group select fires onGroup", async () => {
    const onGroup = vi.fn();
    render(<RuleIdentityCard name="" enabled groupId="g1" groups={groups} onName={() => {}} onEnabled={() => {}} onGroup={onGroup} />);
    await userEvent.selectOptions(screen.getByRole("combobox"), "g2");
    expect(onGroup).toHaveBeenCalledWith("g2");
  });
});
