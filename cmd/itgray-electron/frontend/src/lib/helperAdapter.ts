import { useEffect, useRef, useState } from 'react';
import {
  Status as HelperStatus,
  StatusPackageManaged,
  Install as HelperInstall,
  Start as HelperStart,
  Stop as HelperStop,
  Restart as HelperRestart,
  Reinstall as HelperReinstall,
  InstallLinux as HelperInstallLinux,
  UninstallLinux as HelperUninstallLinux,
  StartLinux as HelperStartLinux,
  StopLinux as HelperStopLinux,
  RestartLinux as HelperRestartLinux,
} from '@/lib/itg/HelperService';
import { Environment } from '@/lib/itg/runtime';

export type HelperState = 'missing' | 'stopped' | 'running' | 'error' | 'pending';

// mapHelperStatus translates the backend Status string ("running" |
// "stopped" | "missing") into the typed HelperState union. Unexpected
// strings (e.g. backend bug, schema drift) collapse to 'error' so the
// UI surfaces something actionable instead of crashing.
export function mapHelperStatus(raw: string): HelperState {
  if (raw === 'running' || raw === 'stopped' || raw === 'missing') return raw;
  return 'error';
}

const ELEVATED_CLI_PREFIX = /^elevated cli \[[^\]]+\] failed: /;

// formatError trims the verbose Wails-wrapped error coming back from
// elevateCLI down to something fit for an inline UI block. Strips the
// redundant "elevated cli [helper start] failed: " prefix and caps the
// length at 200 chars (truncated with U+2026).
export function formatError(err: unknown): string {
  let raw: string;
  if (err instanceof Error) {
    raw = err.message;
  } else if (typeof err === 'string') {
    raw = err;
  } else {
    raw = String(err);
  }
  const stripped = raw.replace(ELEVATED_CLI_PREFIX, '');
  return stripped.length > 200 ? stripped.slice(0, 199) + '…' : stripped;
}

let cachedIsWindows: boolean | null = null;

// detectIsWindows asynchronously resolves the runtime platform via the
// supplied Wails Environment() function and caches the answer so
// subsequent calls are synchronous-fast. The env argument is injected
// (rather than imported) so the function is trivially mockable.
export async function detectIsWindows(env: () => Promise<{ platform: string }>): Promise<boolean> {
  if (cachedIsWindows !== null) return cachedIsWindows;
  const info = await env();
  cachedIsWindows = info.platform === 'windows';
  return cachedIsWindows;
}

// __resetIsWindowsCacheForTests clears the cached platform answer.
// Test-only — the leading underscore convention matches lib/settings.ts.
export function __resetIsWindowsCacheForTests(): void {
  cachedIsWindows = null;
}

export type UseHelperState = {
  state: HelperState;
  opError: string | null;
  isWindows: boolean | null;
  isLinux: boolean | null;
  // True when a distro package owns the helper — the install/reinstall and
  // uninstall actions target /usr/local and must stay hidden.
  packageManaged: boolean;

  install: () => Promise<void>;
  start: () => Promise<void>;
  stop: () => Promise<void>;
  restart: () => Promise<void>;
  reinstall: () => Promise<void>;
  installLinux: () => Promise<void>;
  uninstallLinux: () => Promise<void>;
  startLinux: () => Promise<void>;
  stopLinux: () => Promise<void>;
  restartLinux: () => Promise<void>;
  dismissError: () => void;
};

const POLL_MS = 2_000;

