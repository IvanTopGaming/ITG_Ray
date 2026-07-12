import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
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
import { StatusPill } from '@/components/controls/StatusPill';
import { useHelperState } from '@/lib/helperAdapter';
import type { HelperState } from '@/lib/helperAdapter';
import { ScrollSpy, useScrollSpy, scrollToSection } from '@/components/controls/ScrollSpy';
import { Get as GetSettings } from '@/lib/itg/SettingsService';
import { SetAutostart } from '@/lib/itg/AppService';
import { Refresh as GeoRefresh } from '@/lib/itg/GeoService';
import { useGeoProgress, geoBegin, geoEnd } from '@/lib/geoStore';
import type { GeoPreset } from '@/lib/settings';
import type { hub } from '@/lib/itg/models';

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

const SECTION_IDS = [
  'general',
  'connection',
  'killswitch',
  'subscriptions',
  'notifications',
  'helper',
  'logs',
  'about',
];

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

function isMtuValid(value: number): boolean {
  return Number.isInteger(value) && value >= 576 && value <= 9000;
}

export function Settings() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [s, update] = useSettings();
  const active = useScrollSpy(SECTION_IDS);
  const SECTIONS = [
    { id: 'general', label: t('settings.sections.general') },
    { id: 'connection', label: t('settings.sections.connection') },
    { id: 'killswitch', label: t('settings.sections.killswitch') },
    { id: 'subscriptions', label: t('settings.sections.subscriptions') },
    { id: 'notifications', label: t('settings.sections.notifications') },
    { id: 'helper', label: t('settings.sections.helper') },
    { id: 'logs', label: t('settings.sections.logs') },
    { id: 'about', label: t('settings.sections.about') },
  ];
  const [logDir, setLogDir] = useState<{ path: string; sizeBytes: number }>({ path: '', sizeBytes: 0 });
  const [updateState, setUpdateState] = useState<'idle' | 'checking' | 'uptodate'>('idle');
  const [stuck, setStuck] = useState(false);
  const [about, setAbout] = useState<hub.AboutSettings | null>(null);
  const [tunAdvancedOpen, setTunAdvancedOpen] = useState(false);

  // Draft-string state for numeric inputs decouples user typing from the
  // store flush. Without this, every keystroke pushes a parsed Number
  // through update(); intermediate values that fail backend range
  // validation (e.g. "1500" → backspace → "150") get rejected on the
  // round-trip and the EventSettings echo snaps the field back to the
  // last persisted value, making it effectively uneditable. The draft
  // mirrors the canonical store value and only commits when parseable
  // AND validator-clean. type="text" + inputMode="numeric" keeps the
  // numeric keyboard on mobile without the browser-level coercion that
  // type="number" applies mid-typing.
  //
  // Validity-driven border + aria-invalid key off the draft (not the
  // canonical), so the user gets immediate red-border feedback while
  // typing intermediate-invalid values. The danger color disappears as
  // soon as the draft parses to a validator-clean value, at which point
  // update() also fires.
  const [tunMtuDraft, setTunMtuDraft] = useState(String(s.tunMtu));
  const [socksPortDraft, setSocksPortDraft] = useState(String(s.socksPort));
  const [httpPortDraft, setHttpPortDraft] = useState(String(s.httpPort));

  // Refs gate the EventSettings → draft sync below: when the user is
  // mid-typing in one of these inputs, an external store push (cross-
  // process settings change, debounced echo of a different field) must
  // not clobber what they have on screen. The activeElement check skips
  // the sync only for the focused field; the others sync as before. On
  // blur the existing onBlur logic resets invalid drafts, so any
  // divergence opened by the gate closes itself.
  const tunMtuRef = useRef<HTMLInputElement>(null);
  const socksPortRef = useRef<HTMLInputElement>(null);
  const httpPortRef = useRef<HTMLInputElement>(null);

  // Sync the drafts when the canonical store value changes from the
  // outside (backend EventSettings push, post-flush re-fetch, navigation).
  useEffect(() => {
    if (document.activeElement === tunMtuRef.current) return;
    setTunMtuDraft(String(s.tunMtu));
  }, [s.tunMtu]);
  useEffect(() => {
    if (document.activeElement === socksPortRef.current) return;
    setSocksPortDraft(String(s.socksPort));
  }, [s.socksPort]);
  useEffect(() => {
    if (document.activeElement === httpPortRef.current) return;
    setHttpPortDraft(String(s.httpPort));
  }, [s.httpPort]);

  useEffect(() => {
    GetSettings()
      .then((view) => setAbout(view.about))
      .catch((err) => console.warn('SettingsService.Get failed', err));
  }, []);

  useEffect(() => {
    void (window.itg.logs.dirInfo() as Promise<{ path: string; sizeBytes: number }>)
      .then(setLogDir)
      .catch(() => {});
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

  const helper = useHelperState();
  const [reinstallOpen, setReinstallOpen] = useState(false);

  const helperPill: HelperState = helper.state;
  const isWindowsHelper = helper.isWindows === true;
  const isLinuxHelper = helper.isLinux === true;
  const isLoadingPlatform = helper.isWindows === null;

  const checkUpdates = async () => {
    setUpdateState('checking');
    await new Promise((r) => setTimeout(r, 1500));
    setUpdateState('uptodate');
  };

  const geo = useGeoProgress();
  const [geoRefreshing, setGeoRefreshing] = useState(false);
  const updateGeoDatabases = async () => {
    setGeoRefreshing(true);
    geoBegin();
    try {
      await GeoRefresh();
    } finally {
      geoEnd();
      setGeoRefreshing(false);
    }
  };
  const geoPercent = geo.total > 0 ? Math.round((geo.done / geo.total) * 100) : 0;

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
          {t('settings.title')}
        </h1>
        {!stuck && (
          <p className="-mt-2 mb-3 text-[13px] text-white/50">
            {t('settings.description')}
          </p>
        )}
        <ScrollSpy sections={SECTIONS} active={active} onSelect={scrollToSection} />
      </div>

      {/* General */}
      <motion.div id="general" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.general.title')} />
        <SettingRow label={t('settings.general.language')} hint={t('settings.general.languageHint')}>
          <Segmented
            value={s.language}
            onChange={(v) => update({ language: v })}
            options={[
              { value: 'en', label: t('settings.general.languageEn') },
              { value: 'ru', label: t('settings.general.languageRu') },
            ] as const}
          />
        </SettingRow>
        <SettingRow label={t('settings.general.autostart')} hint={t('settings.general.autostartHint')}>
          <Toggle
            value={s.autostart}
            aria-label={t('settings.general.autostart')}
            onChange={(v) => {
              update({ autostart: v });
              void SetAutostart(v);
            }}
          />
        </SettingRow>
        <SettingRow label={t('settings.general.startMinimized')} hint={t('settings.general.startMinimizedHint')}>
          <Toggle value={s.startMinimized} onChange={(v) => update({ startMinimized: v })} />
        </SettingRow>
      </motion.div>

      {/* Connection */}
      <motion.div id="connection" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.connection.title')} />
        <SettingRow label={t('settings.connection.dns')} hint={t('settings.connection.dnsHint')}>
          <Dropdown
            value={s.dnsMode}
            onChange={(v) => update({ dnsMode: v })}
            options={[
              { value: 'auto', label: t('settings.connection.dnsAuto') },
              { value: 'custom', label: t('settings.connection.dnsCustomOption') },
            ] as const}
          />
        </SettingRow>
        <Reveal show={s.dnsMode === 'custom'}>
          <SettingRow label={t('settings.connection.customDns')} hint={t('settings.connection.customDnsHint')} stacked>
            <input
              type="text"
              value={s.dnsCustom}
              onChange={(e) => update({ dnsCustom: e.target.value })}
              placeholder={t('settings.connection.customDnsPlaceholder')}
              className={cn(
                'w-full px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white placeholder:text-white/[0.30] outline-none transition-colors',
                isDnsValid(s.dnsCustom)
                  ? 'border-white/[0.12] focus:border-white/[0.30]'
                  : 'border-danger/50',
              )}
            />
          </SettingRow>
        </Reveal>
        <SettingRow label={t('settings.connection.allowLan')} hint={t('settings.connection.allowLanHint')}>
          <Toggle value={s.allowLan} onChange={(v) => update({ allowLan: v })} />
        </SettingRow>
        <SettingRow label={t('settings.connection.socksPort')} hint={t('settings.connection.socksPortHint')}>
          <input
            ref={socksPortRef}
            type="text"
            inputMode="numeric"
            value={socksPortDraft}
            onChange={(e) => {
              setSocksPortDraft(e.target.value);
              const n = Number(e.target.value);
              if (Number.isFinite(n) && isPortValid(n)) {
                update({ socksPort: n });
              }
            }}
            onBlur={() => {
              const n = Number(socksPortDraft);
              if (!Number.isFinite(n) || !isPortValid(n)) {
                setSocksPortDraft(String(s.socksPort));
              }
            }}
            aria-invalid={!isPortValid(Number(socksPortDraft))}
            className={cn(
              'w-24 px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white text-center outline-none transition-colors tabular-nums',
              isPortValid(Number(socksPortDraft))
                ? 'border-white/[0.12] focus:border-white/[0.30]'
                : 'border-danger/50',
            )}
          />
        </SettingRow>
        <SettingRow label={t('settings.connection.httpPort')} hint={t('settings.connection.httpPortHint')}>
          <input
            ref={httpPortRef}
            type="text"
            inputMode="numeric"
            value={httpPortDraft}
            onChange={(e) => {
              setHttpPortDraft(e.target.value);
              const n = Number(e.target.value);
              if (Number.isFinite(n) && isPortValid(n)) {
                update({ httpPort: n });
              }
            }}
            onBlur={() => {
              const n = Number(httpPortDraft);
              if (!Number.isFinite(n) || !isPortValid(n)) {
                setHttpPortDraft(String(s.httpPort));
              }
            }}
            aria-invalid={!isPortValid(Number(httpPortDraft))}
            className={cn(
              'w-24 px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white text-center outline-none transition-colors tabular-nums',
              isPortValid(Number(httpPortDraft))
                ? 'border-white/[0.12] focus:border-white/[0.30]'
                : 'border-danger/50',
            )}
          />
        </SettingRow>
        <SettingRow
          label={t('settings.connection.ipv6')}
          hint={t('settings.connection.ipv6Hint')}
        >
          <Segmented
            value={s.ipv6Mode}
            onChange={(v) => update({ ipv6Mode: v })}
            options={[
              { value: 'prefer-v4', label: t('settings.connection.ipv6PreferV4') },
              { value: 'prefer-v6', label: t('settings.connection.ipv6PreferV6') },
              { value: 'disabled', label: t('settings.connection.ipv6Disable') },
            ] as const}
          />
        </SettingRow>
        <SettingRow
          label={t('settings.connection.geoSource')}
          hint={t('settings.connection.geoSourceHint')}
        >
          <Dropdown
            value={s.geoPreset}
            onChange={(v) => update({ geoPreset: v as GeoPreset })}
            options={[
              { value: 'runetfreedom', label: t('settings.connection.geoSourceRunetfreedom') },
              { value: 'sagernet', label: t('settings.connection.geoSourceSagerNet') },
              { value: 'custom', label: t('settings.connection.geoSourceCustom') },
            ]}
          />
        </SettingRow>
        <Reveal show={s.geoPreset === 'custom'}>
          <SettingRow
            label={t('settings.connection.geoCustomUrl')}
            hint={t('settings.connection.geoCustomUrlHint')}
            stacked
          >
            <input
              type="text"
              aria-label={t('settings.connection.geoCustomUrl')}
              value={s.geoCustomURL}
              onChange={(e) => update({ geoCustomURL: e.target.value })}
              placeholder={t('settings.connection.geoCustomUrlPlaceholder')}
              className="w-full rounded-[10px] border border-white/[0.10] bg-white/[0.04] px-3 py-1.5 text-[13px] text-white/[0.92] placeholder:text-white/[0.35] focus:border-accent/40 focus:bg-white/[0.06] focus:outline-none"
            />
          </SettingRow>
        </Reveal>
        <SettingRow
          label={t('settings.connection.geoDatabases')}
          hint={t('settings.connection.geoDatabasesHint')}
        >
          <button
            type="button"
            onClick={updateGeoDatabases}
            disabled={geoRefreshing}
            className="px-3.5 py-1.5 text-xs font-semibold rounded-[10px] bg-gradient-to-b from-accent-start to-accent-mid text-white disabled:opacity-60"
          >
            {geoRefreshing
              ? (geo.total > 0
                  ? t('settings.connection.geoUpdatingPct', { pct: geoPercent })
                  : t('settings.connection.geoUpdating'))
              : t('settings.connection.geoUpdate')}
          </button>
        </SettingRow>
          <div className="mt-2">
            <button
              type="button"
              onClick={() => setTunAdvancedOpen((v) => !v)}
              aria-expanded={tunAdvancedOpen}
              aria-controls="tun-advanced-panel"
              id="tun-advanced-summary"
              className="flex items-center gap-1.5 cursor-pointer text-[13px] text-white/[0.55] hover:text-white/[0.75] select-none py-2 transition-colors w-full text-left"
            >
              <motion.span
                aria-hidden="true"
                animate={{ rotate: tunAdvancedOpen ? 90 : 0 }}
                transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
                className="inline-flex"
              >
                <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                  <path d="M3 2L7 5L3 8" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </motion.span>
              {t('settings.connection.advancedTun')}
            </button>
            <div
              id="tun-advanced-panel"
              role="region"
              aria-labelledby="tun-advanced-summary"
            >
              <Reveal show={tunAdvancedOpen}>
                <div className="mt-1 pl-3 border-l border-white/[0.08]">
                  <SettingRow label={t('settings.connection.interfaceCidr')} hint={t('settings.connection.interfaceCidrHint')}>
                    <input
                      type="text"
                      value={s.tunCidr}
                      onChange={(e) => update({ tunCidr: e.target.value })}
                      placeholder={t('settings.connection.interfaceCidrPlaceholder')}
                      className="w-40 px-3 py-1.5 bg-white/[0.06] border border-white/[0.12] rounded-[10px] text-[13px] text-white placeholder:text-white/[0.30] outline-none transition-colors focus:border-white/[0.30] tabular-nums"
                    />
                  </SettingRow>
                  <SettingRow label={t('settings.connection.mtu')} hint={t('settings.connection.mtuHint')}>
                    <input
                      ref={tunMtuRef}
                      type="text"
                      inputMode="numeric"
                      value={tunMtuDraft}
                      onChange={(e) => {
                        setTunMtuDraft(e.target.value);
                        const n = Number(e.target.value);
                        if (Number.isFinite(n) && isMtuValid(n)) {
                          update({ tunMtu: n });
                        }
                      }}
                      onBlur={() => {
                        const n = Number(tunMtuDraft);
                        if (!Number.isFinite(n) || !isMtuValid(n)) {
                          setTunMtuDraft(String(s.tunMtu));
                        }
                      }}
                      aria-invalid={!isMtuValid(Number(tunMtuDraft))}
                      className={cn(
                        'w-24 px-3 py-1.5 bg-white/[0.06] border rounded-[10px] text-[13px] text-white text-center outline-none transition-colors tabular-nums',
                        isMtuValid(Number(tunMtuDraft))
                          ? 'border-white/[0.12] focus:border-white/[0.30]'
                          : 'border-danger/50',
                      )}
                    />
                  </SettingRow>
                </div>
              </Reveal>
            </div>
          </div>
      </motion.div>

      {/* Kill switch */}
      <motion.div id="killswitch" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.killSwitch.title')} />
        <SettingRow
          label={t('settings.killSwitch.blockOnDrop')}
          hint={t('settings.killSwitch.blockOnDropHint')}
        >
          <Toggle
            value={s.killSwitchEnabled}
            onChange={(v) => update({ killSwitchEnabled: v })}
          />
        </SettingRow>
        <SettingRow
          label={t('settings.killSwitch.alwaysOn')}
          hint={t('settings.killSwitch.alwaysOnHint')}
          className={!s.killSwitchEnabled ? 'opacity-50' : undefined}
        >
          <Toggle
            value={s.killSwitchAlwaysOn}
            disabled={!s.killSwitchEnabled}
            onChange={(v) => update({ killSwitchAlwaysOn: v })}
          />
        </SettingRow>
      </motion.div>

      {/* Subscriptions */}
      <motion.div id="subscriptions" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.subscriptions.title')} />
        <SettingRow
          label={t('settings.subscriptions.defaultUpdateInterval')}
          hint={t('settings.subscriptions.defaultUpdateIntervalHint')}
        >
          <input
            type="text"
            inputMode="numeric"
            value={s.defaultUpdateInterval}
            onChange={(e) => {
              const n = Number(e.target.value);
              if (Number.isFinite(n) && n > 0) update({ defaultUpdateInterval: n });
            }}
            className="w-24 rounded-[10px] border border-white/[0.10] bg-white/[0.04] px-3 py-1.5 text-[13px] text-white/[0.92] tabular-nums focus:border-accent/40 focus:bg-white/[0.06] focus:outline-none"
          />
        </SettingRow>

        <div className="pt-2 pb-1">
          <span className="text-[11px] font-semibold uppercase tracking-[0.18em] text-white/[0.45]">
            {t('settings.subscriptions.identity')}
          </span>
        </div>

        <SettingRow
          label={t('settings.subscriptions.userAgent')}
          hint={t('settings.subscriptions.userAgentHint')}
          stacked
        >
          <input
            type="text"
            aria-label={t('settings.subscriptions.userAgent')}
            value={s.userAgent}
            placeholder={t('settings.subscriptions.userAgentPlaceholder')}
            onChange={(e) => update({ userAgent: e.target.value })}
            className="w-full rounded-[10px] border border-white/[0.10] bg-white/[0.04] px-3 py-1.5 text-[13px] text-white/[0.92] placeholder:text-white/[0.35] focus:border-accent/40 focus:bg-white/[0.06] focus:outline-none"
          />
        </SettingRow>

        <SettingRow
          label={t('settings.subscriptions.sendHwid')}
          hint={t('settings.subscriptions.sendHwidHint')}
        >
          <Toggle
            value={s.hwidEnabled}
            onChange={(v) => update({ hwidEnabled: v })}
            aria-label={t('settings.subscriptions.sendHwid')}
          />
        </SettingRow>
        <SettingRow
          label={t('settings.subscriptions.sendDeviceOs')}
          hint={t('settings.subscriptions.sendDeviceOsHint')}
          className={!s.hwidEnabled ? 'opacity-50' : undefined}
        >
          <Toggle
            value={s.sendDeviceOS}
            disabled={!s.hwidEnabled}
            onChange={(v) => update({ sendDeviceOS: v })}
            aria-label={t('settings.subscriptions.sendDeviceOs')}
          />
        </SettingRow>
        <SettingRow
          label={t('settings.subscriptions.sendOsVersion')}
          hint={t('settings.subscriptions.sendOsVersionHint')}
          className={!s.hwidEnabled ? 'opacity-50' : undefined}
        >
          <Toggle
            value={s.sendOSVersion}
            disabled={!s.hwidEnabled}
            onChange={(v) => update({ sendOSVersion: v })}
            aria-label={t('settings.subscriptions.sendOsVersion')}
          />
        </SettingRow>
        <SettingRow
          label={t('settings.subscriptions.sendDeviceModel')}
          hint={t('settings.subscriptions.sendDeviceModelHint')}
          className={!s.hwidEnabled ? 'opacity-50' : undefined}
        >
          <Toggle
            value={s.sendDeviceModel}
            disabled={!s.hwidEnabled}
            onChange={(v) => update({ sendDeviceModel: v })}
            aria-label={t('settings.subscriptions.sendDeviceModel')}
          />
        </SettingRow>
        <p className="pt-3 text-[11.5px] italic leading-snug text-white/[0.45]">
          {t('settings.subscriptions.metadataNote')}
        </p>
      </motion.div>

      {/* Notifications */}
      <motion.div id="notifications" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.notifications.title')} />
        <SettingRow label={t('settings.notifications.onConnect')} hint={t('settings.notifications.onConnectHint')}>
          <Toggle value={s.onConnected} onChange={(v) => update({ onConnected: v })} />
        </SettingRow>
        <SettingRow label={t('settings.notifications.onDisconnect')} hint={t('settings.notifications.onDisconnectHint')}>
          <Toggle value={s.onDisconnected} onChange={(v) => update({ onDisconnected: v })} />
        </SettingRow>
        <SettingRow label={t('settings.notifications.onQuotaLow')} hint={t('settings.notifications.onQuotaLowHint')}>
          <Toggle value={s.onQuotaLow} onChange={(v) => update({ onQuotaLow: v })} />
        </SettingRow>
        <SettingRow label={t('settings.notifications.sound')} hint={t('settings.notifications.soundHint')}>
          <Toggle value={s.notifySound} onChange={(v) => update({ notifySound: v })} />
        </SettingRow>
        <SettingRow label={t('settings.notifications.onSubSynced')} hint={t('settings.notifications.onSubSyncedHint')}>
          <Toggle value={s.onSubSynced} onChange={(v) => update({ onSubSynced: v })} />
        </SettingRow>
      </motion.div>

      {/* Helper */}
      <motion.div id="helper" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader
          title={t('settings.helper.title')}
          right={(isWindowsHelper || isLinuxHelper) ? <StatusPill status={helperPill} /> : undefined}
        />
        {isLoadingPlatform && null}
        {!isLoadingPlatform && !isWindowsHelper && !isLinuxHelper && (
          <p className="text-[13px] text-white/[0.55] leading-relaxed">
            {t('settings.helper.nonWindows')}
          </p>
        )}
        {isLinuxHelper && (
          <>
            <SettingRow
              label={t('settings.helper.status')}
              hint={t('settings.helper.linuxHint')}
            >
              <div className="flex gap-1.5">
                {helperPill !== 'running' && helperPill !== 'pending' && (
                  <button
                    type="button"
                    onClick={helper.installLinux}
                    className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] bg-accent/[0.12] border border-accent/30 text-accent hover:bg-accent/[0.18]"
                  >
                    {t('settings.helper.install')}
                  </button>
                )}
                {helperPill === 'running' && (
                  <button
                    type="button"
                    onClick={helper.uninstallLinux}
                    className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
                  >
                    {t('settings.helper.uninstall')}
                  </button>
                )}
                {helperPill === 'pending' && (
                  <span className="px-3.5 py-1.5 text-xs text-white/[0.55]">{t('settings.helper.working')}</span>
                )}
              </div>
            </SettingRow>
            <Reveal show={!!helper.opError}>
              <div className="mt-2 flex items-start gap-2 rounded-[10px] border border-danger/30 bg-danger/[0.08] px-3 py-2 text-[12px] text-danger">
                <span className="flex-1 leading-relaxed">⚠ {helper.opError}</span>
                <button
                  type="button"
                  onClick={helper.dismissError}
                  aria-label={t('settings.helper.dismissError')}
                  className="text-danger/70 hover:text-danger"
                >
                  ×
                </button>
              </div>
            </Reveal>
          </>
        )}
        {isWindowsHelper && (
          <>
            <SettingRow
              label={t('settings.helper.status')}
              hint={t('settings.helper.statusHint')}
            >
              <div className="flex gap-1.5">
                {helperPill === 'missing' && (
                  <button
                    type="button"
                    onClick={helper.install}
                    className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] bg-accent/[0.12] border border-accent/30 text-accent hover:bg-accent/[0.18]"
                  >
                    {t('settings.helper.install')}
                  </button>
                )}
                {helperPill === 'stopped' && (
                  <>
                    <button
                      type="button"
                      onClick={helper.start}
                      className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] bg-accent/[0.12] border border-accent/30 text-accent hover:bg-accent/[0.18]"
                    >
                      {t('settings.helper.start')}
                    </button>
                    <button
                      type="button"
                      onClick={() => setReinstallOpen(true)}
                      className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
                    >
                      {t('settings.helper.reinstall')}
                    </button>
                  </>
                )}
                {helperPill === 'running' && (
                  <>
                    <ConfirmButton onConfirm={helper.restart} variant="ghost">{t('common.restart')}</ConfirmButton>
                    <button
                      type="button"
                      onClick={() => setReinstallOpen(true)}
                      className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
                    >
                      {t('settings.helper.reinstall')}
                    </button>
                  </>
                )}
                {helperPill === 'pending' && (
                  <span className="px-3.5 py-1.5 text-xs text-white/[0.55]">{t('settings.helper.working')}</span>
                )}
                {helperPill === 'error' && (
                  <span className="px-3.5 py-1.5 text-xs text-white/[0.55]">{t('settings.helper.readingStatus')}</span>
                )}
                <button
                  type="button"
                  onClick={() => navigate('/logs')}
                  className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
                >
                  {t('settings.helper.viewLog')}
                </button>
              </div>
            </SettingRow>
            <Reveal show={!!helper.opError}>
              <div className="mt-2 flex items-start gap-2 rounded-[10px] border border-danger/30 bg-danger/[0.08] px-3 py-2 text-[12px] text-danger">
                <span className="flex-1 leading-relaxed">⚠ {helper.opError}</span>
                <button
                  type="button"
                  onClick={helper.dismissError}
                  aria-label={t('settings.helper.dismissError')}
                  className="text-danger/70 hover:text-danger"
                >
                  ×
                </button>
              </div>
            </Reveal>
          </>
        )}
      </motion.div>

      {isWindowsHelper && (
        <ConfirmDialog
          open={reinstallOpen}
          onClose={() => setReinstallOpen(false)}
          title={t('settings.helper.reinstallTitle')}
          description={t('settings.helper.reinstallDescription')}
          confirmLabel={t('settings.helper.reinstall')}
          confirmVariant="primary"
          onConfirm={() => { setReinstallOpen(false); void helper.reinstall(); }}
        />
      )}

      {/* Logs */}
      <motion.div id="logs" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.logs.title')} />
        <SettingRow
          label={t('settings.logs.logLevel')}
          hint={t('settings.logs.logLevelHint')}
        >
          <Segmented
            value={s.logLevel}
            onChange={(v) => update({ logLevel: v })}
            options={[
              { value: 'error', label: t('settings.logs.levelError') },
              { value: 'info', label: t('settings.logs.levelInfo') },
              { value: 'debug', label: t('settings.logs.levelDebug') },
              { value: 'trace', label: t('settings.logs.levelTrace') },
            ] as const}
          />
        </SettingRow>
        <SettingRow label={t('settings.logs.openFolder')} hint={t('settings.logs.folderHint', { mb: (logDir.sizeBytes / 1_000_000).toFixed(1) })}>
          <button
            type="button"
            onClick={() => void window.itg.logs.openFolder()}
            className="px-3.5 py-1.5 text-xs font-medium rounded-[10px] border border-white/[0.10] text-white/[0.92] hover:bg-white/[0.05]"
          >
            {t('settings.logs.openFolderButton')}
          </button>
        </SettingRow>
      </motion.div>

      {/* About */}
      <motion.div id="about" variants={sectionVariants} className="glass-regular rounded-2xl p-5">
        <SectionHeader title={t('settings.about.title')} />
        <dl className="grid grid-cols-[110px_1fr] gap-y-2 gap-x-4 text-[13px] py-1">
          <dt className="text-white/[0.45]">{t('settings.about.version')}</dt><dd className="text-white/[0.92] tabular-nums">{versionLabel}</dd>
          <dt className="text-white/[0.45]">{t('settings.about.build')}</dt><dd className="text-white/[0.92] tabular-nums">{buildLabel}</dd>
          <dt className="text-white/[0.45]">{t('settings.about.backend')}</dt><dd className="text-white/[0.92] tabular-nums">{about?.backend || '—'}</dd>
          <dt className="text-white/[0.45]">{t('settings.about.helper')}</dt><dd className="text-white/[0.92] tabular-nums">{versionLabel}</dd>
          <dt className="text-white/[0.45]">{t('settings.about.license')}</dt><dd className="text-white/[0.92]">MIT</dd>
        </dl>
        <SettingRow
          label={t('settings.about.checkForUpdates')}
          hint={
            updateState === 'uptodate'
              ? t('settings.about.upToDate')
              : updateState === 'checking'
              ? t('settings.about.checking')
              : t('settings.about.lastChecked')
          }
        >
          <button
            type="button"
            onClick={checkUpdates}
            disabled={updateState === 'checking'}
            className="px-3.5 py-1.5 text-xs font-semibold rounded-[10px] bg-gradient-to-b from-accent-start to-accent-mid text-white disabled:opacity-60"
          >
            {updateState === 'checking' ? t('settings.about.checking') : t('settings.about.checkNow')}
          </button>
        </SettingRow>
      </motion.div>
    </motion.section>
  );
}
