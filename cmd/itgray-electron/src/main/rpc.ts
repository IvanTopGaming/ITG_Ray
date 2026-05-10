// cmd/itgray-electron/src/main/rpc.ts
import { Writable, Readable } from "node:stream";
import readline from "node:readline";
import type { RpcMethod, RpcParams, RpcResult } from "../shared/protocol";

interface PendingCall {
  resolve: (value: unknown) => void;
  reject: (err: Error) => void;
}

/**
 * RpcClient sends JSON-RPC 2.0 requests on `stdin`, parses responses and
 * notifications from `stdout` (newline-delimited), and dispatches
 * notifications to subscribed listeners.
 */
export class RpcClient {
  private nextId = 1;
  private pending = new Map<number, PendingCall>();
  private listeners = new Map<string, Set<(payload: unknown) => void>>();

  constructor(private stdin: Writable, stdout: Readable) {
    const rl = readline.createInterface({ input: stdout, crlfDelay: Infinity });
    rl.on("line", (line) => this.handleLine(line));
    rl.on("close", () => this.failAll(new Error("bridge stdout closed")));
  }

  call<M extends RpcMethod>(method: M, params?: RpcParams<M>): Promise<RpcResult<M>> {
    const id = this.nextId++;
    return new Promise<RpcResult<M>>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (v: unknown) => void, reject });
      const req = { jsonrpc: "2.0", id, method, params: params ?? null };
      this.stdin.write(JSON.stringify(req) + "\n");
    });
  }

  /** Subscribe to a bridge → main notification topic. Returns an unsubscribe. */
  on(topic: string, cb: (payload: unknown) => void): () => void {
    let set = this.listeners.get(topic);
    if (!set) {
      set = new Set();
      this.listeners.set(topic, set);
    }
    set.add(cb);
    return () => set!.delete(cb);
  }

  private handleLine(line: string): void {
    if (!line) return;
    let msg: { id?: number; result?: unknown; error?: { code: number; message: string }; method?: string; params?: unknown };
    try {
      msg = JSON.parse(line);
    } catch {
      console.error("rpc: malformed line:", line);
      return;
    }
    if (typeof msg.id === "number") {
      const p = this.pending.get(msg.id);
      if (!p) return;
      this.pending.delete(msg.id);
      if (msg.error) {
        const err = new Error(msg.error.message);
        (err as Error & { code?: number }).code = msg.error.code;
        p.reject(err);
      } else {
        p.resolve(msg.result);
      }
      return;
    }
    if (typeof msg.method === "string" && msg.method.startsWith("event:")) {
      const topic = msg.method.slice("event:".length);
      const set = this.listeners.get(topic);
      if (set) for (const cb of set) cb(msg.params);
    }
  }

  private failAll(err: Error): void {
    for (const p of this.pending.values()) p.reject(err);
    this.pending.clear();
  }
}
