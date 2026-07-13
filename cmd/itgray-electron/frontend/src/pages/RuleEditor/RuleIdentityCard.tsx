import { useTranslation } from "react-i18next";

export function RuleIdentityCard({
  name,
  enabled,
  groupId,
  groups,
  onName,
  onEnabled,
  onGroup,
}: {
  name: string;
  enabled: boolean;
  groupId: string;
  groups: { id: string; name: string }[];
  onName: (s: string) => void;
  onEnabled: (b: boolean) => void;
  onGroup: (id: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="rounded-2xl border border-white/[0.09] bg-white/[0.035] p-4 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <input
          aria-label={t('ruleEditor.name')}
          value={name}
          onChange={(e) => onName(e.target.value)}
          placeholder={t('ruleEditor.namePlaceholder')}
          className="min-w-0 flex-1 border-none bg-transparent text-[19px] font-semibold text-white placeholder:text-white/25 focus:outline-none"
        />
        <button
          type="button"
          aria-pressed={enabled}
          onClick={() => onEnabled(!enabled)}
          className={
            "shrink-0 rounded-full border px-3 py-1 text-[10px] font-bold uppercase tracking-[0.1em] transition-colors duration-200 " +
            (enabled
              ? "border-emerald-400/40 bg-emerald-400/10 text-emerald-300"
              : "border-white/15 bg-white/[0.03] text-white/40")
          }
        >
          ● {t('ruleEditor.enabled')}
        </button>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2.5 border-t border-white/[0.07] pt-3">
        <span className="text-[10px] font-medium uppercase tracking-[0.12em] text-white/40">
          {t('ruleEditor.group')}
        </span>
        <select
          value={groupId}
          onChange={(e) => onGroup(e.target.value)}
          className="rounded-full border border-white/15 bg-black/30 px-3 py-1 text-[12px] text-white/90 focus:border-sky-400/50 focus:outline-none"
        >
          {groups.map((g) => (
            <option key={g.id} value={g.id} className="bg-[#150a3d] text-white">
              {g.name}
            </option>
          ))}
        </select>
        <span className="text-[11px] italic text-white/45">{t('ruleEditor.orHint')}</span>
      </div>
    </div>
  );
}
