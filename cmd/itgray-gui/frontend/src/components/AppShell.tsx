import { Outlet } from "react-router-dom";
import { Sidebar } from "./Sidebar";

export function AppShell() {
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
          <Outlet />
        </main>
      </div>
    </div>
  );
}
