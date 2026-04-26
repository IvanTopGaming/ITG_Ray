import type { ServerView } from "@/api/client";

// ServerPicker is a small native <select> dropdown — every server is one
// option, current server is preselected via value. Latency is appended in
// parens; servers that have never been probed show an em-dash placeholder.
export function ServerPicker({
  servers,
  value,
  onChange,
}: {
  servers: ServerView[];
  value: string | null;
  onChange: (id: string) => void;
}) {
  return (
    <select
      className="bg-white/[0.06] border border-white/10 rounded-full px-3 py-1 text-sm text-text-primary"
      value={value ?? ""}
      onChange={(e) => onChange(e.target.value)}
    >
      <option value="" disabled>
        Select server…
      </option>
      {servers.map((s) => (
        <option key={s.id} value={s.id}>
          {s.name} ({s.latencyMs ? `${s.latencyMs}ms` : "—"})
        </option>
      ))}
    </select>
  );
}
