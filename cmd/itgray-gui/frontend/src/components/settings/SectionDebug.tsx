import { useStore } from "@/store";
import { Update } from "../../../wailsjs/go/bindings/SettingsService";
import { Row, SectionShell, Select } from "./primitives";

const fallback = { logLevel: "info" };

export function SectionDebug() {
  const d = useStore((s) => s.settings?.debug) ?? fallback;
  const save = (patch: Record<string, unknown>) => {
    void Update(undefined as never, "debug", patch);
  };
  return (
    <SectionShell id="debug" title="Debug">
      <Row label="Log level" hint="Verbosity for the GUI process and helper">
        <Select
          value={d.logLevel || "info"}
          onChange={(v) => save({ logLevel: v })}
          options={[
            { value: "debug", label: "Debug" },
            { value: "info", label: "Info" },
            { value: "warn", label: "Warning" },
            { value: "error", label: "Error" },
          ]}
        />
      </Row>
      <Row label="Open logs folder" hint="Reveals %AppData%/ITG Ray/logs in the OS file manager">
        <button
          type="button"
          className="text-sm px-3 py-1 rounded-md bg-white/5 border border-white/10 text-text-primary hover:bg-white/10"
          onClick={() => {
            // C.T13 will wire BrowserOpenURL to the logs path; for now
            // surface a placeholder so the affordance exists.
            // eslint-disable-next-line no-console
            console.info("settings: open logs folder requested");
          }}
        >
          Open
        </button>
      </Row>
    </SectionShell>
  );
}
