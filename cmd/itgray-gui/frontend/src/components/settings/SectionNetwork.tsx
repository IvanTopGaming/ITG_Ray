import { useStore } from "@/store";
import { Update } from "../../../wailsjs/go/bindings/SettingsService";
import { NumberInput, Row, SectionShell, Select, TextInput } from "./primitives";

const fallback = {
  defaultMode: "auto",
  tunCidr: "198.18.0.1/15",
  tunName: "ITGRay-TUN",
  socksPort: 1080,
  xrayPort: 1081,
};

export function SectionNetwork() {
  const n = useStore((s) => s.settings?.network) ?? fallback;
  const save = (patch: Record<string, unknown>) => {
    void Update("network", patch);
  };
  return (
    <SectionShell id="network" title="Network">
      <Row label="Default mode" hint="Auto picks TUN when the helper is running">
        <Select
          value={n.defaultMode || "auto"}
          onChange={(v) => save({ defaultMode: v })}
          options={[
            { value: "auto", label: "Auto" },
            { value: "tun", label: "TUN" },
            { value: "sysproxy", label: "System Proxy" },
          ]}
        />
      </Row>
      <Row label="TUN CIDR" hint="Adapter address range, e.g. 198.18.0.1/15">
        <TextInput value={n.tunCidr || ""} onChange={(v) => save({ tunCidr: v })} placeholder="198.18.0.1/15" />
      </Row>
      <Row label="SOCKS port">
        <NumberInput value={n.socksPort || 0} min={1} max={65535} onChange={(v) => save({ socksPort: v })} />
      </Row>
      <Row label="HTTP/Xray port">
        <NumberInput value={n.xrayPort || 0} min={1} max={65535} onChange={(v) => save({ xrayPort: v })} />
      </Row>
    </SectionShell>
  );
}
