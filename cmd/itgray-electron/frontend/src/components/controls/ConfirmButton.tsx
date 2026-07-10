import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { motion } from 'framer-motion';
import { cn } from '@/lib/cn';

export type ConfirmButtonVariant = 'ghost' | 'danger' | 'primary';

export type ConfirmButtonProps = {
  onConfirm: () => void;
  children: string;
  confirmLabel?: string;
  variant?: ConfirmButtonVariant;
  timeoutMs?: number;
  className?: string;
};

const VARIANT_CLASSES: Record<ConfirmButtonVariant, string> = {
  ghost: 'bg-transparent border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]',
  danger: 'bg-danger/10 border-danger/30 text-danger hover:bg-danger/[0.15]',
  primary:
    'bg-gradient-to-b from-accent-start to-accent-mid border-transparent text-white font-semibold',
};

const ARMED_CLASSES = 'bg-danger/10 border-danger/40 text-danger';

export function ConfirmButton({
  onConfirm,
  children,
  confirmLabel,
  variant = 'ghost',
  timeoutMs = 3000,
  className,
}: ConfirmButtonProps) {
  const { t } = useTranslation();
  const [armed, setArmed] = useState(false);

  useEffect(() => {
    if (!armed) return;
    const t = setTimeout(() => setArmed(false), timeoutMs);
    return () => clearTimeout(t);
  }, [armed, timeoutMs]);

  const handleClick = () => {
    if (armed) {
      setArmed(false);
      onConfirm();
    } else {
      setArmed(true);
    }
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      className={cn(
        'relative px-3.5 py-1.5 text-xs font-medium rounded-[10px] border transition-colors',
        armed ? ARMED_CLASSES : VARIANT_CLASSES[variant],
        className,
      )}
    >
      <motion.span
        key={armed ? 'armed' : 'idle'}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ duration: 0.12 }}
        className="inline-block"
      >
        {armed ? (confirmLabel ?? t('common.clickToConfirm')) : children}
      </motion.span>
    </button>
  );
}
