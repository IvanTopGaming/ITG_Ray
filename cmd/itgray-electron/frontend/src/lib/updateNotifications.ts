import { useEffect } from "react";
import type { TFunction } from "i18next";
import { Pending, Skip, OpenReleases, OnAvailable, type AvailableUpdate } from "@/lib/itg/UpdateService";
import { StatusPackageManaged } from "@/lib/itg/HelperService";
import { dismiss, notify } from "@/lib/notificationStore";

export function updateNotificationId(version: string): string {
  return `update-${version}`;
}

export function useUpdateNotifications(t: TFunction): void {
  useEffect(() => {
    let cancelled = false;
    const show = async (u: AvailableUpdate) => {
      const packaged = await StatusPackageManaged().catch(() => false);
      if (cancelled) return;
      const id = updateNotificationId(u.latest);
      notify({
        id,
        tone: "info",
        persistent: true,
        title: t("notifications.update.title", { version: u.latest }),
        body: t(packaged ? "notifications.update.bodyPackaged" : "notifications.update.body"),
        actions: [
          {
            label: t("notifications.update.skip"),
            run: async () => {
              await Skip(u.latest);
              dismiss(id);
            },
          },
          {
            label: t("notifications.update.open"),
            primary: true,
            run: async () => {
              await OpenReleases(u.htmlUrl);
              dismiss(id);
            },
          },
        ],
      });
    };
    void Pending()
      .then((u) => {
        if (u) void show(u);
      })
      .catch(() => {});
    const off = OnAvailable((u) => void show(u));
    return () => {
      cancelled = true;
      off();
    };
  }, [t]);
}
