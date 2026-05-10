import { useEffect, useState } from "react";
import {
  WindowMinimise,
  WindowToggleMaximise,
  WindowIsMaximised,
  WindowClose,
} from "@/lib/itg/runtime";

// Custom 32px title bar for the frameless Wails window. Provides a
// drag region (CSS --wails-draggable: drag) and Windows-style
// minimize / maximize-restore / close buttons. Replaces the native
// title bar removed by `Frameless: true` in cmd/itgray-gui/main.go.
export function TitleBar() {
  const [maximized, setMaximized] = useState(false);

  // Keep the maximize/restore icon synced with actual window state.
  // The user can toggle maximize via Win+Up/Down or by double-clicking
  // the bar, neither of which fires our onClick handler — so poll
  // WindowIsMaximised() on every window resize.
  useEffect(() => {
    let cancelled = false;
    const refresh = async () => {
      try {
        const max = await WindowIsMaximised();
        if (!cancelled) setMaximized(max);
      } catch {
        // No-op: WindowIsMaximised may throw in non-Wails contexts
        // (e.g. unit tests where the runtime is mocked or absent).
      }
    };
    refresh();
    window.addEventListener("resize", refresh);
    return () => {
      cancelled = true;
      window.removeEventListener("resize", refresh);
    };
  }, []);

  return (
    <div
      className="relative z-30 flex h-8 shrink-0 select-none items-center justify-between border-b border-white/[0.06] bg-bg-1/60 backdrop-blur-md"
      style={{ ["--wails-draggable" as never]: "drag" }}
    >
      <div className="flex items-center gap-2 px-3 text-[11px] font-medium text-white/55">
        <div
          aria-hidden
          className="h-3.5 w-3.5 rounded-[4px] bg-orb-accent shadow-[0_0_8px_rgba(120,200,255,0.55)]"
        />
        <span className="tracking-tight">ITG Ray</span>
      </div>
      <div
        className="flex h-full items-center"
        style={{ ["--wails-draggable" as never]: "no-drag" }}
      >
        <button
          type="button"
          onClick={() => void WindowMinimise()}
          aria-label="Minimize"
          className="flex h-full w-12 items-center justify-center text-white/55 transition-colors hover:bg-white/5 hover:text-white/90"
        >
          <svg width="10" height="1" viewBox="0 0 10 1" fill="currentColor">
            <rect width="10" height="1" />
          </svg>
        </button>
        <button
          type="button"
          onClick={async () => {
            await WindowToggleMaximise();
            setMaximized((m) => !m);
          }}
          aria-label={maximized ? "Restore" : "Maximize"}
          className="flex h-full w-12 items-center justify-center text-white/55 transition-colors hover:bg-white/5 hover:text-white/90"
        >
          {maximized ? (
            <svg
              width="10"
              height="10"
              viewBox="0 0 10 10"
              fill="none"
              stroke="currentColor"
              strokeWidth="1"
            >
              <rect x="2" y="0.5" width="7" height="7" />
              <rect x="0.5" y="2" width="7" height="7" />
            </svg>
          ) : (
            <svg
              width="10"
              height="10"
              viewBox="0 0 10 10"
              fill="none"
              stroke="currentColor"
              strokeWidth="1"
            >
              <rect x="0.5" y="0.5" width="9" height="9" />
            </svg>
          )}
        </button>
        <button
          type="button"
          onClick={() => void WindowClose()}
          aria-label="Close"
          className="flex h-full w-12 items-center justify-center text-white/55 transition-colors hover:bg-danger hover:text-white"
        >
          <svg
            width="10"
            height="10"
            viewBox="0 0 10 10"
            fill="none"
            stroke="currentColor"
            strokeWidth="1"
          >
            <line x1="0.5" y1="0.5" x2="9.5" y2="9.5" />
            <line x1="9.5" y1="0.5" x2="0.5" y2="9.5" />
          </svg>
        </button>
      </div>
    </div>
  );
}
