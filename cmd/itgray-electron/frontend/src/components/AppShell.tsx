import { Suspense, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Outlet, useNavigate } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { TitleBar } from "./TitleBar";
import {
  useReconnectNeeded,
  useSettings,
  getConnectSnapshot,
  clearActiveServerEdited,
  dismissNetworkDiff,
  setRulesDismissed,
  getDesiredServer,
  clearDesiredServer,
} from "@/lib/settings";
import { dashReconnect, getDashState } from "@/lib/dashStore";
import { rulesRevertToBaseline, rulesHasRevertBaseline } from "@/lib/rulesStore";
import { setPendingImportLink } from "@/lib/deeplinkStore";
import { ReconnectToast } from "./ReconnectToast";
import { useTrayBridge } from "@/lib/trayBridge";
import { ConfirmDialog } from "./controls/ConfirmDialog";
import { NotificationStack } from "./NotificationStack";
import { useHelperNotifications } from "@/lib/helperNotifications";
import { useUpdateNotifications } from "@/lib/updateNotifications";

export function AppShell() {
  const reconnectNeeded = useReconnectNeeded();
  const { t, i18n } = useTranslation();
  const [settings] = useSettings();
  const navigate = useNavigate();
  useTrayBridge(navigate, settings.language);
  useHelperNotifications(t);
  useUpdateNotifications(t);
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  useEffect(() => {
    if (i18n.language !== settings.language)
      void i18n.changeLanguage(settings.language);
  }, [settings.language, i18n]);

  useEffect(() => {
    const off = window.itg.on("deeplink", (payload: unknown) => {
      const url = typeof payload === "string" ? payload : "";
      if (url.startsWith("itgray://rules/import/")) {
        setPendingImportLink(url);
        navigate("/routing");
      }
    });
    return off;
  }, [navigate]);

  useEffect(() => {
    void window.itg.app.getPendingDeeplink().then((url) => {
      if (typeof url === "string" && url.startsWith("itgray://rules/import/")) {
        setPendingImportLink(url);
        navigate("/routing");
      }
    });
  }, [navigate]);

  const handleReconnect = async () => {
    const snap = getConnectSnapshot();
    const serverId =
      getDesiredServer() ?? snap?.serverId ?? getDashState().currentServer?.id;
    const mode = snap?.mode ?? getDashState().mode;
    if (!serverId) return;
    // Reconnecting applies every pending change (deferred server switch,
    // edited active server, dirty rules, changed network settings), so
    // optimistically clear all reconnect signals — otherwise the toast
    // lingers even after a successful reconnect. The connected event
    // rebuilds the snapshot and re-derives the diff, so this can only
    // under-report.
    clearActiveServerEdited();
    setRulesDismissed();
    dismissNetworkDiff();
    clearDesiredServer();
    try {
      await dashReconnect(serverId, mode);
    } catch {
      // dashStore set lastError; Dashboard's Reveal surfaces it.
    }
  };

  // clearAllReconnectSignals clears EVERY signal reconnectNeeded() ORs
  // together — deferred server switch, active-server edit, dirty rules and
  // network-diff — else a toast armed by a signal we forget to clear can
  // never be dismissed. We do NOT drop lastConnectSnapshot: keeping it means
  // networkDiffersFromSnapshot() can still flip back to true on a future
  // edit, which clears networkDiffDismissed in useSettings.update() and
  // re-arms the toast.
  const clearAllReconnectSignals = () => {
    clearActiveServerEdited();
    setRulesDismissed();
    dismissNetworkDiff();
    clearDesiredServer();
  };

  const handleDismiss = () => {
    if (rulesHasRevertBaseline()) {
      setConfirmDiscard(true);
      return;
    }
    clearAllReconnectSignals();
  };

  const handleConfirmDiscard = () => {
    clearAllReconnectSignals();
    // Rule edits commit to the backend as they are made, so unlike the other
    // signals "dismiss" would otherwise silently keep them. Roll back to the
    // model the running tunnel was built from.
    void rulesRevertToBaseline();
  };

  return (
    <div className="relative flex h-screen w-screen flex-col overflow-hidden">
      <TitleBar />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 overflow-hidden"
      >
        <div
          className="absolute -top-20 left-1/4 h-[420px] w-[420px] rounded-full"
          style={{ background: "rgba(120,200,255,0.28)", filter: "blur(80px)" }}
        />
        <div
          className="absolute -bottom-24 -right-16 h-[440px] w-[440px] rounded-full"
          style={{ background: "rgba(180,100,255,0.28)", filter: "blur(90px)" }}
        />
      </div>

      <div className="relative z-10 flex w-full min-h-0 flex-1">
        <Sidebar />
        <main className="relative flex-1 overflow-y-auto px-8 py-8">
          <Suspense fallback={null}>
            <Outlet />
          </Suspense>
        </main>
      </div>

      <ReconnectToast
        visible={reconnectNeeded}
        message={t("appShell.reconnectMessage")}
        dismissLabel={
          rulesHasRevertBaseline()
            ? t("appShell.discardChanges")
            : t("common.dismiss")
        }
        onReconnect={handleReconnect}
        onDismiss={handleDismiss}
      />
      <NotificationStack raised={reconnectNeeded} />
      <ConfirmDialog
        open={confirmDiscard}
        title={t("appShell.discardRulesTitle")}
        description={t("appShell.discardRulesDescription")}
        confirmLabel={t("appShell.discardRulesConfirm")}
        confirmVariant="danger"
        onClose={() => setConfirmDiscard(false)}
        onConfirm={handleConfirmDiscard}
      />
    </div>
  );
}
