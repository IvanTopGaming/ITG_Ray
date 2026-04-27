import { useState } from 'react';
import { motion, type Variants } from 'framer-motion';
import { useSettings } from '@/lib/settings';
import { cn } from '@/lib/cn';
import { Toggle } from '@/components/controls/Toggle';
import { Segmented } from '@/components/controls/Segmented';
import { Dropdown } from '@/components/controls/Dropdown';
import { Reveal } from '@/components/controls/Reveal';
import { SettingRow, SectionHeader } from '@/components/controls/SettingRow';
import { ConfirmButton } from '@/components/controls/ConfirmButton';
import { ConfirmDialog } from '@/components/controls/ConfirmDialog';
import { StatusPill, type StatusPillStatus } from '@/components/controls/StatusPill';
import { ScrollSpy, useScrollSpy, scrollToSection } from '@/components/controls/ScrollSpy';

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

const SECTIONS = [
  { id: 'general', label: 'General' },
  { id: 'connection', label: 'Connection' },
  { id: 'notifications', label: 'Notifications' },
  { id: 'helper', label: 'Helper' },
  { id: 'logs', label: 'Logs' },
  { id: 'about', label: 'About' },
];
const SECTION_IDS = SECTIONS.map((s) => s.id);

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
  const active = useScrollSpy(SECTION_IDS);
  const [helperStatus, setHelperStatus] = useState<StatusPillStatus>('running');
  const [reinstallOpen, setReinstallOpen] = useState(false);
  const [logFolderSize, setLogFolderSize] = useState(47); // MB
  const [updateState, setUpdateState] = useState<'idle' | 'checking' | 'uptodate'>('idle');

  const restartHelper = async () => {
    setHelperStatus('pending');
    await new Promise((r) => setTimeout(r, 800));
    setHelperStatus('running');
  };

  const reinstallHelper = async () => {
    setHelperStatus('stopped');
    await new Promise((r) => setTimeout(r, 2000));
    setHelperStatus('running');
  };

  const clearLogs = () => setLogFolderSize(0);

  const checkUpdates = async () => {
    setUpdateState('checking');
    await new Promise((r) => setTimeout(r, 1500));
    setUpdateState('uptodate');
  };

  return (
    <motion.section
      className="flex flex-col gap-3"
      initial="hidden"
      animate="show"
      variants={pageVariants}
    >
      <h1 className="text-[22px] font-semibold tracking-tight">Settings</h1>

      <div
        className={cn(
          'sticky top-0 z-20 -mx-8 -my-1 px-8 py-2.5',
          'bg-bg-1/[0.65] backdrop-blur-xl border-b border-white/[0.06]',
        )}
      >
        <ScrollSpy sections={SECTIONS} active={active} onSelect={scrollToSection} />
      </div>

      {/* General */}
      <motion.div id="general" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="General" />
        <SettingRow label="Launch on system startup" hint="Start ITG Ray automatically when you log in.">
          <Toggle value={s.launchOnStartup} onChange={(v) => update({ launchOnStartup: v })} />
        </SettingRow>
        <SettingRow label="Start minimized to tray" hint="Open in background, no main window on launch.">
          <Toggle value={s.startMinimized} onChange={(v) => update({ startMinimized: v })} />
        </SettingRow>
      </motion.div>

      {/* Connection */}
      <motion.div id="connection" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Connection" />
        <SettingRow
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
        </SettingRow>
        <SettingRow label="DNS" hint="Override DNS while connected. Uses remote VPN DNS by default.">
          <Dropdown
            value={s.dnsMode}
            onChange={(v) => update({ dnsMode: v })}
            options={[
              { value: 'auto', label: 'Auto (remote)' },
              { value: 'custom', label: 'Custom' },
            ] as const}
          />
        </SettingRow>
        <Reveal show={s.dnsMode === 'custom'}>
          <SettingRow label="Custom DNS" hint="Comma-separated list of resolvers (e.g. 1.1.1.1, 8.8.8.8)." stacked>
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
          </SettingRow>
        </Reveal>
        <SettingRow label="Allow LAN access" hint="Reach local network devices (printers, NAS) while VPN is on.">
          <Toggle value={s.allowLan} onChange={(v) => update({ allowLan: v })} />
        </SettingRow>
      </motion.div>

      {/* Notifications */}
      <motion.div id="notifications" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Notifications" />
        <SettingRow label="Connection toasts" hint="Show OS notifications on connect, disconnect, and errors.">
          <Toggle value={s.notifyConnection} onChange={(v) => update({ notifyConnection: v })} />
        </SettingRow>
        <SettingRow label="Sound" hint="Play a soft chime on state changes.">
          <Toggle value={s.notifySound} onChange={(v) => update({ notifySound: v })} />
        </SettingRow>
        <SettingRow label="Subscription failures" hint="Notify when a subscription sync fails.">
          <Toggle value={s.notifySubFailure} onChange={(v) => update({ notifySubFailure: v })} />
        </SettingRow>
      </motion.div>

      {/* Helper */}
      <motion.div id="helper" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Helper service" right={<StatusPill status={helperStatus} />} />
        <SettingRow
          label="Status"
          hint="Privileged background service that manages TUN and routing. v1.4.2"
        >
          <div className="flex gap-1.5">
            <ConfirmButton onConfirm={restartHelper} variant="ghost">Restart</ConfirmButton>
            <button
              type="button"
              onClick={() => console.log('[mock] view helper log')}
              className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
            >
              View log
            </button>
          </div>
        </SettingRow>
        <SettingRow label="Reinstall" hint="If something goes wrong with privileges or service registration.">
          <button
            type="button"
            onClick={() => setReinstallOpen(true)}
            className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] bg-white/[0.08] border border-white/[0.14] text-white/[0.92] hover:bg-white/[0.12]"
          >
            Reinstall helper
          </button>
        </SettingRow>
      </motion.div>

      <ConfirmDialog
        open={reinstallOpen}
        onClose={() => setReinstallOpen(false)}
        title="Reinstall helper service?"
        description="This will request elevated privileges and restart the service. Active connection will drop briefly."
        confirmLabel="Reinstall"
        confirmVariant="primary"
        onConfirm={reinstallHelper}
      />

      {/* Logs */}
      <motion.div id="logs" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Logs" />
        <SettingRow
          label="Log level"
          hint="More detail helps when debugging connection issues, but fills disk faster."
        >
          <Segmented
            value={s.logLevel}
            onChange={(v) => update({ logLevel: v })}
            options={[
              { value: 'error', label: 'Error' },
              { value: 'info', label: 'Info' },
              { value: 'debug', label: 'Debug' },
              { value: 'trace', label: 'Trace' },
            ] as const}
          />
        </SettingRow>
        <SettingRow label="Open log folder" hint={`%LOCALAPPDATA%\\ITG Ray\\logs (${logFolderSize} MB)`}>
          <button
            type="button"
            onClick={() => console.log('[mock] open log folder')}
            className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
          >
            Open folder
          </button>
        </SettingRow>
        <SettingRow label="Clear old logs" hint="Delete log files older than 7 days.">
          <ConfirmButton onConfirm={clearLogs} variant="danger">Clear</ConfirmButton>
        </SettingRow>
      </motion.div>

      {/* About */}
      <motion.div id="about" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="About" />
        <dl className="grid grid-cols-[110px_1fr] gap-y-2 gap-x-4 text-[13px] py-1">
          <dt className="text-white/[0.45]">Version</dt><dd className="text-white/[0.92] tabular-nums">0.4.0-rc1</dd>
          <dt className="text-white/[0.45]">Build</dt><dd className="text-white/[0.92] tabular-nums">2ba7e54 · 2026-04-27</dd>
          <dt className="text-white/[0.45]">Backend</dt><dd className="text-white/[0.92] tabular-nums">xray-core 25.3.6</dd>
          <dt className="text-white/[0.45]">Helper</dt><dd className="text-white/[0.92] tabular-nums">1.4.2</dd>
          <dt className="text-white/[0.45]">License</dt><dd className="text-white/[0.92]">MIT</dd>
        </dl>
        <SettingRow
          label="Check for updates"
          hint={
            updateState === 'uptodate'
              ? "You're up to date."
              : updateState === 'checking'
              ? 'Checking…'
              : 'Last checked 12 minutes ago.'
          }
        >
          <button
            type="button"
            onClick={checkUpdates}
            disabled={updateState === 'checking'}
            className="px-3.5 py-1.5 text-xs font-semibold rounded-[10px] bg-gradient-to-b from-accent-start to-accent-mid text-white disabled:opacity-60"
          >
            {updateState === 'checking' ? 'Checking…' : 'Check now'}
          </button>
        </SettingRow>
      </motion.div>
    </motion.section>
  );
}
