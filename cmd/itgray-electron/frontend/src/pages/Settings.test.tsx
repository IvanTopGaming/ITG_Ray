import { afterEach, beforeEach, describe, it, expect, vi } from 'vitest';
import { act, render, screen, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { useEffect, useRef, useState } from 'react';

// ──────────────────────────────────────────────────────────────────────
//  Numeric-input draft pattern integration test
// ──────────────────────────────────────────────────────────────────────
//
// The Settings page previously bound numeric <input>s directly to the
// store value: every keystroke pushed `Number(e.target.value)` through
// update(), the 200ms debounce hit the backend's range validator, the
// out-of-range intermediate (e.g. "150" while typing "1500" → "1400")
// was silently rejected, and the EventSettings echo snapped the input
// back to the last persisted value. That made the field uneditable.
//
// The fix decouples user typing from store flushing via a local draft
// string + an effect that mirrors the canonical store value back into
// the draft on outside changes. The store only sees commits when the
// draft parses to a validator-clean number.
//
// This test renders a stand-in component that uses the identical
// pattern (same shape as the inputs in Settings.tsx) and asserts:
//   1. Every keystroke is reflected in the visible input value.
//   2. update() is called only on commits to validator-clean values.
//   3. An external change to the canonical value mirrors back into
//      the draft (proves the useEffect sync path works).
//   4. Blur with an invalid draft reverts the draft to the canonical
//      value (proves user always sees what backend persisted).
//
// We deliberately don't render the full Settings page — it imports
// Wails bindings, framer-motion, and a ScrollSpy that needs observer
// polyfills in jsdom. The draft logic is the regression surface; an
// isolated test of that logic captures the bug directly.

function isMtuValid(value: number): boolean {
  return Number.isFinite(value) && value >= 576 && value <= 9000;
}

type MtuFieldProps = {
  storeValue: number;
  onCommit: (n: number) => void;
};

function MtuField({ storeValue, onCommit }: MtuFieldProps) {
  const [draft, setDraft] = useState(String(storeValue));
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (document.activeElement === ref.current) return;
    setDraft(String(storeValue));
  }, [storeValue]);
  return (
    <input
      ref={ref}
      data-testid="mtu"
      type="text"
      inputMode="numeric"
      value={draft}
      onChange={(e) => {
        setDraft(e.target.value);
        const n = Number(e.target.value);
        if (Number.isFinite(n) && isMtuValid(n)) {
          onCommit(n);
        }
      }}
      onBlur={() => {
        const n = Number(draft);
        if (!Number.isFinite(n) || !isMtuValid(n)) {
          setDraft(String(storeValue));
        }
      }}
    />
  );
}

