import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const {
  eventHandlers,
  listMock,
  addGroupMock,
  getDashStateMock,
  setCurrentRulesSignatureMock,
  importPreviewMock,
  importApplyMock,
  exportGroupMock,
  connectSnapshotHandlers,
  replaceAllMock,
} = vi.hoisted(() => ({
  connectSnapshotHandlers: new Set<() => void>(),
  replaceAllMock: vi.fn(),
  eventHandlers: {} as Record<string, (...args: any[]) => void>,
  listMock: vi.fn(),
  addGroupMock: vi.fn(),
  getDashStateMock: vi.fn(),
  setCurrentRulesSignatureMock: vi.fn(),
  importPreviewMock: vi.fn(),
  importApplyMock: vi.fn(),
  exportGroupMock: vi.fn(),
}));

vi.mock("@/lib/itg/runtime", () => ({
  EventsOn: (name: string, cb: (...args: any[]) => void) => {
    eventHandlers[name] = cb;
    return () => {
      delete eventHandlers[name];
    };
  },
}));

vi.mock("@/lib/itg/RulesService", () => ({
  List: () => listMock(),
  GroupAdd: (params: { name: string }) => addGroupMock(params),
  ReplaceAll: (params: any) => replaceAllMock(params),
  GroupEdit: vi.fn(),
  GroupRemove: vi.fn(),
  RuleAdd: vi.fn(),
  RuleEdit: vi.fn(),
  RuleRemove: vi.fn(),
  RuleToggle: vi.fn(),
  RuleMove: vi.fn(),
  ImportPreview: (params: { link: string }) => importPreviewMock(params),
  ImportApply: (params: { link: string }) => importApplyMock(params),
  ExportGroup: (params: { groupId: string }) => exportGroupMock(params),
}));

vi.mock("@/lib/dashStore", () => ({
  getDashState: () => getDashStateMock(),
}));

vi.mock("@/lib/settings", () => ({
  setCurrentRulesSignature: (sig: string) => setCurrentRulesSignatureMock(sig),
  onConnectSnapshot: (cb: () => void) => {
    connectSnapshotHandlers.add(cb);
    return () => {
      connectSnapshotHandlers.delete(cb);
    };
  },
}));

import {
  __bootRulesForTest,
  __resetRulesForTest,
  getRulesState,
  rulesAddGroup,
  rulesExportGroup,
  rulesImportApply,
  rulesImportPreview,
  rulesSignature,
  rulesReplaceAll,
  rulesRevertToBaseline,
} from "./rulesStore";

beforeEach(() => {
  for (const k of Object.keys(eventHandlers)) delete eventHandlers[k];
  listMock.mockReset();
  addGroupMock.mockReset();
  getDashStateMock.mockReset();
  setCurrentRulesSignatureMock.mockReset();
  importPreviewMock.mockReset();
  importApplyMock.mockReset();
  exportGroupMock.mockReset();
  replaceAllMock.mockReset();
  replaceAllMock.mockResolvedValue(undefined);
  connectSnapshotHandlers.clear();
  // Default: chain is idle so mutations do NOT arm the toast unless a
  // test opts in by overriding the dash status.
  getDashStateMock.mockReturnValue({ status: "idle" });
  __resetRulesForTest();
});

afterEach(() => vi.useRealTimers());

const baseView = {
  defaultAction: "proxy",
  groups: [
    { id: "safety", name: "Safety", locked: true, enabled: true, rules: [] },
    { id: "user", name: "My Rules", locked: false, enabled: true, rules: [] },
  ],
};

describe("rulesStore", () => {
  it("rulesSignature is stable and reflects defaultAction + groups", () => {
    const a = rulesSignature({ defaultAction: "proxy", groups: [], loading: false, lastError: null, bootstrapped: true });
    const b = rulesSignature({ defaultAction: "proxy", groups: [], loading: false, lastError: null, bootstrapped: true });
    const c = rulesSignature({ defaultAction: "direct", groups: [], loading: false, lastError: null, bootstrapped: true });
    expect(a).toBe(b);
    expect(a).not.toBe(c);
  });

  it("bootstraps from RulesService.List on first read", async () => {
    listMock.mockResolvedValue(baseView);
    await __bootRulesForTest();
    expect(getRulesState().groups[0].id).toBe("safety");
  });

  it("re-fetches on rules:changed event", async () => {
    listMock.mockResolvedValue(baseView);
    await __bootRulesForTest();
    listMock.mockResolvedValue({
      ...baseView,
      groups: [
        ...baseView.groups,
        { id: "g3", name: "Custom", locked: false, enabled: true, rules: [] },
      ],
    });
    eventHandlers["rules:changed"]?.();
    await new Promise((r) => setTimeout(r, 0));
    expect(getRulesState().groups).toHaveLength(3);
  });

  it("rulesAddGroup calls RulesService and refetches", async () => {
    listMock.mockResolvedValue(baseView);
    await __bootRulesForTest();
    addGroupMock.mockResolvedValue({ id: "g3" });
    listMock.mockResolvedValue(baseView);
    await rulesAddGroup("Streaming");
    expect(addGroupMock).toHaveBeenCalledWith({ name: "Streaming" });
  });

  it("rulesAddGroup republishes the canonical rules signature", async () => {
    listMock.mockResolvedValue(baseView);
    await __bootRulesForTest();
    addGroupMock.mockResolvedValue({ id: "g3" });
    listMock.mockResolvedValue(baseView);
    setCurrentRulesSignatureMock.mockReset();
    await rulesAddGroup("Streaming");
    expect(setCurrentRulesSignatureMock).toHaveBeenCalledTimes(1);
    expect(setCurrentRulesSignatureMock).toHaveBeenCalledWith(
      rulesSignature(getRulesState()),
    );
  });
});

