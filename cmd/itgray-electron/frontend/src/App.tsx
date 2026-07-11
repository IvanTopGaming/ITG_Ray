import { lazy } from "react";
import { HashRouter, Routes, Route, Navigate } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { Dashboard } from "./pages/Dashboard";

// Dashboard ships in the main bundle — it's the index route and the
// most visible cold-start screen, so paying its parse cost upfront
// keeps first paint identical to the pre-split build. The other pages
// (Servers, Subscriptions, Settings) load on first navigation. The
// Suspense boundary lives INSIDE AppShell around the <Outlet>, so a
// first-time chunk load only blanks the content area — the shell
// (titlebar, sidebar, background) stays mounted (no whole-app flicker).
const Servers = lazy(() =>
  import("./pages/Servers").then((m) => ({ default: m.Servers })),
);
const Subscriptions = lazy(() =>
  import("./pages/Subscriptions").then((m) => ({ default: m.Subscriptions })),
);
const Settings = lazy(() =>
  import("./pages/Settings").then((m) => ({ default: m.Settings })),
);
const Routing = lazy(() =>
  import("./pages/Routing").then((m) => ({ default: m.Routing })),
);
const RuleEditor = lazy(() =>
  import("./pages/RuleEditor").then((m) => ({ default: m.RuleEditor })),
);

export default function App() {
  return (
    <HashRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<Dashboard />} />
          <Route path="servers" element={<Servers />} />
          <Route path="subscriptions" element={<Subscriptions />} />
          <Route path="routing" element={<Routing />} />
          <Route path="routing/:ruleId" element={<RuleEditor />} />
          <Route path="settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </HashRouter>
  );
}
