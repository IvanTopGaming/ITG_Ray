import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";
import { useNetworkChangedSinceConnect, getConnectSnapshot } from "@/lib/settings";
import { Connect, Disconnect } from "../../wailsjs/go/bindings/RunService";

export function AppShell() {
  const networkChanged = useNetworkChangedSinceConnect();

  const handleReconnect = async () => {
    const snap = getConnectSnapshot();
    if (!snap) return;
    try {
      await Disconnect();
      await Connect(snap.serverId, snap.mode);
    } catch (err) {
      console.warn('AppShell reconnect failed:', err);
    }
  };

  return (
    <div className="relative flex h-screen w-screen overflow-hidden">
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

      <div className="relative z-10 flex h-full w-full">
        <Sidebar />
        <main className="relative flex-1 overflow-y-auto px-8 py-8">
          {networkChanged && (
            <div
              role="status"
              className="sticky top-0 z-20 -mx-2 mb-4 px-3 py-2 rounded-[10px] bg-amber-500/15 border border-amber-500/30 text-[12px] text-amber-100 flex items-center justify-between gap-3 backdrop-blur-md"
            >
              <span>Settings will apply after reconnect.</span>
              <button
                onClick={handleReconnect}
                className="px-2 py-1 rounded bg-amber-500/30 hover:bg-amber-500/45 text-amber-50 transition-colors"
              >
                Reconnect
              </button>
            </div>
          )}
          <Outlet />
        </main>
      </div>
    </div>
  );
}
