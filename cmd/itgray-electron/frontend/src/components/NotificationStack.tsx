import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";
import { AlertTriangle, ArrowUpCircle, ShieldAlert, X } from "lucide-react";
import { cn } from "@/lib/cn";
import {
  dismiss,
  useNotifications,
  type AppNotification,
  type NotificationAction,
  type NotificationTone,
} from "@/lib/notificationStore";

const MAX_VISIBLE = 3;
export const AUTO_HIDE_MS = 6000;

const TONE: Record<NotificationTone, { icon: typeof AlertTriangle; color: string; ring: string }> = {
  info: { icon: ArrowUpCircle, color: "text-sky-300", ring: "border-sky-400/25" },
  warning: { icon: AlertTriangle, color: "text-amber-300", ring: "border-amber-400/30" },
  danger: { icon: ShieldAlert, color: "text-rose-300", ring: "border-rose-400/30" },
};

function ActionButton({ action }: { action: NotificationAction }) {
  const [busy, setBusy] = useState(false);
  const click = async () => {
    setBusy(true);
    try {
      await action.run();
    } finally {
      setBusy(false);
    }
  };
  return (
    <button
      type="button"
      disabled={busy}
      onClick={() => void click()}
      className={cn(
        "rounded-[10px] px-3 py-1.5 text-[11.5px] font-medium transition-colors disabled:opacity-60",
        action.primary
          ? "border border-accent/30 bg-accent/[0.12] text-accent hover:bg-accent/[0.18]"
          : "border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]",
      )}
    >
      {action.label}
    </button>
  );
}

function NotificationCard({ n }: { n: AppNotification }) {
  const { t } = useTranslation();
  const timer = useRef<number | null>(null);
  const tone = TONE[n.tone];
  const Icon = tone.icon;

  const stop = () => {
    if (timer.current !== null) window.clearTimeout(timer.current);
    timer.current = null;
  };
  const start = () => {
    stop();
    if (n.persistent) return;
    timer.current = window.setTimeout(() => dismiss(n.id), AUTO_HIDE_MS);
  };

  useEffect(() => {
    start();
    return stop;
  }, [n.id, n.persistent]);

  return (
    <motion.div
      layout
      initial={{ x: 48, opacity: 0 }}
      animate={{ x: 0, opacity: 1 }}
      exit={{ x: 48, opacity: 0 }}
      transition={{ duration: 0.26, ease: [0.32, 0.72, 0, 1] }}
      onMouseEnter={stop}
      onMouseLeave={start}
      data-testid={`notification-${n.id}`}
      className={cn(
        "glass-regular pointer-events-auto flex gap-3 rounded-2xl border bg-bg-1/80 p-3.5 shadow-[0_18px_36px_-12px_rgba(0,0,0,0.6)] backdrop-blur-xl",
        tone.ring,
      )}
    >
      <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", tone.color)} />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-start gap-2">
          <span className="flex-1 text-[13px] font-semibold text-white/95">{n.title}</span>
          <button
            type="button"
            onClick={() => dismiss(n.id)}
            aria-label={t("common.dismiss")}
            className="-mr-1 -mt-0.5 rounded-md p-0.5 text-white/40 transition-colors hover:bg-white/[0.06] hover:text-white/80"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
        {n.body && <p className="text-[12px] leading-relaxed text-white/60">{n.body}</p>}
        {n.actions && n.actions.length > 0 && (
          <div className="mt-1.5 flex flex-wrap justify-end gap-1.5">
            {n.actions.map((a) => (
              <ActionButton key={a.label} action={a} />
            ))}
          </div>
        )}
      </div>
    </motion.div>
  );
}

export function NotificationStack({ raised = false }: { raised?: boolean }) {
  const items = useNotifications().slice(0, MAX_VISIBLE);
  return (
    <div
      role="region"
      aria-live="polite"
      className={cn(
        "pointer-events-none fixed right-6 z-50 flex w-[340px] max-w-[calc(100vw-48px)] flex-col gap-2 transition-[bottom] duration-300",
        raised ? "bottom-20" : "bottom-6",
      )}
    >
      <AnimatePresence initial={false}>
        {items.map((n) => (
          <NotificationCard key={n.id} n={n} />
        ))}
      </AnimatePresence>
    </div>
  );
}
