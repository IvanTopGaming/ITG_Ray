import { useSyncExternalStore } from "react";

import * as RulesService from "@/lib/itg/RulesService";
import { EventsOn } from "@/lib/itg/runtime";

// rulesStore mirrors serversStore: a singleton store backed by
// useSyncExternalStore, lazy-bootstrapped on first hook mount, with a
// single-flight mutex around mutations. Backend is authoritative; every
// mutation refetches via RulesService.List, and the 'rules:changed' event
// triggers a passive refetch when something else (CLI / configgen) edits
// the model.

export type Action = "proxy" | "direct" | "block";

export type DomainMatcher = { kind: "exact" | "suffix" | "keyword" | "regex"; value: string };
export type PortSpec = { single?: number; from?: number; to?: number };

export type Conditions = {
  processes?: string[];
  domains?: DomainMatcher[];
  ip_cidrs?: string[];
  geo?: string[];
  ports?: PortSpec[];
  protocols?: string[];
};

export type RuleView = {
  id: string;
  name: string;
  enabled: boolean;
  action: Action;
  conditions: Conditions;
};

export type GroupView = {
  id: string;
  name: string;
  locked: boolean;
  enabled: boolean;
  rules: RuleView[];
};

export type RulesState = {
  defaultAction: Action;
  groups: GroupView[];
  loading: boolean;
  lastError: string | null;
  bootstrapped: boolean;
};

const initialState = (): RulesState => ({
  defaultAction: "proxy",
  groups: [],
  loading: false,
  lastError: null,
  bootstrapped: false,
});

let state: RulesState = initialState();
const listeners = new Set<() => void>();

function notify() {
  for (const l of listeners) l();
}

function setState(next: RulesState) {
  state = next;
  notify();
}

function subscribe(cb: () => void) {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

let bootInFlight: Promise<void> | null = null;
let mutationInFlight: Promise<void> | null = null;
let unsubscribeEvent: (() => void) | null = null;

async function refetch(): Promise<void> {
  try {
    const v = (await RulesService.List()) as
      | { defaultAction: Action; groups: GroupView[] }
      | null;
    setState({
      defaultAction: (v?.defaultAction ?? "proxy") as Action,
      groups: v?.groups ?? [],
      loading: false,
      lastError: null,
      bootstrapped: true,
    });
  } catch (err: any) {
    setState({
      ...state,
      loading: false,
      lastError: err?.message ?? String(err),
      bootstrapped: true,
    });
  }
}

function ensureBoot(): Promise<void> {
  if (state.bootstrapped) return Promise.resolve();
  if (!bootInFlight) {
    // Subscribe inside boot (not at module load) so __resetRulesForTest can
    // tear down the subscription and a fresh boot will re-register. Mirrors
    // the serversStore pattern.
    if (!unsubscribeEvent) {
      unsubscribeEvent = EventsOn("rules:changed", () => {
        void refetch();
      });
    }
    bootInFlight = refetch().finally(() => {
      bootInFlight = null;
    });
  }
  return bootInFlight;
}

function withSingleFlight<T>(fn: () => Promise<T>): Promise<T> {
  if (mutationInFlight) return mutationInFlight.then(fn);
  const p = fn().finally(() => {
    mutationInFlight = null;
  });
  mutationInFlight = p.then(
    () => undefined,
    () => undefined,
  );
  return p;
}

export function useRules(): RulesState {
  if (!state.bootstrapped && !bootInFlight) void ensureBoot();
  return useSyncExternalStore(
    subscribe,
    () => state,
    () => state,
  );
}

export function getRulesState(): RulesState {
  return state;
}

export function rulesAddGroup(name: string): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.GroupAdd({ name });
    await refetch();
  });
}

export function rulesEditGroup(id: string, name: string, enabled: boolean): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.GroupEdit({ id, name, enabled });
    await refetch();
  });
}

export function rulesRemoveGroup(id: string): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.GroupRemove({ id });
    await refetch();
  });
}

export function rulesAddRule(
  groupId: string,
  rule: Omit<RuleView, "id">,
): Promise<string> {
  return withSingleFlight(async () => {
    const { id } = await RulesService.RuleAdd({ groupId, rule: { id: "", ...rule } });
    await refetch();
    return id;
  });
}

export function rulesEditRule(rule: RuleView): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.RuleEdit({ rule });
    await refetch();
  });
}

export function rulesRemoveRule(id: string): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.RuleRemove({ id });
    await refetch();
  });
}

export function rulesToggleRule(id: string): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.RuleToggle({ id });
    await refetch();
  });
}

export function rulesMoveRule(id: string, toGroupId: string): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.RuleMove({ id, toGroupId });
    await refetch();
  });
}

export function rulesReplaceAll(model: {
  defaultAction: Action;
  groups: GroupView[];
}): Promise<void> {
  return withSingleFlight(async () => {
    await RulesService.ReplaceAll({ model });
    await refetch();
  });
}

// Test-only helpers.
export function __resetRulesForTest(): void {
  state = initialState();
  listeners.clear();
  bootInFlight = null;
  mutationInFlight = null;
  if (unsubscribeEvent) {
    unsubscribeEvent();
    unsubscribeEvent = null;
  }
}

export function __bootRulesForTest(): Promise<void> {
  return ensureBoot();
}
