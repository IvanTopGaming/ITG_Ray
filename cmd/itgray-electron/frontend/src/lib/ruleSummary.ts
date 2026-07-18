import type { TFunction } from "i18next";

export function summarise(
  c: {
    domains?: unknown[];
    ip_cidrs?: unknown[];
    geo?: unknown[];
    ports?: unknown[];
    processes?: unknown[];
    protocols?: unknown[];
  },
  t: TFunction,
): string {
  const parts: string[] = [];
  if (c.domains?.length) parts.push(t("routing.cond.domains", { count: c.domains.length }));
  if (c.ip_cidrs?.length) parts.push(t("routing.cond.ipCidrs", { count: c.ip_cidrs.length }));
  if (c.geo?.length) parts.push(t("routing.cond.geo", { count: c.geo.length }));
  if (c.ports?.length) parts.push(t("routing.cond.ports", { count: c.ports.length }));
  if (c.processes?.length) parts.push(t("routing.cond.processes", { count: c.processes.length }));
  if (c.protocols?.length) parts.push(t("routing.cond.protocols", { count: c.protocols.length }));
  return parts.join(" · ") || t("routing.cond.none");
}
