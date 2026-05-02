import { useEffect, useState } from 'react';
import { cn } from '@/lib/cn';

export type ScrollSpySection = {
  id: string;
  label: string;
};

export type ScrollSpyProps = {
  sections: ScrollSpySection[];
  active: string;
  onSelect: (id: string) => void;
  className?: string;
};

export function ScrollSpy({ sections, active, onSelect, className }: ScrollSpyProps) {
  return (
    <div className={cn('flex flex-wrap gap-1.5', className)}>
      {sections.map((s) => {
        const isActive = s.id === active;
        return (
          <button
            key={s.id}
            type="button"
            aria-current={isActive}
            onClick={() => onSelect(s.id)}
            className={cn(
              'px-2.5 py-1 text-[11px] rounded-full border transition-colors',
              isActive
                ? 'bg-accent-start/[0.10] border-accent-start/35 text-accent-start'
                : 'bg-white/[0.05] border-white/[0.10] text-white/[0.55] hover:text-white/[0.80]',
            )}
          >
            {s.label}
          </button>
        );
      })}
    </div>
  );
}

export function useScrollSpy(ids: readonly string[], rootMargin = '-50% 0px -50% 0px'): string {
  const [active, setActive] = useState(ids[0] ?? '');
  useEffect(() => {
    if (ids.length === 0) return;
    const elements = ids
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => el !== null);
    if (elements.length === 0) return;
    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries.filter((e) => e.isIntersecting);
        if (visible.length > 0) {
          setActive(visible[0].target.id);
        }
      },
      { rootMargin, threshold: 0 },
    );
    for (const el of elements) observer.observe(el);
    return () => observer.disconnect();
  }, [ids.join('|'), rootMargin]);
  return active;
}

export function scrollToSection(id: string): void {
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' });
}
