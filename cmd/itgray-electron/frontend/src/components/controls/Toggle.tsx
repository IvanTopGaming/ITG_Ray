import { cn } from '@/lib/cn';

export type ToggleProps = {
  value: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  className?: string;
  'aria-label'?: string;
};

export function Toggle({ value, onChange, disabled, className, 'aria-label': ariaLabel }: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={value}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onChange(!value)}
      className={cn(
        'relative inline-flex h-5 w-9 flex-shrink-0 items-center rounded-full transition-colors',
        'border border-white/[0.12]',
        value ? 'bg-gradient-to-b from-accent-start to-accent-mid' : 'bg-white/[0.15]',
        disabled && 'opacity-40 cursor-not-allowed',
        className,
      )}
    >
      <span
        className={cn(
          'ml-0.5 inline-block h-4 w-4 rounded-full bg-white shadow transition-transform duration-150 ease-out',
          value && 'translate-x-4',
        )}
      />
    </button>
  );
}