describe("rules import/export", () => {
  it("rulesImportPreview returns the backend preview without mutating", async () => {
    const preview = {
      name: "Streaming",
      groups: [{ id: "g1", name: "S", locked: false, enabled: true, rules: [] }],
      proxyCount: 3,
      directCount: 1,
      blockCount: 0,
    };
    importPreviewMock.mockResolvedValue(preview);
    const got = await rulesImportPreview("itgray://rules/import/abc");
    expect(importPreviewMock).toHaveBeenCalledWith({ link: "itgray://rules/import/abc" });
    expect(got).toEqual(preview);
    expect(listMock).not.toHaveBeenCalled();
  });

  it("rulesImportApply calls RulesService.ImportApply and refetches", async () => {
    listMock.mockResolvedValue(baseView);
    await __bootRulesForTest();
    importApplyMock.mockResolvedValue(null);
    listMock.mockResolvedValue(baseView);
    await rulesImportApply("itgray://rules/import/abc");
    expect(importApplyMock).toHaveBeenCalledWith({ link: "itgray://rules/import/abc" });
    expect(listMock).toHaveBeenCalledTimes(2);
  });

  it("rulesImportApply republishes the canonical rules signature", async () => {
    listMock.mockResolvedValue(baseView);
    await __bootRulesForTest();
    importApplyMock.mockResolvedValue(null);
    listMock.mockResolvedValue(baseView);
    setCurrentRulesSignatureMock.mockReset();
    await rulesImportApply("itgray://rules/import/abc");
    expect(setCurrentRulesSignatureMock).toHaveBeenCalledTimes(1);
    expect(setCurrentRulesSignatureMock).toHaveBeenCalledWith(
      rulesSignature(getRulesState()),
    );
  });

  it("rulesExportGroup returns the link string", async () => {
    exportGroupMock.mockResolvedValue({ link: "itgray://rules/import/xyz" });
    const link = await rulesExportGroup("g1");
    expect(exportGroupMock).toHaveBeenCalledWith({ groupId: "g1" });
    expect(link).toBe("itgray://rules/import/xyz");
  });

  describe("revert baseline", () => {
    const reordered = {
      defaultAction: "proxy" as const,
      groups: [baseView.groups[1], baseView.groups[0]],
    };

    it("restores the pre-edit model when the user dismisses the reconnect prompt", async () => {
      listMock.mockResolvedValue(baseView);
      await __bootRulesForTest();

      listMock.mockResolvedValue(reordered);
      await rulesReplaceAll(reordered);
      expect(getRulesState().groups[0].id).toBe("user");

      listMock.mockResolvedValue(baseView);
      replaceAllMock.mockClear();
      const reverted = await rulesRevertToBaseline();

      expect(reverted).toBe(true);
      expect(replaceAllMock).toHaveBeenCalledWith({
        model: { default_action: "proxy", groups: baseView.groups },
      });
      expect(getRulesState().groups[0].id).toBe("safety");
    });

    it("captures the baseline once so a run of edits reverts to the original", async () => {
      listMock.mockResolvedValue(baseView);
      await __bootRulesForTest();

      listMock.mockResolvedValue(reordered);
      await rulesReplaceAll(reordered);
      addGroupMock.mockResolvedValue(undefined);
      await rulesAddGroup("Another");

      listMock.mockResolvedValue(baseView);
      replaceAllMock.mockClear();
      await rulesRevertToBaseline();

      expect(replaceAllMock).toHaveBeenCalledWith({
        model: { default_action: "proxy", groups: baseView.groups },
      });
    });

    it("is a no-op when nothing was edited", async () => {
      listMock.mockResolvedValue(baseView);
      await __bootRulesForTest();
      replaceAllMock.mockClear();

      expect(await rulesRevertToBaseline()).toBe(false);
      expect(replaceAllMock).not.toHaveBeenCalled();
    });

    it("republishes the signature so the reconnect toast disarms", async () => {
      listMock.mockResolvedValue(baseView);
      await __bootRulesForTest();
      listMock.mockResolvedValue(reordered);
      await rulesReplaceAll(reordered);

      listMock.mockResolvedValue(baseView);
      setCurrentRulesSignatureMock.mockReset();
      await rulesRevertToBaseline();

      expect(setCurrentRulesSignatureMock).toHaveBeenCalledWith(
        rulesSignature(getRulesState()),
      );
    });

    it("drops the baseline once a reconnect commits the edits", async () => {
      listMock.mockResolvedValue(baseView);
      await __bootRulesForTest();
      listMock.mockResolvedValue(reordered);
      await rulesReplaceAll(reordered);

      // A fresh connect snapshot means the edited model is now the live one.
      for (const cb of connectSnapshotHandlers) cb();

      replaceAllMock.mockClear();
      expect(await rulesRevertToBaseline()).toBe(false);
      expect(replaceAllMock).not.toHaveBeenCalled();
    });

    it("reverting twice does not walk further back", async () => {
      listMock.mockResolvedValue(baseView);
      await __bootRulesForTest();
      listMock.mockResolvedValue(reordered);
      await rulesReplaceAll(reordered);

      listMock.mockResolvedValue(baseView);
      await rulesRevertToBaseline();
      replaceAllMock.mockClear();

      expect(await rulesRevertToBaseline()).toBe(false);
      expect(replaceAllMock).not.toHaveBeenCalled();
    });
  });
});
