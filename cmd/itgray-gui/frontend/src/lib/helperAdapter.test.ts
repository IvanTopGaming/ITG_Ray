import { describe, it, expect, vi, beforeEach } from 'vitest';
import { mapHelperStatus, formatError, detectIsWindows, __resetIsWindowsCacheForTests } from './helperAdapter';

describe('mapHelperStatus', () => {
  it.each([
    ['running', 'running'],
    ['stopped', 'stopped'],
    ['missing', 'missing'],
  ] as const)('%s → %s', (raw, expected) => {
    expect(mapHelperStatus(raw)).toBe(expected);
  });
  it('unknown string maps to error', () => {
    expect(mapHelperStatus('garbage')).toBe('error');
    expect(mapHelperStatus('')).toBe('error');
  });
});

describe('formatError', () => {
  it('strips elevated cli prefix', () => {
    const e = new Error("elevated cli [helper start] failed: exit status 1 (output: foo)");
    expect(formatError(e)).toBe("exit status 1 (output: foo)");
  });
  it('passes plain errors through', () => {
    expect(formatError(new Error('plain'))).toBe('plain');
  });
  it('truncates long messages with ellipsis', () => {
    const long = 'x'.repeat(500);
    const out = formatError(new Error(long));
    expect(out.length).toBeLessThanOrEqual(200);
    expect(out.endsWith('…')).toBe(true);
  });
  it('handles non-Error throwables', () => {
    expect(formatError('plain string')).toBe('plain string');
    expect(formatError({ toString: () => 'objectish' })).toBe('objectish');
  });
});

describe('detectIsWindows', () => {
  beforeEach(() => __resetIsWindowsCacheForTests());

  it('returns true on windows', async () => {
    const env = vi.fn().mockResolvedValue({ platform: 'windows' });
    await expect(detectIsWindows(env)).resolves.toBe(true);
  });
  it('returns false on linux', async () => {
    const env = vi.fn().mockResolvedValue({ platform: 'linux' });
    await expect(detectIsWindows(env)).resolves.toBe(false);
  });
  it('caches the first call', async () => {
    const env = vi.fn().mockResolvedValue({ platform: 'darwin' });
    await detectIsWindows(env);
    await detectIsWindows(env);
    expect(env).toHaveBeenCalledTimes(1);
  });
});
