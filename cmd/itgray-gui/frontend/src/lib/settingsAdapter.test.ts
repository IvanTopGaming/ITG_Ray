import { describe, it, expect } from 'vitest';
import type { hub } from '../../wailsjs/go/models';
import { backendToFrontend } from './settingsAdapter';

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
      launchOnStartup: true,
      startMinimized: true,
      networkMode: 'tun',
      socksPort: 12345,
      notifyConnection: false,
      notifySubFailure: false,
      logLevel: 'debug',
    });
  });

  it("translates defaultMode 'sysproxy' to networkMode 'system-proxy'", () => {
    const view = makeView({ network: { defaultMode: 'sysproxy' } });
    const patch = backendToFrontend(view);
    expect(patch.networkMode).toBe('system-proxy');
  });

  it("omits networkMode when defaultMode is 'auto'", () => {
    const view = makeView({ network: { defaultMode: 'auto' } });
    const patch = backendToFrontend(view);
    expect(patch).not.toHaveProperty('networkMode');
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
