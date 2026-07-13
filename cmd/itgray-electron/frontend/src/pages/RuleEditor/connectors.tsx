import { useTranslation } from "react-i18next";

export function AndConnector() {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-2.5 my-1">
      <div className="h-px flex-1 bg-gradient-to-r from-transparent via-amber-400/30 to-transparent" />
      <span className="rounded-full border border-amber-400/30 bg-amber-400/10 px-2.5 py-0.5 text-[10px] font-extrabold tracking-[0.18em] text-amber-300">
        {t("ruleEditor.and")}
      </span>
      <div className="h-px flex-1 bg-gradient-to-r from-transparent via-amber-400/30 to-transparent" />
    </div>
  );
}

export function ThenConnector() {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-2.5 mt-4 mb-3">
      <span className="text-[11px] font-extrabold tracking-[0.16em] text-sky-300">
        {t("ruleEditor.then")} →
      </span>
      <div className="h-px flex-1 bg-gradient-to-r from-sky-300/25 to-transparent" />
    </div>
  );
}
