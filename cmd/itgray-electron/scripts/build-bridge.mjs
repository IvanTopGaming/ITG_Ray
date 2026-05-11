#!/usr/bin/env node
// build-bridge.mjs — cross-platform invocation of `go build` for the
// itgray-bridge binary. Replaces inline `GOOS=... GOARCH=... go build ...`
// in package.json (which is bash-only — failed on Windows hosts) and
// threads a git-derived version into `handlers.Version` via -ldflags.
//
// Target selection:
//   * TARGET_OS env, or
//   * Current platform (linux/darwin/win32→windows) when unset
//   * TARGET_ARCH env, or amd64 default
//
// Version source (first hit wins):
//   1. GIT_VERSION env (set by CI for reproducible builds)
//   2. `git describe --always --tags --dirty` (dev workstation)
//   3. literal "dev" (fallback when git is unavailable, e.g. tarball builds)
import { execFileSync, spawnSync } from "node:child_process";
import { existsSync, mkdirSync } from "node:fs";
import { resolve } from "node:path";

const platformToGOOS = { linux: "linux", darwin: "darwin", win32: "windows" };
const targetOS = process.env.TARGET_OS || platformToGOOS[process.platform] || "linux";
const targetArch = process.env.TARGET_ARCH || "amd64";
const ext = targetOS === "windows" ? ".exe" : "";

function resolveVersion() {
  if (process.env.GIT_VERSION) return process.env.GIT_VERSION;
  try {
    return execFileSync("git", ["describe", "--always", "--tags", "--dirty"], {
      encoding: "utf-8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return "dev";
  }
}

const version = resolveVersion();
const outDir = resolve(process.cwd(), "dist-bridge");
if (!existsSync(outDir)) mkdirSync(outDir, { recursive: true });
const outPath = resolve(outDir, `itgray-bridge${ext}`);

const ldflags = `-X github.com/itg-team/itg-ray/cmd/itgray-bridge/handlers.Version=${version}`;
const args = ["build", "-ldflags", ldflags, "-o", outPath, "../../cmd/itgray-bridge"];

console.log(`[build-bridge] GOOS=${targetOS} GOARCH=${targetArch} version=${version}`);
const result = spawnSync("go", args, {
  env: { ...process.env, GOOS: targetOS, GOARCH: targetArch },
  stdio: "inherit",
});
process.exit(result.status ?? 1);
