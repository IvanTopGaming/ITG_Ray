import { useStore } from "@/store";
import { Update } from "../../../wailsjs/go/bindings/SettingsService";
import { NumberInput, Row, SectionShell, TextInput } from "./primitives";

const fallback = {
  defaultUpdateInterval: 3600,
  userAgent: "ITG-Ray/0.1",
};

export function SectionSubs() {
  const s = useStore((st) => st.settings?.subscriptions) ?? fallback;
  const save = (patch: Record<string, unknown>) => {
    void Update("subscriptions", patch);
  };
  return (
    <SectionShell id="subs" title="Subscriptions">
      <Row label="Default update interval" hint="Seconds between auto-syncs for new subscriptions">
        <NumberInput
          value={s.defaultUpdateInterval || 3600}
          min={60}
          step={60}
          onChange={(v) => save({ defaultUpdateInterval: v })}
        />
      </Row>
      <Row label="User-Agent" hint="Sent on subscription HTTP fetches">
        <TextInput value={s.userAgent || ""} onChange={(v) => save({ userAgent: v })} placeholder="ITG-Ray/0.1" />
      </Row>
    </SectionShell>
  );
}
