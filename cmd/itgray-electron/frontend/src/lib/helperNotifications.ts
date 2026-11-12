import { useEffect } from "react";
import type { TFunction } from "i18next";
import {
  StatusDetails,
  Install as HelperInstall,
  Start as HelperStart,
  Reinstall as HelperReinstall,
  InstallLinux as HelperInstallLinux,
  StartLinux as HelperStartLinux,
  RestartLinux as HelperRestartLinux,
  type HelperStatusDetails,
} from "@/lib/itg/HelperService";
import { Environment } from "@/lib/itg/runtime";
import { getDashState } from "@/lib/dashStore";
import { formatError } from "@/lib/helperAdapter";
import { notify, retract, type AppNotification } from "@/lib/notificationStore";

export const HELPER_NOTIFICATION_IDS = ["helper-missing", "helper-stopped", "helper-outdated"] as const;
export const HELPER_ERROR_ID = "helper-error";
export const HELPER_POLL_MS = 5000;

export type HelperPlatform = "windows" | "linux";
export type HelperOp = () => Promise<unknown>;
export type HelperContext = { connected: boolean; tunMode: boolean };

export function normalizeVersion(v?: string): string | null {
  const s = (v ?? "").trim().replace(/^v/i, "");
  if (!s || s === "dev" || s === "0.0.0-dev") return null;
  return s;
}

export function isHelperOutdated(d: HelperStatusDetails): boolean {
  if (d.state !== "running") return false;
  const installed = normalizeVersion(d.version);
  const expected = normalizeVersion(d.expectedVersion);
  return !!installed && !!expected && installed !== expected;
}

export function buildHelperNotification(
  d: HelperStatusDetails,
  platform: HelperPlatform,
  ctx: HelperContext,
  t: TFunction,
  act: (op: HelperOp) => Promise<void>,
): AppNotification | null {
  const linux = platform === "linux";
  if (d.state === "missing") {
    if (linux && !ctx.tunMode) return null;
    const packaged = linux && d.packageManaged;
    const op: HelperOp = !linux ? () => HelperInstall() : packaged ? HelperStartLinux : HelperInstallLinux;
    return {
      id: "helper-missing",
      tone: "warning",
      persistent: true,
      title: t(packaged ? "notifications.helperMissing.titlePackaged" : "notifications.helperMissing.title"),
      body: t(packaged ? "notifications.helperMissing.bodyPackaged" : linux ? "notifications.helperMissing.bodyTun" : "notifications.helperMissing.body"),
      actions: [{ label: t(packaged ? "notifications.actions.start" : "notifications.actions.install"), primary: true, run: () => act(op) }],
    };
  }
  if (d.state === "stopped") {
    return {
      id: "helper-stopped",
      tone: "warning",
      persistent: true,
      title: t("notifications.helperStopped.title"),
      body: t("notifications.helperStopped.body"),
      actions: [{ label: t("notifications.actions.start"), primary: true, run: () => act(linux ? HelperStartLinux : HelperStart) }],
    };
  }
  if (isHelperOutdated(d)) {
    const restartOnly = linux && d.packageManaged;
    const op: HelperOp = !linux ? HelperReinstall : restartOnly ? HelperRestartLinux : HelperInstallLinux;
    const body = t("notifications.helperOutdated.body", { installed: d.version, expected: d.expectedVersion });
    return {
      id: "helper-outdated",
      tone: "danger",
      persistent: true,
      title: t("notifications.helperOutdated.title"),
      body: ctx.connected ? `${body} ${t("notifications.helperOutdated.dropsConnection")}` : body,
      actions: [
        {
          label: t(restartOnly ? "notifications.actions.restart" : "notifications.actions.update"),
          primary: true,
          run: () => act(op),
        },
      ],
    };
  }
  return null;
}

export function useHelperNotifications(t: TFunction): void {
  useEffect(() => {
    let cancelled = false;
    let platform: HelperPlatform | null = null;
    let inFlight = false;
    let busy = false;

    const apply = (d: HelperStatusDetails) => {
      if (!platform) return;
      const dash = getDashState();
      const n = buildHelperNotification(d, platform, { connected: dash.status === "connected", tunMode: dash.mode === "tun" }, t, act);
      for (const id of HELPER_NOTIFICATION_IDS) if (n?.id !== id) retract(id);
      if (n) notify(n);
    };

    const poll = async () => {
      if (cancelled || !platform || inFlight || busy) return;
      if (document.visibilityState !== "visible") return;
      inFlight = true;
      try {
        const d = await StatusDetails();
        if (!cancelled) apply(d);
      } catch {
        return;
      } finally {
        inFlight = false;
      }
    };

    const act = async (op: HelperOp) => {
      busy = true;
      try {
        await op();
        retract(HELPER_ERROR_ID);
      } catch (e) {
        notify({ id: HELPER_ERROR_ID, tone: "danger", title: t("notifications.helperFailed.title"), body: formatError(e) });
      } finally {
        busy = false;
      }
      await poll();
    };

    void Environment().then((info) => {
      if (cancelled) return;
      if (info.platform === "windows" || info.platform === "linux") {
        platform = info.platform;
        void poll();
      }
    });
    const id = window.setInterval(() => void poll(), HELPER_POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [t]);
}
