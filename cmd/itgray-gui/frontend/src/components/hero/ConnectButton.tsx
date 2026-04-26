import type { ChainStatus } from "@/api/client";
import { motion } from "framer-motion";
import { useTranslation } from "react-i18next";

// ConnectButton renders the big circular Connect/Disconnect affordance at
// the centre of the Hero card. The label and disabled state are derived
// from the current chain status; while connecting we run a soft pulse via
// framer-motion's animate prop so the user sees the chain is in progress.
export function ConnectButton({
  status,
  onClick,
}: {
  status: ChainStatus;
  onClick: () => void;
}) {
  const { t } = useTranslation();
  const label =
    status === "connected"
      ? t("hero.disconnect")
      : status === "connecting"
      ? t("hero.connecting")
      : status === "disconnecting"
      ? t("hero.disconnecting")
      : t("hero.connect");
  const disabled = status === "connecting" || status === "disconnecting";
  return (
    <motion.button
      whileTap={{ scale: 0.97 }}
      animate={
        status === "connecting"
          ? { scale: [1, 1.04, 1], transition: { repeat: Infinity, duration: 1.2 } }
          : {}
      }
      disabled={disabled}
      onClick={onClick}
      className="w-32 h-32 rounded-full bg-gradient-to-br from-indigo-500 to-pink-500 text-white font-semibold tracking-wide shadow-[0_0_40px_rgba(99,102,241,0.4)] disabled:opacity-60"
    >
      {label}
    </motion.button>
  );
}
