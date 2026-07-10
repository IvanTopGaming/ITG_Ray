import React, { useState } from "react";
import { AnimatePresence, motion, type Variants } from "framer-motion";
import {
  AlertTriangle,
  Copy,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";
import { type Sub } from "@/lib/subsAdapter";
import { useSubs, humanizeError } from "@/lib/subsStore";
import type { TFunction } from "i18next";

type SyncStatus = "ok" | "error" | "syncing" | "never";

const DAY = 24 * 60 * 60 * 1000;

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.06, delayChildren: 0.05 },
  },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 10 },
  show: { opacity: 1, y: 0, transition: { duration: 0.35, ease: SNAP_EASE } },
};

type ModalState =
  | { kind: "closed" }
  | { kind: "add" }
  | { kind: "edit"; sub: Sub };

export function Subscriptions() {
  const { t } = useTranslation();
  const { state, actions } = useSubs();
  const [modal, setModal] = useState<ModalState>({ kind: "closed" });
  const subs = state.load.kind === "ready" ? state.load.subs : [];

  return (
    <>
      <motion.section
        variants={containerVariants}
        initial="hidden"
        animate="show"
        className="flex flex-col gap-5"
      >
        <motion.div
          variants={itemVariants}
          className="flex items-center justify-between gap-4"
        >
          <h1 className="text-[22px] font-semibold tracking-tight">
            {t("subscriptions.title")}
          </h1>
          <div className="flex items-center gap-2">
            <SyncAllButton
              onClick={() => void actions.syncAll()}
              disabled={subs.length === 0 || state.inFlight.syncing.size > 0}
            />
            <AddButton onClick={() => setModal({ kind: "add" })} />
          </div>
        </motion.div>

        {state.load.kind === "loading" ? (
          <motion.div
            variants={itemVariants}
            className="flex flex-col gap-3"
          >
            {[0, 1, 2].map((i) => (
              <div
                key={i}
                className="glass-regular h-24 animate-pulse rounded-2xl"
              />
            ))}
          </motion.div>
        ) : state.load.kind === "error" ? (
          <motion.div
            variants={itemVariants}
            className="glass-regular flex items-center justify-between gap-4 rounded-2xl p-5"
          >
            <div>
              <div className="text-[14px] font-medium text-white">
                {t("subscriptions.failedToLoad")}
              </div>
              <div className="text-[12px] text-white/60">{state.load.message}</div>
            </div>
            <button
              type="button"
              onClick={() => void actions.refresh()}
              className="glass-regular rounded-full px-4 py-1.5 text-[12px] text-white hover:bg-white/10"
            >
              {t("subscriptions.retry")}
            </button>
          </motion.div>
        ) : state.load.subs.length === 0 ? (
          <motion.div
            variants={itemVariants}
            className="glass-regular rounded-2xl p-10 text-center text-[13px] text-white/55"
          >
            {t("subscriptions.emptyBefore")}<span className="text-white">{t("subscriptions.emptyAddLink")}</span>{t("subscriptions.emptyAfter")}
          </motion.div>
        ) : (
          <AnimatePresence mode="popLayout">
            {state.load.subs.map((sub) => (
              <motion.div
                key={sub.id}
                layout
                variants={itemVariants}
                exit={{
                  opacity: 0,
                  x: -24,
                  scale: 0.96,
                  transition: { duration: 0.26, ease: SNAP_EASE },
                }}
              >
                <SubCard
                  sub={sub}
                  onSync={() => void actions.syncOne(sub.id)}
                  onEdit={() => setModal({ kind: "edit", sub })}
                />
              </motion.div>
            ))}
          </AnimatePresence>
        )}
      </motion.section>

      <AnimatePresence>
        {modal.kind !== "closed" && (
          <SubModal
            modal={modal}
            onClose={() => setModal({ kind: "closed" })}
            onAdd={actions.add}
            onEditSave={actions.edit}
            onDelete={actions.remove}
          />
        )}
      </AnimatePresence>
    </>
  );
}

