import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { TitleBar } from "./TitleBar";
import {
  useReconnectNeeded,
  useSettings,
  getConnectSnapshot,
  clearActiveServerEdited,
  dismissNetworkDiff,
} from "@/lib/settings";
import { dashReconnect } from "@/lib/dashStore";
import { ReconnectToast } from "./ReconnectToast";

export function AppShell() {
  const reconnectNeeded = useReconnectNeeded();
  const { t, i18n } = useTranslation();
  const [settings] = useSettings();
  useEffect(() => {
    if (i18n.language !== settings.language)
      void i18n.changeLanguage(settings.language);
  }, [settings.language, i18n]);

  const handleReconnect = async () => {
    const snap = getConnectSnapshot();
    if (!snap) return;
    try {
      await dashReconnect(snap.serverId, snap.mode);
    } catch {
      // dashStore set lastError; Dashboard's Reveal surfaces it.
    }
  };

  const handleDismiss = () => {
    // Clear the active-edit signal AND mark the current network-diff as
    // dismissed. We do NOT drop lastConnectSnapshot here — keeping it
    // means networkDiffersFromSnapshot() can still flip back to true on
    // a future edit, which clears networkDiffDismissed in useSettings
    // .update() and re-arms the toast. Dropping the snapshot would
    // disarm network-diff detection until the next reconnect.
    clearActiveServerEdited();
    dismissNetworkDiff();
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
          <Outlet />
        </main>
      </div>

      <ReconnectToast
        visible={reconnectNeeded}
        message={t("appShell.reconnectMessage")}
        onReconnect={handleReconnect}
        onDismiss={handleDismiss}
      />
    </div>
  );
}
