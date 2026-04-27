import { motion } from 'framer-motion';
import { cn } from '@/lib/cn';

export type ToggleProps = {
  value: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  className?: string;
};

export function Toggle({ value, onChange, disabled, className }: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={value}
      disabled={disabled}
      onClick={() => onChange(!value)}
      className={cn(
        'relative inline-flex h-5 w-9 flex-shrink-0 items-center rounded-full transition-colors',
        'border border-white/[0.12]',
        value ? 'bg-gradient-to-b from-accent-start to-accent-mid' : 'bg-white/15',
        disabled && 'opacity-40 cursor-not-allowed',
        className,
      )}
    >
      <motion.span
        layout
        transition={{ type: 'spring', stiffness: 500, damping: 30 }}
        className={cn(
          'inline-block h-4 w-4 rounded-full bg-white shadow',
          value ? 'ml-auto mr-0.5' : 'ml-0.5 mr-auto',
        )}
      />
    </button>
  );
}