function SubCard({
  sub,
  onSync,
  onEdit,
}: {
  sub: Sub;
  onSync: () => void;
  onEdit: () => void;
}) {
  const { t } = useTranslation();
  const isError = sub.status === "error";
  const isSyncing = sub.status === "syncing";

  return (
    <div
      className={cn(
        "glass-regular relative flex items-start gap-5 rounded-2xl p-5 transition-colors duration-standard ease-snap",
        isError && "!border-danger/35 bg-danger/[0.04]",
      )}
    >
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[15px] font-semibold">{sub.name}</span>
          <StatusBadge status={sub.status} />
          {sub.serverCount > 0 && (
            <span className="font-mono text-[10px] tabular-nums text-white/45">
              {t("subscriptions.serverCount", { count: sub.serverCount })}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <span className="truncate font-mono text-[11px] text-white/55">
            {sub.url}
          </span>
          <button
            onClick={() => void navigator.clipboard.writeText(sub.url)}
            className="shrink-0 rounded p-1 text-white/35 transition-colors hover:bg-white/[0.06] hover:text-white"
            title={t("subscriptions.copyUrl")}
          >
            <Copy className="h-3 w-3" />
          </button>
        </div>

        {isError && sub.lastSyncMessage && (
          <div className="mt-1 flex items-center gap-1.5 text-[11px] text-[#ff9a9a]">
            <AlertTriangle className="h-3.5 w-3.5" />
            {sub.lastSyncMessage}
          </div>
        )}

        <SubStats sub={sub} />

        <div className="mt-1 text-[10px] font-medium uppercase tracking-[0.14em] text-white/40">
          {sub.lastSyncAt
            ? t("subscriptions.syncedRelative", { time: formatRelative(sub.lastSyncAt, t) })
            : t("subscriptions.neverSynced")}
        </div>
      </div>

      <div className="flex shrink-0 items-center gap-2">
        {isError ? (
          <RetryButton onClick={onSync} disabled={isSyncing} />
        ) : (
          <SyncButton
            onClick={onSync}
            disabled={isSyncing}
            syncing={isSyncing}
          />
        )}
        <button
          onClick={onEdit}
          aria-label={t("common.edit")}
          className="rounded-lg border border-white/15 bg-white/[0.04] p-2 text-white/65 transition-colors hover:bg-white/[0.08] hover:text-white"
          title={t("common.edit")}
        >
          <Pencil className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}

function SubStats({ sub }: { sub: Sub }) {
  const { t } = useTranslation();
  const hasTraffic = sub.total != null;
  const hasExpiry = sub.expire != null;
  if (!hasTraffic && !hasExpiry) return null;

  const used = (sub.upload ?? 0) + (sub.download ?? 0);
  const total = sub.total ?? 0;
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0;
  const trafficColor =
    pct >= 95 ? "#ff5e5e" : pct >= 80 ? "#ffb13c" : "#00e892";

  return (
    <div className="mt-2 flex flex-col gap-2.5 border-t border-white/[0.06] pt-3">
      {hasTraffic && (
        <div className="flex flex-col gap-1.5">
          <div className="flex items-baseline justify-between gap-3 text-[11px]">
            <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-white/45">
              {t("subscriptions.traffic")}
            </span>
            <div className="flex items-baseline gap-2">
              <span className="font-mono tabular-nums text-white/85">
                {formatBytes(used)}
                <span className="text-white/40"> / {formatBytes(total)}</span>
              </span>
              <span
                className="font-mono text-[10px] font-semibold tabular-nums"
                style={{ color: trafficColor }}
              >
                {pct.toFixed(0)}%
              </span>
            </div>
          </div>
          <div className="h-1.5 w-full overflow-hidden rounded-full bg-white/[0.06]">
            <motion.div
              key={`${sub.id}-${used}`}
              initial={{ width: 0 }}
              animate={{ width: `${pct}%` }}
              transition={{ duration: 0.7, ease: SNAP_EASE }}
              className="h-full rounded-full"
              style={{
                background: `linear-gradient(90deg, ${trafficColor}aa, ${trafficColor})`,
                boxShadow: `0 0 8px ${trafficColor}40`,
              }}
            />
          </div>
        </div>
      )}

      {hasExpiry && (
        <div className="flex items-baseline justify-between gap-3 text-[11px]">
          <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-white/45">
            {t("subscriptions.expires")}
          </span>
          {(() => {
            const e = formatExpiry(sub.expire!, t);
            return (
              <div className="flex items-baseline gap-2">
                <span
                  className={cn("font-mono font-semibold tabular-nums", e.cls)}
                >
                  {e.text}
                </span>
                <span className="font-mono text-[10px] tabular-nums text-white/40">
                  {new Date(sub.expire!).toLocaleDateString("en-CA")}
                </span>
              </div>
            );
          })()}
        </div>
      )}
    </div>
  );
}

function formatBytes(b: number): string {
  if (b < 1024) return `${b.toFixed(0)} B`;
  if (b < 1024 ** 2) return `${(b / 1024).toFixed(0)} KB`;
  if (b < 1024 ** 3) return `${(b / 1024 ** 2).toFixed(1)} MB`;
  return `${(b / 1024 ** 3).toFixed(1)} GB`;
}

function formatExpiry(epochMs: number, t: TFunction): { text: string; cls: string } {
  const diff = epochMs - Date.now();
  if (diff <= 0) return { text: t("subscriptions.expired"), cls: "text-danger" };
  const days = Math.ceil(diff / DAY);
  const text = t("subscriptions.expiresInDays", { days });
  if (days <= 2) return { text, cls: "text-danger" };
  if (days <= 7) return { text, cls: "text-warn" };
  if (days <= 30) return { text, cls: "text-white/85" };
  return { text, cls: "text-white/65" };
}

function StatusBadge({ status }: { status: SyncStatus }) {
  const { t } = useTranslation();
  const cfg = {
    ok: {
      label: t("subscriptions.status.ok"),
      cls: "bg-success/15 text-success border-success/30",
    },
    error: {
      label: t("subscriptions.status.error"),
      cls: "bg-danger/15 text-[#ff9a9a] border-danger/30",
    },
    syncing: {
      label: t("subscriptions.status.syncing"),
      cls: "bg-warn/15 text-warn border-warn/30",
    },
    never: {
      label: t("subscriptions.status.never"),
      cls: "bg-white/[0.08] text-white/55 border-white/15",
    },
  }[status];
  return (
    <span
      className={cn(
        "rounded border px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.16em]",
        cfg.cls,
      )}
    >
      {cfg.label}
    </span>
  );
}

function SyncButton({
  onClick,
  disabled,
  syncing,
}: {
  onClick: () => void;
  disabled?: boolean;
  syncing?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      aria-label={t("subscriptions.sync")}
      className={cn(
        "flex items-center gap-1.5 rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 text-[12px] font-medium text-white transition-colors duration-instant ease-snap",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:border-white/25 hover:bg-white/[0.08]",
      )}
    >
      <RefreshCw
        className={cn("h-3.5 w-3.5", syncing && "animate-spin")}
      />
      {syncing ? t("subscriptions.syncing") : t("subscriptions.sync")}
    </button>
  );
}

function RetryButton({
  onClick,
  disabled,
}: {
  onClick: () => void;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      aria-label={t("subscriptions.retrySync")}
      className={cn(
        "flex items-center gap-1.5 rounded-lg border border-danger/40 bg-danger/[0.10] px-3 py-2 text-[12px] font-semibold text-[#ff9a9a] transition-colors duration-instant ease-snap",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:bg-danger/[0.18]",
      )}
    >
      <RefreshCw className="h-3.5 w-3.5" />
      {t("subscriptions.retry")}
    </button>
  );
}

function SyncAllButton({
  onClick,
  disabled,
}: {
  onClick: () => void;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      aria-label={t("subscriptions.syncAll")}
      className={cn(
        "flex items-center gap-1.5 rounded-lg border border-white/15 bg-white/[0.04] px-3 py-1.5 text-[11px] font-medium text-white transition-colors duration-instant ease-snap",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:border-white/25 hover:bg-white/[0.08]",
      )}
    >
      <RefreshCw className="h-3.5 w-3.5" />
      {t("subscriptions.syncAll")}
    </button>
  );
}

function AddButton({ onClick }: { onClick: () => void }) {
  const { t } = useTranslation();
  return (
    <motion.button
      onClick={onClick}
      whileHover={{ y: -1 }}
      whileTap={{ scale: 0.96 }}
      transition={{ duration: 0.15, ease: SNAP_EASE }}
      className="flex items-center gap-1.5 rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-3 py-1.5 text-[11px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-shadow duration-instant ease-snap hover:shadow-[0_0_22px_rgba(120,200,255,0.45)]"
    >
      <Plus className="h-3.5 w-3.5" />
      {t("subscriptions.addSubscription")}
    </motion.button>
  );
}

function SubModal({
  modal,
  onClose,
  onAdd,
  onEditSave,
  onDelete,
}: {
  modal: Exclude<ModalState, { kind: "closed" }>;
  onClose: () => void;
  onAdd: (name: string, url: string, userAgent: string) => Promise<void>;
  onEditSave: (id: string, name: string, url: string, userAgent: string) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}) {
  const { t } = useTranslation();
  const isAdd = modal.kind === "add";
  const sub = !isAdd ? modal.sub : null;
  const [name, setName] = useState(sub?.name ?? "");
  const [url, setUrl] = useState(sub?.url ?? "");
  const [userAgent, setUserAgent] = useState(sub?.userAgent ?? "");
  const [urlError, setUrlError] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function validateUrl(value: string): string | null {
    if (!value.trim()) return t("subscriptions.required");
    if (!/^https?:\/\//i.test(value.trim())) {
      return t("subscriptions.mustStartWith");
    }
    return null;
  }

  const valid = !!name.trim() && !!url.trim() && validateUrl(url) === null;

  async function submit() {
    const e = validateUrl(url);
    if (e) {
      setUrlError(e);
      return;
    }
    if (!valid) return;
    setSubmitError(null);
    setBusy(true);
    try {
      if (isAdd) await onAdd(name.trim(), url.trim(), userAgent.trim());
      else if (sub) await onEditSave(sub.id, name.trim(), url.trim(), userAgent.trim());
      onClose();
    } catch (err) {
      setSubmitError(humanizeError(err));
      setBusy(false);
    }
  }

  async function handleDeleteClick() {
    if (!sub) return;
    setSubmitError(null);
    setBusy(true);
    try {
      await onDelete(sub.id);
      onClose();
    } catch (err) {
      setSubmitError(humanizeError(err));
      setBusy(false);
    }
  }

  const title = isAdd ? t("subscriptions.addTitle") : t("subscriptions.editTitle");

  return (
    <motion.div
      // initial={false} skips the enter animation — see Servers.tsx ServerModal.
      initial={false}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.18, ease: SNAP_EASE }}
      className="fixed inset-0 z-50 flex items-center justify-center"
    >
      <button
        onClick={onClose}
        aria-label={t("common.close")}
        className="absolute inset-0 cursor-default bg-bg-0/70 backdrop-blur-md"
      />
      <motion.div
        className="glass-elevated relative z-10 flex w-[500px] flex-col rounded-2xl"
        initial={{ scale: 0.96, y: 8 }}
        animate={{ scale: 1, y: 0 }}
        exit={{ scale: 0.96, y: 8 }}
        transition={{ duration: 0.22, ease: SNAP_EASE }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-white/[0.08] px-6 py-5">
          <h2 className="text-[16px] font-semibold tracking-tight">{title}</h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1 text-white/55 transition-colors hover:bg-white/[0.06] hover:text-white"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="flex flex-col gap-4 px-6 py-5">
          <Field label={t("subscriptions.name")}>
            <input
              type="text"
              value={name}
              onChange={(e) => { setName(e.target.value); setSubmitError(null); }}
              placeholder={t("subscriptions.namePlaceholder")}
              className="w-full rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 text-[13px] text-white placeholder:text-white/35 focus:border-accent-start/50 focus:bg-white/[0.06] focus:outline-none"
            />
          </Field>
          <Field label={t("subscriptions.urlLabel")} error={urlError}>
            <input
              type="text"
              value={url}
              onChange={(e) => {
                setUrl(e.target.value);
                setUrlError(null);
                setSubmitError(null);
              }}
              placeholder={t("subscriptions.urlPlaceholder")}
              className={cn(
                "w-full rounded-lg border bg-white/[0.04] px-3 py-2 font-mono text-[11px] text-white placeholder:text-white/35 focus:bg-white/[0.06] focus:outline-none",
                urlError
                  ? "border-danger/40 focus:border-danger/60"
                  : "border-white/15 focus:border-accent-start/50",
              )}
            />
          </Field>
          <Field label={t("subscriptions.userAgentLabel")}>
            <input
              type="text"
              value={userAgent}
              onChange={(e) => { setUserAgent(e.target.value); setSubmitError(null); }}
              placeholder={t("subscriptions.userAgentPlaceholder")}
              className="w-full rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 font-mono text-[11px] text-white placeholder:text-white/35 focus:border-accent-start/50 focus:bg-white/[0.06] focus:outline-none"
            />
          </Field>
          <div className="rounded-lg border border-white/[0.08] bg-white/[0.02] p-3 text-[11px] text-white/55">
            {t("subscriptions.saveNote")}
          </div>
        </div>

        <AnimatePresence>
          {submitError && (
            <motion.div
              key="submit-error"
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.22, ease: SNAP_EASE }}
              className="overflow-hidden"
            >
              <div className="mx-6 mb-4 flex items-start gap-2 rounded-lg border border-danger/40 bg-danger/[0.10] p-3">
                <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-danger" />
                <span className="text-[12px] text-danger">{submitError}</span>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        <div className="flex items-center justify-between gap-3 border-t border-white/[0.08] px-6 py-4">
          {!isAdd && sub ? (
            <button
              onClick={handleDeleteClick}
              disabled={busy}
              className="flex items-center gap-1.5 rounded-lg border border-danger/40 bg-danger/[0.10] px-3 py-2 text-[12px] font-medium text-danger transition-colors hover:bg-danger/[0.20] disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Trash2 className="h-3.5 w-3.5" />
              {t("common.delete")}
            </button>
          ) : (
            <span />
          )}
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="rounded-lg px-4 py-2 text-[12px] font-medium text-white/65 transition-colors hover:bg-white/[0.06] hover:text-white"
            >
              {t("common.cancel")}
            </button>
            <button
              onClick={submit}
              disabled={!valid || busy}
              className="rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-4 py-2 text-[12px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-all hover:shadow-[0_0_22px_rgba(120,200,255,0.45)] disabled:opacity-40 disabled:shadow-none"
            >
              {isAdd ? t("common.add") : t("common.save")}
            </button>
          </div>
        </div>
      </motion.div>
    </motion.div>
  );
}

function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string | null;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="text-[10px] font-medium uppercase tracking-[0.18em] text-white/45">
        {label}
      </span>
      {children}
      {error && <span className="text-[10px] text-danger">{error}</span>}
    </label>
  );
}

function formatRelative(epoch: number, t: TFunction): string {
  const diff = Date.now() - epoch;
  if (diff < 10_000) return t("subscriptions.justNow");
  const min = Math.floor(diff / 60_000);
  if (min < 60) return t("subscriptions.minAgo", { n: min });
  const h = Math.floor(diff / 3_600_000);
  if (h < 24) return t("subscriptions.hoursAgo", { n: h });
  const d = Math.floor(diff / 86_400_000);
  return t("subscriptions.daysAgo", { n: d });
}
