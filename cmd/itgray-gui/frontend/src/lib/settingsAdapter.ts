import type { hub } from '../../wailsjs/go/models';
import type { Settings } from './settings';

/**
 * Maps a backend SettingsView into a partial frontend Settings patch.
 * Fields without a clean 1:1 mapping (backend 'auto', 'warn'; frontend
 * fields with no backend equivalent) are omitted — caller merges this
 * over existing state so unmapped frontend fields are preserved.
 */
export function backendToFrontend(view: hub.SettingsView): Partial<Settings> {
  const patch: Partial<Settings> = {};

  const language = view.general?.language;
  if (language === 'en' || language === 'ru') {
    patch.language = language;
  }

  if (typeof view.general?.autostart === 'boolean') {
    patch.autostart = view.general.autostart;
  }

  if (typeof view.general?.startMinimized === 'boolean') {
    patch.startMinimized = view.general.startMinimized;
  }

  const defaultMode = view.network?.defaultMode;
  if (defaultMode === 'tun' || defaultMode === 'sysproxy') {
    patch.defaultMode = defaultMode;
  }

  if (typeof view.network?.socksPort === 'number') {
    patch.socksPort = view.network.socksPort;
  }

  if (typeof view.notifications?.onConnected === 'boolean') {
    patch.onConnected = view.notifications.onConnected;
  }

  if (typeof view.notifications?.onSubSynced === 'boolean') {
    patch.onSubSynced = view.notifications.onSubSynced;
  }

  const logLevel = view.debug?.logLevel;
  if (logLevel === 'debug' || logLevel === 'info' || logLevel === 'error') {
    patch.logLevel = logLevel;
  }

  return patch;
}

/**
 * Splits a frontend Settings patch into per-section backend patches
 * suitable for sequential or parallel SettingsService.Update RPCs.
 *
 * Returned map omits sections that have no mapped keys in the input
 * patch, so callers can iterate without empty-section noise.
 *
 * Backend patch keys are aligned with Go config / hub.SettingsView JSON
 * tags (the Go-side surface). Frontend identifiers may diverge slightly
 * where JS conventions differ (e.g. `onQuotaLow` -> `quotaLow`,
 * `dnsCustom` -> `dnsServers` after CSV split).
 */
export function frontendToBackend(
  patch: Partial<Settings>,
): Map<string, Record<string, unknown>> {
  const out = new Map<string, Record<string, unknown>>();
  const ensure = (section: string): Record<string, unknown> => {
    let s = out.get(section);
    if (!s) {
      s = {};
      out.set(section, s);
    }
    return s;
  };

  // general
  if (patch.language !== undefined) ensure('general').language = patch.language;
  if (patch.autostart !== undefined) ensure('general').autostart = patch.autostart;
  if (patch.startMinimized !== undefined) ensure('general').startMinimized = patch.startMinimized;

  // network
  if (patch.defaultMode !== undefined) ensure('network').defaultMode = patch.defaultMode;
  if (patch.socksPort !== undefined) ensure('network').socksPort = patch.socksPort;
  if (patch.httpPort !== undefined) ensure('network').httpPort = patch.httpPort;
  if (patch.allowLan !== undefined) ensure('network').allowLan = patch.allowLan;
  if (patch.ipv6Mode !== undefined) ensure('network').ipv6Mode = patch.ipv6Mode;
  if (patch.dnsMode !== undefined) ensure('network').dnsMode = patch.dnsMode;
  if (patch.dnsCustom !== undefined) {
    ensure('network').dnsServers = patch.dnsCustom
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
  }
  if (patch.tunCidr !== undefined) ensure('network').tunCidr = patch.tunCidr;
  if (patch.tunMtu !== undefined) ensure('network').tunMtu = patch.tunMtu;

  // killswitch
  if (patch.killSwitchEnabled !== undefined) ensure('killswitch').enabled = patch.killSwitchEnabled;
  if (patch.killSwitchAlwaysOn !== undefined) ensure('killswitch').alwaysOn = patch.killSwitchAlwaysOn;

  // notifications
  if (patch.onConnected !== undefined) ensure('notifications').onConnected = patch.onConnected;
  if (patch.onDisconnected !== undefined) ensure('notifications').onDisconnected = patch.onDisconnected;
  if (patch.onQuotaLow !== undefined) ensure('notifications').quotaLow = patch.onQuotaLow;
  if (patch.onSubSynced !== undefined) ensure('notifications').onSubSynced = patch.onSubSynced;
  if (patch.notifySound !== undefined) ensure('notifications').sound = patch.notifySound;

  // debug
  if (patch.logLevel !== undefined) ensure('debug').logLevel = patch.logLevel;

  return out;
}
