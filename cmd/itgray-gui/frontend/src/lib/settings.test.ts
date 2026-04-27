import { describe, it, expect, beforeEach, vi } from 'vitest';
import { DEFAULTS, STORAGE_KEY, loadSettings, saveSettings, flushSettings } from './settings';

describe('DEFAULTS', () => {
  it('contains all expected keys with correct types', () => {
    expect(DEFAULTS.language).toBe('en');
    expect(DEFAULTS.launchOnStartup).toBe(false);
    expect(DEFAULTS.startMinimized).toBe(false);
    expect(DEFAULTS.networkMode).toBe('tun');
    expect(DEFAULTS.dnsMode).toBe('auto');
    expect(DEFAULTS.dnsCustom).toBe('');
    expect(DEFAULTS.allowLan).toBe(true);
    expect(DEFAULTS.notifyConnection).toBe(true);
    expect(DEFAULTS.notifySound).toBe(false);
    expect(DEFAULTS.notifySubFailure).toBe(true);
    expect(DEFAULTS.logLevel).toBe('info');
  });
});

describe('loadSettings', () => {
  beforeEach(() => localStorage.clear());

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
    expect(s.networkMode).toBe('tun'); // from DEFAULTS
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
