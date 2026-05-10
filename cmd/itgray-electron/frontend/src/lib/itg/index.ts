// cmd/itgray-electron/frontend/src/lib/itg/index.ts
//
// Single typed entry point that fans out to window.itg.* preload methods.
// Replaces the per-service modules under wails-shim/bindings/. Call-site
// API is identical so the codemod is a one-line import swap.

const itg = () => window.itg as Record<string, Record<string, (...a: unknown[]) => Promise<unknown>>>;
const app = () => itg().app ?? {};
const helper = () => itg().helper ?? {};
const onboarding = () => itg().onboarding ?? {};
const settings = () => itg().settings ?? {};
const servers = () => itg().servers ?? {};
const subs = () => itg().subs ?? {};
const run = () => itg().run ?? {};

// ---- AppService (Wails-shim names preserved) ----
export const GetSnapshot = (): Promise<unknown> => app().getSnapshot?.() ?? Promise.resolve(null);
export const GetPublicIP = (): Promise<string> =>
  (app().getPublicIP?.() as Promise<string>) ?? Promise.resolve("");

// ---- HelperService ----
export const HelperStatus = (): Promise<string> => {
  const fn = helper().status;
  if (!fn) return Promise.resolve("unknown");
  return fn().then((r) => {
    if (r && typeof r === "object" && "state" in (r as Record<string, unknown>)) {
      const s = (r as { state?: unknown }).state;
      return typeof s === "string" ? s : "unknown";
    }
    return typeof r === "string" ? r : "unknown";
  });
};
export const HelperInstall = (): Promise<unknown> => helper().install?.() ?? Promise.resolve(null);
export const HelperStart = (): Promise<unknown> => helper().start?.() ?? Promise.resolve(null);
export const HelperStop = (): Promise<unknown> => helper().stop?.() ?? Promise.resolve(null);
export const HelperRestart = (): Promise<unknown> => helper().restart?.() ?? Promise.resolve(null);
export const HelperReinstall = (): Promise<unknown> => helper().reinstall?.() ?? Promise.resolve(null);

// ---- OnboardingService ----
export const OnboardingGetState = (): Promise<unknown> => onboarding().getState?.() ?? Promise.resolve(null);
export const OnboardingComplete = (): Promise<unknown> => onboarding().complete?.() ?? Promise.resolve(null);
export const OnboardingSkip = (): Promise<unknown> => onboarding().skip?.() ?? Promise.resolve(null);

// ---- SettingsService ----
export const SettingsGet = (): Promise<unknown> => settings().get?.() ?? Promise.resolve(null);
export const SettingsUpdate = (section: string, patch: Record<string, unknown>): Promise<unknown> =>
  settings().update?.(section, patch) ?? Promise.resolve(null);

// ---- ServersService ----
export const ServersList = (): Promise<unknown> => servers().list?.() ?? Promise.resolve([]);
export const ServersAdd = (uri: string, name: string): Promise<unknown> =>
  servers().add?.({ uri, name }) ?? Promise.resolve(null);
export const ServersEdit = (id: string, uri: string, name: string): Promise<unknown> =>
  servers().edit?.({ id, uri, name }) ?? Promise.resolve(null);
export const ServersRemove = (id: string): Promise<unknown> => servers().remove?.({ id }) ?? Promise.resolve(null);
export const ServersToggleFavorite = (id: string): Promise<unknown> =>
  servers().toggleFavorite?.({ id }) ?? Promise.resolve(null);
export const ServersTestLatency = (id: string): Promise<unknown> =>
  servers().testLatency?.({ id }) ?? Promise.resolve(null);

// ---- SubsService ----
export const SubsList = (): Promise<unknown> => subs().list?.() ?? Promise.resolve([]);
export const SubsAdd = (url: string, name: string): Promise<unknown> =>
  subs().add?.({ url, name }) ?? Promise.resolve(null);
export const SubsEdit = (id: string, url: string, name: string): Promise<unknown> =>
  subs().edit?.({ id, url, name }) ?? Promise.resolve(null);
export const SubsRemove = (id: string): Promise<unknown> => subs().remove?.({ id }) ?? Promise.resolve(null);
export const SubsSyncOne = (id: string): Promise<unknown> => subs().syncOne?.({ id }) ?? Promise.resolve(null);
export const SubsSyncAll = (): Promise<unknown> => subs().syncAll?.() ?? Promise.resolve(null);

// ---- RunService ----
export const Connect = (serverId: string, mode: string): Promise<unknown> =>
  run().connect?.({ serverId, mode }) ?? Promise.resolve(null);
export const Disconnect = (): Promise<unknown> => run().disconnect?.() ?? Promise.resolve(null);

// Re-export runtime helpers for call sites that import them as siblings.
export * from "./runtime";
