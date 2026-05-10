import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';

export type SettingRowProps = {
  label: string;
  hint?: string;
  stacked?: boolean;
  children: ReactNode;
  className?: string;
};

export function SettingRow({ label, hint, stacked, children, className }: SettingRowProps) {
  return (
    <div
      className={cn(
        'flex gap-4 py-3 border-b border-white/[0.06] last:border-b-0',
        stacked ? 'flex-col items-stretch' : 'flex-row items-center justify-between',
        className,
      )}
    >
      <div className={cn('flex flex-col gap-0.5', stacked ? '' : 'max-w-[60%]')}>
        <span className="text-[13px] text-white/[0.92]">{label}</span>
        {hint && <span className="text-[11.5px] leading-snug text-white/[0.45]">{hint}</span>}
      </div>
      <div className={cn(stacked && 'mt-2')}>{children}</div>
    </div>
  );
}

export type SectionHeaderProps = {
  title: string;
  right?: ReactNode;
  className?: string;
};

export function SectionHeader({ title, right, className }: SectionHeaderProps) {
  return (
    <div
      className={cn(
        'flex items-baseline justify-between pb-3 mb-1 border-b border-white/[0.08]',
        className,
      )}
    >
      <span className="text-[14px] font-semibold tracking-wide text-white/[0.92]">{title}</span>
      {right}
    </div>
  );
}
