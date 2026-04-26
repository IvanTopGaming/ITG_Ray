import { useEffect } from "react";
import { Routes, Route, Navigate } from "react-router-dom";
import { Header } from "./Header";
import { Sidebar } from "./Sidebar";
import { DashboardPage } from "@/pages/DashboardPage";
import { ServersPage } from "@/pages/ServersPage";
import { SubsPage } from "@/pages/SubsPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { Wizard } from "@/components/onboarding/Wizard";
import { useStore } from "@/store";
import { api } from "@/api/client";
import { attachEvents } from "@/api/events";

export function AppShell() {
  const setSnapshot = useStore((s) => s.setSnapshot);
  // Render the first-run wizard above the shell when the .onboarded
  // marker is missing. The flag flips back to true via the Wizard's
  // Skip / Complete handlers, which both write the marker on disk.
  const onboarded = useStore((s) => s.onboarded);
  useEffect(() => {
    api.getSnapshot().then(setSnapshot).catch((err) => {
      // eslint-disable-next-line no-console
      console.error("getSnapshot failed:", err);
    });
    attachEvents();
  }, [setSnapshot]);
  return (
    <div className="h-screen flex flex-col bg-surface-base text-text-primary">
      <Header />
      <div className="flex flex-1 min-h-0">
        <Sidebar />
        <main className="flex-1 overflow-auto p-6">
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/servers" element={<ServersPage />} />
            <Route path="/subs" element={<SubsPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Routes>
        </main>
      </div>
      {!onboarded && <Wizard />}
    </div>
  );
}
