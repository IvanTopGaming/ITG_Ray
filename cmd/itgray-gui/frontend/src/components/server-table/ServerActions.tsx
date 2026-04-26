import { Zap, Timer, MoreHorizontal, Star } from "lucide-react";
import {
  TestLatency as wailsTestLatency,
  ToggleFavorite as wailsToggleFavorite,
} from "../../../wailsjs/go/bindings/ServersService";

// Wails generates TS signatures with a leading context.Context arg even
// though the runtime injects it transparently. Cast to single-arg shapes
// so call sites stay clean. Mirrors the api/client.ts pattern used by
// AppService.GetSnapshot.
const TestLatency = wailsTestLatency as unknown as (id: string) => Promise<void>;
const ToggleFavorite = wailsToggleFavorite as unknown as (id: string) => Promise<void>;

// ServerActions renders the four hover-revealed icon buttons in a row.
// Connect is a placeholder until C.T10 wires the run binding.
export function ServerActions({ id, favorite }: { id: string; favorite: boolean }) {
  return (
    <div className="flex gap-1 justify-end opacity-60 hover:opacity-100 transition">
      <button
        title="Connect"
        className="w-6 h-6 rounded bg-gradient-to-br from-indigo-500 to-pink-500 text-white grid place-items-center"
        onClick={() => alert("Connect lands in C.T10")}
      >
        <Zap size={12} />
      </button>
      <button
        title="Test latency"
        className="w-6 h-6 rounded bg-white/5 grid place-items-center"
        onClick={() => {
          void TestLatency(id);
        }}
      >
        <Timer size={12} />
      </button>
      <button
        title="Toggle favorite"
        className="w-6 h-6 rounded bg-white/5 grid place-items-center"
        onClick={() => {
          void ToggleFavorite(id);
        }}
      >
        <Star size={12} className={favorite ? "fill-amber-400 text-amber-400" : ""} />
      </button>
      <button title="More" className="w-6 h-6 rounded bg-white/5 grid place-items-center">
        <MoreHorizontal size={12} />
      </button>
    </div>
  );
}
