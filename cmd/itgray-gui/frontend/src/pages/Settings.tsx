import { useEffect, useState } from 'react';
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
import { Get as GetSettings } from '../../wailsjs/go/bindings/SettingsService';
import type { hub } from '../../wailsjs/go/models';

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
  { id: 'killswitch', label: 'Kill switch' },
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

function isPortValid(value: number): boolean {
  return Number.isInteger(value) && value >= 1 && value <= 65535;
}

export function Settings() {
  const [s, update] = useSettings();
  const active = useScrollSpy(SECTION_IDS);
  const [helperStatus, setHelperStatus] = useState<StatusPillStatus>('running');
  const [reinstallOpen, setReinstallOpen] = useState(false);
  const [logFolderSize, setLogFolderSize] = useState(47); // MB
  const [updateState, setUpdateState] = useState<'idle' | 'checking' | 'uptodate'>('idle');
  const [stuck, setStuck] = useState(false);
  const [about, setAbout] = useState<hub.AboutSettings | null>(null);

  useEffect(() => {
    GetSettings()
      .then((view) => setAbout(view.about))
      .catch((err) => console.warn('SettingsService.Get failed', err));
  }, []);

  const versionLabel = about?.version || '—';
  const buildLabel = (() => {
    if (!about) return '—';
    const rev = about.gitRev?.trim() ?? '';
    const date = about.buildDate?.trim() ?? '';
    if (rev && date) return `${rev} · ${date}`;
    if (rev) return rev;
    if (date) return date;
    return '—';
  })();

  useEffect(() => {
    const main = document.querySelector('main');
    if (!main) return;
    const onScroll = () => setStuck(main.scrollTop > 24);
    onScroll();
    main.addEventListener('scroll', onScroll, { passive: true });
    return () => main.removeEventListener('scroll', onScroll);
  }, []);

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
      <div
        className={cn(
          'sticky -top-8 z-20 -mx-8 px-8 transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]',
          stuck
            ? 'pt-10 pb-2 bg-bg-1/[0.92] backdrop-blur-2xl border-b border-white/[0.10] shadow-[0_8px_32px_-12px_rgba(0,0,0,0.55)]'
            : 'pt-0 pb-3 bg-transparent border-b border-transparent shadow-none',
        )}
      >
        <h1
          className={cn(
            'font-semibold tracking-tight text-white/[0.92] transition-all duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]',
            stuck ? 'text-[15px] mb-2' : 'text-[22px] mb-3',
          )}
        >
          Settings
        </h1>
        <ScrollSpy sections={SECTIONS} active={active} onSelect={scrollToSection} />
      </div>

      {/* General */}
      <motion.div id="general" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="General" />
        <SettingRow label="Language" hint="Interface display language.">
          <Segmented
            value={s.language}
            onChange={(v) => update({ language: v })}
            options={[
              { value: 'en', label: 'English' },
              { value: 'ru', label: 'Русский' },
            ] as const}
          />
        </SettingRow>
        <SettingRow label="Launch on system startup" hint="Start ITG Ray automatically when you log in.">
          <Toggle value={s.autostart} onChange={(v) => update({ autostart: v })} />
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
            value={s.defaultMode}
            onChange={(v) => update({ defaultMode: v })}
            options={[
              { value: 'tun', label: 'TUN' },
              { value: 'sysproxy', label: 'System proxy' },
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
        <SettingRow label="SOCKS port" hint="Local SOCKS5 proxy port. Default 10808.">
          <input
            type="number"
            min={1}
            max={65535}
            value={s.socksPort}
            onChange={(e) => update({ socksPort: Number(e.target.value) || 0 })}
            className={cn(
              'w-24 px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white text-center outline-none transition-colors tabular-nums',
              isPortValid(s.socksPort)
                ? 'border-white/[0.12] focus:border-white/[0.30]'
                : 'border-danger/50',
            )}
          />
        </SettingRow>
        <SettingRow label="HTTP port" hint="Local HTTP proxy port. Default 10809.">
          <input
            type="number"
            min={1}
            max={65535}
            value={s.httpPort}
            onChange={(e) => update({ httpPort: Number(e.target.value) || 0 })}
            className={cn(
              'w-24 px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white text-center outline-none transition-colors tabular-nums',
              isPortValid(s.httpPort)
                ? 'border-white/[0.12] focus:border-white/[0.30]'
                : 'border-danger/50',
            )}
          />
        </SettingRow>
        <SettingRow
          label="IPv6 routing"
          hint="Disable IPv6 if your provider's IPv6 is broken and routing fails."
        >
          <Segmented
            value={s.ipv6Mode}
            onChange={(v) => update({ ipv6Mode: v })}
            options={[
              { value: 'prefer-v4', label: 'Prefer v4' },
              { value: 'prefer-v6', label: 'Prefer v6' },
              { value: 'disabled', label: 'Disable' },
            ] as const}
          />
        </SettingRow>
        {s.defaultMode === 'tun' && (
          <details className="mt-2 group">
            <summary className="cursor-pointer text-[13px] text-white/[0.55] hover:text-white/[0.75] select-none py-2 transition-colors">
              Advanced TUN parameters
            </summary>
            <div className="mt-1 pl-3 border-l border-white/[0.08]">
              <SettingRow label="Interface CIDR" hint="TUN adapter IPv4 address and subnet. Default 198.18.0.1/15.">
                <input
                  type="text"
                  value={s.tunCidr}
                  onChange={(e) => update({ tunCidr: e.target.value })}
                  placeholder="198.18.0.1/15"
                  className="w-40 px-3 py-1.5 bg-white/[0.06] border border-white/[0.12] rounded-[10px] text-[13px] text-white placeholder:text-white/[0.30] outline-none transition-colors focus:border-white/[0.30] tabular-nums"
                />
              </SettingRow>
              <SettingRow label="MTU" hint="TUN interface MTU in bytes. Default 1500. Range 576–9000.">
                <input
                  type="number"
                  min={576}
                  max={9000}
                  step={1}
                  value={s.tunMtu}
                  onChange={(e) => {
                    const n = Number(e.target.value);
                    if (Number.isFinite(n) && n >= 576 && n <= 9000) {
                      update({ tunMtu: n });
                    }
                  }}
                  className="w-24 px-3 py-1.5 bg-white/[0.06] border border-white/[0.12] rounded-[10px] text-[13px] text-white text-center outline-none transition-colors focus:border-white/[0.30] tabular-nums"
                />
              </SettingRow>
            </div>
          </details>
        )}
      </motion.div>

      {/* Kill switch */}
      <motion.div id="killswitch" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Kill switch" />
        <SettingRow
          label="Block traffic when VPN drops"
          hint="Cut the network if the tunnel goes down so traffic never leaks unprotected."
        >
          <Toggle
            value={s.killSwitchEnabled}
            onChange={(v) => update({ killSwitchEnabled: v })}
          />
        </SettingRow>
        <SettingRow
          label="Always-on"
          hint="Require an active VPN connection to use the network at all."
          className={!s.killSwitchEnabled ? 'opacity-50' : undefined}
        >
          <Toggle
            value={s.killSwitchAlwaysOn}
            disabled={!s.killSwitchEnabled}
            onChange={(v) => update({ killSwitchAlwaysOn: v })}
          />
        </SettingRow>
      </motion.div>

      {/* Notifications */}
      <motion.div id="notifications" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title="Notifications" />
        <SettingRow label="Connection toasts" hint="Show OS notifications on connect, disconnect, and errors.">
          <Toggle value={s.onConnected} onChange={(v) => update({ onConnected: v })} />
        </SettingRow>
        <SettingRow label="Sound" hint="Play a soft chime on state changes.">
          <Toggle value={s.notifySound} onChange={(v) => update({ notifySound: v })} />
        </SettingRow>
        <SettingRow label="Subscription failures" hint="Notify when a subscription sync fails.">
          <Toggle value={s.onSubSynced} onChange={(v) => update({ onSubSynced: v })} />
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
          <dt className="text-white/[0.45]">Version</dt><dd className="text-white/[0.92] tabular-nums">{versionLabel}</dd>
          <dt className="text-white/[0.45]">Build</dt><dd className="text-white/[0.92] tabular-nums">{buildLabel}</dd>
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
