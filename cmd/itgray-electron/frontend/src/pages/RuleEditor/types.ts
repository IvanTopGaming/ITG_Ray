import type { RuleView, Conditions } from "@/lib/rulesStore";

export type ConditionType = "domains" | "ip_cidrs" | "geo" | "ports" | "processes" | "protocols";

export type ConditionDef = {
  key: ConditionType;
  icon: string;
  label: string;
};

export const CONDITION_TYPES: ConditionDef[] = [
  { key: "domains", icon: "\u{1F310}", label: "Domain matcher" },
  { key: "ip_cidrs", icon: "\u{1F522}", label: "IP CIDRs" },
  { key: "geo", icon: "\u{1F4CD}", label: "Geo" },
  { key: "ports", icon: "\u{1F6AA}", label: "Ports" },
  { key: "processes", icon: "⚙️", label: "Processes" },
  { key: "protocols", icon: "\u{1F4E1}", label: "Protocols" },
];

export const EMPTY_DRAFT: Omit<RuleView, "id"> = {
  name: "",
  enabled: true,
  action: "proxy",
  conditions: {},
};

export function hasAnyCondition(rule: RuleView): boolean {
  for (const c of CONDITION_TYPES) {
    const arr = rule.conditions[c.key];
    if (Array.isArray(arr) && arr.length > 0) return true;
  }
  return false;
}

export function conditionCount(rule: RuleView, key: ConditionType): number {
  const arr = rule.conditions[key];
  return Array.isArray(arr) ? arr.length : 0;
}

export type { Conditions };
