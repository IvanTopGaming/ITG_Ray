import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';

// Mocks must be set up BEFORE importing the SUT.
const getMock = vi.fn();
const updateMock = vi.fn();
const eventsOnMock = vi.fn();
const eventsOffMock = vi.fn();

vi.mock('../../wailsjs/go/bindings/SettingsService', () => ({
  Get: (...args: unknown[]) => getMock(...args),
  Update: (...args: unknown[]) => updateMock(...args),
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({
  EventsOn: (...args: unknown[]) => eventsOnMock(...args),
  EventsOff: (...args: unknown[]) => eventsOffMock(...args),
}));

import { useSettings, flushSettings, __resetForTests, DEFAULTS } from './settings';

beforeEach(() => {
  vi.useFakeTimers();
  // Pretend we are in a Wails-injected window.
  (window as any).go = {};
  getMock.mockReset();
  updateMock.mockReset();
  eventsOnMock.mockReset();
  eventsOffMock.mockReset();
  __resetForTests();
});

afterEach(() => {
  vi.useRealTimers();
  delete (window as any).go;
});

describe('useSettings (mocked SettingsService)', () => {
  it('returns DEFAULTS synchronously, then merges Get() result', async () => {
    getMock.mockResolvedValue({
      general: { language: 'ru' },
      network: { defaultMode: 'sysproxy' },
      notifications: {},
      debug: {},
    });

    const { result } = renderHook(() => useSettings());

    expect(result.current[0]).toEqual(DEFAULTS);

    await act(async () => {
      await Promise.resolve(); // flush microtasks
      await Promise.resolve();
    });
    expect(result.current[0].language).toBe('ru');
    expect(result.current[0].defaultMode).toBe('sysproxy');
    expect(getMock).toHaveBeenCalledTimes(1);
  });

  it('update() merges optimistically and fires Update RPC after 200ms', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => {
      result.current[1]({ autostart: true });
    });
    expect(result.current[0].autostart).toBe(true);
    expect(updateMock).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(199);
    });
    expect(updateMock).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(1);
      await Promise.resolve();
    });
    expect(updateMock).toHaveBeenCalledWith('general', { autostart: true });
  });

  it('coalesces multiple updates within debounce window into one section RPC', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ socksPort: 1080 }));
    act(() => { vi.advanceTimersByTime(50); });
    act(() => result.current[1]({ socksPort: 1081 }));
    act(() => { vi.advanceTimersByTime(70); });
    act(() => result.current[1]({ allowLan: true }));

    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
    });

    expect(updateMock).toHaveBeenCalledTimes(1);
    expect(updateMock).toHaveBeenCalledWith('network', { socksPort: 1081, allowLan: true });
  });

  it('multi-section patch fires one RPC per section in parallel', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ language: 'en', socksPort: 1080, onConnected: false }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(updateMock).toHaveBeenCalledTimes(3);
    const calls = updateMock.mock.calls.map(([section, patch]) => [section, patch]).sort(
      (a, b) => String(a[0]).localeCompare(String(b[0])),
    );
    expect(calls).toEqual([
      ['general', { language: 'en' }],
      ['network', { socksPort: 1080 }],
      ['notifications', { onConnected: false }],
    ]);
  });

  it('flushSettings() forces immediate RPC', async () => {
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    expect(updateMock).not.toHaveBeenCalled();

    await act(async () => {
      flushSettings();
      await Promise.resolve();
    });
    expect(updateMock).toHaveBeenCalledWith('general', { autostart: true });
  });

  it('Update rejection is logged; post-flush Get rolls state back to disk truth', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    // Disk explicitly has autostart=false. ConfigStore.toView always
    // emits the full struct, so the post-flush Get sees it and the
    // adapter includes it in the merge.
    getMock.mockResolvedValue({
      general: { autostart: false },
      network: {},
      notifications: {},
      debug: {},
    });
    updateMock.mockRejectedValue(new Error('disk full'));

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });

    // Disk never received the write (Update rejected). Post-flush Get
    // re-syncs from disk: autostart returns to its disk value (false).
    // This implicitly rolls back optimistic state on failed writes.
    expect(result.current[0].autostart).toBe(false);
    expect(warnSpy).toHaveBeenCalledWith(
      'SettingsService.Update failed',
      expect.objectContaining({ message: 'disk full' }),
    );
    warnSpy.mockRestore();
  });

  it('post-flush Get picks up cross-process disk drift (multi-writer)', async () => {
    // Bootstrap: disk has language=en, autostart=false.
    getMock.mockResolvedValueOnce({ general: {}, network: {}, notifications: {}, debug: {} });
    // Post-flush Get: another writer (CLI) changed language=ru on disk
    // during our flush window.
    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    updateMock.mockResolvedValue({});

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(updateMock).toHaveBeenCalledTimes(1);
    expect(getMock).toHaveBeenCalledTimes(2); // bootstrap + post-flush
    expect(result.current[0].language).toBe('ru'); // disk drift picked up
    expect(result.current[0].autostart).toBe(true); // our write persisted
  });

  it('flushNow leaves the in-flight counter at zero on empty section map', async () => {
    // Regression: an empty section map (e.g. patch with only undefined
    // keys) used to leave the inFlightUpdate flag stuck at true,
    // permanently muting backend EventSettings refresh.
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockResolvedValue({});
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    // Force a pending patch with no mappable backend keys by passing
    // only undefined values. update() still records pendingPatch but
    // frontendToBackend returns an empty Map.
    act(() => result.current[1]({ autostart: undefined as unknown as boolean }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
    });

    // Now an EventsOn echo should still trigger a fresh Get, proving
    // the in-flight gate didn't get stuck.
    const handler = eventsOnMock.mock.calls[0][1] as () => void;
    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    await act(async () => {
      handler();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current[0].language).toBe('ru');
  });

  it('retries Get on remount after initial bootstrap failure', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    getMock.mockRejectedValueOnce(new Error('rpc not ready'));

    const first = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(warnSpy).toHaveBeenCalledWith('SettingsService.Get failed', expect.any(Error));
    expect(getMock).toHaveBeenCalledTimes(1);
    first.unmount();

    // Bootstrap was a one-shot module-level guard; without retry-on-fail
    // it would never fire again. Verify a successful remount picks up
    // disk truth.
    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(getMock).toHaveBeenCalledTimes(2);
    expect(result.current[0].language).toBe('ru');
    warnSpy.mockRestore();
  });

  it('EventsOn("settings:changed") triggers Get + state merge', async () => {
    getMock.mockResolvedValueOnce({ general: {}, network: {}, notifications: {}, debug: {} });
    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    expect(eventsOnMock).toHaveBeenCalledWith('settings:changed', expect.any(Function));
    const handler = eventsOnMock.mock.calls[0][1] as () => void;

    getMock.mockResolvedValueOnce({
      general: { language: 'ru' },
      network: {},
      notifications: {},
      debug: {},
    });
    await act(async () => {
      handler();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current[0].language).toBe('ru');
  });
});
