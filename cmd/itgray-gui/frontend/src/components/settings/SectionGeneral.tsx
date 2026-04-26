import i18n from "@/i18n";
import { useStore } from "@/store";
import { Update } from "../../../wailsjs/go/bindings/SettingsService";
import { NumberInput as _NumberInput, Row, SectionShell, Select, Toggle } from "./primitives";

// SectionGeneral edits language / autostart / tray behaviour. The patch
// is fire-and-forget: the hub.EventSettings event will refresh the store
// once C.T15 wires settings:changed. Until then the local optimistic
// update via the returned SettingsView is enough.
//
// _NumberInput is deliberately imported (and aliased to _) only to keep
// vite's tree-shaker honest if we add the autostart-delay knob later
// without re-touching this file. Strip if it ever bites.
void _NumberInput;

const fallback = {
  language: "auto",
  theme: "dark",
  autostart: false,
  closeToTray: true,
  startMinimized: false,
};

export function SectionGeneral() {
  const g = useStore((s) => s.settings?.general) ?? fallback;
  const save = (patch: Record<string, unknown>) => {
    void Update("general", patch);
  };
  return (
    <SectionShell id="general" title="General">
      <Row label="Language">
        <Select
          value={g.language || "auto"}
          onChange={(v) => {
            // Persist the user's choice via the SettingsService binding so it
            // survives restart, and live-switch the running UI: "auto" resets
            // to the language detector's pick, otherwise force the explicit
            // locale. The detector's localStorage cache stays in sync because
            // changeLanguage updates it.
            save({ language: v });
            if (v === "auto") {
              const detected =
                (typeof navigator !== "undefined" && navigator.language?.slice(0, 2)) || "en";
              void i18n.changeLanguage(detected);
            } else {
              void i18n.changeLanguage(v);
            }
          }}
          options={[
            { value: "auto", label: "Auto" },
            { value: "en", label: "English" },
            { value: "ru", label: "Русский" },
          ]}
        />
      </Row>
      <Row label="Close to tray" hint="Minimise to tray instead of quitting">
        <Toggle value={!!g.closeToTray} onChange={(v) => save({ closeToTray: v })} ariaLabel="Close to tray" />
      </Row>
      <Row label="Autostart" hint="Launch ITG Ray at login">
        <Toggle value={!!g.autostart} onChange={(v) => save({ autostart: v })} ariaLabel="Autostart" />
      </Row>
      <Row label="Start minimized">
        <Toggle value={!!g.startMinimized} onChange={(v) => save({ startMinimized: v })} ariaLabel="Start minimized" />
      </Row>
    </SectionShell>
  );
}
