import { describe, it, expect } from 'vitest';
import { DEFAULTS } from './settings';

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
