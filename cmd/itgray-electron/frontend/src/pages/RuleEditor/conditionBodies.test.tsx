import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ConditionBody } from "./conditionBodies";
import { EMPTY_DRAFT } from "./types";

vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (k: string) => k }) }));

describe("ConditionBody", () => {
  it("renders the protocols body without crashing", () => {
    const draft = { id: "r1", ...EMPTY_DRAFT, conditions: { protocols: [] as string[] } };
    render(<ConditionBody type="protocols" draft={draft} setDraft={() => {}} />);
    expect(screen.getByText(/tcp/i)).toBeTruthy();
    expect(screen.getByText(/udp/i)).toBeTruthy();
  });
});
