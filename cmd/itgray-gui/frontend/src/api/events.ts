import { EventsOn } from "../../wailsjs/runtime/runtime";
import { List as listServers } from "../../wailsjs/go/bindings/ServersService";
import { useStore } from "../store";
import type { ChainStatus, ServerView } from "./client";

// attachEvents subscribes the Zustand store to the six Wails events emitted
// by the in-process hub. Called once from AppShell on mount; never detached
// because the store outlives the React tree.
export function attachEvents(): void {
  EventsOn("vpn:status", (status: string) => {
    useStore.getState().applyVPNStatus(status as ChainStatus);
  });
  EventsOn("vpn:speed", (e: { upBps: number; downBps: number }) => {
    useStore.getState().applySpeed(e);
  });
  EventsOn("sub:synced", (e: { id: string; status: string; at: string; importedCount: number; message?: string }) => {
    useStore.getState().applySubSync(e);
    // SyncOne writes new servers to disk in a background goroutine and emits
    // sub:synced; refresh the server list so the Servers tab and Hero picker
    // reflect imports without requiring a GUI restart.
    if (e.status === "OK" && e.importedCount > 0) {
      listServers()
        .then((servers) => {
          useStore.setState({ servers: servers as unknown as ServerView[] });
        })
        .catch(() => {
          /* swallow — store keeps last known list */
        });
    }
  });
  EventsOn("probe:result", (e: { results: Array<{ id: string; latencyMs: number; error?: string }> }) => {
    useStore.getState().applyProbeResult(e);
  });
  EventsOn("helper:state", (s: string) => {
    useStore.getState().applyHelperState(s);
  });
  EventsOn("chain:error", (e: { kind: string; message: string }) => {
    useStore.getState().applyChainError(e);
  });
}
