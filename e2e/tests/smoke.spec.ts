import { test, expect } from "@playwright/test";

/**
 * C.T16 v0.1 smoke test — confirms the AppShell mounts and the four
 * top-level nav items render. This is the minimum signal that the
 * frontend bundle, router, and i18n bootstrap all work end-to-end.
 *
 * Gated behind WAILS_DEV_AVAILABLE because the Linux dev host lacks
 * webkit2gtk (C.T1 finding) and `wails dev` cannot launch its embedded
 * webview there. Set WAILS_DEV_AVAILABLE=1 after starting `wails dev`
 * or `npm run dev` inside cmd/itgray-gui/frontend manually.
 */
test.describe("AppShell smoke", () => {
  test.skip(
    !process.env.WAILS_DEV_AVAILABLE,
    "wails dev server not available — set WAILS_DEV_AVAILABLE=1 to run",
  );

  test("renders sidebar with 4 nav items and loads default route", async ({
    page,
  }) => {
    await page.goto("/");

    // The router defaults to /dashboard via HashRouter; both AppShell and
    // Sidebar must mount before the test can assert.
    const sidebar = page.getByRole("navigation").first();
    await expect(sidebar).toBeVisible();

    const expectedLabels = ["Dashboard", "Servers", "Subscriptions", "Settings"];
    for (const label of expectedLabels) {
      await expect(sidebar.getByText(label, { exact: true })).toBeVisible();
    }

    // Sanity: exactly 4 NavLinks (Sidebar.tsx renders <a> via NavLink).
    const links = sidebar.locator("a");
    await expect(links).toHaveCount(4);
  });
});
