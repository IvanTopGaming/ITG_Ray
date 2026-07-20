// cmd/itgray-electron/src/main/updates.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  parseVersion,
  compareVersions,
  pickLatestRelease,
  checkForUpdate,
  safeReleasesURL,
  RELEASES_PAGE_URL,
  type GithubRelease,
} from "./updates";

// ── safeReleasesURL (shell.openExternal gate) ─────────────────────────

test("safeReleasesURL passes through an https github.com release URL", () => {
  const u = "https://github.com/IvanTopGaming/ITG_Ray/releases/tag/v0.1.0-beta.1";
  assert.equal(safeReleasesURL(u), u);
});

test("safeReleasesURL falls back for off-domain, non-https, and hostile schemes", () => {
  for (const bad of [
    "http://github.com/x",              // not https
    "https://evil.com/x",               // off-domain
    "https://github.com.evil.com/x",    // lookalike host
    "file:///etc/passwd",               // file scheme
    "smb://attacker/share",             // smb scheme
    "javascript:alert(1)",              // js scheme
    "not a url",                         // unparseable
    "",                                  // empty
    undefined,                           // missing
  ]) {
    assert.equal(safeReleasesURL(bad as string | undefined), RELEASES_PAGE_URL);
  }
});

// ── parseVersion ──────────────────────────────────────────────────────

test("parseVersion strips a leading v and reads major.minor.patch", () => {
  assert.deepEqual(parseVersion("v0.1.0"), { major: 0, minor: 1, patch: 0, prerelease: null });
});

test("parseVersion captures a dot-separated prerelease suffix", () => {
  assert.deepEqual(parseVersion("v0.1.0-beta.1"), {
    major: 0,
    minor: 1,
    patch: 0,
    prerelease: ["beta", "1"],
  });
});

test("parseVersion returns null for a malformed tag", () => {
  assert.equal(parseVersion("not-a-version"), null);
  assert.equal(parseVersion("v1.2"), null);
  assert.equal(parseVersion(""), null);
});

// ── compareVersions ───────────────────────────────────────────────────

test("compareVersions: beta.1 < beta.2 (numeric prerelease field)", () => {
  assert.ok(compareVersions("0.1.0-beta.1", "0.1.0-beta.2")! < 0);
  assert.ok(compareVersions("0.1.0-beta.2", "0.1.0-beta.1")! > 0);
});

test("compareVersions: a prerelease is lower than the same core release", () => {
  assert.ok(compareVersions("0.1.0-beta.1", "0.1.0")! < 0);
  assert.ok(compareVersions("0.1.0", "0.1.0-beta.1")! > 0);
});

test("compareVersions: prerelease id ordering (beta < rc)", () => {
  assert.ok(compareVersions("0.1.0-beta.2", "0.1.0-rc.1")! < 0);
});

test("compareVersions: release ordering falls through to patch/minor/major", () => {
  assert.ok(compareVersions("0.1.0", "0.1.1")! < 0);
  assert.ok(compareVersions("0.1.1", "0.2.0")! < 0);
  assert.ok(compareVersions("0.2.0", "1.0.0")! < 0);
});

test("compareVersions: equal versions compare equal", () => {
  assert.equal(compareVersions("0.1.0-beta.1", "v0.1.0-beta.1"), 0);
  assert.equal(compareVersions("0.1.0", "v0.1.0"), 0);
});

test("compareVersions: returns null when either side is malformed", () => {
  assert.equal(compareVersions("garbage", "0.1.0"), null);
  assert.equal(compareVersions("0.1.0", "garbage"), null);
});

// ── pickLatestRelease ─────────────────────────────────────────────────

function release(partial: Partial<GithubRelease>): GithubRelease {
  return { tag_name: "v0.0.0", draft: false, prerelease: false, html_url: "", ...partial };
}

test("pickLatestRelease picks the newest prerelease when it is the highest tag", () => {
  const releases = [
    release({ tag_name: "v0.1.0-beta.1", html_url: "https://x/1" }),
    release({ tag_name: "v0.1.0-beta.2", html_url: "https://x/2" }),
  ];
  const picked = pickLatestRelease(releases);
  assert.equal(picked?.version, "0.1.0-beta.2");
  assert.equal(picked?.htmlUrl, "https://x/2");
});

test("pickLatestRelease ignores drafts", () => {
  const releases = [
    release({ tag_name: "v9.9.9", draft: true }),
    release({ tag_name: "v0.1.0-beta.1" }),
  ];
  const picked = pickLatestRelease(releases);
  assert.equal(picked?.version, "0.1.0-beta.1");
});

test("pickLatestRelease skips malformed tags instead of throwing", () => {
  const releases = [release({ tag_name: "not-a-version" }), release({ tag_name: "v0.1.0" })];
  const picked = pickLatestRelease(releases);
  assert.equal(picked?.version, "0.1.0");
});

test("pickLatestRelease returns null for an empty or all-draft list", () => {
  assert.equal(pickLatestRelease([]), null);
  assert.equal(pickLatestRelease([release({ tag_name: "v1.0.0", draft: true })]), null);
});

// ── checkForUpdate (mocked fetch — never hits the network) ─────────────

function fakeFetch(body: unknown, ok = true): typeof fetch {
  return (async () =>
    ({
      ok,
      status: ok ? 200 : 500,
      json: async () => body,
    }) as Response) as typeof fetch;
}

test("checkForUpdate: newest release beats current -> available", async () => {
  const releases: GithubRelease[] = [
    release({ tag_name: "v0.1.0-beta.2", html_url: "https://x/2" }),
    release({ tag_name: "v0.1.0-beta.1", html_url: "https://x/1" }),
  ];
  const result = await checkForUpdate("0.1.0-beta.1", fakeFetch(releases));
  assert.equal(result.status, "available");
  assert.equal(result.latest, "0.1.0-beta.2");
  assert.equal(result.htmlUrl, "https://x/2");
});

test("checkForUpdate: current version already the newest -> uptodate", async () => {
  const releases: GithubRelease[] = [release({ tag_name: "v0.1.0-beta.1" })];
  const result = await checkForUpdate("0.1.0-beta.1", fakeFetch(releases));
  assert.equal(result.status, "uptodate");
});

test("checkForUpdate: current version newer than anything published -> uptodate", async () => {
  const releases: GithubRelease[] = [release({ tag_name: "v0.1.0-beta.1" })];
  const result = await checkForUpdate("9.0.0", fakeFetch(releases));
  assert.equal(result.status, "uptodate");
});

test("checkForUpdate: no releases at all -> uptodate", async () => {
  const result = await checkForUpdate("0.1.0", fakeFetch([]));
  assert.equal(result.status, "uptodate");
});

test("checkForUpdate: non-ok HTTP response -> error, never throws", async () => {
  const result = await checkForUpdate("0.1.0", fakeFetch([], false));
  assert.equal(result.status, "error");
});

test("checkForUpdate: fetch rejecting (network failure) -> error, never throws", async () => {
  const rejecting = (async () => {
    throw new Error("network down");
  }) as unknown as typeof fetch;
  const result = await checkForUpdate("0.1.0", rejecting);
  assert.equal(result.status, "error");
});

test("checkForUpdate: unparsable response body -> error", async () => {
  const result = await checkForUpdate("0.1.0", fakeFetch({ not: "an array" }));
  assert.equal(result.status, "error");
});

test("checkForUpdate: draft releases never count as available updates", async () => {
  const releases: GithubRelease[] = [release({ tag_name: "v9.9.9", draft: true })];
  const result = await checkForUpdate("0.1.0", fakeFetch(releases));
  assert.equal(result.status, "uptodate");
});
