import type { TFunction } from "i18next";

export function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const KB = 1024;
  const MB = KB * 1024;
  const GB = MB * 1024;
  if (bytes < KB) return `${bytes.toFixed(0)} B`;
  if (bytes < MB) return `${(bytes / KB).toFixed(0)} KB`;
  if (bytes < GB) return `${(bytes / MB).toFixed(1)} MB`;
  return `${(bytes / GB).toFixed(2)} GB`;
}

export function formatBytesShort(bytes: number): string {
  if (!bytes) return "0";
  const KB = 1024;
  const MB = KB * 1024;
  const GB = MB * 1024;
  if (bytes < KB) return `${bytes.toFixed(0)} B`;
  if (bytes < MB) return `${(bytes / KB).toFixed(0)} KB`;
  if (bytes < GB) return `${(bytes / MB).toFixed(bytes < 10 * MB ? 1 : 0)} MB`;
  return `${(bytes / GB).toFixed(1)} GB`;
}

export function formatCount(n: number, locale: string): string {
  return n.toLocaleString(locale === "ru" ? "ru-RU" : "en-US");
}

export function formatRate(bps: number): string {
  if (!bps) return "—";
  return `${formatBytes(bps)}/s`;
}

export function formatDuration(seconds: number, t: TFunction): string {
  const total = Math.max(0, Math.floor(seconds / 60));
  const h = Math.floor(total / 60);
  const m = total % 60;
  if (h === 0) return t("stats.duration.m", { m });
  return t("stats.duration.hm", { h, m });
}

export type StatsPoint = {
  start: string;
  down: number;
  up: number;
  downProxy: number;
  downDirect: number;
  upProxy: number;
  upDirect: number;
};

export type StatsApp = { app: string; down: number; up: number; downProxy: number; upProxy: number };

export type StatsRule = {
  ruleId: string;
  ruleName: string;
  action: string;
  route: string;
  serverId?: string;
  serverName?: string;
  deleted?: boolean;
  conns: number;
};

export type StatsOverview = {
  period: string;
  down: number;
  up: number;
  downProxy: number;
  downDirect: number;
  upProxy: number;
  upDirect: number;
  blocked: number;
  onlineSeconds: number;
  sessions: number;
  series: StatsPoint[];
  apps: StatsApp[];
  rules: StatsRule[];
};

export type StatsConn = {
  id: string;
  app: string;
  host: string;
  suffix?: string;
  destIp?: string;
  network?: string;
  ruleId?: string;
  ruleName?: string;
  ruleGroupId?: string;
  ruleGroupLocked?: boolean;
  route: string;
  serverId?: string;
  serverName?: string;
  downBps: number;
  upBps: number;
  start?: string;
};

export type Period = "today" | "7d" | "30d";
