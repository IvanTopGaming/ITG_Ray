import { describe, it, expect } from 'vitest';
import type { hub } from '../../wailsjs/go/models';
import { backendToFrontend, frontendToBackend } from './settingsAdapter';

function makeView(overrides: Partial<{
  general: Partial<hub.GeneralSettings>;
  network: Partial<hub.NetworkSettings>;
  notifications: Partial<hub.NotificationSettings>;
  debug: Partial<hub.DebugSettings>;
}> = {}): hub.SettingsView {
  return {
    general: {
      language: 'en',
      theme: 'dark',
      autostart: false,
      closeToTray: false,
      startMinimized: false,
      ...overrides.general,
    },
    network: {
      defaultMode: 'tun',
      tunCidr: '',
      tunName: '',
      socksPort: 10808,
      httpPort: 0,
      ...overrides.network,
    },
    subscriptions: { defaultUpdateInterval: 0, userAgent: '' },
    notifications: {
      onConnected: true,
      onDisconnected: false,
      quotaLow: false,
      onSubSynced: true,
      ...overrides.notifications,
    },
    debug: { logLevel: 'info', ...overrides.debug },
    about: { version: '', gitRev: '', buildDate: '' },
    security: { method: '', available: false },
  } as unknown as hub.SettingsView;
}

describe('backendToFrontend', () => {
  it('maps all happy-path fields', () => {
    const view = makeView({
      general: { language: 'ru', autostart: true, startMinimized: true },
      network: { defaultMode: 'tun', socksPort: 12345 },
      notifications: { onConnected: false, onSubSynced: false },
      debug: { logLevel: 'debug' },
    });
    const patch = backendToFrontend(view);
    expect(patch).toEqual({
      language: 'ru',
      autostart: true,
      startMinimized: true,
      defaultMode: 'tun',
      socksPort: 12345,
      onConnected: false,
      onSubSynced: false,
      logLevel: 'debug',
    });
  });

  it("translates defaultMode 'sysproxy' to defaultMode 'sysproxy'", () => {
    const view = makeView({ network: { defaultMode: 'sysproxy' } });
    const patch = backendToFrontend(view);
    expect(patch.defaultMode).toBe('sysproxy');
  });

  it("omits defaultMode when defaultMode is 'auto'", () => {
    const view = makeView({ network: { defaultMode: 'auto' } });
    const patch = backendToFrontend(view);
    expect(patch).not.toHaveProperty('defaultMode');
  });

  it("omits language when general.language is 'auto'", () => {
    const view = makeView({ general: { language: 'auto' } });
    const patch = backendToFrontend(view);
    expect(patch).not.toHaveProperty('language');
  });

  it("omits logLevel when debug.logLevel is 'warn'", () => {
    const view = makeView({ debug: { logLevel: 'warn' } });
    const patch = backendToFrontend(view);
    expect(patch).not.toHaveProperty('logLevel');
  });

  it('does not include unmapped frontend keys', () => {
    const view = makeView();
    const patch = backendToFrontend(view);
    expect(patch).not.toHaveProperty('dnsMode');
    expect(patch).not.toHaveProperty('dnsCustom');
    expect(patch).not.toHaveProperty('allowLan');
    expect(patch).not.toHaveProperty('httpPort');
    expect(patch).not.toHaveProperty('ipv6Mode');
    expect(patch).not.toHaveProperty('notifySound');
  });
});

describe('frontendToBackend', () => {
  it('returns empty map for empty patch', () => {
    expect(frontendToBackend({}).size).toBe(0);
  });

  it('routes a single general field to the general section', () => {
    const map = frontendToBackend({ language: 'ru' });
    expect(map.size).toBe(1);
    expect(map.get('general')).toEqual({ language: 'ru' });
  });

  it('routes a single network field to the network section', () => {
    const map = frontendToBackend({ socksPort: 1080 });
    expect(map.get('network')).toEqual({ socksPort: 1080 });
  });

  it('splits a multi-section patch into per-section entries', () => {
    const map = frontendToBackend({
      language: 'en',
      socksPort: 1080,
      onConnected: true,
    });
    expect(map.get('general')).toEqual({ language: 'en' });
    expect(map.get('network')).toEqual({ socksPort: 1080 });
    expect(map.get('notifications')).toEqual({ onConnected: true });
  });

  it('transforms dnsCustom CSV into dnsServers array', () => {
    const map = frontendToBackend({ dnsCustom: '1.1.1.1, 8.8.8.8 ,,1.0.0.1' });
    expect(map.get('network')).toEqual({
      dnsServers: ['1.1.1.1', '8.8.8.8', '1.0.0.1'],
    });
  });

  it('produces dnsServers: [] when dnsCustom is the empty string', () => {
    expect(frontendToBackend({ dnsCustom: '' }).get('network')).toEqual({
      dnsServers: [],
    });
  });

  it('routes onQuotaLow (frontend) to quotaLow (backend) under notifications', () => {
    const map = frontendToBackend({ onQuotaLow: true });
    expect(map.get('notifications')).toEqual({ quotaLow: true });
  });

  it('routes killSwitchEnabled to enabled under killswitch section', () => {
    const map = frontendToBackend({ killSwitchEnabled: false, killSwitchAlwaysOn: true });
    expect(map.get('killswitch')).toEqual({ enabled: false, alwaysOn: true });
  });

  it('omits sections with no mapped keys', () => {
    const map = frontendToBackend({ language: 'en' });
    expect(map.has('network')).toBe(false);
    expect(map.has('notifications')).toBe(false);
  });
});
