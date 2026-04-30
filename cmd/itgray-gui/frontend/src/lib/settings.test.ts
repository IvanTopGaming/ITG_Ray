import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, act } from '@testing-library/react';

vi.mock('../../wailsjs/go/bindings/SettingsService', () => ({
  Get: vi.fn(),
}));

import { DEFAULTS, STORAGE_KEY, loadSettings, saveSettings, flushSettings, useSettings, __resetForTests } from './settings';
import { Get as GetSettings } from '../../wailsjs/go/bindings/SettingsService';

describe('DEFAULTS', () => {
  it('contains all expected keys with correct types', () => {
    expect(DEFAULTS.language).toBe('en');
    expect(DEFAULTS.autostart).toBe(false);
    expect(DEFAULTS.startMinimized).toBe(false);
    expect(DEFAULTS.defaultMode).toBe('tun');
    expect(DEFAULTS.dnsMode).toBe('auto');
    expect(DEFAULTS.dnsCustom).toBe('');
    expect(DEFAULTS.allowLan).toBe(false);
    expect(DEFAULTS.socksPort).toBe(1080);
    expect(DEFAULTS.httpPort).toBe(8888);
    expect(DEFAULTS.ipv6Mode).toBe('prefer-v4');
    expect(DEFAULTS.onConnected).toBe(true);
    expect(DEFAULTS.notifySound).toBe(true);
    expect(DEFAULTS.onSubSynced).toBe(true);
    expect(DEFAULTS.logLevel).toBe('info');
  });
});

describe('loadSettings', () => {
  beforeEach(() => {
    __resetForTests();
    localStorage.clear();
  });

  it('returns DEFAULTS when localStorage is empty', () => {
    expect(loadSettings()).toEqual(DEFAULTS);
  });

  it('returns DEFAULTS without writing to localStorage on first load', () => {
    loadSettings();
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it('returns saved values merged over DEFAULTS', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ language: 'ru', allowLan: false }));
    const s = loadSettings();
    expect(s.language).toBe('ru');
    expect(s.allowLan).toBe(false);
    expect(s.defaultMode).toBe('tun'); // from DEFAULTS
  });

  it('returns DEFAULTS and warns on corrupt JSON', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    localStorage.setItem(STORAGE_KEY, '{not json');
    expect(loadSettings()).toEqual(DEFAULTS);
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });
});

describe('saveSettings', () => {
  beforeEach(() => {
    __resetForTests();
    localStorage.clear();
    vi.useFakeTimers();
  });

  it('writes JSON to localStorage after debounce window', () => {
    saveSettings({ ...DEFAULTS, language: 'ru' });
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull(); // not yet flushed
    vi.advanceTimersByTime(250);
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.language).toBe('ru');
  });

  it('coalesces rapid calls into one write', () => {
    saveSettings({ ...DEFAULTS, language: 'ru' });
    saveSettings({ ...DEFAULTS, language: 'en', allowLan: false });
    vi.advanceTimersByTime(250);
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.language).toBe('en');
    expect(stored.allowLan).toBe(false);
  });
});

describe('flushSettings', () => {
  beforeEach(() => {
    __resetForTests();
    localStorage.clear();
    vi.useFakeTimers();
  });

  it('writes pending value immediately without waiting for debounce', () => {
    saveSettings({ ...DEFAULTS, language: 'ru' });
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
    flushSettings();
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.language).toBe('ru');
  });

  it('is a no-op when nothing is pending', () => {
    expect(() => flushSettings()).not.toThrow();
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });

  it('cancels the scheduled timer so no second write fires', () => {
    saveSettings({ ...DEFAULTS, language: 'ru' });
    flushSettings();
    localStorage.clear();
    vi.advanceTimersByTime(250);
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
  });
});

describe('useSettings', () => {
  beforeEach(() => {
    __resetForTests();
    localStorage.clear();
    vi.useFakeTimers();
  });

  it('returns DEFAULTS initially', () => {
    const { result } = renderHook(() => useSettings());
    expect(result.current[0]).toEqual(DEFAULTS);
  });

  it('updates state and persists when patch is applied', () => {
    const { result } = renderHook(() => useSettings());
    act(() => result.current[1]({ language: 'ru' }));
    expect(result.current[0].language).toBe('ru');
    expect(result.current[0].allowLan).toBe(true); // unchanged
    vi.advanceTimersByTime(250);
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.language).toBe('ru');
  });

  it('shares state across multiple hook subscribers', () => {
    const { result: a } = renderHook(() => useSettings());
    const { result: b } = renderHook(() => useSettings());
    act(() => a.current[1]({ allowLan: false }));
    expect(b.current[0].allowLan).toBe(false);
  });

  it('persists pending changes immediately on flushSettings()', () => {
    const { result } = renderHook(() => useSettings());
    act(() => result.current[1]({ language: 'ru' }));
    // Don't advance timers; flush manually.
    flushSettings();
    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY)!);
    expect(stored.language).toBe('ru');
  });

  it('reloads state when another tab writes to localStorage', () => {
    const { result } = renderHook(() => useSettings());
    act(() => {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...DEFAULTS, language: 'ru' }));
      window.dispatchEvent(new StorageEvent('storage', { key: STORAGE_KEY, newValue: localStorage.getItem(STORAGE_KEY) }));
    });
    expect(result.current[0].language).toBe('ru');
  });

  it('returns a stable update reference across renders', () => {
    const { result, rerender } = renderHook(() => useSettings());
    const first = result.current[1];
    rerender();
    expect(result.current[1]).toBe(first);
  });
});

describe('useSettings backend load', () => {
  beforeEach(() => {
    vi.useRealTimers();
    __resetForTests();
    localStorage.clear();
    vi.mocked(GetSettings).mockReset();
    (window as any).go = {};
  });
  afterEach(() => {
    delete (window as any).go;
  });

  it('merges backend mapped fields over localStorage on first subscribe', async () => {
    vi.mocked(GetSettings).mockResolvedValue({
      general: { language: 'ru', theme: 'dark', autostart: true, closeToTray: false, startMinimized: false },
      network: { defaultMode: 'tun', tunCidr: '', tunName: '', socksPort: 12345, httpPort: 0 },
      subscriptions: { defaultUpdateInterval: 0, userAgent: '' },
      notifications: { onConnected: false, onDisconnected: false, quotaLow: false, onSubSynced: false },
      debug: { logLevel: 'debug' },
      about: { version: '', gitRev: '', buildDate: '' },
      security: { method: '', available: false },
    } as any);

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); });

    expect(result.current[0].language).toBe('ru');
    expect(result.current[0].autostart).toBe(true);
    expect(result.current[0].socksPort).toBe(12345);
    expect(result.current[0].logLevel).toBe('debug');
    expect(result.current[0].httpPort).toBe(10809);
    expect(result.current[0].dnsMode).toBe('auto');
  });

  it('leaves state unchanged when backend rejects', async () => {
    vi.mocked(GetSettings).mockRejectedValue(new Error('binding failed'));
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});

    const { result } = renderHook(() => useSettings());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(result.current[0]).toEqual(DEFAULTS);
    expect(warn).toHaveBeenCalled();
    warn.mockRestore();
  });
});