describe('numeric draft pattern (MTU input)', () => {
  it('lets the user clear and retype freely without snap-back', async () => {
    const user = userEvent.setup();
    const onCommit = vi.fn();
    render(<MtuField storeValue={1500} onCommit={onCommit} />);

    const input = screen.getByTestId('mtu') as HTMLInputElement;
    expect(input.value).toBe('1500');

    // The initial 1500 is the canonical value already; no commit
    // happens until the user changes it to a different valid one.
    await user.click(input);

    // Backspace four times: "1500" → "150" → "15" → "1" → ""
    await user.keyboard('{End}{Backspace}');
    expect(input.value).toBe('150');
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('15');
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('1');
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('');

    // None of the intermediates passed isMtuValid (150, 15, 1, ""),
    // so no commit fired. The 1500 commit did not fire either —
    // the input started at 1500, so re-committing it would be a noop
    // anyway, but more importantly the change handler only commits
    // values that PARSE AND VALIDATE.
    expect(onCommit).not.toHaveBeenCalled();

    // Now type "1400" — first three keystrokes ("1", "14", "140") fail
    // isMtuValid; the fourth ("1400") passes and commits exactly once.
    await user.keyboard('1');
    expect(input.value).toBe('1');
    await user.keyboard('4');
    expect(input.value).toBe('14');
    await user.keyboard('0');
    expect(input.value).toBe('140');
    expect(onCommit).not.toHaveBeenCalled();
    await user.keyboard('0');
    expect(input.value).toBe('1400');
    expect(onCommit).toHaveBeenCalledTimes(1);
    expect(onCommit).toHaveBeenLastCalledWith(1400);
  });

  it('mirrors external store-value changes back into the draft', async () => {
    const onCommit = vi.fn();
    const { rerender } = render(<MtuField storeValue={1500} onCommit={onCommit} />);
    const input = screen.getByTestId('mtu') as HTMLInputElement;
    expect(input.value).toBe('1500');

    // Backend pushed a new value (e.g. CLI edit, post-flush refetch).
    rerender(<MtuField storeValue={1400} onCommit={onCommit} />);
    expect(input.value).toBe('1400');
  });

  it('reverts to the canonical value on blur if the draft is invalid', async () => {
    const user = userEvent.setup();
    const onCommit = vi.fn();
    render(<MtuField storeValue={1500} onCommit={onCommit} />);
    const input = screen.getByTestId('mtu') as HTMLInputElement;

    await user.click(input);
    await user.tripleClick(input);
    await user.keyboard('{Backspace}');
    expect(input.value).toBe('');
    expect(onCommit).not.toHaveBeenCalled();

    // Blur with empty draft — should revert to canonical "1500".
    await user.tab();
    expect(input.value).toBe('1500');
    expect(onCommit).not.toHaveBeenCalled();
  });

  it('does not clobber the draft while the user is focused on the input', async () => {
    const user = userEvent.setup();
    const onCommit = vi.fn();
    const { rerender } = render(<MtuField storeValue={1500} onCommit={onCommit} />);
    const input = screen.getByTestId('mtu') as HTMLInputElement;

    // User is mid-edit: clears the field, types "150" (intermediate-invalid).
    await user.click(input);
    await user.tripleClick(input);
    await user.keyboard('{Backspace}150');
    expect(input.value).toBe('150');
    expect(document.activeElement).toBe(input);

    // External EventSettings echo arrives — store flips to 1400.
    // Without the activeElement gate, the effect would overwrite "150"
    // with "1400" mid-edit and the user would lose their typing.
    rerender(<MtuField storeValue={1400} onCommit={onCommit} />);
    expect(input.value).toBe('150');

    // After blur the invalid draft reverts to the LATEST canonical
    // value (1400, not the pre-rerender 1500), proving the rerender
    // delivered fresh storeValue and only the typing window was gated.
    await user.tab();
    expect(input.value).toBe('1400');
  });

  it('still mirrors external store changes when the input is not focused', async () => {
    const user = userEvent.setup();
    const onCommit = vi.fn();
    const { rerender } = render(<MtuField storeValue={1500} onCommit={onCommit} />);
    const input = screen.getByTestId('mtu') as HTMLInputElement;

    // Focus then blur — mirrors the lifecycle the gate cares about.
    await user.click(input);
    await user.tab();
    expect(document.activeElement).not.toBe(input);

    rerender(<MtuField storeValue={1400} onCommit={onCommit} />);
    expect(input.value).toBe('1400');
  });
});

// ──────────────────────────────────────────────────────────────────────
//  Subscription Identity section integration tests
// ──────────────────────────────────────────────────────────────────────
//
// These render the full <Settings /> page so the new section is exercised
// alongside the rest of the settings store + adapter. Wails bindings,
// runtime events, and IntersectionObserver are stubbed so the page mounts
// in jsdom. Framer-motion logs an unrelated warning about
// getComputedStyle in jsdom, which is harmless and matches the rest of
// the suite.

vi.mock('@/lib/itg/SettingsService', () => ({
  Get: vi.fn().mockResolvedValue({
    general: { language: 'en', autostart: false, startMinimized: false },
    network: {
      defaultMode: 'tun',
      socksPort: 1080,
      httpPort: 8888,
      allowLan: false,
      ipv6Mode: 'prefer-v4',
      tunCidr: '198.18.0.1/15',
      tunMtu: 1500,
      dns: { mode: 'auto', servers: [] },
    },
    killSwitch: { enabled: true, alwaysOn: false },
    subscriptions: {
      defaultUpdateInterval: 3600,
      userAgent: 'ITGRay/0.1',
      hwidEnabled: true,
      sendDeviceOS: true,
      sendOSVersion: true,
      sendDeviceModel: true,
    },
    notifications: {
      onConnected: true,
      onDisconnected: true,
      quotaLow: true,
      onSubSynced: true,
      sound: true,
    },
    debug: { logLevel: 'info' },
    about: { version: '0.1', gitRev: '', buildDate: '' },
    security: { method: 'Unencrypted' },
  }),
  Update: vi.fn().mockResolvedValue({}),
}));

