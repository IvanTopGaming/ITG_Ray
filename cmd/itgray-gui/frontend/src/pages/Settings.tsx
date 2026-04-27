import { motion, type Variants } from 'framer-motion';
import { useSettings } from '@/lib/settings';
import { cn } from '@/lib/cn';
import { Toggle } from '@/components/controls/Toggle';
import { Segmented } from '@/components/controls/Segmented';
import { Dropdown } from '@/components/controls/Dropdown';
import { Reveal } from '@/components/controls/Reveal';

const pageVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { delayChildren: 0.05, staggerChildren: 0.04 },
  },
};

const sectionVariants: Variants = {
  hidden: { opacity: 0, y: 8 },
  show: { opacity: 1, y: 0, transition: { duration: 0.24, ease: [0.16, 1, 0.3, 1] } },
};

// Empty string is valid (= no override). Otherwise, comma-separated tokens
// must each parse as IPv4 or IPv6. Visual-only — backend re-validates.
function isDnsValid(value: string): boolean {
  if (value.trim() === '') return true;
  const tokens = value.split(',').map((t) => t.trim()).filter(Boolean);
  if (tokens.length === 0) return false;
  const v4 = /^(\d{1,3}\.){3}\d{1,3}$/;
  const v6 = /^[0-9a-fA-F:]+$/;
  return tokens.every((t) => v4.test(t) || (v6.test(t) && t.includes(':')));
}

export function Settings() {
  const [s, update] = useSettings();

  return (
    <motion.section
      className="flex flex-col gap-3"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      <h1 className="text-[22px] font-semibold tracking-tight">Settings</h1>

      {/* General */}
      <motion.div variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="General" />
        <Row label="Launch on system startup" hint="Start ITG Ray automatically when you log in.">
          <Toggle value={s.launchOnStartup} onChange={(v) => update({ launchOnStartup: v })} />
        </Row>
        <Row label="Start minimized to tray" hint="Open in background, no main window on launch.">
          <Toggle value={s.startMinimized} onChange={(v) => update({ startMinimized: v })} />
        </Row>
      </motion.div>

      {/* Connection */}
      <motion.div variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Connection" />
        <Row
          label="Network mode"
          hint="TUN intercepts all traffic at OS level. System proxy uses Windows proxy settings only."
        >
          <Segmented
            value={s.networkMode}
            onChange={(v) => update({ networkMode: v })}
            options={[
              { value: 'tun', label: 'TUN' },
              { value: 'system-proxy', label: 'System proxy' },
              { value: 'off', label: 'Off' },
            ] as const}
          />
        </Row>
        <Row label="DNS" hint="Override DNS while connected. Uses remote VPN DNS by default.">
          <Dropdown
            value={s.dnsMode}
            onChange={(v) => update({ dnsMode: v })}
            options={[
              { value: 'auto', label: 'Auto (remote)' },
              { value: 'custom', label: 'Custom' },
            ] as const}
          />
        </Row>
        <Reveal show={s.dnsMode === 'custom'}>
          <Row label="Custom DNS" hint="Comma-separated list of resolvers (e.g. 1.1.1.1, 8.8.8.8)." stacked>
            <input
              type="text"
              value={s.dnsCustom}
              onChange={(e) => update({ dnsCustom: e.target.value })}
              placeholder="1.1.1.1, 8.8.8.8"
              className={cn(
                'w-full px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white placeholder:text-white/[0.30] outline-none transition-colors',
                isDnsValid(s.dnsCustom)
                  ? 'border-white/[0.12] focus:border-white/[0.30]'
                  : 'border-danger/50',
              )}
            />
          </Row>
        </Reveal>
        <Row label="Allow LAN access" hint="Reach local network devices (printers, NAS) while VPN is on.">
          <Toggle value={s.allowLan} onChange={(v) => update({ allowLan: v })} />
        </Row>
      </motion.div>
    </motion.section>
  );
}

// Temporary inline scaffolding — will be replaced by <SettingRow> in T8.
function SectionHeader({ title }: { title: string }) {
  return (
    <div className="flex items-baseline justify-between pb-3 mb-1 border-b border-white/[0.08]">
      <span className="text-[14px] font-semibold tracking-wide text-white/[0.92]">{title}</span>
    </div>
  );
}

function Row({
  label,
  hint,
  stacked,
  children,
}: {
  label: string;
  hint?: string;
  stacked?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        'flex gap-4 py-3 border-b border-white/[0.06] last:border-b-0',
        stacked ? 'flex-col items-stretch' : 'flex-row items-center justify-between',
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
