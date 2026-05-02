import { describe, expect, it } from "vitest";
import { hub } from "../../wailsjs/go/models";
import { backendToFrontend } from "./subsAdapter";

function makeView(overrides: Partial<hub.SubView> = {}): hub.SubView {
  return hub.SubView.createFrom({
    id: "s1",
    name: "A",
    url: "https://a.test",
    updateInterval: 0,
    lastSyncAt: "0001-01-01T00:00:00Z",
    lastSyncStatus: "",
    lastSyncMessage: "",
    serverCount: 0,
    upload: 0,
    download: 0,
    total: 0,
    expire: null,
    ...overrides,
  });
}

describe("backendToFrontend", () => {
  it("maps a never-synced sub to status 'never' and lastSyncAt null", () => {
    const sub = backendToFrontend(makeView());
    expect(sub.status).toBe("never");
    expect(sub.lastSyncAt).toBeNull();
    expect(sub.lastSyncMessage).toBeUndefined();
  });

  it("maps successful sync to status 'ok' with lastSyncAt epoch ms", () => {
    const sub = backendToFrontend(
      makeView({
        lastSyncStatus: "ok",
        lastSyncAt: "2026-04-30T10:30:00Z",
        lastSyncMessage: "imported=3",
      }),
    );
    expect(sub.status).toBe("ok");
    expect(sub.lastSyncAt).toBe(Date.parse("2026-04-30T10:30:00Z"));
    expect(sub.lastSyncMessage).toBe("imported=3");
  });

  it("maps failed sync to status 'error' with message preserved", () => {
    const sub = backendToFrontend(
      makeView({
        lastSyncStatus: "error",
        lastSyncAt: "2026-04-30T10:30:00Z",
        lastSyncMessage: "connection refused",
      }),
    );
    expect(sub.status).toBe("error");
    expect(sub.lastSyncMessage).toBe("connection refused");
  });

  it("normalizes legacy uppercase 'OK' / 'ERROR' / ' Ok ' from older builds", () => {
    expect(backendToFrontend(makeView({ lastSyncStatus: "OK", lastSyncAt: "2026-04-30T10:30:00Z" })).status).toBe("ok");
    expect(backendToFrontend(makeView({ lastSyncStatus: "ERROR", lastSyncAt: "2026-04-30T10:30:00Z" })).status).toBe("error");
    expect(backendToFrontend(makeView({ lastSyncStatus: " Ok ", lastSyncAt: "2026-04-30T10:30:00Z" })).status).toBe("ok");
    // Prefixed ERROR form falls through to error (fail-safe correct).
    expect(backendToFrontend(makeView({ lastSyncStatus: "ERROR: timeout", lastSyncAt: "2026-04-30T10:30:00Z" })).status).toBe("error");
  });

  it("normalizes legacy prefixed 'OK <summary>' written by pre-Tier-2a CLI", () => {
    // Pre-PR cmd/itgray-cli/subs.go wrote `"OK "+meta.Summary` (e.g.
    // "OK imported=3 invalid=0 skipped=0"). On upgrade those rows must
    // render as ok, not silently flip to a red ERROR badge.
    expect(
      backendToFrontend(makeView({ lastSyncStatus: "OK imported=3 invalid=0 skipped=0", lastSyncAt: "2026-04-30T10:30:00Z" })).status,
    ).toBe("ok");
  });

  it("treats unknown status as error (fail-safe)", () => {
    const sub = backendToFrontend(
      makeView({
        lastSyncStatus: "weird",
        lastSyncAt: "2026-04-30T10:30:00Z",
      }),
    );
    expect(sub.status).toBe("error");
  });

  it("surfaces upload/download/total when non-zero", () => {
    const sub = backendToFrontend(
      makeView({ upload: 100, download: 200, total: 1024 }),
    );
    expect(sub.upload).toBe(100);
    expect(sub.download).toBe(200);
    expect(sub.total).toBe(1024);
  });

  it("treats zero quota fields as undefined (omitempty round-trip)", () => {
    const sub = backendToFrontend(makeView());
    expect(sub.upload).toBeUndefined();
    expect(sub.download).toBeUndefined();
    expect(sub.total).toBeUndefined();
  });

  it("converts expire to epoch ms or undefined when absent", () => {
    const withExpire = backendToFrontend(
      makeView({ expire: "2027-01-01T00:00:00Z" as any }),
    );
    expect(withExpire.expire).toBe(Date.parse("2027-01-01T00:00:00Z"));

    const withoutExpire = backendToFrontend(makeView({ expire: null as any }));
    expect(withoutExpire.expire).toBeUndefined();
  });

  it("copies id, name, url, serverCount unchanged", () => {
    const sub = backendToFrontend(
      makeView({ id: "x", name: "Y", url: "https://z", serverCount: 7 }),
    );
    expect(sub.id).toBe("x");
    expect(sub.name).toBe("Y");
    expect(sub.url).toBe("https://z");
    expect(sub.serverCount).toBe(7);
  });

  it("maps userAgent through, treating empty as undefined", () => {
    const a = backendToFrontend(makeView({ userAgent: "Custom/1.0" }));
    expect(a.userAgent).toBe("Custom/1.0");
    const b = backendToFrontend(makeView({ userAgent: "" }));
    expect(b.userAgent).toBeUndefined();
  });
});
