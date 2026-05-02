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
