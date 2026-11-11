import { test } from "node:test";
import assert from "node:assert/strict";
import { createTrayController, type TrayControllerDeps } from "./tray-controller";
import type { TrayCommand, TrayItem, TrayStatus } from "./tray-menu";

const snapshot = {
  status: "idle",
  currentServer: null,
  mode: "tun",
  servers: [
    { id: "a", name: "A", country: "NL", origin: "P", favorite: false },
    { id: "b", name: "B", country: "FI", origin: "P", favorite: true },
  ],
  subs: [{ id: "s1" }],
  settings: { general: { language: "en" } },
};

function fixture(over: Partial<TrayControllerDeps> = {}) {
  const calls: Array<{ method: string; params?: unknown }> = [];
  const sent: TrayCommand[] = [];
  const menus: TrayItem[][] = [];
  const icons: TrayStatus[] = [];
  const tooltips: string[] = [];
  const navigations: string[] = [];
  let last: string | null = null;
  let renderer = false;
  let shown = 0;
  const rpcResults: Record<string, () => Promise<unknown>> = {};
  const deps: TrayControllerDeps = {
    rpc: async (method, params) => {
      calls.push({ method, params });
      if (rpcResults[method]) return rpcResults[method]();
      if (method === "app.getSnapshot") return snapshot;
      return {};
    },
    hasRenderer: () => renderer,
    sendToRenderer: (cmd) => sent.push(cmd),
    showWindow: async () => { shown++; },
    queueNavigate: (p) => navigations.push(p),
    quit: () => {},
    setMenu: (items) => menus.push(items),
    setTooltip: (s) => tooltips.push(s),
    setIcon: (s) => icons.push(s),
    locale: () => "en-US",
    loadLastServer: () => last,
    saveLastServer: (id) => { last = id; },
    idleTimeoutMs: 200,
    warn: () => {},
    ...over,
  };
  const ctl = createTrayController(deps);
  return {
    ctl,
    calls,
    sent,
    menus,
    icons,
    tooltips,
    navigations,
    rpcResults,
    setRenderer: (v: boolean) => { renderer = v; },
    setLast: (v: string | null) => { last = v; },
    shown: () => shown,
    methods: () => calls.map((c) => c.method).filter((m) => m !== "app.getSnapshot"),
  };
}

test("refresh loads servers and renders a menu", async () => {
  const f = fixture();
  await f.ctl.refresh();
  assert.equal(f.ctl.view().servers.length, 2);
  assert.equal(f.ctl.view().hasSubs, true);
  assert.ok(f.menus.length >= 1);
});

test("vpn.status connected stores the last server and drives the icon", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connecting" });
  f.ctl.onVpnStatus({ status: "connected", serverId: "a", mode: "sysproxy" });
  assert.deepEqual(f.icons, ["connecting", "connected"]);
  assert.equal(f.ctl.view().serverId, "a");
  assert.equal(f.ctl.view().mode, "sysproxy");
  f.ctl.onVpnStatus({ status: "idle" });
  assert.equal(f.ctl.view().serverId, "a");
});

test("with a live renderer, commands are forwarded instead of calling the bridge", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.setRenderer(true);
  await f.ctl.dispatch({ kind: "connect" });
  await f.ctl.dispatch({ kind: "setMode", mode: "sysproxy" });
  await f.ctl.dispatch({ kind: "syncAll" });
  assert.deepEqual(f.sent.map((c) => c.kind), ["connect", "setMode", "syncAll"]);
  assert.deepEqual(f.methods(), []);
});

test("without a renderer, connect targets the last server, else a favorite", async () => {
  const f = fixture();
  await f.ctl.refresh();
  await f.ctl.dispatch({ kind: "connect" });
  assert.deepEqual(f.calls.at(-1), { method: "run.connect", params: { serverId: "b", mode: "tun" } });
  f.setLast("a");
  await f.ctl.dispatch({ kind: "connect" });
  assert.deepEqual(f.calls.at(-1), { method: "run.connect", params: { serverId: "a", mode: "tun" } });
  f.setLast("gone");
  await f.ctl.dispatch({ kind: "connect" });
  assert.deepEqual(f.calls.at(-1), { method: "run.connect", params: { serverId: "b", mode: "tun" } });
});

test("switching server while connected disconnects, waits for idle, then connects", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a", mode: "tun" });
  const p = f.ctl.dispatch({ kind: "selectServer", id: "b" });
  await new Promise((r) => setImmediate(r));
  assert.deepEqual(f.methods(), ["run.disconnect"]);
  f.ctl.onVpnStatus({ status: "idle" });
  await p;
  assert.deepEqual(f.methods(), ["run.disconnect", "run.connect"]);
  assert.deepEqual(f.calls.at(-1)!.params, { serverId: "b", mode: "tun" });
});

test("selecting the already-connected server is a no-op", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a" });
  await f.ctl.dispatch({ kind: "selectServer", id: "a" });
  assert.deepEqual(f.methods(), []);
});

test("mode change while idle only persists the setting", async () => {
  const f = fixture();
  await f.ctl.refresh();
  await f.ctl.dispatch({ kind: "setMode", mode: "sysproxy" });
  assert.deepEqual(f.calls.at(-1), { method: "settings.update", params: { section: "network", patch: { defaultMode: "sysproxy" } } });
  assert.equal(f.ctl.view().mode, "sysproxy");
});

test("mode change while connected reconnects in the new mode", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a", mode: "tun" });
  const p = f.ctl.dispatch({ kind: "setMode", mode: "sysproxy" });
  await new Promise((r) => setImmediate(r));
  f.ctl.onVpnStatus({ status: "idle" });
  await p;
  assert.deepEqual(f.methods(), ["settings.update", "run.disconnect", "run.connect"]);
  assert.deepEqual(f.calls.at(-1)!.params, { serverId: "a", mode: "sysproxy" });
});

