import { useSyncExternalStore } from "react";
import { GetPublicIP } from "../../wailsjs/go/bindings/AppService";

export type IpState = {
  value: string | null;
  loading: boolean;
  error: string | null;
};

const initial = (): IpState => ({ value: null, loading: false, error: null });

let state: IpState = initial();
const listeners = new Set<() => void>();
// gen is bumped on every ipRefresh entry and on ipReset. In-flight retry
// loops capture the gen at start and abort if it changes — handles the
// chain teardown / re-entrant-refresh races without AbortController plumbing.
let gen = 0;
// retryDelaysMs is a var-not-const so tests don't have to wait the full
// production window. Production: ~5s budget tolerating 4 transient
// connection-refused failures while xray binds 127.0.0.1:1081 after chain
// start.
export const retryDelaysMs: number[] = [0, 250, 750, 1500, 2500];

function notify() { for (const l of listeners) l(); }
function setState(next: IpState) { state = next; notify(); }
function subscribe(cb: () => void) { listeners.add(cb); return () => { listeners.delete(cb); }; }
const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

export function useIp(): IpState {
  return useSyncExternalStore(subscribe, () => state, () => state);
}

export function getIpState(): IpState { return state; }

export async function ipRefresh(): Promise<void> {
  const myGen = ++gen;
  setState({ ...state, loading: true, error: null });
  let lastErr: string | null = null;
  for (const delay of retryDelaysMs) {
    if (myGen !== gen) return;
    if (delay > 0) {
      await sleep(delay);
      if (myGen !== gen) return;
    }
    try {
      const value = await GetPublicIP();
      if (myGen !== gen) return;
      setState({ value, loading: false, error: null });
      return;
    } catch (err: any) {
      lastErr = err?.message ?? String(err);
    }
  }
  if (myGen !== gen) return;
  setState({ ...state, loading: false, error: lastErr });
}

export function ipReset(): void {
  gen++;
  setState(initial());
}

export function __resetIpForTest(): void {
  state = initial();
  listeners.clear();
  gen = 0;
}
