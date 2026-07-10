import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/cn';

export type StatusPillStatus = 'running' | 'stopped' | 'error' | 'pending' | 'missing';

export type StatusPillProps = {
  status: StatusPillStatus;
  label?: string;
  className?: string;
};

const STYLES: Record<StatusPillStatus, { container: string; dot: string }> = {
  running: {
    container: 'bg-success/[0.12] border-success/30 text-success',
    dot: 'bg-success shadow-[0_0_6px_#00e676]',
  },
  stopped: {
    container: 'bg-danger/[0.12] border-danger/30 text-danger',
    dot: 'bg-danger shadow-[0_0_6px_#ff5e5e]',
  },
  error: {
    container: 'bg-danger/[0.12] border-danger/30 text-danger',
    dot: 'bg-danger shadow-[0_0_6px_#ff5e5e]',
  },
  pending: {
    container: 'bg-warn/[0.12] border-warn/30 text-warn',
    dot: 'bg-warn shadow-[0_0_6px_#ffb13c] animate-pulse',
  },
  missing: {
    container: 'bg-danger/[0.12] border-danger/30 text-danger',
    dot: 'bg-danger shadow-[0_0_6px_#ff5e5e]',
  },
};

export function StatusPill({ status, label, className }: StatusPillProps) {
  const { t } = useTranslation();
  const s = STYLES[status];
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] rounded-full border',
        s.container,
        className,
      )}
    >
      <span className={cn('w-1.5 h-1.5 rounded-full', s.dot)} />
      {label ?? t(`statusPill.${status}`)}
    </span>
  );
}