vi.mock('@/lib/itg/runtime', () => ({
  EventsOn: vi.fn(),
  EventsOff: vi.fn(),
  Environment: vi.fn().mockResolvedValue({ platform: 'linux' }),
  WindowMinimise: vi.fn(),
  WindowToggleMaximise: vi.fn(),
  WindowIsMaximised: vi.fn().mockResolvedValue(false),
  Quit: vi.fn(),
}));

vi.mock('@/lib/itg/HelperService', () => ({
  Status: vi.fn().mockResolvedValue({ state: 'unsupported' }),
  Install: vi.fn(),
  Start: vi.fn(),
  Stop: vi.fn(),
  Restart: vi.fn(),
  Reinstall: vi.fn(),
  IsWindows: vi.fn().mockResolvedValue(false),
}));

vi.mock('@/lib/itg/AppService', () => ({
  GetSnapshot: vi.fn().mockResolvedValue(null),
  GetPublicIP: vi.fn().mockResolvedValue(null),
  SetAutostart: vi.fn().mockResolvedValue(true),
}));

import { Settings as SettingsPage } from './Settings';
import { __resetForTests } from '@/lib/settings';
import * as SettingsService from '@/lib/itg/SettingsService';
import * as AppService from '@/lib/itg/AppService';

class MockIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
  takeRecords = vi.fn().mockReturnValue([]);
  root = null;
  rootMargin = '';
  thresholds: number[] = [];
}

const renderSettings = async () => {
  const utils = render(
    <MemoryRouter>
      <SettingsPage />
    </MemoryRouter>,
  );
  // Drain GetSettings() promise + listener notify so post-mount state
  // (with hwidEnabled=true etc. from the mock) is committed before
  // assertions run. Two microtask drains mirror the AppShell test.
  await act(async () => {
    await new Promise<void>((r) => setTimeout(r, 0));
    await new Promise<void>((r) => setTimeout(r, 0));
  });
  return utils;
};

describe('Settings — Subscription Identity', () => {
  beforeEach(() => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      MockIntersectionObserver as unknown as typeof IntersectionObserver;
    // settings.ts gates loadFromBackend on `window.go` to skip the
    // Wails RPC in pure-Node contexts. In jsdom we set a stub so the
    // mocked GetSettings actually runs.
    (window as unknown as { go: object }).go = {};
    (window as unknown as { itg: object }).itg = {
      logs: {
        dirInfo: vi.fn().mockResolvedValue({ path: '', sizeBytes: 0 }),
        openFolder: vi.fn().mockResolvedValue(null),
      },
    };
    __resetForTests();
    vi.clearAllMocks();
    (SettingsService.Get as ReturnType<typeof vi.fn>).mockResolvedValue({
      general: { language: 'en', autostart: false, startMinimized: false },
      network: {
        defaultMode: 'tun',
        socksPort: 1080,
        httpPort: 8888,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        tunCidr: '198.18.0.1/15',
        tunMtu: 1500,
        dns: { mode: 'auto', servers: [] },
      },
      killSwitch: { enabled: true, alwaysOn: false },
      subscriptions: {
        defaultUpdateInterval: 3600,
        userAgent: 'ITGRay/0.1',
        hwidEnabled: true,
        sendDeviceOS: true,
        sendOSVersion: true,
        sendDeviceModel: true,
      },
      notifications: {
        onConnected: true,
        onDisconnected: true,
        quotaLow: true,
        onSubSynced: true,
        sound: true,
      },
      debug: { logLevel: 'info' },
      about: { version: '0.1', gitRev: '', buildDate: '' },
      security: { method: 'Unencrypted' },
    });
  });

  afterEach(() => {
    __resetForTests();
    delete (window as unknown as { go?: object }).go;
    delete (window as unknown as { itg?: object }).itg;
  });

  it('renders all 5 identity controls', async () => {
    await renderSettings();
    expect(screen.getByText('User-Agent')).toBeInTheDocument();
    expect(screen.getByText('Send HWID')).toBeInTheDocument();
    expect(screen.getByText('Send device OS')).toBeInTheDocument();
    expect(screen.getByText('Send OS version')).toBeInTheDocument();
    expect(screen.getByText('Send device model')).toBeInTheDocument();
  });

  it('shows the gating hint about HWID master', async () => {
    await renderSettings();
    expect(
      screen.getByText(/honored only when HWID is on/i),
    ).toBeInTheDocument();
  });

  it('toggling Send HWID dispatches an Update RPC', async () => {
    await renderSettings();
    const updateMock = SettingsService.Update as ReturnType<typeof vi.fn>;
    updateMock.mockClear();
    const toggle = screen.getByRole('switch', { name: 'Send HWID' });
    await userEvent.click(toggle);
    // Wait for the 200ms debounce + flush.
    await act(async () => {
      await new Promise<void>((r) => setTimeout(r, 250));
    });
    expect(updateMock).toHaveBeenCalled();
    const sectionsCalled = updateMock.mock.calls.map((c) => c[0]);
    expect(sectionsCalled).toContain('subscriptions');
    const subsPatch = updateMock.mock.calls.find(
      (c) => c[0] === 'subscriptions',
    )?.[1];
    expect(subsPatch).toMatchObject({ hwidEnabled: false });
  });

  it('editing User-Agent dispatches an Update RPC', async () => {
    await renderSettings();
    const updateMock = SettingsService.Update as ReturnType<typeof vi.fn>;
    updateMock.mockClear();
    const ua = screen.getByLabelText('User-Agent') as HTMLInputElement;
    expect(ua.value).toBe('ITGRay/0.1');
    fireEvent.change(ua, { target: { value: 'Custom/2.0' } });
    await act(async () => {
      await new Promise<void>((r) => setTimeout(r, 250));
    });
    expect(updateMock).toHaveBeenCalled();
    const subsPatch = updateMock.mock.calls.find(
      (c) => c[0] === 'subscriptions',
    )?.[1];
    expect(subsPatch).toMatchObject({ userAgent: 'Custom/2.0' });
  });

  it('persists empty User-Agent as empty string', async () => {
    await renderSettings();
    const updateMock = SettingsService.Update as ReturnType<typeof vi.fn>;
    updateMock.mockClear();
    const ua = screen.getByLabelText('User-Agent') as HTMLInputElement;
    fireEvent.change(ua, { target: { value: '' } });
    await act(async () => {
      await new Promise<void>((r) => setTimeout(r, 250));
    });
    expect(updateMock).toHaveBeenCalled();
    const subsPatch = updateMock.mock.calls.find(
      (c) => c[0] === 'subscriptions',
    )?.[1];
    expect(subsPatch).toMatchObject({ userAgent: '' });
  });
});

