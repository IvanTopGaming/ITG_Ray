// cmd/itgray-electron/src/main/updates.ts
//
// Check-only "is a newer version out" query against the public GitHub repo's
// releases API. Deliberately lives in the Electron main process (Node has
// fetch, no CORS, and this is an app-shell concern) rather than the Go
// bridge. No auto-download/auto-install — the renderer only ever learns a
// status and, optionally, a link to open in the system browser.

import { ipcMain, shell } from "electron";

const REPO_OWNER = "IvanTopGaming";
const REPO_NAME = "ITG_Ray";
const RELEASES_API_URL = `https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases?per_page=10`;
export const RELEASES_PAGE_URL = `https://github.com/${REPO_OWNER}/${REPO_NAME}/releases`;
const FETCH_TIMEOUT_MS = 8_000;

// Only the fields we actually read from the GitHub API response. `tag_name`
// carries the version (with a "v" prefix by convention); `prerelease` marks
// the beta line but isn't load-bearing for selection — the comparator
// already ranks a prerelease below its matching full release, and the list
// endpoint (as opposed to /releases/latest) is what lets a prerelease be
// picked at all when it's the newest thing published.
export interface GithubRelease {
  tag_name: string;
  draft?: boolean;
  prerelease?: boolean;
  html_url?: string;
}

export type UpdateStatus = "uptodate" | "available" | "error";

export interface UpdateCheckResult {
  status: UpdateStatus;
  latest?: string;
  htmlUrl?: string;
}

export interface ParsedVersion {
  major: number;
  minor: number;
  patch: number;
  // Dot-separated prerelease identifiers ("beta.1" -> ["beta", "1"]), or
  // null when the tag names a plain release with no suffix.
  prerelease: string[] | null;
}

const VERSION_RE = /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$/;

/**
 * parseVersion reads a semver-ish tag ("v0.1.0-beta.1") into numeric
 * major/minor/patch plus an optional prerelease identifier chain. Strips a
 * leading "v" (GitHub tag convention). Anything that doesn't match a plain
 * N.N.N core returns null rather than throwing — callers decide whether a
 * malformed tag is fatal (checkForUpdate's own currentVersion) or just
 * skippable (one bad release among many in pickLatestRelease).
 */
export function parseVersion(tag: string): ParsedVersion | null {
  const cleaned = tag.trim().replace(/^v/i, "");
  const match = VERSION_RE.exec(cleaned);
  if (!match) return null;
  const [, major, minor, patch, pre] = match;
  return {
    major: Number(major),
    minor: Number(minor),
    patch: Number(patch),
    prerelease: pre ? pre.split(".") : null,
  };
}

// compareIdentifier ranks a single dot-separated prerelease field per
// semver precedence: identifiers that are all digits compare numerically;
// otherwise they compare lexically (ASCII); a numeric identifier always
// has lower precedence than an alphanumeric one.
function compareIdentifier(a: string, b: string): number {
  const aNumeric = /^\d+$/.test(a);
  const bNumeric = /^\d+$/.test(b);
  if (aNumeric && bNumeric) return Number(a) - Number(b);
  if (aNumeric) return -1;
  if (bNumeric) return 1;
  if (a === b) return 0;
  return a < b ? -1 : 1;
}

/**
 * compareVersions returns <0 if a<b, 0 if equal, >0 if a>b — or null when
 * either side fails to parse (the caller decides how to treat "can't
 * tell"). Core version fields (major/minor/patch) take priority; when
 * those tie, a build with a prerelease suffix ranks below the same core
 * with no suffix (0.1.0-beta.1 < 0.1.0), and two prerelease chains compare
 * identifier-by-identifier with a shorter chain losing ties (0.1.0-alpha <
 * 0.1.0-alpha.1), matching semver precedence rules.
 */
