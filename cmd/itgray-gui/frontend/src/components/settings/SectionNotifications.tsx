import { useStore } from "@/store";
import { Update } from "../../../wailsjs/go/bindings/SettingsService";
import { Row, SectionShell, Toggle } from "./primitives";

const fallback = {
  onConnected: true,
  onDisconnected: true,
  onError: true,
  onSubSynced: false,
};

export function SectionNotifications() {
  const n = useStore((s) => s.settings?.notifications) ?? fallback;
  const save = (patch: Record<string, unknown>) => {
    void Update(undefined as never, "notifications", patch);
  };
  return (
    <SectionShell id="notifications" title="Notifications">
      <Row label="On connected">
        <Toggle value={!!n.onConnected} onChange={(v) => save({ onConnected: v })} ariaLabel="Notify on connect" />
      </Row>
      <Row label="On disconnected">
        <Toggle
          value={!!n.onDisconnected}
          onChange={(v) => save({ onDisconnected: v })}
          ariaLabel="Notify on disconnect"
        />
      </Row>
      <Row label="On error">
        <Toggle value={!!n.onError} onChange={(v) => save({ onError: v })} ariaLabel="Notify on error" />
      </Row>
      <Row label="On subscription sync">
        <Toggle
          value={!!n.onSubSynced}
          onChange={(v) => save({ onSubSynced: v })}
          ariaLabel="Notify on subscription sync"
        />
      </Row>
    </SectionShell>
  );
}
