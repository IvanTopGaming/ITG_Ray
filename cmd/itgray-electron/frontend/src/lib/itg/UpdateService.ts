export type AvailableUpdate = { latest: string; htmlUrl?: string };

type UpdateApi = {
  pending?: () => Promise<AvailableUpdate | null>;
  skip?: (version: string) => Promise<void>;
  openReleases?: (htmlUrl?: string) => Promise<void>;
  getAutoCheck?: () => Promise<boolean>;
  setAutoCheck?: (enabled: boolean) => Promise<boolean>;
};

const api = () => ((window.itg as unknown as { update?: UpdateApi }).update ?? {}) as UpdateApi;

export function Pending(): Promise<AvailableUpdate | null> {
  return api().pending?.() ?? Promise.resolve(null);
}

export function Skip(version: string): Promise<void> {
  return api().skip?.(version) ?? Promise.resolve();
}

export function OpenReleases(htmlUrl?: string): Promise<void> {
  return api().openReleases?.(htmlUrl) ?? Promise.resolve();
}

export function GetAutoCheck(): Promise<boolean> {
  return api().getAutoCheck?.() ?? Promise.resolve(true);
}

export function SetAutoCheck(enabled: boolean): Promise<boolean> {
  return api().setAutoCheck?.(enabled) ?? Promise.resolve(enabled);
}

export function OnAvailable(cb: (u: AvailableUpdate) => void): () => void {
  return window.itg.on("update.available", (p) => {
    const u = p as AvailableUpdate | null;
    if (u && typeof u.latest === "string" && u.latest) cb(u);
  });
}
