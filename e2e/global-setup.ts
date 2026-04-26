import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

/**
 * Minimal global setup: provision a fresh dataDir for each e2e run.
 *
 * The path is exposed via the ITGRAY_E2E_DATADIR env var so tests (and
 * any future fake-VPN endpoint that replaces the real backend) can read
 * it. We deliberately do NOT spawn the real itgray-helper here — the
 * deferred fake-VPN endpoint will handle that in a follow-up plan.
 *
 * Returning an async teardown lets Playwright clean up automatically.
 */
export default async function globalSetup(): Promise<() => Promise<void>> {
  const dataDir = await mkdtemp(join(tmpdir(), "itgray-e2e-"));
  process.env.ITGRAY_E2E_DATADIR = dataDir;

  return async () => {
    await rm(dataDir, { recursive: true, force: true });
  };
}
