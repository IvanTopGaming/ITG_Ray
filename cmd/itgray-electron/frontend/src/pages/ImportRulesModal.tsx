import { useEffect, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { AlertTriangle, X } from "lucide-react";
import { rulesImportPreview, rulesImportApply, type ImportPreview } from "@/lib/rulesStore";
import { summarise } from "@/lib/ruleSummary";
import { cn } from "@/lib/cn";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

function actionStyle(action: string): string {
  if (action === "proxy") return "bg-sky-500/20 text-sky-200";
  if (action === "direct") return "bg-amber-500/20 text-amber-200";
  return "bg-rose-500/20 text-rose-200";
}

type Props = { open: boolean; initialLink?: string; onClose: () => void };

function mapImportError(message: string, t: TFunction): string {
  if (message.includes("ruleshare: malformed link")) return t("routing.importErrorMalformed");
  if (message.includes("ruleshare: unsupported schema version")) return t("routing.importErrorVersion");
  if (message.includes("ruleshare: payload too large")) return t("routing.importErrorTooLarge");
  if (message.includes("ruleshare: no rules in payload")) return t("routing.importErrorEmpty");
  return t("routing.importErrorGeneric");
}

export function ImportRulesModal({ open, initialLink, onClose }: Props) {
  const { t } = useTranslation();
  const [link, setLink] = useState(initialLink ?? "");
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLink(initialLink ?? "");
    setPreview(null);
    setError(null);
  }, [open, initialLink]);

  useEffect(() => {
    if (open && initialLink) void doPreview(initialLink);
  }, [open, initialLink]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [open, onClose]);

  async function doPreview(l: string) {
    setError(null);
    setBusy(true);
    try {
      setPreview(await rulesImportPreview(l));
    } catch (err) {
      setPreview(null);
      setError(mapImportError(err instanceof Error ? err.message : String(err), t));
    } finally {
      setBusy(false);
    }
  }

  async function doApply() {
    setBusy(true);
    try {
      await rulesImportApply(link);
      onClose();
    } catch (err) {
      setError(mapImportError(err instanceof Error ? err.message : String(err), t));
    } finally {
      setBusy(false);
    }
  }

  const hasWarning = !!preview && (preview.directCount > 0 || preview.blockCount > 0);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={false}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.18, ease: SNAP_EASE }}
          className="fixed inset-0 z-50 flex items-center justify-center"
          role="dialog"
          aria-modal="true"
          aria-label={t("routing.importTitle")}
        >
          <button
            type="button"
            onClick={onClose}
            aria-label={t("common.close")}
            className="absolute inset-0 cursor-default bg-bg-0/70 backdrop-blur-md"
          />
          <motion.div
            initial={{ scale: 0.96, y: 8, opacity: 0 }}
            animate={{ scale: 1, y: 0, opacity: 1 }}
            exit={{ scale: 0.96, y: 8, opacity: 0 }}
            transition={{ duration: 0.22, ease: SNAP_EASE }}
            onClick={(e) => e.stopPropagation()}
            className="glass-elevated relative z-10 flex w-[480px] max-w-[90vw] flex-col rounded-2xl"
          >
            <div className="flex items-center justify-between border-b border-white/[0.08] px-6 py-5">
              <h2 className="text-[16px] font-semibold tracking-tight">{t("routing.importTitle")}</h2>
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg p-1 text-white/55 transition-colors hover:bg-white/[0.06] hover:text-white"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <div className="flex flex-col gap-4 px-6 py-5">
              <p className="text-[12px] text-white/55">{t("routing.importDescription")}</p>
              <div className="flex items-center gap-2">
                <input
                  value={link}
                  onChange={(e) => setLink(e.target.value)}
                  placeholder={t("routing.importPlaceholder")}
                  className="flex-1 rounded-md border border-white/10 bg-transparent px-2 py-1.5 text-[13px] outline-none focus:border-sky-400/40"
                />
                <button
                  type="button"
                  disabled={busy || !link.trim()}
                  onClick={() => void doPreview(link)}
                  className="rounded-md bg-sky-500/30 px-3 py-1.5 text-[12px] font-medium text-sky-100 hover:bg-sky-500/40 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  {t("routing.importPreviewBtn")}
                </button>
              </div>

              {error && (
                <div
                  role="alert"
                  className="flex items-start gap-2 rounded-md border border-rose-500/40 bg-rose-500/10 px-3 py-2 text-[12px] text-rose-200"
                >
                  <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  <span className="break-words">{error}</span>
                </div>
              )}

              {preview && (
                <div className="flex flex-col gap-2 rounded-lg border border-white/[0.08] bg-white/[0.03] p-3">
                  <div className="flex items-baseline justify-between gap-2">
                    <h3 className="truncate text-[14px] font-medium text-white/90">{preview.name || t("routing.importUnnamed")}</h3>
                    <span className="shrink-0 text-[11px] text-white/45">
                      {t("routing.importCounts", {
                        proxy: preview.proxyCount,
                        direct: preview.directCount,
                        block: preview.blockCount,
                      })}
                    </span>
                  </div>
                  <div className="flex max-h-60 flex-col gap-2 overflow-y-auto pr-1">
                    {preview.groups.map((g, gi) => (
                      <div key={gi} className="flex flex-col gap-1">
                        {preview.groups.length > 1 && (
                          <div className="px-0.5 text-[11px] font-medium uppercase tracking-wider text-white/40">{g.name}</div>
                        )}
                        {g.rules.map((r, ri) => (
                          <div key={ri} className="flex min-w-0 items-center gap-2.5 rounded-md bg-white/[0.03] px-2.5 py-1.5">
                            <span className={cn("shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider", actionStyle(r.action))}>
                              {r.action}
                            </span>
                            <span className="truncate text-[12px] text-white/85">{r.name || t("routing.importUnnamedRule")}</span>
                            <span className="ml-auto shrink-0 truncate text-[11px] text-white/45">{summarise(r.conditions, t)}</span>
                          </div>
                        ))}
                        {g.rules.length === 0 && (
                          <div className="px-2.5 py-1.5 text-[11px] text-white/35">{t("routing.importGroupEmpty")}</div>
                        )}
                      </div>
                    ))}
                  </div>
                  {hasWarning && (
                    <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-2.5 py-1.5 text-[11.5px] text-amber-200">
                      {t("routing.importWarning")}
                    </p>
                  )}
                </div>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-white/[0.08] px-6 py-4">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg px-4 py-2 text-[12px] font-medium text-white/65 transition-colors hover:bg-white/[0.06] hover:text-white"
              >
                {t("common.cancel")}
              </button>
              {preview && (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void doApply()}
                  className="rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-4 py-2 text-[12px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-all hover:shadow-[0_0_22px_rgba(120,200,255,0.45)] disabled:opacity-40 disabled:shadow-none"
                >
                  {t("routing.importApplyBtn")}
                </button>
              )}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
