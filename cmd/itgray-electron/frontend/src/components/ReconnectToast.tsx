import { AnimatePresence, motion } from "framer-motion";
import { useTranslation } from "react-i18next";

export type ReconnectToastProps = {
  visible: boolean;
  message: string;
  onReconnect?: () => void | Promise<void>;
  onDismiss: () => void;
};

export function ReconnectToast({
  visible,
  message,
  onReconnect,
  onDismiss,
}: ReconnectToastProps) {
  const { t } = useTranslation();
  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          role="status"
          aria-live="polite"
          initial={{ y: 36, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: 36, opacity: 0 }}
          transition={{ duration: 0.26, ease: [0.32, 0.72, 0, 1] }}
          className="fixed bottom-6 left-1/2 z-50 -translate-x-1/2 flex items-center gap-3 rounded-full border border-amber-400/45 bg-amber-500/90 px-4 py-2 text-[12.5px] font-medium text-amber-50 shadow-lg shadow-amber-500/25 backdrop-blur-md"
        >
          <span>{message}</span>
          {onReconnect && (
            <button
              onClick={() => void onReconnect()}
              className="rounded-full bg-amber-100/25 px-2.5 py-1 text-[11.5px] font-semibold text-amber-50 transition-colors hover:bg-amber-100/40"
            >
              {t("common.reconnect")}
            </button>
          )}
          <button
            onClick={onDismiss}
            aria-label={t("common.dismiss")}
            className="rounded-full px-2 py-0.5 text-[16px] leading-none text-amber-50/80 transition-colors hover:bg-amber-100/20 hover:text-amber-50"
          >
            ×
          </button>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
