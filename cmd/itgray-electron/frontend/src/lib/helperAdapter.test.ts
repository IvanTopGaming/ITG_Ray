import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';

// Wails-binding mocks must be set up BEFORE importing the SUT module.
const statusMock         = vi.fn();
const installMock        = vi.fn();
const startMock          = vi.fn();
const stopMock           = vi.fn();
const restartMock        = vi.fn();
const reinstallMock      = vi.fn();
const installLinuxMock   = vi.fn();
const uninstallLinuxMock = vi.fn();
const envMock            = vi.fn();

vi.mock('@/lib/itg/HelperService', () => ({
  Status:         (...args: unknown[]) => statusMock(...args),
  Install:        (...args: unknown[]) => installMock(...args),
  Start:          (...args: unknown[]) => startMock(...args),
  Stop:           (...args: unknown[]) => stopMock(...args),
  Restart:        (...args: unknown[]) => restartMock(...args),
  Reinstall:      (...args: unknown[]) => reinstallMock(...args),
  InstallLinux:   (...args: unknown[]) => installLinuxMock(...args),
  UninstallLinux: (...args: unknown[]) => uninstallLinuxMock(...args),
}));
vi.mock('@/lib/itg/runtime', () => ({
  Environment: (...args: unknown[]) => envMock(...args),
}));

import {
  mapHelperStatus,
  formatError,
  detectIsWindows,
  __resetIsWindowsCacheForTests,
  useHelperState,
} from './helperAdapter';

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

describe('useHelperState (hook)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    __resetIsWindowsCacheForTests();
    statusMock.mockReset();
    installMock.mockReset();
    startMock.mockReset();
    stopMock.mockReset();
    restartMock.mockReset();
    reinstallMock.mockReset();
    installLinuxMock.mockReset();
    uninstallLinuxMock.mockReset();
    envMock.mockReset();
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('unsupported platform (darwin): sets state, never calls Status()', async () => {
    envMock.mockResolvedValue({ platform: 'darwin' });
    const { result } = renderHook(() => useHelperState());

    expect(result.current.state).toBe('pending');
    expect(result.current.isWindows).toBe(null);

    await act(async () => { await Promise.resolve(); });
    expect(result.current.isWindows).toBe(false);
    expect(result.current.isLinux).toBe(false);
    expect(statusMock).not.toHaveBeenCalled();
  });

  it('Linux mount: detects isLinux, fetches Status once', async () => {
    envMock.mockResolvedValue({ platform: 'linux' });
    statusMock.mockResolvedValue('running');

    const { result } = renderHook(() => useHelperState());

    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(result.current.isLinux).toBe(true);
    expect(result.current.isWindows).toBe(false);
    expect(statusMock).toHaveBeenCalledTimes(1);
    expect(result.current.state).toBe('running');
  });

  it('Linux installLinux action: invokes the InstallLinux RPC and refetches Status', async () => {
    envMock.mockResolvedValue({ platform: 'linux' });
    statusMock.mockResolvedValueOnce('missing');
    installLinuxMock.mockResolvedValue(undefined);
    statusMock.mockResolvedValueOnce('running');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(result.current.state).toBe('missing');

    await act(async () => { await result.current.installLinux(); });
    expect(installLinuxMock).toHaveBeenCalledTimes(1);
    expect(result.current.state).toBe('running');
    expect(result.current.opError).toBe(null);
  });

  it('Linux uninstallLinux action: invokes the UninstallLinux RPC', async () => {
    envMock.mockResolvedValue({ platform: 'linux' });
    statusMock.mockResolvedValueOnce('running');
    uninstallLinuxMock.mockResolvedValue(undefined);
    statusMock.mockResolvedValueOnce('missing');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    await act(async () => { await result.current.uninstallLinux(); });
    expect(uninstallLinuxMock).toHaveBeenCalledTimes(1);
    expect(result.current.state).toBe('missing');
  });

  it('Windows mount: calls Status() once and sets state from result', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValue('running');

    const { result } = renderHook(() => useHelperState());

    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(result.current.isWindows).toBe(true);
    expect(statusMock).toHaveBeenCalledTimes(1);
    expect(result.current.state).toBe('running');
  });

  it('polls Status every 2 s and updates state when value changes', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValueOnce('running');
    statusMock.mockResolvedValueOnce('stopped');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(result.current.state).toBe('running');

    await act(async () => {
      vi.advanceTimersByTime(2_000);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current.state).toBe('stopped');
    expect(statusMock).toHaveBeenCalledTimes(2);
  });

  it('skips polling tick when document is hidden', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValue('running');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(statusMock).toHaveBeenCalledTimes(1);

    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' });
    await act(async () => { vi.advanceTimersByTime(6_000); });
    expect(statusMock).toHaveBeenCalledTimes(1); // still 1: ticks were skipped

    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
    await act(async () => { vi.advanceTimersByTime(2_000); await Promise.resolve(); });
    expect(statusMock).toHaveBeenCalledTimes(2);
    expect(result.current.state).toBe('running');
  });

  it('install action: pending mid-flight, refetches Status, clears pending', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValueOnce('missing');
    installMock.mockResolvedValue(undefined);
    statusMock.mockResolvedValueOnce('stopped');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(result.current.state).toBe('missing');

    let installPromise!: Promise<void>;
    act(() => { installPromise = result.current.install(); });
    expect(result.current.state).toBe('pending');

    await act(async () => { await installPromise; });
    expect(installMock).toHaveBeenCalledTimes(1);
    expect(result.current.state).toBe('stopped');
    expect(result.current.opError).toBe(null);
  });

  it('install action: on rejection sets opError and still refetches', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValueOnce('missing');
    installMock.mockRejectedValue(new Error('elevated cli [helper install] failed: exit status 1 (output: User declined)'));
    statusMock.mockResolvedValueOnce('missing');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    await act(async () => { await result.current.install(); });
    expect(result.current.state).toBe('missing');
    expect(result.current.opError).toContain('User declined');
  });

  it('dismissError clears the inline error', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValueOnce('running');
    restartMock.mockRejectedValue(new Error('boom'));
    statusMock.mockResolvedValueOnce('running');

    const { result } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    await act(async () => { await result.current.restart(); });
    expect(result.current.opError).toBe('boom');

    act(() => { result.current.dismissError(); });
    expect(result.current.opError).toBe(null);
  });

  it('cleans up the polling interval on unmount', async () => {
    envMock.mockResolvedValue({ platform: 'windows' });
    statusMock.mockResolvedValue('running');

    const { unmount } = renderHook(() => useHelperState());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(statusMock).toHaveBeenCalledTimes(1);

    unmount();
    await act(async () => { vi.advanceTimersByTime(10_000); });
    expect(statusMock).toHaveBeenCalledTimes(1); // no further calls after unmount
  });
});
