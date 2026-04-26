import { useEffect, useState } from "react";
import { Add as wailsAdd, SyncAll as wailsSyncAll } from "../../../wailsjs/go/bindings/SubsService";
import { useStore } from "@/store";
import type { SubView } from "@/api/client";

// Wails generates TS signatures with a leading context.Context arg even
// though the runtime injects it transparently. Cast to clean shapes so
// call sites stay readable. Mirrors api/client.ts and ServerActions.tsx.
const Add = wailsAdd as unknown as (url: string, name: string) => Promise<SubView>;
const SyncAll = wailsSyncAll as unknown as () => Promise<void>;

// AddSubDialog renders a centered modal with URL + name inputs and an
// inline error region. Validation lives server-side (subs.go's
// validateSubURL); the dialog only surfaces the message it gets back.
// open=false collapses the component to null so it can be mounted
// permanently in SubsPage without a portal.
export function AddSubDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [url, setUrl] = useState("");
  const [name, setName] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // Esc closes the dialog. Escape is keyboard table-stakes for modals; tab
  // trapping and focus-restore are deferred until the a11y polish pass.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, busy, onClose]);

  if (!open) return null;
  const submit = async () => {
    setErr(null);
    setBusy(true);
    try {
      const view = await Add(url, name);
      // Optimistic insert: the kicked-off SyncOne goroutine will emit a
      // sub:synced event later, but applySubSync only mutates rows that are
      // already in the store. Push the new SubView directly so the user
      // sees the card immediately, then setSnapshot's same-id replacement
      // semantics let the eventual sync update it in place.
      useStore.setState((cur) => ({ subs: [...cur.subs, view] }));
      setUrl(""); setName(""); onClose();
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };
  const fld = "w-full h-9 bg-white/5 border border-white/10 rounded px-3 text-sm";
  return (
    <div className="fixed inset-0 bg-black/50 grid place-items-center z-50" onClick={onClose}>
      <div className="bg-surface-base border border-white/10 rounded-xl p-5 w-[420px]" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg mb-3">Add subscription</h3>
        <input className={`${fld} mb-2`} placeholder="https://provider/sub/..." value={url} onChange={(e) => setUrl(e.target.value)} autoFocus />
        <input className={`${fld} mb-3`} placeholder="Friendly name (optional)" value={name} onChange={(e) => setName(e.target.value)} />
        {err && <div className="text-xs text-rose-400 mb-2">{err}</div>}
        <div className="flex gap-2 justify-end">
          <button className="px-3 h-8 rounded bg-white/5 border border-white/10 text-sm" onClick={onClose} disabled={busy}>Cancel</button>
          <button className="px-3 h-8 rounded bg-gradient-to-br from-indigo-500 to-pink-500 text-sm disabled:opacity-50" onClick={() => void submit()} disabled={busy || url.trim() === ""}>
            {busy ? "Adding…" : "Add"}
          </button>
        </div>
      </div>
    </div>
  );
}

// syncAllSubs is a thin re-export consumed by SubsPage's "Sync all" button.
// Errors propagate to a console rejection — the per-sub sub:synced events
// surface individual failures inline on each SubCard.
export function syncAllSubs(): Promise<void> { return SyncAll(); }
