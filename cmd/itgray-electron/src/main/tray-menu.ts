import type { TrayStrings } from "./tray-i18n";

export type TrayStatus = "idle" | "connecting" | "connected" | "disconnecting" | "error";
export type TrayMode = "tun" | "sysproxy";
export type TrayRoutingMode = "rule" | "global" | "direct";

export interface TrayServer {
  id: string;
  name: string;
  country: string;
  origin: string;
  favorite: boolean;
}

export interface TrayView {
  status: TrayStatus;
  serverId: string | null;
  mode: TrayMode;
  servers: TrayServer[];
  hasSubs: boolean;
  syncing: boolean;
  routingMode: TrayRoutingMode;
}

export type TrayCommand =
  | { kind: "connect" }
  | { kind: "disconnect" }
  | { kind: "selectServer"; id: string }
  | { kind: "setMode"; mode: TrayMode }
  | { kind: "setRoutingMode"; mode: TrayRoutingMode }
  | { kind: "syncAll" }
  | { kind: "cancelSync" }
  | { kind: "open" }
  | { kind: "navigate"; path: string }
  | { kind: "quit" };

export interface TrayItem {
  label?: string;
  type?: "normal" | "separator" | "radio";
  enabled?: boolean;
  checked?: boolean;
  submenu?: TrayItem[];
  command?: TrayCommand;
}

const SEP: TrayItem = { type: "separator" };

export function flagEmoji(code: string): string {
  if (!/^[A-Za-z]{2}$/.test(code)) return "";
  const base = 0x1f1e6;
  const up = code.toUpperCase();
  return String.fromCodePoint(base + up.charCodeAt(0) - 65, base + up.charCodeAt(1) - 65);
}

export function serverLabel(s: TrayServer): string {
  const flag = flagEmoji(s.country);
  return flag ? `${flag} ${s.name}` : s.name;
}

export function statusLabel(status: TrayStatus, t: TrayStrings): string {
  switch (status) {
    case "connecting":
      return t.statusConnecting;
    case "connected":
      return t.statusConnected;
    case "disconnecting":
      return t.statusDisconnecting;
    case "error":
      return t.statusError;
    default:
      return t.statusIdle;
  }
}

export function statusLine(view: TrayView, t: TrayStrings): string {
  const label = statusLabel(view.status, t);
  if (view.status !== "connected" && view.status !== "connecting") return label;
  const srv = view.servers.find((s) => s.id === view.serverId);
  return srv ? `● ${label} · ${serverLabel(srv)}` : `● ${label}`;
}

function formatMbps(bps: number): string {
  return `${(bps / 125_000).toFixed(1)} Mbps`;
}

export function tooltip(view: TrayView, t: TrayStrings, speed: { downBps: number; upBps: number } | null): string {
  const head = `ITG Ray — ${statusLine(view, t).replace(/^● /, "")}`;
  if (view.status !== "connected" || !speed) return head;
  return `${head}\n↓ ${formatMbps(speed.downBps)}  ↑ ${formatMbps(speed.upBps)}`;
}

function radio(s: TrayServer, view: TrayView, enabled: boolean): TrayItem {
  return {
    label: serverLabel(s),
    type: "radio",
    checked: s.id === view.serverId,
    enabled,
    command: { kind: "selectServer", id: s.id },
  };
}

function serverSubmenu(view: TrayView, t: TrayStrings, enabled: boolean): TrayItem[] {
  if (view.servers.length === 0) return [{ label: t.noServers, enabled: false }];
  const favorites = view.servers.filter((s) => s.favorite);
  const groups = new Map<string, TrayServer[]>();
  for (const s of view.servers) {
    const key = s.origin || "manual";
    const list = groups.get(key);
    if (list) list.push(s);
    else groups.set(key, [s]);
  }
  if (favorites.length === 0 && groups.size === 1) {
    return view.servers.map((s) => radio(s, view, enabled));
  }
  const out: TrayItem[] = [];
  if (favorites.length > 0) {
    out.push({ label: t.favorites, enabled: false });
    for (const s of favorites) out.push(radio(s, view, enabled));
  }
  for (const [origin, list] of groups) {
    if (out.length > 0) out.push(SEP);
    out.push({ label: origin === "manual" ? t.manual : origin, enabled: false });
    for (const s of list) out.push(radio(s, view, enabled));
  }
  return out;
}

function connectItem(view: TrayView, t: TrayStrings): TrayItem {
  switch (view.status) {
    case "connected":
      return { label: t.disconnect, command: { kind: "disconnect" } };
    case "connecting":
      return { label: t.cancelConnect, command: { kind: "disconnect" } };
    case "disconnecting":
      return { label: t.disconnect, enabled: false };
    default:
      return { label: t.connect, enabled: view.servers.length > 0, command: { kind: "connect" } };
  }
}

export function buildTrayMenu(view: TrayView, t: TrayStrings): TrayItem[] {
  const transitioning = view.status === "disconnecting";
  const modeLocked = view.status === "connecting" || view.status === "disconnecting";
  return [
    { label: statusLine(view, t), enabled: false },
    SEP,
    connectItem(view, t),
    { label: t.server, submenu: serverSubmenu(view, t, !transitioning) },
    {
      label: t.mode,
      submenu: [
        { label: t.modeTun, type: "radio", checked: view.mode === "tun", enabled: !modeLocked, command: { kind: "setMode", mode: "tun" } },
        { label: t.modeSysproxy, type: "radio", checked: view.mode === "sysproxy", enabled: !modeLocked, command: { kind: "setMode", mode: "sysproxy" } },
      ],
    },
    {
      label: t.routing,
      submenu: ([
        ["rule", t.routingRule],
        ["global", t.routingGlobal],
        ["direct", t.routingDirect],
      ] as const).map(([mode, label]) => ({
        label,
        type: "radio" as const,
        checked: view.routingMode === mode,
        enabled: view.status === "connected",
        command: { kind: "setRoutingMode" as const, mode },
      })),
    },
    SEP,
    view.syncing
      ? { label: t.cancelSync, command: { kind: "cancelSync" } }
      : { label: t.syncSubs, enabled: view.hasSubs, command: { kind: "syncAll" } },
    SEP,
    { label: t.open, command: { kind: "open" } },
    { label: t.logs, command: { kind: "navigate", path: "/logs" } },
    SEP,
    { label: t.quit, command: { kind: "quit" } },
  ];
}
