import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright config for the ITG Ray GUI e2e skeleton (Task C.T16 v0.1).
 *
 * The dev target is the Wails frontend Vite dev server, which runs on
 * http://localhost:34115 (see cmd/itgray-gui/frontend/vite.config.ts).
 * Wails normally rewrites this to its embedded webview, but for e2e we
 * point a regular Chromium at the Vite server directly. Wails-only Go
 * bindings are mocked by the test runtime when not present (TODO in
 * tests/smoke.spec.ts).
 *
 * Currently only ONE smoke test ships; see e2e/README.md for deferred work.
 */
const baseURL = process.env.E2E_BASE_URL ?? "http://localhost:34115";

export default defineConfig({
  testDir: "./tests",
  globalSetup: "./global-setup.ts",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  timeout: 30_000,
  expect: { timeout: 5_000 },

  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "off",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],

  // No `webServer` block in v0.1 — running the Vite dev server requires
  // working Wails bindings, which depend on webkit2gtk on Linux (missing
  // on the dev host per C.T1). Start `wails dev` (or `npm run dev` inside
  // the frontend) manually, then run e2e with WAILS_DEV_AVAILABLE=1.
  // See e2e/README.md for full instructions and the planned fake-VPN
  // endpoint that will replace the real backend in CI.
});
