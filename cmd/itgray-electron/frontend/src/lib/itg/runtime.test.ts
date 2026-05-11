import { beforeEach, describe, expect, it } from "vitest";
import { EventsOn } from "./runtime";

// Integration smoke for the renderer ↔ main IPC seam.
//
// Background: the renderer's stores subscribe through EventsOn using
// Wails-style colon-separated topics ("vpn:status"). The bridge emits
// JSON-RPC notifications using dot-separated wire topics ("vpn.status"),
// and the Electron main process forwards them on Electron IPC channel
// `event:<wireTopic>`. The preload exposes window.itg.on which
// subscribes to `event:<topic>` directly. The colon→dot translation
// happens inside runtime.ts EventsOn — if that step regresses, the
// renderer subscribes to event:vpn:status while main sends to
// event:vpn.status and every hub event is silently dropped (this
// exact regression was introduced in 6c43d79 and shipped on main
// before the live-app smoke at 2026-05-11 surfaced it).
//
// Unit tests of dashStore mock EventsOn directly, so they cannot
// catch the channel-name mismatch. This test stands in for the seam
// by emulating the preload's `window.itg.on` implementation
// (subscribe to `event:<topic>`) and the main forwarder
// (`webContents.send(\`event:<wireTopic>\`, payload)`). A working
// EventsOn must produce subscriber channels that match what main
// publishes on.

const listenersByChannel = new Map<string, Set<(p: unknown) => void>>();

function ipcRendererOn(channel: string, handler: (p: unknown) => void) {
  let set = listenersByChannel.get(channel);
  if (!set) {
    set = new Set();
    listenersByChannel.set(channel, set);
  }
  set.add(handler);
}

function ipcRendererOff(channel: string, handler: (p: unknown) => void) {
  listenersByChannel.get(channel)?.delete(handler);
}

function webContentsSend(channel: string, payload: unknown) {
  const set = listenersByChannel.get(channel);
  if (!set) return;
  for (const h of [...set]) h(payload);
}

beforeEach(() => {
  listenersByChannel.clear();
  // Emulate the preload's window.itg.on (cmd/itgray-electron/src/preload/preload.ts).
  (window as unknown as { itg: { on: (t: string, cb: (p: unknown) => void) => () => void } }).itg = {
    on: (topic: string, cb: (p: unknown) => void) => {
      const channel = `event:${topic}`;
      ipcRendererOn(channel, cb);
      return () => ipcRendererOff(channel, cb);
    },
  };
});

// Wire topics declared by the bridge protocol (mirrors EventTopic in
// src/shared/protocol.ts and BRIDGE_TOPICS in src/main/ipc.ts).
// bridge.state is omitted because it is supervisor-driven, not emitted
// by the bridge subprocess, so the colon-form translation contract
// doesn't apply.
const WIRE_TOPICS = [
  "chain.error",
  "helper.state",
  "probe.result",
  "servers.changed",
  "sub.synced",
  "vpn.speed",
  "vpn.status",
] as const;

describe("EventsOn IPC seam", () => {
  for (const wireTopic of WIRE_TOPICS) {
    const rendererTopic = wireTopic.replace(/\./g, ":");
    it(`renderer EventsOn("${rendererTopic}") receives main webContents.send("event:${wireTopic}")`, () => {
      const received: unknown[] = [];
      EventsOn(rendererTopic, (p) => received.push(p));
      webContentsSend(`event:${wireTopic}`, { ping: 1 });
      expect(received).toEqual([{ ping: 1 }]);
    });
  }

  it("unsubscribe removes the listener so subsequent sends do not fire", () => {
    const received: unknown[] = [];
    const off = EventsOn("vpn:status", (p) => received.push(p));
    webContentsSend("event:vpn.status", "first");
    off();
    webContentsSend("event:vpn.status", "second");
    expect(received).toEqual(["first"]);
  });

  // Pre-fix regression pin: if EventsOn ever drops the colon→dot
  // translation again (the 2026-05-11 6c43d79 regression), subscribers
  // would land on `event:vpn:status` instead of `event:vpn.status`
  // and the main-side `event:vpn.status` send would have zero
  // listeners.
  it("does not subscribe to the literal colon-form channel", () => {
    EventsOn("vpn:status", () => {});
    expect(listenersByChannel.has("event:vpn:status")).toBe(false);
    expect(listenersByChannel.has("event:vpn.status")).toBe(true);
  });
});
