import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConditionCard } from "./ConditionCard";

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (k: string) => k.split(".").pop()! }) }));
vi.mock("./conditionBodies", () => ({ ConditionBody: () => <div data-testid="body" /> }));

const def = { key: "domains", icon: "🌐", label: "Domains" } as const;
const draft = { id: "r1", name: "", enabled: true, action: "proxy" as const,
  conditions: { domains: [{ kind: "suffix" as const, value: "netflix.com" }] } };

describe("ConditionCard", () => {
  it("collapsed: shows chip summary, hides the body", () => {
    render(<ConditionCard def={def} draft={draft} setDraft={() => {}} onRemove={() => {}} />);
    expect(screen.getByText("suffix: netflix.com")).toBeTruthy();
    expect(screen.queryByTestId("body")).toBeNull();
  });
  it("edit expands the reused body", async () => {
    render(<ConditionCard def={def} draft={draft} setDraft={() => {}} onRemove={() => {}} />);
    await userEvent.click(screen.getByRole("button", { name: /edit/i }));
    expect(screen.getByTestId("body")).toBeTruthy();
  });
  it("remove fires onRemove", async () => {
    const onRemove = vi.fn();
    render(<ConditionCard def={def} draft={draft} setDraft={() => {}} onRemove={onRemove} />);
    await userEvent.click(screen.getByRole("button", { name: /remove/i }));
    expect(onRemove).toHaveBeenCalledOnce();
  });
  it("startExpanded renders the body immediately", () => {
    render(<ConditionCard def={def} draft={draft} setDraft={() => {}} onRemove={() => {}} startExpanded />);
    expect(screen.getByTestId("body")).toBeTruthy();
  });
});
