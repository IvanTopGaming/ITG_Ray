// cmd/itgray-electron/src/main/notifications.ts

export interface NotifPrefs {
  onConnected: boolean;
  onDisconnected: boolean;
  onSubSynced: boolean;
  sound: boolean;
}

export interface NotifierDeps {
  /** Show an OS notification. `silent` suppresses the sound. */
  notify: (title: string, body: string, opts: { silent: boolean }) => void;
  /** Resolve current notification prefs (e.g. from app.getSnapshot). */
  getSettings: () => Promise<NotifPrefs>;
}

export interface Notifier {
  onVpnStatus: (payload: unknown) => Promise<void>;
  onSubSynced: (payload: unknown) => Promise<void>;
}

type VpnStatus = "idle" | "connecting" | "connected" | "error" | string;

/**
 * makeNotifier builds an OS-notification dispatcher driven by bridge events.
 * Connect/disconnect fire on TRANSITIONS only (edge-detected against the
 * last seen status) so repeated vpn.status echoes don't double-notify.
 * Prefs are read per-event so a settings change takes effect without a
 * restart. A getSettings failure suppresses the notification (never throws).
 */
export function makeNotifier(deps: NotifierDeps): Notifier {
  let last: VpnStatus = "idle";

  async function prefs(): Promise<NotifPrefs | null> {
    try {
      return await deps.getSettings();
    } catch {
      return null;
    }
  }

  return {
    async onVpnStatus(payload: unknown): Promise<void> {
      const status = (payload as { status?: VpnStatus } | null)?.status ?? "idle";
      const prev = last;
      last = status;
      if (status === prev) return;

      if (status === "connected" && prev !== "connected") {
        const p = await prefs();
        if (p?.onConnected) deps.notify("ITG Ray — Connected", "VPN tunnel is up.", { silent: !p.sound });
        return;
      }
      if (prev === "connected" && status !== "connected") {
        const p = await prefs();
        if (p?.onDisconnected) deps.notify("ITG Ray — Disconnected", "VPN tunnel is down.", { silent: !p.sound });
      }
    },

    async onSubSynced(payload: unknown): Promise<void> {
      const data = payload as { status?: string; importedCount?: number } | null;
      // The sub:synced event fires for both outcomes; status is "ok" | "error".
      // Suppress the "updated" toast on a failed sync (avoid a false success).
      if (data?.status === "error") return;
      const p = await prefs();
      if (!p?.onSubSynced) return;
      const count = data?.importedCount;
      const body =
        typeof count === "number" && count > 0
          ? `Imported ${count} server${count === 1 ? "" : "s"}.`
          : "A subscription was updated.";
      deps.notify("ITG Ray — Subscription updated", body, { silent: !p.sound });
    },
  };
}
