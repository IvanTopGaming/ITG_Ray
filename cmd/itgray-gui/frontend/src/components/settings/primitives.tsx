// Shared primitives for the Settings section components. Inline-styled
// (Tailwind utility classes) so each section can stay <100 LoC without
// pulling a heavyweight component library. These are intentionally NOT
// exported from a barrel — sections import directly to keep the
// dependency graph flat for tree-shaking.

import type { ReactNode } from "react";

export function Row({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-1">
      <div className="min-w-0">
        <div className="text-sm text-text-secondary">{label}</div>
        {hint ? <div className="text-xs text-text-muted">{hint}</div> : null}
      </div>
      <div className="flex-shrink-0">{children}</div>
    </div>
  );
}

export function Toggle({ value, onChange, ariaLabel }: { value: boolean; onChange: (v: boolean) => void; ariaLabel?: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={value}
      aria-label={ariaLabel}
      className={`w-10 h-6 rounded-full relative transition-colors ${value ? "bg-accent-primary" : "bg-white/10"}`}
      onClick={() => onChange(!value)}
    >
      <span
        className={`absolute top-0.5 w-5 h-5 rounded-full bg-white transition-all ${value ? "left-[1.125rem]" : "left-0.5"}`}
      />
    </button>
  );
}

export function Select({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  return (
    <select
      className="bg-white/5 border border-white/10 rounded-md px-2 py-1 text-sm text-text-primary focus:outline-none focus:ring-2 focus:ring-accent-primary"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

export function NumberInput({
  value,
  onChange,
  min,
  max,
  step,
}: {
  value: number;
  onChange: (v: number) => void;
  min?: number;
  max?: number;
  step?: number;
}) {
  return (
    <input
      type="number"
      className="bg-white/5 border border-white/10 rounded-md px-2 py-1 text-sm text-text-primary w-24 focus:outline-none focus:ring-2 focus:ring-accent-primary"
      value={value}
      min={min}
      max={max}
      step={step}
      onChange={(e) => {
        const n = Number(e.target.value);
        if (!Number.isNaN(n)) onChange(n);
      }}
    />
  );
}

export function TextInput({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <input
      type="text"
      className="bg-white/5 border border-white/10 rounded-md px-2 py-1 text-sm text-text-primary w-56 focus:outline-none focus:ring-2 focus:ring-accent-primary"
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}

export function SectionShell({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section id={id} className="space-y-3 rounded-lg bg-white/[0.02] border border-white/5 p-4">
      <h3 className="text-base font-semibold text-text-primary">{title}</h3>
      <div className="space-y-2">{children}</div>
    </section>
  );
}
