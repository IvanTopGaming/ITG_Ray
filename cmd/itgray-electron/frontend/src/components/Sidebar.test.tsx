import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

const { getSettingsMock } = vi.hoisted(() => ({ getSettingsMock: vi.fn() }));

vi.mock("@/lib/dashStore", () => ({
  useDash: () => ({ status: "connected", lastError: null }),
  effectiveStatus: (s: { status: string }) => s.status,
}));

vi.mock("@/lib/itg/SettingsService", () => ({
  Get: () => getSettingsMock(),
}));

import { Sidebar, formatVersion } from "./Sidebar";

describe("formatVersion", () => {
  it("keeps a single leading v for tag-stamped and bare versions", () => {
    expect(formatVersion("v0.1.2-beta.1")).toBe("v0.1.2-beta.1");
    expect(formatVersion("0.1.2-beta.1")).toBe("v0.1.2-beta.1");
    expect(formatVersion(" V0.1.2 ")).toBe("v0.1.2");
  });
});

describe("Sidebar status footer", () => {
  it("renders a release version without doubling the v prefix", async () => {
    getSettingsMock.mockResolvedValue({ about: { version: "v0.1.2-beta.1" } });
    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>,
    );
    expect(await screen.findByText("v0.1.2-beta.1")).toBeInTheDocument();
    expect(screen.queryByText(/vv0\.1\.2/)).not.toBeInTheDocument();
  });
});
