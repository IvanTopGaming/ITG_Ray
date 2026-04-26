import { useEffect, useState } from "react";
import {
  Status as wailsStatus,
  Install as wailsInstall,
  Start as wailsStart,
} from "../../../wailsjs/go/bindings/HelperService";

// Wails-generated TS signatures include a context arg the runtime drops.
// An empty Install() path lets the Go side resolve the helper exe via
// os.Executable() so the user does not have to type a Windows path.
const Status = wailsStatus as unknown as () => Promise<string>;
const Install = wailsInstall as unknown as (path: string) => Promise<void>;
const Start = wailsStart as unknown as () => Promise<void>;

// StepWelcome polls Helper.Status, surfaces a "Fix it" button when the
// service is missing or stopped, and gates the Continue button on a
// running helper. The Continue handler is owned by the Wizard so this
// component stays purely about Helper state.
export function StepWelcome({ onNext }: { onNext: () => void }) {
  const [helper, setHelper] = useState<string>("...");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const refresh = () =>
    Status()
      .then((s) => {
        setHelper(s);
        setErr(null);
      })
      .catch((e) => {
        setHelper("missing");
        setErr(String(e?.message ?? e));
      });

  useEffect(() => {
    refresh();
  }, []);

  const fix = async () => {
    setBusy(true);
    setErr(null);
    try {
      if (helper === "missing") await Install("");
      await Start();
      await refresh();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const ok = helper === "running";
  return (
    <div className="space-y-4">
      <h2 className="text-xl font-semibold">Welcome to ITG Ray</h2>
      <p className="text-text-secondary text-sm">
        Privacy-respecting VLESS VPN client. Two-step setup: confirm the
        helper service is running, then add your first subscription.
      </p>
      <div
        className={`rounded-lg p-3 border text-sm ${
          ok
            ? "bg-emerald-500/10 border-emerald-500/30"
            : "bg-amber-500/10 border-amber-500/30"
        }`}
      >
        Helper service: <strong>{helper}</strong>
        {!ok && (
          <button
            disabled={busy}
            className="ml-3 px-2 h-7 rounded bg-gradient-to-br from-indigo-500 to-pink-500 text-xs disabled:opacity-50"
            onClick={fix}
          >
            {busy ? "Working..." : "Fix it"}
          </button>
        )}
        {err && <div className="mt-2 text-xs text-rose-300">{err}</div>}
      </div>
      <div className="flex gap-2 justify-end">
        <button
          disabled={!ok}
          className="px-4 h-9 rounded bg-gradient-to-br from-indigo-500 to-pink-500 disabled:opacity-50 text-sm font-medium"
          onClick={onNext}
        >
          Continue
        </button>
      </div>
    </div>
  );
}