// useHelperState owns the Helper-section state machine: async platform
// detection, 2 s polling (visibility-gated), and the action wrappers
// that run the spec §6.4 pattern (set pending → IPC → finally refetch).
export function useHelperState(): UseHelperState {
  const [state, setState]     = useState<HelperState>('pending');
  const [opError, setOpError] = useState<string | null>(null);
  const [isWindows, setIsWin] = useState<boolean | null>(null);
  const [isLinux, setIsLin]   = useState<boolean | null>(null);
  const [packageManaged, setPkgManaged] = useState(false);

  // stateRef mirrors `state` so the polling tick can read the latest
  // value without re-creating the interval on each setState.
  const stateRef = useRef<HelperState>(state);
  useEffect(() => { stateRef.current = state; }, [state]);

  // Detect platform once on mount and, if Windows, immediately fetch
  // the first Status() so the UI doesn't sit on 'pending' for the full
  // poll interval. Sequencing both calls in a single async flow keeps
  // the microtask chain short — important for renderHook tests where
  // each `await Promise.resolve()` flushes only one microtask.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const info = await (Environment as unknown as () => Promise<{ platform: string }>)();
      if (cancelled) return;
      const isWin = info.platform === 'windows';
      const isLin = info.platform === 'linux';
      setIsWin(isWin);
      setIsLin(isLin);
      // Fire-and-forget so the awaited chain below keeps its existing
      // microtask sequencing; a failure just leaves the flag false.
      try {
        void StatusPackageManaged()
          .then((v) => { if (!cancelled) setPkgManaged(v); })
          .catch(() => {});
      } catch { /* binding missing (older bridge) — stay false */ }
      // Both Windows (SCM) and Linux (daemon socket probe) expose a live
      // helper status; other platforms have no helper service to poll.
      if (!isWin && !isLin) return;
      try {
        const raw = await HelperStatus();
        if (!cancelled) setState(mapHelperStatus(raw));
      } catch (e) {
        if (!cancelled) {
          setState('error');
          setOpError(formatError(e));
        }
      }
    })();
    return () => { cancelled = true; };
  }, []);

  // Polling loop — registers once the platform resolves to one with a
  // helper service (Windows or Linux).
  useEffect(() => {
    if (isWindows !== true && isLinux !== true) return;

    let cancelled = false;
    let inFlight = false;

    const tick = async () => {
      if (cancelled) return;
      if (document.visibilityState !== 'visible') return;
      if (stateRef.current === 'pending') return;
      if (inFlight) return;
      inFlight = true;
      try {
        const raw = await HelperStatus();
        if (!cancelled) setState(mapHelperStatus(raw));
      } catch (e) {
        if (!cancelled) {
          setState('error');
          setOpError(formatError(e));
        }
      } finally {
        inFlight = false;
      }
    };

    const id = window.setInterval(() => { void tick(); }, POLL_MS);
    return () => { cancelled = true; window.clearInterval(id); };
  }, [isWindows, isLinux]);

  // Action wrappers: set pending, run IPC, capture op error, refetch
  // Status regardless of result. The post-action Status overrides the
  // 'pending' state with whatever ground truth reports.
  const runOp = async (op: () => Promise<unknown>) => {
    setState('pending');
    setOpError(null);
    let capturedError: string | null = null;
    try {
      await op();
    } catch (e) {
      capturedError = formatError(e);
    }
    try {
      const raw = await HelperStatus();
      setState(mapHelperStatus(raw));
    } catch (e) {
      setState('error');
      if (capturedError === null) capturedError = formatError(e);
    }
    if (capturedError !== null) setOpError(capturedError);
  };

  return {
    state,
    opError,
    isWindows,
    isLinux,
    packageManaged,
    install:        () => runOp(() => HelperInstall("")),
    start:          () => runOp(() => HelperStart()),
    stop:           () => runOp(() => HelperStop()),
    restart:        () => runOp(() => HelperRestart()),
    reinstall:      () => runOp(() => HelperReinstall()),
    installLinux:   () => runOp(() => HelperInstallLinux()),
    uninstallLinux: () => runOp(() => HelperUninstallLinux()),
    startLinux:     () => runOp(() => HelperStartLinux()),
    stopLinux:      () => runOp(() => HelperStopLinux()),
    restartLinux:   () => runOp(() => HelperRestartLinux()),
    dismissError:   () => setOpError(null),
  };
}
