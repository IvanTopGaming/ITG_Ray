import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/cn";

const SNAP_EASE: [number, number, number, number] = [0.16, 1, 0.3, 1];

export type DropdownOption<T extends string = string> = {
  value: T;
  label: string;
};

export type DropdownProps<T extends string = string> = {
  value: T;
  onChange: (v: T) => void;
  disabled?: boolean;
  options: readonly DropdownOption<T>[];
  className?: string;
  triggerClassName?: string;
  menuClassName?: string;
  ariaLabel?: string;
};

export function Dropdown<T extends string = string>({
  value,
  onChange,
  disabled,
  options,
  className,
  triggerClassName,
  menuClassName,
  ariaLabel,
}: DropdownProps<T>) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const current = options.find((o) => o.value === value);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
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

  return (
    <div ref={ref} className={cn("relative", className)}>
      <button
        type="button"
        onClick={() => !disabled && setOpen((o) => !o)}
        disabled={disabled}
        aria-label={ariaLabel}
        className={cn(
          "flex w-full items-center justify-between rounded-lg border bg-white/[0.04] px-3 py-2 text-left text-[12px] text-white transition-colors duration-instant ease-snap",
          open
            ? "border-accent-start/50 bg-white/[0.06]"
            : "border-white/15 hover:border-white/25 hover:bg-white/[0.05]",
          disabled && "cursor-not-allowed opacity-60",
          triggerClassName
        )}
      >
        <span className={cn(!current?.label && "text-white/35")}>
          {current?.label ?? value ?? "—"}
        </span>
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 text-white/45 transition-transform duration-instant ease-snap",
            open && "rotate-180",
          )}
        />
      </button>
      <AnimatePresence>
        {open && (
          <motion.div
            initial={{ opacity: 0, y: -4, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -4, scale: 0.98 }}
            transition={{ duration: 0.15, ease: SNAP_EASE }}
            className={cn(
              "absolute right-0 top-full z-30 mt-1 max-h-[220px] w-max min-w-full max-w-[min(20rem,90vw)] overflow-y-auto rounded-lg border border-white/[0.18] bg-bg-1/95 p-1 shadow-[0_18px_36px_-10px_rgba(0,0,0,0.6)] backdrop-blur-xl",
              menuClassName
            )}
          >
            {options.map((o) => (
              <button
                key={o.value}
                type="button"
                onClick={() => {
                  onChange(o.value);
                  setOpen(false);
                }}
                className={cn(
                  "flex w-full items-center whitespace-nowrap rounded-md px-2.5 py-1.5 text-left text-[12px] transition-colors duration-instant ease-snap",
                  o.value === value
                    ? "bg-white/[0.12] text-white"
                    : "text-white/75 hover:bg-white/[0.06] hover:text-white",
                )}
              >
                {o.label}
              </button>
            ))}
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