export function compareVersions(a: string, b: string): number | null {
  const va = parseVersion(a);
  const vb = parseVersion(b);
  if (!va || !vb) return null;

  if (va.major !== vb.major) return va.major - vb.major;
  if (va.minor !== vb.minor) return va.minor - vb.minor;
  if (va.patch !== vb.patch) return va.patch - vb.patch;

  if (va.prerelease === null && vb.prerelease === null) return 0;
  if (va.prerelease === null) return 1;
  if (vb.prerelease === null) return -1;

  const len = Math.max(va.prerelease.length, vb.prerelease.length);
  for (let i = 0; i < len; i++) {
    if (i >= va.prerelease.length) return -1;
    if (i >= vb.prerelease.length) return 1;
    const c = compareIdentifier(va.prerelease[i], vb.prerelease[i]);
    if (c !== 0) return c;
  }
  return 0;
}

export interface PickedRelease {
  /** Normalized version string, leading "v" already stripped. */
  version: string;
  htmlUrl?: string;
}

/**
 * pickLatestRelease selects the highest-versioned entry from a GitHub
 * releases list. Drafts are always ignored (they aren't published).
 * Malformed tags are skipped rather than aborting the whole scan — one bad
 * release shouldn't hide a good one. Returns null when nothing usable is
 * left (empty list, all drafts, all malformed).
 */
export function pickLatestRelease(releases: GithubRelease[]): PickedRelease | null {
  let best: PickedRelease | null = null;
  for (const r of releases) {
    if (!r || r.draft) continue;
    const tag = typeof r.tag_name === "string" ? r.tag_name : "";
    if (!parseVersion(tag)) continue;
    const version = tag.trim().replace(/^v/i, "");
    if (!best || compareVersions(version, best.version)! > 0) {
      best = { version, htmlUrl: r.html_url };
    }
  }
  return best;
}

/**
 * checkForUpdate queries the repo's releases list and compares the newest
 * publishable entry to the currently running version. Uses the LIST
 * endpoint (not /releases/latest, which excludes prereleases) so the beta
 * line — shipped entirely as prereleases — is still discoverable. Every
 * failure mode (network error, non-2xx, unparsable body, unparsable
 * currentVersion) collapses to `{ status: 'error' }`; this function never
 * throws, since it's called directly from an ipcMain.handle.
 */
export async function checkForUpdate(
  currentVersion: string,
  fetchImpl: typeof fetch = globalThis.fetch,
): Promise<UpdateCheckResult> {
  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
    let res: Response;
    try {
      res = await fetchImpl(RELEASES_API_URL, {
        headers: { Accept: "application/vnd.github+json" },
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timer);
    }
    if (!res.ok) return { status: "error" };

    const body: unknown = await res.json();
    if (!Array.isArray(body)) return { status: "error" };

    const picked = pickLatestRelease(body as GithubRelease[]);
    if (!picked) return { status: "uptodate" };

    const cmp = compareVersions(picked.version, currentVersion);
    if (cmp === null) return { status: "error" };
    if (cmp > 0) {
      return { status: "available", latest: picked.version, htmlUrl: picked.htmlUrl || RELEASES_PAGE_URL };
    }
    return { status: "uptodate" };
  } catch {
    return { status: "error" };
  }
}

/**
 * registerUpdateIPC wires the two renderer-facing handlers: `update.check`
 * runs the comparison above, `update.openReleases` hands a URL (or the
 * repo's releases page, as a fallback) to the OS default browser. Both are
 * plain ipcMain.handle registrations, mirroring the rest of ipc.ts.
 */
export function registerUpdateIPC(): void {
  ipcMain.handle("update.check", async (_event, params: { currentVersion?: string } | undefined) => {
    return checkForUpdate(params?.currentVersion ?? "");
  });

  ipcMain.handle("update.openReleases", async (_event, htmlUrl?: string) => {
    await shell.openExternal(htmlUrl && htmlUrl.trim() ? htmlUrl : RELEASES_PAGE_URL);
  });
}
