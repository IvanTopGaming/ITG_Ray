import { TRAY_STRINGS, resolveTrayLang, type TrayLang } from "./tray-i18n";
import {
  buildTrayMenu,
  tooltip,
  type TrayCommand,
  type TrayItem,
  type TrayMode,
  type TrayRoutingMode,
  type TrayServer,
  type TrayStatus,
  type TrayView,
} from "./tray-menu";

export interface RendererReport {
  mode?: string;
  syncing?: boolean;
  language?: string;
  lastServerId?: string | null;
}

export interface TrayControllerDeps {
  rpc: (method: string, params?: unknown) => Promise<unknown>;
  hasRenderer: () => boolean;
  sendToRenderer: (cmd: TrayCommand) => void;
  showWindow: () => Promise<void>;
  queueNavigate: (path: string) => void;
  quit: () => void;
  setMenu: (items: TrayItem[]) => void;
  setTooltip: (text: string) => void;
  setIcon: (status: TrayStatus) => void;
  locale: () => string;
  loadLastServer: () => string | null;
  saveLastServer: (id: string) => void;
  idleTimeoutMs?: number;
  warn?: (msg: string, err: unknown) => void;
}

export interface TrayController {
  refresh: () => Promise<void>;
  onVpnStatus: (payload: unknown) => void;
  onVpnSpeed: (payload: unknown) => void;
  onRoutingMode: (payload: unknown) => void;
  onRendererReport: (report: RendererReport) => void;
  onRendererGone: () => void;
  dispatch: (cmd: TrayCommand) => Promise<void>;
  view: () => TrayView;
}

const STATUSES: TrayStatus[] = ["idle", "connecting", "connected", "disconnecting", "error"];

function asStatus(v: unknown): TrayStatus | null {
  return typeof v === "string" && (STATUSES as string[]).includes(v) ? (v as TrayStatus) : null;
}

function asRoutingMode(v: unknown): TrayRoutingMode | null {
  return v === "rule" || v === "global" || v === "direct" ? v : null;
}

function asMode(v: unknown): TrayMode | null {
  return v === "tun" || v === "sysproxy" ? v : null;
}

function toServers(v: unknown): TrayServer[] {
  if (!Array.isArray(v)) return [];
  const out: TrayServer[] = [];
  for (const raw of v) {
    const s = raw as Partial<TrayServer> | null;
    if (!s || typeof s.id !== "string") continue;
    out.push({
      id: s.id,
      name: typeof s.name === "string" && s.name ? s.name : s.id,
      country: typeof s.country === "string" ? s.country : "",
      origin: typeof s.origin === "string" ? s.origin : "manual",
      favorite: s.favorite === true,
    });
  }
  return out;
}

