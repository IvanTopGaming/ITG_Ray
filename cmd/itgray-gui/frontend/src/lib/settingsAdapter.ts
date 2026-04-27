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
    patch.launchOnStartup = view.general.autostart;
  }

  if (typeof view.general?.startMinimized === 'boolean') {
    patch.startMinimized = view.general.startMinimized;
  }

  const defaultMode = view.network?.defaultMode;
  if (defaultMode === 'tun') {
    patch.networkMode = 'tun';
  } else if (defaultMode === 'sysproxy') {
    patch.networkMode = 'system-proxy';
  }

  if (typeof view.network?.socksPort === 'number') {
    patch.socksPort = view.network.socksPort;
  }

  if (typeof view.notifications?.onConnected === 'boolean') {
    patch.notifyConnection = view.notifications.onConnected;
  }

  if (typeof view.notifications?.onSubSynced === 'boolean') {
    patch.notifySubFailure = view.notifications.onSubSynced;
  }

  const logLevel = view.debug?.logLevel;
  if (logLevel === 'debug' || logLevel === 'info' || logLevel === 'error') {
    patch.logLevel = logLevel;
  }

  return patch;
}
