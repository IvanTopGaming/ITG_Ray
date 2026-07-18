import { describe, expect, it, vi } from "vitest";
import {
  consumePendingImportLink,
  setPendingImportLink,
  subscribePendingImport,
} from "./deeplinkStore";

describe("deeplinkStore", () => {
  it("consumePendingImportLink returns the link then null on second consume", () => {
    setPendingImportLink("itgray://rules/import/abc");
    expect(consumePendingImportLink()).toBe("itgray://rules/import/abc");
    expect(consumePendingImportLink()).toBeNull();
  });

  it("returns null when nothing is pending", () => {
    expect(consumePendingImportLink()).toBeNull();
  });

  it("subscribePendingImport fires listeners on set", () => {
    const cb = vi.fn();
    const unsubscribe = subscribePendingImport(cb);
    setPendingImportLink("itgray://rules/import/xyz");
    expect(cb).toHaveBeenCalledTimes(1);
    unsubscribe();
  });

  it("unsubscribe stops the listener from firing", () => {
    const cb = vi.fn();
    const unsubscribe = subscribePendingImport(cb);
    unsubscribe();
    setPendingImportLink("itgray://rules/import/again");
    expect(cb).not.toHaveBeenCalled();
  });
});
