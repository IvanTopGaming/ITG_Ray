# ITG Ray e2e suite (skeleton)

This directory holds the Playwright end-to-end test harness for the
ITG Ray Wails GUI. **This is the v0.1 skeleton** delivered by Task C.T16;
only one smoke test ships today. The full suite is tracked under
follow-up plan **`plan-c-e2e`**.

## Status

| Item | State |
|------|-------|
| Playwright config + harness | shipped |
| Smoke test (AppShell + 4 sidebar items) | shipped (gated) |
| Global setup with isolated dataDir | shipped |
| 4 additional happy-path tests | **deferred** |
| Fake-VPN backend endpoint | **deferred** |
| CI matrix (Linux/macOS/Windows) | **deferred** |

## Prerequisites

- Node 20+ (the repo dev host runs Node 25.x).
- A running Wails frontend dev server. On Linux this requires
  `webkit2gtk` for the full Wails toolchain; the frontend Vite server
  alone (`cmd/itgray-gui/frontend && npm run dev`) is enough for the
  smoke test because it talks to plain Chromium, not the embedded
  webview.
- Chromium (installed via `npm run install-browsers` once).

## Running locally

```sh
# 1. Start the frontend dev server in another terminal.
cd cmd/itgray-gui/frontend
npm run dev   # serves on http://localhost:34115

# 2. Run e2e against it.
cd e2e
npm install
npm run install-browsers      # one-time chromium download
WAILS_DEV_AVAILABLE=1 npm test
```

Tests are gated behind `WAILS_DEV_AVAILABLE=1` so that local runs and CI
on hosts without the dev server don't fail spuriously. Without that
flag, Playwright simply marks the suite as skipped.

To override the base URL (e.g. when pointing at a packaged build's
embedded server):

```sh
E2E_BASE_URL=http://127.0.0.1:5173 WAILS_DEV_AVAILABLE=1 npm test
```

## Linux dev host caveat

The Linux dev host used by this project is missing `webkit2gtk`
(documented in C.T1 findings). That blocks the full `wails dev`
command, but **not** the underlying Vite server, which is what
Playwright actually drives. So the skeleton is runnable on Linux as
long as the frontend Vite dev server is started by hand.

When the fake-VPN endpoint lands (see below), CI will not need
`webkit2gtk` at all because the test will run against a stand-alone
build that talks to a fake helper over loopback.

## Deferred tests (plan-c-e2e)

These are the four happy-path scenarios that the original C.T16 plan
called for. They are **not** implemented yet:

1. **Onboarding wizard** — first-run user completes the 2-step wizard
   (subscription URL + dataDir), lands on Dashboard with the correct
   default subscription.
2. **Subscription import + server list** — paste a sub URL, watch the
   server table populate, sort by latency, mark a favourite.
3. **Connect / disconnect lifecycle** — click Connect, verify the hero
   transitions through Connecting → Connected, sidebar status indicator
   updates, then Disconnect resolves cleanly.
4. **Settings round-trip** — toggle a setting in each of the 6 editable
   sections, confirm the change persists across a hard reload.

Plus the smoke test that already ships.

## Fake-VPN endpoint architecture (sketch)

To make the deferred tests deterministic the suite will eventually run
the GUI against a fake backend. Sketch:

- A small Go binary `cmd/itgray-fakehelper/` that mirrors the real
  helper's JSON-RPC surface (the methods exposed via Wails bindings in
  `cmd/itgray-gui/bindings/`) but never opens a real outbound
  connection.
- The fake helper serves a deterministic in-memory state machine:
  `Idle → Connecting → Connected → Disconnecting → Idle`, with
  configurable transition delays so tests can drive timing-sensitive
  UI states.
- Subscription fetches are stubbed against a fixture file shipped under
  `e2e/fixtures/` (a small VLESS sub with 3 servers in 2 regions).
- Latency probes return canned values from the same fixture.
- The fake's listener address is injected via env var
  (`ITGRAY_HELPER_ADDR=127.0.0.1:0` then read back from a pid file)
  and the GUI is launched with `ITGRAY_E2E_DATADIR` already pointing
  at the per-test temp dir created in `global-setup.ts`.
- Playwright's `webServer` config will spawn both the fake helper and
  the GUI (or a packaged static build) before the suite runs.

This keeps the e2e tier hermetic — no external sub URL, no real
networking, no webkit2gtk dependency in CI.

## Layout

```
e2e/
├── README.md            # this file
├── package.json         # @playwright/test + scripts
├── playwright.config.ts # config (no webServer block in v0.1)
├── tsconfig.json        # type-check the harness
├── global-setup.ts      # provisions per-run dataDir
├── tests/
│   └── smoke.spec.ts    # AppShell + 4 sidebar items
└── .gitignore           # node_modules / reports / results
```
