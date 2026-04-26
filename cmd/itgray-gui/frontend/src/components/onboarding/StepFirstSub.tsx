import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Add as wailsAdd } from "../../../wailsjs/go/bindings/SubsService";
import type { SubView } from "@/api/client";

// Wails generates TS signatures with a leading context.Context arg the
// runtime injects transparently. Cast to clean shape for readability —
// matches the same pattern used in AddSubDialog.tsx.
const Add = wailsAdd as unknown as (url: string, name: string) => Promise<SubView>;

// StepFirstSub asks for one subscription URL + optional friendly name and
// imports it via the existing SubsService.Add binding. Skip and Import
// callbacks bubble up to the Wizard which writes the .onboarded marker.
export function StepFirstSub({
  onDone,
  onSkip,
}: {
  onDone: () => void;
  onSkip: () => void;
}) {
  const { t } = useTranslation();
  const [url, setUrl] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async () => {
    setBusy(true);
    setErr(null);
    try {
      await Add(url.trim(), name.trim());
      onDone();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-semibold">{t("onboarding.addSubTitle")}</h2>
      <p className="text-text-secondary text-sm">{t("onboarding.addSubTagline")}</p>
      <input
        className="w-full h-9 bg-white/5 border border-white/10 rounded px-3 text-sm"
        placeholder={t("onboarding.subUrlPlaceholder")}
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        autoFocus
      />
      <input
        className="w-full h-9 bg-white/5 border border-white/10 rounded px-3 text-sm"
        placeholder={t("onboarding.friendlyNamePlaceholder")}
        value={name}
        onChange={(e) => setName(e.target.value)}
      />
      {err && <div className="text-xs text-rose-400">{err}</div>}
      <div className="flex gap-2 justify-end">
        <button
          disabled={busy}
          className="px-4 h-9 rounded bg-white/5 border border-white/10 text-sm disabled:opacity-50"
          onClick={onSkip}
        >
          {t("onboarding.skip")}
        </button>
        <button
          disabled={!url.trim() || busy}
          className="px-4 h-9 rounded bg-gradient-to-br from-indigo-500 to-pink-500 text-sm font-medium disabled:opacity-50"
          onClick={submit}
        >
          {busy ? t("onboarding.importing") : t("onboarding.import")}
        </button>
      </div>
    </div>
  );
}
