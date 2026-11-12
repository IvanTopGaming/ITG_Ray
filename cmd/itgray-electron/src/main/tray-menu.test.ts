import { test } from "node:test";
import assert from "node:assert/strict";
import { buildTrayMenu, flagEmoji, statusLine, tooltip, type TrayItem, type TrayView } from "./tray-menu";
import { TRAY_STRINGS, resolveTrayLang } from "./tray-i18n";

const t = TRAY_STRINGS.en;

function view(over: Partial<TrayView> = {}): TrayView {
  return {
    status: "idle",
    serverId: null,
    mode: "tun",
    servers: [
      { id: "a", name: "Netherlands 2", country: "NL", origin: "Provider", favorite: false },
      { id: "b", name: "Finland", country: "FI", origin: "Provider", favorite: true },
      { id: "c", name: "Home box", country: "", origin: "manual", favorite: false },
    ],
    hasSubs: true,
    syncing: false,
    routingMode: "rule",
    ...over,
  };
}

function find(items: TrayItem[], label: string): TrayItem {
  const it = items.find((i) => i.label === label);
  assert.ok(it, `menu item ${label} missing`);
  return it;
}

test("flagEmoji turns ISO codes into flags and ignores junk", () => {
  assert.equal(flagEmoji("nl"), "🇳🇱");
  assert.equal(flagEmoji(""), "");
  assert.equal(flagEmoji("NLD"), "");
});

test("status line names the server only while connected or connecting", () => {
  assert.equal(statusLine(view({ status: "connected", serverId: "a" }), t), "● Connected · 🇳🇱 Netherlands 2");
  assert.equal(statusLine(view({ status: "idle", serverId: "a" }), t), "Disconnected");
});

test("primary action follows the connection state", () => {
  assert.deepEqual(find(buildTrayMenu(view(), t), "Connect").command, { kind: "connect" });
  assert.deepEqual(find(buildTrayMenu(view({ status: "connected" }), t), "Disconnect").command, { kind: "disconnect" });
  assert.deepEqual(find(buildTrayMenu(view({ status: "connecting" }), t), "Cancel connecting").command, { kind: "disconnect" });
  assert.equal(find(buildTrayMenu(view({ status: "disconnecting" }), t), "Disconnect").enabled, false);
  assert.equal(find(buildTrayMenu(view({ servers: [] }), t), "Connect").enabled, false);
});

test("server submenu lists favorites first, then groups by origin, and checks the current one", () => {
  const sub = find(buildTrayMenu(view({ status: "connected", serverId: "a" }), t), "Server").submenu!;
  const labels = sub.map((i) => i.type === "separator" ? "---" : i.label);
  assert.deepEqual(labels, ["★ Favorites", "🇫🇮 Finland", "---", "Provider", "🇳🇱 Netherlands 2", "🇫🇮 Finland", "---", "Added manually", "Home box"]);
  const checked = sub.filter((i) => i.checked).map((i) => i.command);
  assert.deepEqual(checked, [{ kind: "selectServer", id: "a" }]);
});

test("a single group without favorites is listed flat", () => {
  const servers = [
    { id: "a", name: "A", country: "", origin: "P", favorite: false },
    { id: "b", name: "B", country: "", origin: "P", favorite: false },
  ];
  const sub = find(buildTrayMenu(view({ servers }), t), "Server").submenu!;
  assert.deepEqual(sub.map((i) => i.label), ["A", "B"]);
});

test("mode radios reflect the mode and lock during transitions", () => {
  const sub = find(buildTrayMenu(view({ mode: "sysproxy" }), t), "Mode").submenu!;
  assert.equal(sub.find((i) => i.label === "System proxy")!.checked, true);
  assert.equal(sub.find((i) => i.label === "TUN")!.checked, false);
  const locked = find(buildTrayMenu(view({ status: "connecting" }), t), "Mode").submenu!;
  assert.ok(locked.every((i) => i.enabled === false));
});

test("subscription item toggles between update and cancel", () => {
  assert.deepEqual(find(buildTrayMenu(view(), t), "Update subscriptions").command, { kind: "syncAll" });
  assert.deepEqual(find(buildTrayMenu(view({ syncing: true }), t), "Cancel subscription update").command, { kind: "cancelSync" });
  assert.equal(find(buildTrayMenu(view({ hasSubs: false }), t), "Update subscriptions").enabled, false);
});

test("tooltip shows speed only while connected", () => {
  const speed = { downBps: 1_050_000, upBps: 12_500 };
  assert.equal(tooltip(view({ status: "connected", serverId: "a" }), t, speed), "ITG Ray — Connected · 🇳🇱 Netherlands 2\n↓ 8.4 Mbps  ↑ 0.1 Mbps");
  assert.equal(tooltip(view(), t, speed), "ITG Ray — Disconnected");
});

test("language resolves from the setting, falling back to the OS locale", () => {
  assert.equal(resolveTrayLang("ru", "en-US"), "ru");
  assert.equal(resolveTrayLang("auto", "ru-RU"), "ru");
  assert.equal(resolveTrayLang(undefined, "de-DE"), "en");
  assert.equal(find(buildTrayMenu(view(), TRAY_STRINGS.ru), "Подключить").command!.kind, "connect");
});

test("routing submenu reflects the mode and is locked unless connected", () => {
  const idle = find(buildTrayMenu(view(), t), "Routing").submenu!;
  assert.ok(idle.every((i) => i.enabled === false));
  const sub = find(buildTrayMenu(view({ status: "connected", routingMode: "direct" }), t), "Routing").submenu!;
  assert.deepEqual(sub.map((i) => i.label), ["By rules", "All via proxy", "All direct"]);
  assert.equal(sub.find((i) => i.label === "All direct")!.checked, true);
  assert.ok(sub.every((i) => i.enabled !== false));
  assert.deepEqual(sub[1].command, { kind: "setRoutingMode", mode: "global" });
  const items = buildTrayMenu(view({ status: "connected" }), t).map((i) => i.label);
  assert.equal(items.indexOf("Routing"), items.indexOf("Mode") + 1);
  assert.ok(find(buildTrayMenu(view(), TRAY_STRINGS.ru), "Маршрутизация"));
});
