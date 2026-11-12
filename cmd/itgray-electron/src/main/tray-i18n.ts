export type TrayLang = "en" | "ru";

export interface TrayStrings {
  statusIdle: string;
  statusConnecting: string;
  statusConnected: string;
  statusDisconnecting: string;
  statusError: string;
  connect: string;
  disconnect: string;
  cancelConnect: string;
  server: string;
  favorites: string;
  manual: string;
  noServers: string;
  mode: string;
  modeTun: string;
  modeSysproxy: string;
  routing: string;
  routingRule: string;
  routingGlobal: string;
  routingDirect: string;
  syncSubs: string;
  cancelSync: string;
  open: string;
  logs: string;
  stats: string;
  quit: string;
}

export const TRAY_STRINGS: Record<TrayLang, TrayStrings> = {
  en: {
    statusIdle: "Disconnected",
    statusConnecting: "Connecting…",
    statusConnected: "Connected",
    statusDisconnecting: "Disconnecting…",
    statusError: "Connection error",
    connect: "Connect",
    disconnect: "Disconnect",
    cancelConnect: "Cancel connecting",
    server: "Server",
    favorites: "★ Favorites",
    manual: "Added manually",
    noServers: "No servers",
    mode: "Mode",
    modeTun: "TUN",
    modeSysproxy: "System proxy",
    routing: "Routing",
    routingRule: "By rules",
    routingGlobal: "All via proxy",
    routingDirect: "All direct",
    syncSubs: "Update subscriptions",
    cancelSync: "Cancel subscription update",
    open: "Open ITG Ray",
    logs: "Logs",
    stats: "Statistics",
    quit: "Quit ITG Ray",
  },
  ru: {
    statusIdle: "Отключено",
    statusConnecting: "Подключение…",
    statusConnected: "Подключено",
    statusDisconnecting: "Отключение…",
    statusError: "Ошибка подключения",
    connect: "Подключить",
    disconnect: "Отключить",
    cancelConnect: "Отменить подключение",
    server: "Сервер",
    favorites: "★ Избранное",
    manual: "Добавленные вручную",
    noServers: "Нет серверов",
    mode: "Режим",
    modeTun: "TUN",
    modeSysproxy: "Системный прокси",
    routing: "Маршрутизация",
    routingRule: "По правилам",
    routingGlobal: "Всё через прокси",
    routingDirect: "Всё напрямую",
    syncSubs: "Обновить подписки",
    cancelSync: "Отменить обновление подписок",
    open: "Открыть ITG Ray",
    logs: "Логи",
    stats: "Статистика",
    quit: "Выйти",
  },
};

export function resolveTrayLang(setting: string | undefined, locale: string): TrayLang {
  if (setting === "ru" || setting === "en") return setting;
  return locale.toLowerCase().startsWith("ru") ? "ru" : "en";
}
