// cmd/itgray-electron/src/shared/protocol.ts
//
// Phase 0: handwritten. Phase 1 replaces this file with codegen output
// from internal/bridge/protocol/schema.go. Keep the shape stable so the
// Phase 1 cutover is a no-op for consumers.

export interface PingResult {
  pong: number;
  version: string;
}

// Method-name → (params type, result type) mapping. Each future service
// extends this. Phase 0 only carries app.ping.
export interface RpcMethods {
  "app.ping": { params: void; result: PingResult };
}

export type RpcMethod = keyof RpcMethods;
export type RpcParams<M extends RpcMethod> = RpcMethods[M]["params"];
export type RpcResult<M extends RpcMethod> = RpcMethods[M]["result"];

export type EventTopic = "bridge.state";

export interface BridgeStateEvent {
  state: "running" | "restarting" | "failed";
  reason?: string;
}
