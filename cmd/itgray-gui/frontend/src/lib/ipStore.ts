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

function notify() { for (const l of listeners) l(); }
function setState(next: IpState) { state = next; notify(); }
function subscribe(cb: () => void) { listeners.add(cb); return () => { listeners.delete(cb); }; }

export function useIp(): IpState {
  return useSyncExternalStore(subscribe, () => state, () => state);
}

export function getIpState(): IpState { return state; }

export async function ipRefresh(): Promise<void> {
  setState({ ...state, loading: true, error: null });
  try {
    const value = await GetPublicIP();
    setState({ value, loading: false, error: null });
  } catch (err: any) {
    setState({ ...state, loading: false, error: err?.message ?? String(err) });
  }
}

export function ipReset(): void {
  setState(initial());
}

export function __resetIpForTest(): void {
  state = initial();
  listeners.clear();
}