test("a stuck disconnect gives up instead of hanging", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a" });
  await f.ctl.dispatch({ kind: "selectServer", id: "b" });
  assert.deepEqual(f.methods(), ["run.disconnect"]);
});

test("direct sync shows the cancel item until the bridge call settles", async () => {
  const f = fixture();
  await f.ctl.refresh();
  let finish!: () => void;
  f.rpcResults["subs.syncAll"] = () => new Promise((r) => { finish = () => r({}); });
  const p = f.ctl.dispatch({ kind: "syncAll" });
  assert.equal(f.ctl.view().syncing, true);
  await f.ctl.dispatch({ kind: "cancelSync" });
  assert.ok(f.methods().includes("subs.cancelAll"));
  finish();
  await p;
  assert.equal(f.ctl.view().syncing, false);
});

test("renderer reports drive syncing, mode and language; losing the renderer clears syncing", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onRendererReport({ syncing: true, mode: "sysproxy", language: "ru" });
  assert.equal(f.ctl.view().syncing, true);
  assert.equal(f.ctl.view().mode, "sysproxy");
  const lastMenu = f.menus.at(-1)!;
  assert.ok(lastMenu.some((i) => i.label === "Отменить обновление подписок"));
  f.ctl.onRendererGone();
  assert.equal(f.ctl.view().syncing, false);
});

test("navigate without a renderer queues the path and opens the window", async () => {
  const f = fixture();
  await f.ctl.dispatch({ kind: "navigate", path: "/logs" });
  assert.deepEqual(f.navigations, ["/logs"]);
  assert.equal(f.shown(), 1);
  f.setRenderer(true);
  await f.ctl.dispatch({ kind: "navigate", path: "/logs" });
  assert.deepEqual(f.sent, [{ kind: "navigate", path: "/logs" }]);
});

test("speed updates only touch the tooltip, not the menu", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a" });
  const menus = f.menus.length;
  f.ctl.onVpnSpeed({ downBps: 125_000, upBps: 0 });
  f.ctl.onVpnSpeed({ downBps: 250_000, upBps: 0 });
  assert.equal(f.menus.length, menus);
  assert.match(f.tooltips.at(-1)!, /↓ 2\.0 Mbps/);
});

test("bridge failures are swallowed so a tray click never throws", async () => {
  const warned: string[] = [];
  const f = fixture({
    rpc: async () => { throw new Error("bridge down"); },
    warn: (m) => warned.push(m),
  });
  await f.ctl.refresh();
  await f.ctl.dispatch({ kind: "disconnect" });
  assert.deepEqual(warned, ["tray: snapshot refresh failed", "tray: disconnect failed"]);
});

test("the renderer's remembered server seeds main's only when main has none", async () => {
  const f = fixture();
  f.ctl.onRendererReport({ lastServerId: "a" });
  assert.equal(f.ctl.view().serverId, "a");
  f.ctl.onRendererReport({ lastServerId: "b" });
  assert.equal(f.ctl.view().serverId, "a");
});

test("refresh adopts a tunnel the helper kept alive across an app restart", async () => {
  const f = fixture({
    rpc: async () => ({ ...snapshot, status: "connected", currentServer: { id: "b" }, mode: "sysproxy" }),
  });
  await f.ctl.refresh();
  assert.deepEqual(f.icons, ["connected"]);
  assert.equal(f.ctl.view().serverId, "b");
  assert.equal(f.ctl.view().mode, "sysproxy");
});

test("a menu exists before the first snapshot arrives", () => {
  const f = fixture();
  assert.equal(f.menus.length, 1);
  assert.ok(f.menus[0].some((i) => i.label === "Quit ITG Ray"));
});

test("routing.mode events drive the view and idle resets to rule", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a" });
  f.ctl.onRoutingMode({ mode: "global" });
  assert.equal(f.ctl.view().routingMode, "global");
  f.ctl.onRoutingMode({ mode: "bogus" });
  assert.equal(f.ctl.view().routingMode, "global");
  f.ctl.onVpnStatus({ status: "idle" });
  assert.equal(f.ctl.view().routingMode, "rule");
});

test("setRoutingMode goes to the bridge without a renderer and to the renderer when present", async () => {
  const f = fixture();
  await f.ctl.refresh();
  f.ctl.onVpnStatus({ status: "connected", serverId: "a" });
  await f.ctl.dispatch({ kind: "setRoutingMode", mode: "direct" });
  assert.deepEqual(f.calls.at(-1), { method: "run.setRoutingMode", params: { mode: "direct" } });
  f.setRenderer(true);
  await f.ctl.dispatch({ kind: "setRoutingMode", mode: "global" });
  assert.deepEqual(f.sent.at(-1), { kind: "setRoutingMode", mode: "global" });
});

test("refresh reads the live routing mode of an adopted chain", async () => {
  const f = fixture({
    rpc: async (method) => {
      if (method === "app.getSnapshot") return { ...snapshot, status: "connected", currentServer: { id: "a" } };
      if (method === "run.getRoutingMode") return { mode: "global" };
      return {};
    },
  });
  await f.ctl.refresh();
  assert.equal(f.ctl.view().routingMode, "global");
});

test("refresh does not ask for the routing mode while idle", async () => {
  const methods: string[] = [];
  const f = fixture({
    rpc: async (method) => {
      methods.push(method);
      return method === "app.getSnapshot" ? snapshot : {};
    },
  });
  await f.ctl.refresh();
  assert.ok(!methods.includes("run.getRoutingMode"));
  assert.equal(f.ctl.view().routingMode, "rule");
});
