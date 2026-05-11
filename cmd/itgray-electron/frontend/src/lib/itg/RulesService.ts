// cmd/itgray-electron/frontend/src/lib/itg/RulesService.ts
//
// Mirror wails-shim/bindings/*Service.ts. Methods route to
// window.itg.rules.* through the preload bridge — see
// cmd/itgray-electron/src/preload/preload.ts for the actual
// ipcMain channels backing each call. Wire names are the flat
// one-dot form mounted in T4 (rules.list, rules.replaceAll, ...).

const svc = () => (window.itg.rules ?? {}) as Record<string, (...args: unknown[]) => Promise<unknown>>;

export function List(): Promise<unknown> {
  return svc().list?.() ?? Promise.resolve(null);
}

export function ReplaceAll(params: { model: unknown }): Promise<unknown> {
  return svc().replaceAll?.(params) ?? Promise.resolve(null);
}

export function GroupAdd(params: { name: string }): Promise<{ id: string }> {
  return (svc().groupAdd?.(params) as Promise<{ id: string }>) ?? Promise.resolve({ id: "" });
}

export function GroupEdit(params: { id: string; name: string; enabled: boolean }): Promise<unknown> {
  return svc().groupEdit?.(params) ?? Promise.resolve(null);
}

export function GroupRemove(params: { id: string }): Promise<unknown> {
  return svc().groupRemove?.(params) ?? Promise.resolve(null);
}

export function RuleAdd(params: { groupId: string; rule: unknown }): Promise<{ id: string }> {
  return (svc().ruleAdd?.(params) as Promise<{ id: string }>) ?? Promise.resolve({ id: "" });
}

export function RuleEdit(params: { rule: unknown }): Promise<unknown> {
  return svc().ruleEdit?.(params) ?? Promise.resolve(null);
}

export function RuleRemove(params: { id: string }): Promise<unknown> {
  return svc().ruleRemove?.(params) ?? Promise.resolve(null);
}

export function RuleToggle(params: { id: string }): Promise<unknown> {
  return svc().ruleToggle?.(params) ?? Promise.resolve(null);
}

export function RuleMove(params: { id: string; toGroupId: string }): Promise<unknown> {
  return svc().ruleMove?.(params) ?? Promise.resolve(null);
}
