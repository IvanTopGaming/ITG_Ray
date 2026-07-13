import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { createPortal } from "react-dom";
import { motion } from "framer-motion";
import { CONDITION_TYPES, type ConditionType } from "./types";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

export function AddConditionPicker({
  present,
  onAdd,
  prominent,
}: {
  present: Set<ConditionType>;
  onAdd: (key: ConditionType) => void;
  prominent?: boolean;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [coords, setCoords] = useState<{ left: number; top: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const popRef = useRef<HTMLDivElement>(null);

  const availableTypes = CONDITION_TYPES.filter((c) => !present.has(c.key));

  useEffect(() => {
    if (!open || !btnRef.current) return;
    const r = btnRef.current.getBoundingClientRect();
    setCoords({ left: r.left, top: r.bottom + 4 });
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as Node;
      if (btnRef.current?.contains(target) || popRef.current?.contains(target)) return;
      setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", handler);
      document.removeEventListener("keydown", esc);
    };
  }, [open]);

  if (availableTypes.length === 0) return null;

  return (
    <>
      <motion.button
        ref={btnRef}
        type="button"
        onClick={() => setOpen((o) => !o)}
        whileHover={{ scale: 1.02 }}
        whileTap={{ scale: 0.96 }}
        transition={{ duration: 0.18, ease: SNAP_EASE }}
        aria-haspopup="menu"
        aria-expanded={open}
        className={
          prominent
            ? "group flex w-full items-center justify-center gap-2 rounded-xl border border-dashed border-white/[0.17] bg-white/[0.02] px-4 py-3 text-[12.5px] font-medium text-[#cfd6ff] transition-all hover:border-white/[0.3] hover:bg-white/[0.05]"
            : "group flex items-center gap-2 self-start rounded-full border border-dashed border-white/[0.17] bg-white/[0.02] px-3.5 py-1.5 text-[12.5px] font-medium text-white/70 transition-all hover:border-white/[0.3] hover:bg-white/[0.06] hover:text-white/95"
        }
      >
        <span className="text-lg leading-none opacity-70 group-hover:opacity-100">{open ? "×" : "+"}</span>
        <span>{t("ruleEditor.addCondition")}</span>
      </motion.button>
      {open && coords && createPortal(
        <motion.div
          ref={popRef}
          role="menu"
          initial={{ opacity: 0, y: -4, scale: 0.96, filter: "blur(4px)" }}
          animate={{ opacity: 1, y: 0, scale: 1, filter: "blur(0px)" }}
          transition={{ duration: 0.16, ease: SNAP_EASE }}
          style={{ position: "fixed", left: coords.left, top: coords.top, zIndex: 1000 }}
          className="flex w-56 flex-col gap-0.5 rounded-xl border border-white/[0.12] bg-[#1a1c24]/95 p-1.5 shadow-[0_16px_40px_-8px_rgba(0,0,0,0.6)] backdrop-blur-xl"
        >
          {availableTypes.map((ct) => (
            <button
              key={ct.key}
              type="button"
              role="menuitem"
              onClick={() => { onAdd(ct.key); setOpen(false); }}
              className="group flex items-center gap-3 rounded-lg px-2.5 py-2 text-left text-[13px] font-medium text-white/70 transition-colors duration-150 hover:bg-white/[0.06] hover:text-white/95"
            >
              <span aria-hidden className="flex h-6 w-6 items-center justify-center rounded-md bg-white/[0.04] text-[14px] shadow-sm transition-colors group-hover:bg-white/[0.08]">{ct.icon}</span>
              <span>{t(`ruleEditor.conditions.${ct.key}`)}</span>
            </button>
          ))}
        </motion.div>,
        document.body,
      )}
    </>
  );
}
