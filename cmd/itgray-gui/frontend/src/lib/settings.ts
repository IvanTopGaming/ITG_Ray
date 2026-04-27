export type Language = 'en' | 'ru';
export type NetworkMode = 'tun' | 'system-proxy' | 'off';
export type DnsMode = 'auto' | 'custom';
export type LogLevel = 'error' | 'info' | 'debug' | 'trace';

export type Settings = {
  language: Language;
  launchOnStartup: boolean;
  startMinimized: boolean;
  networkMode: NetworkMode;
  dnsMode: DnsMode;
  dnsCustom: string;
  allowLan: boolean;
  notifyConnection: boolean;
  notifySound: boolean;
  notifySubFailure: boolean;
  logLevel: LogLevel;
};

export const DEFAULTS = {
  language: 'en',
  launchOnStartup: false,
  startMinimized: false,
  networkMode: 'tun',
  dnsMode: 'auto',
  dnsCustom: '',
  allowLan: true,
  notifyConnection: true,
  notifySound: false,
  notifySubFailure: true,
  logLevel: 'info',
} as const satisfies Settings;

export const STORAGE_KEY = 'itgray.settings.v1';
