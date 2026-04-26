import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import type { ServerView } from "@/api/client";
import { ServerRow } from "./ServerRow";
import { TestLatency as wailsTestLatency } from "../../../wailsjs/go/bindings/ServersService";

// Wails generates TS signatures with a leading context.Context arg even
// though the runtime injects it transparently. Cast to drop it — the JS
// shim ignores the extra slot when the runtime fills it in.
const TestAll = wailsTestLatency as unknown as (id: string) => Promise<void>;

// ServerTable renders the searchable, scrollable server list with a sticky
// 5-column header. Sorting is deferred to a later task.
export function ServerTable({ servers }: { servers: ServerView[] }) {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase();
    if (!needle) return servers;
    return servers.filter((s) =>
      (s.name + s.country + s.origin + s.transport).toLowerCase().includes(needle),
    );
  }, [servers, q]);
  return (
    <div className="flex flex-col gap-3 h-full min-h-0">
      <div className="flex gap-2">
        <input
          className="flex-1 h-8 bg-white/[0.04] border border-white/10 rounded-md px-3 text-sm"
          placeholder={t("servers.search")}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <button
          className="px-3 h-8 rounded-md bg-white/[0.06] border border-white/10 text-sm"
          onClick={() => {
            void TestAll("");
          }}
        >
          {t("servers.testAll")}
        </button>
        <button
          className="px-3 h-8 rounded-md bg-gradient-to-br from-indigo-500 to-pink-500 text-sm opacity-50 cursor-not-allowed"
          disabled
          title="Manual server entry lands in C.T7"
        >
          {t("servers.addManual")}
        </button>
      </div>
      <div className="flex-1 min-h-0 bg-white/[0.02] border border-white/[0.06] rounded-lg overflow-auto">
        <div className="grid grid-cols-[1.2fr_1fr_0.7fr_0.6fr_0.6fr] gap-3 px-3 py-2 text-[10px] uppercase tracking-wider text-text-muted border-b border-white/[0.06]">
          <div>{t("servers.colName")}</div>
          <div>{t("servers.colTransport")}</div>
          <div>{t("servers.colLatency")}</div>
          <div>{t("servers.colOrigin")}</div>
          <div></div>
        </div>
        {filtered.map((s) => (
          <ServerRow key={s.id} s={s} />
        ))}
        {filtered.length === 0 && (
          <div className="px-3 py-8 text-center text-text-muted text-sm">
            {q ? t("servers.emptySearch") : t("servers.emptyAll")}
          </div>
        )}
      </div>
    </div>
  );
}