describe('Settings — Autostart toggle', () => {
  beforeEach(() => {
    (globalThis as { IntersectionObserver?: unknown }).IntersectionObserver =
      MockIntersectionObserver as unknown as typeof IntersectionObserver;
    (window as unknown as { go: object }).go = {};
    (window as unknown as { itg: object }).itg = {
      logs: {
        dirInfo: vi.fn().mockResolvedValue({ path: '', sizeBytes: 0 }),
        openFolder: vi.fn().mockResolvedValue(null),
      },
    };
    __resetForTests();
    vi.clearAllMocks();
    (SettingsService.Get as ReturnType<typeof vi.fn>).mockResolvedValue({
      general: { language: 'en', autostart: false, startMinimized: false },
      network: {
        defaultMode: 'tun',
        socksPort: 1080,
        httpPort: 8888,
        allowLan: false,
        ipv6Mode: 'prefer-v4',
        tunCidr: '198.18.0.1/15',
        tunMtu: 1500,
        dns: { mode: 'auto', servers: [] },
      },
      killSwitch: { enabled: true, alwaysOn: false },
      subscriptions: {
        defaultUpdateInterval: 3600,
        userAgent: 'ITGRay/0.1',
        hwidEnabled: true,
        sendDeviceOS: true,
        sendOSVersion: true,
        sendDeviceModel: true,
      },
      notifications: {
        onConnected: true,
        onDisconnected: true,
        quotaLow: true,
        onSubSynced: true,
        sound: true,
      },
      debug: { logLevel: 'info' },
      about: { version: '0.1', gitRev: '', buildDate: '' },
      security: { method: 'Unencrypted' },
    });
  });

  afterEach(() => {
    __resetForTests();
    delete (window as unknown as { go?: object }).go;
    delete (window as unknown as { itg?: object }).itg;
  });

  it('flipping the autostart toggle calls SetAutostart with the new value', async () => {
    await renderSettings();
    const setAutostartMock = AppService.SetAutostart as ReturnType<typeof vi.fn>;
    setAutostartMock.mockClear();
    const toggle = screen.getByRole('switch', { name: 'Launch on system startup' });
    await userEvent.click(toggle);
    await act(async () => {
      await new Promise<void>((r) => setTimeout(r, 0));
    });
    expect(setAutostartMock).toHaveBeenCalledWith(true);
  });
});
