import { HashRouter } from "react-router-dom";
import { AppShell } from "./components/layout/AppShell";

// HashRouter is required because Wails serves static assets without
// server-side routing; BrowserRouter would 404 on deep links.
export default function App() {
  return (
    <HashRouter>
      <AppShell />
    </HashRouter>
  );
}
