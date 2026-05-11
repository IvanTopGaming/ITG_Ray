import { lazy, Suspense } from "react";
import { HashRouter, Routes, Route, Navigate } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { Dashboard } from "./pages/Dashboard";

// Dashboard ships in the main bundle — it's the index route and the
// most visible cold-start screen, so paying its parse cost upfront
// keeps first paint identical to the pre-split build. The other pages
// (Servers, Subscriptions, Settings) load on first navigation; React
// Suspense holds the previous route while their chunks stream in.
const Servers = lazy(() =>
  import("./pages/Servers").then((m) => ({ default: m.Servers })),
);
const Subscriptions = lazy(() =>
  import("./pages/Subscriptions").then((m) => ({ default: m.Subscriptions })),
);
const Settings = lazy(() =>
  import("./pages/Settings").then((m) => ({ default: m.Settings })),
);

export default function App() {
  return (
    <HashRouter>
      <Suspense fallback={null}>
        <Routes>
          <Route element={<AppShell />}>
            <Route index element={<Dashboard />} />
            <Route path="servers" element={<Servers />} />
            <Route path="subscriptions" element={<Subscriptions />} />
            <Route path="settings" element={<Settings />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </Suspense>
    </HashRouter>
  );
}
