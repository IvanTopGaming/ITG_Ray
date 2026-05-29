import { afterEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Sidebar } from "./components/Sidebar";
import i18n from "@/i18n";

afterEach(async () => {
  // Reset to English so a lingering 'ru' does not break other tests'
  // getByText assertions.
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

const renderSidebar = () =>
  render(
    <MemoryRouter>
      <Sidebar />
    </MemoryRouter>,
  );

describe("i18n language switch", () => {
  it("switches Sidebar labels between en and ru at runtime", async () => {
    renderSidebar();

    // Default English.
    expect(screen.getByText("Dashboard")).toBeInTheDocument();

    await act(async () => {
      await i18n.changeLanguage("ru");
    });
    expect(screen.getByText("Панель")).toBeInTheDocument();
    expect(screen.queryByText("Dashboard")).toBeNull();

    await act(async () => {
      await i18n.changeLanguage("en");
    });
    expect(screen.getByText("Dashboard")).toBeInTheDocument();
  });
});
