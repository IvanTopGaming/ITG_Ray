import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "@/store";
import { SubCard } from "@/components/sub-card/SubCard";
import { AddSubDialog, syncAllSubs } from "@/components/sub-card/AddSubDialog";

// SubsPage renders the subscription card grid + toolbar. C.T7 enables the
// Sync-all and Add buttons; the Filter input remains a UX placeholder until
// a future task wires client-side filtering. The AddSubDialog is mounted
// permanently and self-collapses when open=false so we don't need a portal.
export function SubsPage() {
  const { t } = useTranslation();
  const subs = useStore((s) => s.subs);
  const [open, setOpen] = useState(false);
  return (
    <div className="flex flex-col gap-3 h-full min-h-0">
      <div className="flex gap-2">
        <input
          className="flex-1 h-8 bg-white/[0.04] border border-white/10 rounded-md px-3 text-sm"
          placeholder={t("subs.filter")}
        />
        <button
          className="px-3 h-8 rounded-md bg-white/[0.06] border border-white/10 text-sm"
          onClick={() => void syncAllSubs()}
        >
          {t("subs.syncAll")}
        </button>
        <button
          className="px-3 h-8 rounded-md bg-gradient-to-br from-indigo-500 to-pink-500 text-sm"
          onClick={() => setOpen(true)}
        >
          {t("subs.addBtn")}
        </button>
      </div>
      <div className="flex-1 min-h-0 overflow-auto">
        {subs.length === 0 ? (
          <div className="px-3 py-8 text-center text-text-muted text-sm">
            {t("subs.empty")}
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-3 auto-rows-min">
            {subs.map((s) => (
              <SubCard key={s.id} s={s} />
            ))}
          </div>
        )}
      </div>
      <AddSubDialog open={open} onClose={() => setOpen(false)} />
    </div>
  );
}