export function createTrayController(deps: TrayControllerDeps): TrayController {
  const warn = deps.warn ?? ((msg, err) => console.warn(msg, err));
  const idleTimeoutMs = deps.idleTimeoutMs ?? 15_000;

  let status: TrayStatus = "idle";
  let activeServerId: string | null = null;
  let mode: TrayMode = "tun";
  let routingMode: TrayRoutingMode = "rule";
  let servers: TrayServer[] = [];
  let hasSubs = false;
  let ownSyncs = 0;
  let rendererSyncing = false;
  let languageSetting: string | undefined;
  let speed: { downBps: number; upBps: number } | null = null;
  let idleWaiters: Array<() => void> = [];
  let lastMenuKey = "";

  function lang(): TrayLang {
    return resolveTrayLang(languageSetting, deps.locale());
  }

  function view(): TrayView {
    const live = status === "connected" || status === "connecting";
    return {
      status,
      serverId: live && activeServerId ? activeServerId : deps.loadLastServer(),
      mode,
      servers,
      hasSubs,
      syncing: ownSyncs > 0 || rendererSyncing,
      routingMode,
    };
  }

  function render(): void {
    const v = view();
    const t = TRAY_STRINGS[lang()];
    const items = buildTrayMenu(v, t);
    const key = JSON.stringify(items);
    if (key !== lastMenuKey) {
      lastMenuKey = key;
      deps.setMenu(items);
    }
    deps.setTooltip(tooltip(v, t, speed));
  }

  function setStatus(next: TrayStatus): void {
    if (next === status) return;
    status = next;
    deps.setIcon(status);
    if (status === "idle" || status === "error") {
      routingMode = "rule";
      const waiters = idleWaiters;
      idleWaiters = [];
      for (const w of waiters) w();
    }
  }

  function waitForIdle(): Promise<void> {
    if (status === "idle" || status === "error") return Promise.resolve();
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        idleWaiters = idleWaiters.filter((w) => w !== done);
        reject(new Error("tray: timed out waiting for disconnect"));
      }, idleTimeoutMs);
      const done = () => {
        clearTimeout(timer);
        resolve();
      };
      idleWaiters.push(done);
    });
  }

  let refreshInFlight: Promise<void> | null = null;
  let refreshAgain = false;

  async function doRefresh(): Promise<void> {
    const snap = (await deps.rpc("app.getSnapshot")) as {
      status?: string;
      currentServer?: { id?: string } | null;
      mode?: string;
      servers?: unknown;
      subs?: unknown[];
      settings?: { general?: { language?: string } };
    } | null;
    if (!snap) return;
    const s = asStatus(snap.status);
    if (s) setStatus(s);
    if (typeof snap.currentServer?.id === "string") activeServerId = snap.currentServer.id;
    const m = asMode(snap.mode);
    if (m) mode = m;
    servers = toServers(snap.servers);
    hasSubs = Array.isArray(snap.subs) && snap.subs.length > 0;
    const l = snap.settings?.general?.language;
    if (typeof l === "string") languageSetting = l;
    if (status === "connected") {
      try {
        const res = (await deps.rpc("run.getRoutingMode")) as { mode?: unknown } | null;
        const rm = asRoutingMode(res?.mode);
        if (rm) routingMode = rm;
      } catch (err) {
        warn("tray: routing mode read failed", err);
      }
    }
  }

  async function refresh(): Promise<void> {
    if (refreshInFlight) {
      refreshAgain = true;
      return refreshInFlight;
    }
    refreshInFlight = (async () => {
      try {
        do {
          refreshAgain = false;
          await doRefresh();
        } while (refreshAgain);
      } catch (err) {
        warn("tray: snapshot refresh failed", err);
      } finally {
        refreshInFlight = null;
        render();
      }
    })();
    return refreshInFlight;
  }

  function onVpnStatus(payload: unknown): void {
    const p = payload as { status?: unknown; serverId?: unknown; mode?: unknown } | null;
    const s = asStatus(p?.status);
    if (!s) return;
    setStatus(s);
    if (s === "connected") {
      if (typeof p?.serverId === "string") {
        activeServerId = p.serverId;
        deps.saveLastServer(p.serverId);
      }
      const m = asMode(p?.mode);
      if (m) mode = m;
    }
    if (s !== "connected") speed = null;
    render();
  }

  function onVpnSpeed(payload: unknown): void {
    const p = payload as { downBps?: unknown; upBps?: unknown } | null;
    if (status !== "connected") return;
    if (typeof p?.downBps !== "number" || typeof p?.upBps !== "number") return;
    speed = { downBps: p.downBps, upBps: p.upBps };
    deps.setTooltip(tooltip(view(), TRAY_STRINGS[lang()], speed));
  }

  function onRoutingMode(payload: unknown): void {
    const m = asRoutingMode((payload as { mode?: unknown } | null)?.mode);
    if (!m) return;
    routingMode = m;
    render();
  }

  function onRendererReport(report: RendererReport): void {
    const m = asMode(report.mode);
    if (m) mode = m;
    if (typeof report.syncing === "boolean") rendererSyncing = report.syncing;
    if (typeof report.language === "string") languageSetting = report.language;
    if (typeof report.lastServerId === "string" && report.lastServerId && !deps.loadLastServer()) {
      deps.saveLastServer(report.lastServerId);
    }
    render();
  }

  function onRendererGone(): void {
    rendererSyncing = false;
    render();
  }

  function pickConnectTarget(): string | null {
    const last = deps.loadLastServer();
    if (last && servers.some((s) => s.id === last)) return last;
    return servers.find((s) => s.favorite)?.id ?? servers[0]?.id ?? null;
  }

  async function connectTo(serverId: string, nextMode: TrayMode): Promise<void> {
    if (status === "connected" || status === "connecting") {
      await deps.rpc("run.disconnect");
      await waitForIdle();
    }
    await deps.rpc("run.connect", { serverId, mode: nextMode });
  }

  async function direct(cmd: TrayCommand): Promise<void> {
    switch (cmd.kind) {
      case "connect": {
        const target = pickConnectTarget();
        if (target) await deps.rpc("run.connect", { serverId: target, mode });
        return;
      }
      case "disconnect":
        await deps.rpc("run.disconnect");
        return;
      case "selectServer":
        if (status === "disconnecting") return;
        if (status !== "connected" && status !== "connecting") deps.saveLastServer(cmd.id);
        if (status === "connected" && activeServerId === cmd.id) return;
        await connectTo(cmd.id, mode);
        return;
      case "setMode": {
        if (cmd.mode === mode) return;
        mode = cmd.mode;
        render();
        await deps.rpc("settings.update", { section: "network", patch: { defaultMode: cmd.mode } });
        if (status === "connected" && activeServerId) await connectTo(activeServerId, cmd.mode);
        return;
      }
      case "syncAll":
        ownSyncs++;
        render();
        try {
          await deps.rpc("subs.syncAll");
        } finally {
          ownSyncs--;
          render();
        }
        return;
      case "cancelSync":
        await deps.rpc("subs.cancelAll");
        return;
      case "setRoutingMode":
        if (status !== "connected") return;
        await deps.rpc("run.setRoutingMode", { mode: cmd.mode });
        return;
    }
  }

  async function dispatch(cmd: TrayCommand): Promise<void> {
    try {
      switch (cmd.kind) {
        case "open":
          await deps.showWindow();
          return;
        case "quit":
          deps.quit();
          return;
        case "navigate":
          if (deps.hasRenderer()) {
            await deps.showWindow();
            deps.sendToRenderer(cmd);
          } else {
            deps.queueNavigate(cmd.path);
            await deps.showWindow();
          }
          return;
      }
      if (deps.hasRenderer()) {
        deps.sendToRenderer(cmd);
        return;
      }
      await direct(cmd);
    } catch (err) {
      warn(`tray: ${cmd.kind} failed`, err);
    }
  }

  render();
  return { refresh, onVpnStatus, onVpnSpeed, onRoutingMode, onRendererReport, onRendererGone, dispatch, view };
}
