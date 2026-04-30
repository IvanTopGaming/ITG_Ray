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

  it('Update rejection is logged but state stays optimistic (Q4 B)', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    getMock.mockResolvedValue({ general: {}, network: {}, notifications: {}, debug: {} });
    updateMock.mockRejectedValue(new Error('disk full'));

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    act(() => result.current[1]({ autostart: true }));
    await act(async () => {
      vi.advanceTimersByTime(200);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(result.current[0].autostart).toBe(true); // optimistic; no rollback
    expect(warnSpy).toHaveBeenCalledWith(
      'SettingsService.Update failed',
      expect.objectContaining({ message: 'disk full' }),
    );
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
