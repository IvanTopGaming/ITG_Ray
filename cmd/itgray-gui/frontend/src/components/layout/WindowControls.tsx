import type { CSSProperties } from "react";
import { WindowMinimise, WindowToggleMaximise, Quit } from "../../../wailsjs/runtime/runtime";

type DragCSSProperties = CSSProperties & { WebkitAppRegion?: "drag" | "no-drag" };

export function WindowControls() {
  const noDrag: DragCSSProperties = { WebkitAppRegion: "no-drag" };
  return (
    <div style={noDrag} className="flex items-center gap-1 ml-2">
      <button
        className="w-7 h-7 hover:bg-white/10 rounded"
        onClick={() => WindowMinimise()}
        aria-label="Minimize"
      >
        —
      </button>
      <button
        className="w-7 h-7 hover:bg-white/10 rounded"
        onClick={() => WindowToggleMaximise()}
        aria-label="Maximize"
      >
        ▢
      </button>
      <button
        className="w-7 h-7 hover:bg-rose-500/20 rounded"
        onClick={() => Quit()}
        aria-label="Close"
      >
        ×
      </button>
    </div>
  );
}
