import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

vi.mock("@/lib/dashStore", () => ({
  getDashState: () => ({ status: "idle" }),
}));
vi.mock("@/lib/settings", () => ({
  setCurrentRulesSignature: () => {},
}));

const rulesImportPreviewMock = vi.fn();
const rulesImportApplyMock = vi.fn();
vi.mock("@/lib/rulesStore", async () => {
  const actual = await vi.importActual<any>("@/lib/rulesStore");
  return {
    ...actual,
    rulesImportPreview: (...a: any[]) => rulesImportPreviewMock(...a),
    rulesImportApply: (...a: any[]) => rulesImportApplyMock(...a),
  };
});

import { ImportRulesModal } from "./ImportRulesModal";

beforeEach(() => {
  rulesImportPreviewMock.mockReset();
  rulesImportApplyMock.mockReset();
});

describe("ImportRulesModal", () => {
  it("previews a pasted link then applies it", async () => {
    rulesImportPreviewMock.mockResolvedValue({
      name: "Streaming",
      groups: [{ id: "g1", name: "S", locked: false, enabled: true, rules: [] }],
      proxyCount: 3,
      directCount: 1,
      blockCount: 0,
    });
    rulesImportApplyMock.mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(<ImportRulesModal open initialLink="" onClose={onClose} />);

    await userEvent.type(screen.getByPlaceholderText(/itgray:\/\//i), "itgray://rules/import/abc");
    await userEvent.click(screen.getByRole("button", { name: /preview/i }));

    await waitFor(() => expect(screen.getByText("Streaming")).toBeInTheDocument());
    expect(rulesImportPreviewMock).toHaveBeenCalledWith("itgray://rules/import/abc");
    expect(screen.getByText(/3 proxy/i)).toBeInTheDocument();
    expect(screen.getByText(/1 direct/i)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /^import$/i }));
    await waitFor(() => expect(rulesImportApplyMock).toHaveBeenCalledWith("itgray://rules/import/abc"));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("highlights direct/block traffic with a warning", async () => {
    rulesImportPreviewMock.mockResolvedValue({
      name: "Mixed",
      groups: [],
      proxyCount: 1,
      directCount: 2,
      blockCount: 1,
    });
    render(<ImportRulesModal open initialLink="" onClose={() => {}} />);
    await userEvent.type(screen.getByPlaceholderText(/itgray:\/\//i), "itgray://rules/import/xyz");
    await userEvent.click(screen.getByRole("button", { name: /preview/i }));
    await waitFor(() => expect(screen.getByText("Mixed")).toBeInTheDocument());
    expect(screen.getByText(/bypass the vpn/i)).toBeInTheDocument();
  });

  it("auto-previews a prefilled deeplink", async () => {
    rulesImportPreviewMock.mockResolvedValue({
      name: "Prefilled",
      groups: [],
      proxyCount: 5,
      directCount: 0,
      blockCount: 0,
    });
    render(<ImportRulesModal open initialLink="itgray://rules/import/pref" onClose={() => {}} />);
    await waitFor(() => expect(rulesImportPreviewMock).toHaveBeenCalledWith("itgray://rules/import/pref"));
    await waitFor(() => expect(screen.getByText("Prefilled")).toBeInTheDocument());
  });

  it("shows a friendly error for a malformed link and does not offer Import", async () => {
    rulesImportPreviewMock.mockRejectedValue(new Error("ruleshare: malformed link"));
    render(<ImportRulesModal open initialLink="" onClose={() => {}} />);
    await userEvent.type(screen.getByPlaceholderText(/itgray:\/\//i), "garbage");
    await userEvent.click(screen.getByRole("button", { name: /preview/i }));
    await waitFor(() => expect(screen.getByText(/malformed/i)).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: /^import$/i })).not.toBeInTheDocument();
  });

  it("falls back to a generic error for an unrecognised failure", async () => {
    rulesImportPreviewMock.mockRejectedValue(new Error("boom"));
    render(<ImportRulesModal open initialLink="" onClose={() => {}} />);
    await userEvent.type(screen.getByPlaceholderText(/itgray:\/\//i), "itgray://rules/import/oops");
    await userEvent.click(screen.getByRole("button", { name: /preview/i }));
    await waitFor(() => expect(screen.getByRole("alert")).toBeInTheDocument());
  });

  it("Cancel closes without applying", async () => {
    const onClose = vi.fn();
    render(<ImportRulesModal open initialLink="" onClose={onClose} />);
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
    expect(rulesImportApplyMock).not.toHaveBeenCalled();
  });

  it("renders nothing when closed", () => {
    render(<ImportRulesModal open={false} onClose={() => {}} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
