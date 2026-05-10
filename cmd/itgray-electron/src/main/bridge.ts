// cmd/itgray-electron/src/main/bridge.ts
import { spawn, ChildProcess } from "node:child_process";
import { EventEmitter } from "node:events";
import { bundledBinary } from "./paths";
import { RpcClient } from "./rpc";

export type BridgeState = "starting" | "running" | "restarting" | "failed";

export class BridgeSupervisor extends EventEmitter {
  private child?: ChildProcess;
  private client?: RpcClient;
  private restartCount = 0;
  private restartWindowStart = 0;
  private state: BridgeState = "starting";
  private restartTimer?: NodeJS.Timeout;

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
      this.handleExit(code, signal);
    });

    this.child = child;
    this.client = new RpcClient(child.stdin!, child.stdout!);
    this.setState("running");
  }

  private handleExit(code: number | null, signal: NodeJS.Signals | null): void {
    if (!this.child) return; // intentional shutdown via stop()
    const now = Date.now();
    if (now - this.restartWindowStart > 60_000) {
      this.restartWindowStart = now;
      this.restartCount = 0;
    }
    this.restartCount++;
    if (this.restartCount > 5) {
      this.setState("failed", `bridge crashed ${this.restartCount}x in 60s (last: code=${code} signal=${signal})`);
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

  private setState(state: BridgeState, reason?: string): void {
    this.state = state;
    this.emit("state", { state, reason });
  }
}
