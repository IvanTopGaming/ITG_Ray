import { AnimatePresence, motion } from 'framer-motion';
import type { ReactNode } from 'react';

export type RevealProps = {
  show: boolean;
  children: ReactNode;
};

const SNAP_EASE = [0.16, 1, 0.3, 1] as const;

/**
 * Reveal — conditionally renders children with a height + opacity animation.
 * Wraps its own AnimatePresence, so it's self-contained: drop in and toggle
 * `show`. For the multi-child + key-based mount pattern (e.g. Servers list
 * deletion), use Servers.tsx's inline Reveal under a parent AnimatePresence.
 */
export function Reveal({ show, children }: RevealProps) {
  return (
    <AnimatePresence initial={false}>
      {show && (
        <motion.div
          initial={{ height: 0, opacity: 0 }}
          animate={{ height: 'auto', opacity: 1 }}
          exit={{ height: 0, opacity: 0 }}
          transition={{ duration: 0.24, ease: SNAP_EASE }}
          style={{ overflow: 'hidden' }}
        >
          {children}
        </motion.div>
      )}
    </AnimatePresence>
  );
}
