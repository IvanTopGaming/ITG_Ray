// cmd/itgray-electron/src/main/bridge.ts
import { spawn, ChildProcess } from "node:child_process";
import { EventEmitter } from "node:events";
import { bundledBinary } from "./paths";
import { RpcClient } from "./rpc";
import type { EventTopic } from "../shared/protocol";

export const BRIDGE_TOPICS: Exclude<EventTopic, "bridge.state">[] = [
  "chain.error",
  "geo.progress",
  "helper.state",
  "log.line",
  "probe.result",
  "rules.changed",
  "servers.changed",
  "sub.synced",
  "vpn.speed",
  "vpn.status",
];

export type BridgeState = "starting" | "running" | "restarting" | "failed";

export class BridgeSupervisor extends EventEmitter {
  private child?: ChildProcess;
  private client?: RpcClient;
  private restartCount = 0;
  private restartWindowStart = 0;
  private state: BridgeState = "starting";
  private restartTimer?: NodeJS.Timeout;
  private eventUnsubscribers: (() => void)[] = [];

  start(): void {
    this.spawnOnce();
  }

  rpc(): RpcClient {
    if (!this.client) throw new Error("bridge: not started");
    return this.client;
  }

  getState(): BridgeState {
    return this.state;
  }

  /** Graceful shutdown — closes stdin so the bridge sees EOF and exits. */
  async stop(timeoutMs = 5000): Promise<void> {
    this.detachClientEvents();
    if (this.restartTimer) {
      clearTimeout(this.restartTimer);
      this.restartTimer = undefined;
    }
    if (!this.child) return;
    const child = this.child;
    this.child = undefined;
    child.stdin?.end();
    await new Promise<void>((resolve) => {
      const timer = setTimeout(() => {
        child.kill("SIGKILL");
        resolve();
      }, timeoutMs);
      child.once("exit", () => {
        clearTimeout(timer);
        resolve();
      });
    });
  }

  private spawnOnce(): void {
    const binPath = bundledBinary(process.platform === "win32" ? "itgray-bridge.exe" : "itgray-bridge");
    const child = spawn(binPath, [], { stdio: ["pipe", "pipe", "pipe"] });
    child.stderr?.on("data", (chunk: Buffer) => {
      // Forward bridge stderr to console for now; later route to a rolling log file.
      process.stderr.write("[bridge] " + chunk.toString());
    });
    child.on("error", (err) => {
      process.stderr.write(`[bridge] spawn error: ${err.message}\n`);
      if (this.child === child) {
        this.handleExit(null, null);
      }
    });
    child.on("exit", (code, signal) => {
      if (this.child === child) this.handleExit(code, signal);
    });

    this.child = child;
    this.client = new RpcClient(child.stdin!, child.stdout!);
    this.eventUnsubscribers = BRIDGE_TOPICS.map((topic) =>
      this.client!.on(topic, (payload) => this.emit(topic, payload)),
    );
    this.setState("running");
  }

  private handleExit(code: number | null, signal: NodeJS.Signals | null): void {
    if (!this.child) return; // intentional shutdown via stop()
    this.detachClientEvents();
    const now = Date.now();
    if (now - this.restartWindowStart > 60_000) {
      this.restartWindowStart = now;
      this.restartCount = 0;
    }
    this.restartCount++;
    if (this.restartCount > 5) {
      this.setState("failed", `bridge crashed ${this.restartCount}x in 60s (last: code=${code} signal=${signal})`);
      // Clear references so a follow-up stop() returns immediately
      // instead of waiting the 5s exit-event timeout against a dead
      // process. Mirrors the restart branch's cleanup below.
      this.child = undefined;
      this.client = undefined;
      return;
    }
    const backoff = [1000, 5000, 30_000][Math.min(this.restartCount - 1, 2)];
    this.setState("restarting", `code=${code} signal=${signal}, retry in ${backoff}ms`);
    this.child = undefined;
    this.client = undefined;
    this.restartTimer = setTimeout(() => {
      this.restartTimer = undefined;
      this.spawnOnce();
    }, backoff);
  }

  private detachClientEvents(): void {
    for (const unsubscribe of this.eventUnsubscribers) unsubscribe();
    this.eventUnsubscribers = [];
  }

  private setState(state: BridgeState, reason?: string): void {
    this.state = state;
    this.emit("state", { state, reason });
  }
}
