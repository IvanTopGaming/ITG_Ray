import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { eventHandlers, listMock, addGroupMock } = vi.hoisted(() => ({
  eventHandlers: {} as Record<string, (...args: any[]) => void>,
  listMock: vi.fn(),
  addGroupMock: vi.fn(),
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
  ReplaceAll: vi.fn(),
  GroupEdit: vi.fn(),
  GroupRemove: vi.fn(),
  RuleAdd: vi.fn(),
  RuleEdit: vi.fn(),
  RuleRemove: vi.fn(),
  RuleToggle: vi.fn(),
  RuleMove: vi.fn(),
}));

import {
  __bootRulesForTest,
  __resetRulesForTest,
  getRulesState,
  rulesAddGroup,
} from "./rulesStore";

beforeEach(() => {
  for (const k of Object.keys(eventHandlers)) delete eventHandlers[k];
  listMock.mockReset();
  addGroupMock.mockReset();
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
});
