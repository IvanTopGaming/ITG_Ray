import { beforeEach, describe, expect, it, vi } from "vitest";
import { Add, Edit } from "./SubsService";

type SubsStub = { add: ReturnType<typeof vi.fn>; edit: ReturnType<typeof vi.fn> };

function stubSubs(): SubsStub {
  const subs: SubsStub = {
    add: vi.fn().mockResolvedValue(null),
    edit: vi.fn().mockResolvedValue(null),
  };
  const w = window as unknown as { itg: Record<string, unknown> };
  w.itg = { ...(w.itg ?? {}), subs };
  return subs;
}

describe("SubsService binding shim", () => {
  beforeEach(() => {
    stubSubs();
  });

  it("Add forwards the per-subscription userAgent to the bridge", async () => {
    const subs = stubSubs();
    await Add("https://x/y", "Custom/1.0");
    expect(subs.add).toHaveBeenCalledWith({
      url: "https://x/y",
      userAgent: "Custom/1.0",
    });
  });

  it("Edit forwards the per-subscription userAgent to the bridge", async () => {
    const subs = stubSubs();
    await Edit("u9", "https://x/y", "Custom/2.0");
    expect(subs.edit).toHaveBeenCalledWith({
      id: "u9",
      url: "https://x/y",
      userAgent: "Custom/2.0",
    });
  });
});
