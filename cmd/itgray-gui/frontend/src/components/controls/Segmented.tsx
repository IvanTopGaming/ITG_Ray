import { useId } from 'react';
import { motion } from 'framer-motion';
import { cn } from '@/lib/cn';

export type SegmentedOption<T extends string = string> = {
  value: T;
  label: string;
};

export type SegmentedProps<T extends string = string> = {
  value: T;
  onChange: (v: T) => void;
  options: readonly SegmentedOption<T>[];
  className?: string;
};

export function Segmented<T extends string>({
  value,
  onChange,
  options,
  className,
}: SegmentedProps<T>) {
  const layoutId = useId();

  return (
    <div
      role="group"
      className={cn(
        'inline-flex items-center gap-0.5 rounded-[10px] border border-white/[0.12] bg-white/[0.06] p-[3px]',
        className,
      )}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(opt.value)}
            className={cn(
              'relative px-3.5 py-1.5 text-xs rounded-[7px] transition-colors',
              active ? 'text-white' : 'text-white/[0.55] hover:text-white/[0.80]',
            )}
          >
            {active && (
              <motion.span
                layoutId={`segmented-pill-${layoutId}`}
                className="absolute inset-0 rounded-[7px] bg-white/[0.14] shadow-[inset_0_0_0_1px_rgba(255,255,255,0.10)]"
                transition={{ type: 'spring', stiffness: 500, damping: 32 }}
              />
            )}
            <span className="relative">{opt.label}</span>
          </button>
        );
      })}
    </div>
  );
}
