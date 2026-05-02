import React, { useCallback, useEffect, useState } from "react";
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
import { cn } from "@/lib/cn";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { List as ListSubs } from "../../wailsjs/go/bindings/SubsService";
import { backendToFrontend, type Sub } from "@/lib/subsAdapter";

type SyncStatus = "ok" | "error" | "syncing" | "never";

const GB = 1024 * 1024 * 1024;
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

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; subs: Sub[] }
  | { kind: "error"; message: string };

export function Subscriptions() {
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });
  const [modal, setModal] = useState<ModalState>({ kind: "closed" });

  const refresh = useCallback(async () => {
    try {
      const views = await ListSubs();
      setLoad({ kind: "ready", subs: views.map(backendToFrontend) });
    } catch (err) {
      setLoad({ kind: "error", message: String(err) });
    }
  }, []);

  useEffect(() => {
    void refresh();
    // Capture the unsubscribe function returned by EventsOn (Wails v2.4+)
    // so cleanup only removes our own listener — EventsOff(name) clears
    // ALL listeners for the event, which would silently tear down any
    // future co-subscribers (planned tray badge / cross-page toasts).
    const off = EventsOn("sub:synced", () => {
      void refresh();
    });
    return off;
  }, [refresh]);

  // Convenience accessors so the existing mock mutation handlers below
  // (syncOne, syncAll, handleAdd, handleEditSave, handleDelete) keep
  // their `(prev) => prev.map(...)` form. Tier 3 will replace them with
  // real SubsService.SyncOne/Add/Remove calls.
  const subs = load.kind === "ready" ? load.subs : [];
  const setSubs = (next: Sub[] | ((prev: Sub[]) => Sub[])) => {
    setLoad((cur) => {
      if (cur.kind !== "ready") return cur;
      const updated =
        typeof next === "function" ? (next as (p: Sub[]) => Sub[])(cur.subs) : next;
      return { kind: "ready", subs: updated };
    });
  };

  function syncOne(id: string) {
    setSubs((prev) =>
      prev.map((s) => (s.id === id ? { ...s, status: "syncing" } : s)),
    );
    const delay = 900 + Math.random() * 1500;
    setTimeout(() => {
      setSubs((prev) =>
        prev.map((s) => {
          if (s.id !== id) return s;
          const success = Math.random() > 0.15;
          if (success) {
            return {
              ...s,
              status: "ok",
              lastSyncAt: Date.now(),
              serverCount: 6 + Math.floor(Math.random() * 8),
              lastSyncMessage: undefined,
              download:
                (s.download ?? 0) + Math.random() * 0.6 * GB,
              upload:
                (s.upload ?? 0) + Math.random() * 0.04 * GB,
            };
          }
          return {
            ...s,
            status: "error",
            lastSyncAt: Date.now(),
            lastSyncMessage: "Subscription endpoint not reachable",
          };
        }),
      );
    }, delay);
  }

  function syncAll() {
    subs.forEach((s, i) => setTimeout(() => syncOne(s.id), i * 150));
  }

  function handleAdd(name: string, url: string) {
    const sub: Sub = {
      id: `sub-${Date.now()}`,
      name,
      url,
      status: "never",
      lastSyncAt: null,
      serverCount: 0,
    };
    setSubs((prev) => [...prev, sub]);
    setModal({ kind: "closed" });
    setTimeout(() => syncOne(sub.id), 250);
  }

  function handleEditSave(id: string, name: string, url: string) {
    setSubs((prev) =>
      prev.map((s) => (s.id === id ? { ...s, name, url } : s)),
    );
    setModal({ kind: "closed" });
  }

  function handleDelete(id: string) {
    setSubs((prev) => prev.filter((s) => s.id !== id));
    setModal({ kind: "closed" });
  }

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
            Subscriptions
          </h1>
          <div className="flex items-center gap-2">
            <SyncAllButton
              onClick={syncAll}
              disabled={subs.length === 0}
            />
            <AddButton onClick={() => setModal({ kind: "add" })} />
          </div>
        </motion.div>

        {load.kind === "loading" ? (
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
        ) : load.kind === "error" ? (
          <motion.div
            variants={itemVariants}
            className="glass-regular flex items-center justify-between gap-4 rounded-2xl p-5"
          >
            <div>
              <div className="text-[14px] font-medium text-white">
                Failed to load subscriptions
              </div>
              <div className="text-[12px] text-white/60">{load.message}</div>
            </div>
            <button
              type="button"
              onClick={() => {
                setLoad({ kind: "loading" });
                void refresh();
              }}
              className="glass-regular rounded-full px-4 py-1.5 text-[12px] text-white hover:bg-white/10"
            >
              Retry
            </button>
          </motion.div>
        ) : load.subs.length === 0 ? (
          <motion.div
            variants={itemVariants}
            className="glass-regular rounded-2xl p-10 text-center text-[13px] text-white/55"
          >
            No subscriptions yet. Click <span className="text-white">+ Add subscription</span> to import a VLESS feed.
          </motion.div>
        ) : (
          <AnimatePresence mode="popLayout">
            {load.subs.map((sub) => (
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
                  onSync={() => syncOne(sub.id)}
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
            onAdd={handleAdd}
            onEditSave={handleEditSave}
            onDelete={handleDelete}
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
              {sub.serverCount} servers
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
            title="Copy URL"
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
            ? `Synced ${formatRelative(sub.lastSyncAt)}`
            : "Never synced"}
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
          className="rounded-lg border border-white/15 bg-white/[0.04] p-2 text-white/65 transition-colors hover:bg-white/[0.08] hover:text-white"
          title="Edit"
        >
          <Pencil className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  );
}

function SubStats({ sub }: { sub: Sub }) {
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
              Traffic
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
            Expires
          </span>
          {(() => {
            const e = formatExpiry(sub.expire!);
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

function formatExpiry(epochMs: number): { text: string; cls: string } {
  const diff = epochMs - Date.now();
  if (diff <= 0) return { text: "Expired", cls: "text-danger" };
  const days = Math.ceil(diff / DAY);
  if (days <= 2) return { text: `in ${days}d`, cls: "text-danger" };
  if (days <= 7) return { text: `in ${days}d`, cls: "text-warn" };
  if (days <= 30) return { text: `in ${days}d`, cls: "text-white/85" };
  return { text: `in ${days}d`, cls: "text-white/65" };
}

function StatusBadge({ status }: { status: SyncStatus }) {
  const cfg = {
    ok: {
      label: "OK",
      cls: "bg-success/15 text-success border-success/30",
    },
    error: {
      label: "ERROR",
      cls: "bg-danger/15 text-[#ff9a9a] border-danger/30",
    },
    syncing: {
      label: "SYNCING",
      cls: "bg-warn/15 text-warn border-warn/30",
    },
    never: {
      label: "PENDING",
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
  return (
    <button
      onClick={onClick}
      disabled={disabled}
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
      {syncing ? "Syncing…" : "Sync"}
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
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "flex items-center gap-1.5 rounded-lg border border-danger/40 bg-danger/[0.10] px-3 py-2 text-[12px] font-semibold text-[#ff9a9a] transition-colors duration-instant ease-snap",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:bg-danger/[0.18]",
      )}
    >
      <RefreshCw className="h-3.5 w-3.5" />
      Retry
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
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        "flex items-center gap-1.5 rounded-lg border border-white/15 bg-white/[0.04] px-3 py-1.5 text-[11px] font-medium text-white transition-colors duration-instant ease-snap",
        disabled
          ? "cursor-not-allowed opacity-60"
          : "hover:border-white/25 hover:bg-white/[0.08]",
      )}
    >
      <RefreshCw className="h-3.5 w-3.5" />
      Sync all
    </button>
  );
}

function AddButton({ onClick }: { onClick: () => void }) {
  return (
    <motion.button
      onClick={onClick}
      whileHover={{ y: -1 }}
      whileTap={{ scale: 0.96 }}
      transition={{ duration: 0.15, ease: SNAP_EASE }}
      className="flex items-center gap-1.5 rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-3 py-1.5 text-[11px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-shadow duration-instant ease-snap hover:shadow-[0_0_22px_rgba(120,200,255,0.45)]"
    >
      <Plus className="h-3.5 w-3.5" />
      Add subscription
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
  onAdd: (name: string, url: string) => void;
  onEditSave: (id: string, name: string, url: string) => void;
  onDelete: (id: string) => void;
}) {
  const isAdd = modal.kind === "add";
  const sub = !isAdd ? modal.sub : null;
  const [name, setName] = useState(sub?.name ?? "");
  const [url, setUrl] = useState(sub?.url ?? "");
  const [urlError, setUrlError] = useState<string | null>(null);

  function validateUrl(value: string): string | null {
    if (!value.trim()) return "Required";
    if (!/^https?:\/\//i.test(value.trim())) {
      return "Must start with http:// or https://";
    }
    return null;
  }

  const valid = !!name.trim() && !!url.trim() && validateUrl(url) === null;

  function submit() {
    const e = validateUrl(url);
    if (e) {
      setUrlError(e);
      return;
    }
    if (!valid) return;
    if (isAdd) onAdd(name.trim(), url.trim());
    else if (sub) onEditSave(sub.id, name.trim(), url.trim());
  }

  const title = isAdd ? "Add subscription" : "Edit subscription";

  return (
    <motion.div
      className="fixed inset-0 z-50 flex items-center justify-center"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.18, ease: SNAP_EASE }}
    >
      <button
        onClick={onClose}
        aria-label="Close"
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
          <Field label="Name">
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Main provider"
              className="w-full rounded-lg border border-white/15 bg-white/[0.04] px-3 py-2 text-[13px] text-white placeholder:text-white/35 focus:border-accent-start/50 focus:bg-white/[0.06] focus:outline-none"
            />
          </Field>
          <Field label="Subscription URL" error={urlError}>
            <input
              type="text"
              value={url}
              onChange={(e) => {
                setUrl(e.target.value);
                setUrlError(null);
              }}
              placeholder="https://provider.example/sub/your-token"
              className={cn(
                "w-full rounded-lg border bg-white/[0.04] px-3 py-2 font-mono text-[11px] text-white placeholder:text-white/35 focus:bg-white/[0.06] focus:outline-none",
                urlError
                  ? "border-danger/40 focus:border-danger/60"
                  : "border-white/15 focus:border-accent-start/50",
              )}
            />
          </Field>
          <div className="rounded-lg border border-white/[0.08] bg-white/[0.02] p-3 text-[11px] text-white/55">
            On save, the subscription is fetched, parsed (base64 / plaintext /
            sing-box JSON), and merged into your server list.
          </div>
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-white/[0.08] px-6 py-4">
          {!isAdd && sub ? (
            <button
              onClick={() => onDelete(sub.id)}
              className="flex items-center gap-1.5 rounded-lg border border-danger/40 bg-danger/[0.10] px-3 py-2 text-[12px] font-medium text-danger transition-colors hover:bg-danger/[0.20]"
            >
              <Trash2 className="h-3.5 w-3.5" />
              Delete
            </button>
          ) : (
            <span />
          )}
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="rounded-lg px-4 py-2 text-[12px] font-medium text-white/65 transition-colors hover:bg-white/[0.06] hover:text-white"
            >
              Cancel
            </button>
            <button
              onClick={submit}
              disabled={!valid}
              className="rounded-lg bg-gradient-to-br from-accent-start to-accent-mid px-4 py-2 text-[12px] font-semibold text-white shadow-[0_0_18px_rgba(120,200,255,0.30)] transition-all hover:shadow-[0_0_22px_rgba(120,200,255,0.45)] disabled:opacity-40 disabled:shadow-none"
            >
              {isAdd ? "Add" : "Save"}
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

function formatRelative(epoch: number): string {
  const diff = Date.now() - epoch;
  if (diff < 10_000) return "just now";
  const min = Math.floor(diff / 60_000);
  if (min < 60) return `${min} min ago`;
  const h = Math.floor(diff / 3_600_000);
  if (h < 24) return `${h} h ago`;
  const d = Math.floor(diff / 86_400_000);
  return `${d} d ago`;
}
