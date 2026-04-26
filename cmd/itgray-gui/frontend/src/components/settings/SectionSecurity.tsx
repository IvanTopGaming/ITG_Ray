import { useStore } from "@/store";
import { Row, SectionShell } from "./primitives";

// SectionSecurity is read-only in v0.1: the detection wiring lands once
// internal/secret ships (v0.2 follow-up). Until then the backend always
// returns Method:"Unknown" / Available:false / a warning string — the UI
// surfaces those verbatim so a future detection change lights up here
// without a frontend rebuild.

const fallback = { method: "Unknown", available: false, warning: "" };

export function SectionSecurity() {
  const s = useStore((st) => st.settings?.security) ?? fallback;
  return (
    <SectionShell id="security" title="Security">
      <Row label="Secret protection method" hint="DPAPI on Windows, Keychain on macOS, SecretService on Linux">
        <span className="text-sm text-text-primary font-mono">{s.method || "Unknown"}</span>
      </Row>
      <Row label="Available">
        <span
          className={`text-xs px-2 py-0.5 rounded-md ${
            s.available ? "bg-emerald-500/15 text-emerald-300" : "bg-amber-500/15 text-amber-300"
          }`}
        >
          {s.available ? "yes" : "no"}
        </span>
      </Row>
      {s.warning ? (
        <div className="text-xs text-amber-300/90 bg-amber-500/10 border border-amber-500/20 rounded-md px-2 py-1.5">
          {s.warning}
        </div>
      ) : null}
    </SectionShell>
  );
}
